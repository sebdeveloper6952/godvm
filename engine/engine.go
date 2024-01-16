package engine

import (
	"context"
	"errors"
	"fmt"
	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sebdeveloper6952/go-dvm/domain"
	"github.com/sebdeveloper6952/go-dvm/lightning"
	"github.com/sebdeveloper6952/go-dvm/nostr"
	"github.com/sirupsen/logrus"
)

type Engine struct {
	dvmsByKind map[int][]domain.Dvmer
	nostrSvc   nostr.Service
	lnSvc      lightning.Service
	log        *logrus.Logger
}

func NewEngine() (*Engine, error) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetLevel(logrus.TraceLevel)

	nostrSvc, err := nostr.NewNostr(
		logger,
	)
	if err != nil {
		return nil, err
	}

	e := &Engine{
		dvmsByKind: make(map[int][]domain.Dvmer),
		nostrSvc:   nostrSvc,
		log:        logger,
	}

	return e, nil
}

func (e *Engine) RegisterDVM(dvm domain.Dvmer) {
	kindSupported := dvm.KindSupported()
	if _, ok := e.dvmsByKind[kindSupported]; !ok {
		e.dvmsByKind[kindSupported] = make([]domain.Dvmer, 0, 2)
	}
	e.dvmsByKind[kindSupported] = append(e.dvmsByKind[kindSupported], dvm)
}

func (e *Engine) SetLnService(ln lightning.Service) {
	e.lnSvc = ln
}

func (e *Engine) Run(
	ctx context.Context,
	initialRelays []string,
) error {
	if initialRelays == nil || len(initialRelays) == 0 {
		return errors.New("must provide at least one relay")
	}

	kindsSupported := e.getKindsSupported()

	go func() {
		if err := e.nostrSvc.Run(ctx, kindsSupported, initialRelays); err != nil {
			e.log.Errorf("[engine] run nostr service %+v", err)
		}

		e.advertiseDvms(ctx)
	}()

	go func() {
		for {
			select {
			case event := <-e.nostrSvc.Events():
				dvmsForKind, ok := e.dvmsByKind[event.Kind]
				if !ok {
					e.log.Debugf("[engine] no dvms for kind %d\n", event.Kind)
					continue
				}

				nip90Input, err := nostr.Nip90InputFromEvent(event)
				if err != nil {
					e.log.Errorf("[engine] nip90Input from event  %+v\n", err)
					continue
				}

				for i := range dvmsForKind {
					if dvmsForKind[i].AcceptJob(nip90Input) {
						go func(dvm domain.Dvmer, input *nostr.Nip90Input) {
							chanToDvm, chanToEngine, chanErr := dvmsForKind[i].Run(ctx, input)

							for {
								select {
								case update := <-chanToEngine:
									if update.Status == domain.StatusPaymentRequired {
										i, err := e.lnSvc.AddInvoice(ctx, int64(update.AmountSats))
										if err != nil {
											chanToDvm <- &domain.JobUpdate{
												Status: domain.StatusError,
											}
											return
										}
										e.log.Trace(i.PayReq)

										update.PaymentRequest = i.PayReq
										if err := e.sendFeedbackEvent(
											ctx,
											dvm,
											input,
											update,
										); err != nil {
											e.log.Errorf("[nostr] sendEventFeedback %+v\n", err)
										}

										u, e := e.lnSvc.TrackInvoice(ctx, i)
									trackInvoiceLoop:
										for {
											select {
											case invoiceUpdate := <-u:
												if invoiceUpdate.Settled {
													chanToDvm <- &domain.JobUpdate{
														Status: domain.StatusPaymentCompleted,
													}
													break trackInvoiceLoop
												}
											case <-e:
												chanToDvm <- &domain.JobUpdate{
													Status: domain.StatusError,
												}
												return
											}
										}

									} else if update.Status == domain.StatusSuccess {
										if err := e.sendFeedbackEvent(
											ctx,
											dvm,
											input,
											update,
										); err != nil {
											e.log.Errorf("[nostr] sendEventFeedback %+v\n", err)
										}
										if err := e.sendJobResultEvent(
											ctx,
											dvm,
											input,
											update,
										); err != nil {
											e.log.Errorf("[nostr] sendEventFeedback %+v\n", err)
										}

										e.log.Tracef("[engine] job completed %+v", update)
									}

								case err := <-chanErr:
									e.log.Tracef("[engine] job failed %+v", err)
									return
								case <-ctx.Done():
									e.log.Tracef("[engine] job context canceled")
									return
								}
							}
						}(dvmsForKind[i], nip90Input)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// advertiseDvms publishes two events:
// - kind 31990 for nip-89 handler information
// - kind 0 for nip-01 profile metadata
func (e *Engine) advertiseDvms(ctx context.Context) {
	for kind, dvms := range e.dvmsByKind {
		for i := range dvms {
			ev := nostr.NewHandlerInformationEvent(
				dvms[i].Pk(),
				dvms[i].Profile(),
				[]int{kind},
			)
			dvms[i].Sign(ev)
			if err := e.nostrSvc.PublishEvent(ctx, *ev); err != nil {
				e.log.Errorf("[engine] publish nip-89 %s %+v", dvms[i].Pk(), err)
			}

			profileEv := nostr.NewProfileMetadataEvent(
				dvms[i].Pk(),
				dvms[i].Profile(),
			)
			dvms[i].Sign(profileEv)
			if err := e.nostrSvc.PublishEvent(ctx, *profileEv); err != nil {
				e.log.Errorf("[engine] publish profile %s %+v", dvms[i].Pk(), err)
			}
		}
	}
}

func (e *Engine) sendFeedbackEvent(
	ctx context.Context,
	dvm domain.Dvmer,
	input *nostr.Nip90Input,
	update *domain.JobUpdate,
) error {
	feedbackEvent := &goNostr.Event{
		PubKey:    dvm.Pk(),
		CreatedAt: goNostr.Now(),
		Kind:      nostr.KindJobFeedback,
		Tags: goNostr.Tags{
			{"e", input.JobRequestId},
			{"p", input.CustomerPubkey},
			{"status", domain.JobStatusToString[update.Status]},
		},
	}

	if update.Status == domain.StatusPaymentRequired {
		tag := goNostr.Tag{
			"amount",
			fmt.Sprintf("%d", update.AmountSats*1000),
			update.PaymentRequest,
		}
		feedbackEvent.Tags = append(feedbackEvent.Tags, tag)
	}

	dvm.Sign(feedbackEvent)
	return e.nostrSvc.PublishEvent(
		ctx,
		*feedbackEvent,
	)
}

func (e *Engine) sendJobResultEvent(
	ctx context.Context,
	dvm domain.Dvmer,
	input *nostr.Nip90Input,
	update *domain.JobUpdate,
) error {
	jobResultEvent := &goNostr.Event{
		PubKey:    dvm.Pk(),
		CreatedAt: goNostr.Now(),
		Kind:      nostr.KindJobFeedback,
		Content:   update.Result,
		Tags: goNostr.Tags{
			{"request", input.JobRequestEventJSON},
			{"e", input.JobRequestId},
			{"p", input.CustomerPubkey},
			{"i", input.Input},
		},
	}

	if update.Status == domain.StatusPaymentRequired {
		tag := goNostr.Tag{
			"amount",
			fmt.Sprintf("%d", update.AmountSats*1000),
			update.PaymentRequest,
		}
		jobResultEvent.Tags = append(jobResultEvent.Tags, tag)
	}

	dvm.Sign(jobResultEvent)
	return e.nostrSvc.PublishEvent(
		ctx,
		*jobResultEvent,
	)
}

func (e *Engine) getKindsSupported() []int {
	kinds := make([]int, 0, len(e.dvmsByKind))
	for kindKey, _ := range e.dvmsByKind {
		kinds = append(kinds, kindKey)
	}

	return kinds
}
