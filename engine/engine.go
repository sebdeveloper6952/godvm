package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"

	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sebdeveloper6952/go-dvm/domain"
	"github.com/sebdeveloper6952/go-dvm/lightning"
	"github.com/sebdeveloper6952/go-dvm/nostr"
	"github.com/sirupsen/logrus"
)

type Engine struct {
	dvmsByKind      map[int][]domain.Dvmer
	nostrSvc        nostr.Service
	lnSvc           lightning.Service
	log             *logrus.Logger
	waitingForEvent map[string][]chan *goNostr.Event
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
		dvmsByKind:      make(map[int][]domain.Dvmer),
		waitingForEvent: make(map[string][]chan *goNostr.Event),
		nostrSvc:        nostrSvc,
		log:             logger,
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
			case event := <-e.nostrSvc.JobRequestEvents():
				dvmsForKind, ok := e.dvmsByKind[event.Kind]
				if !ok {
					e.log.Debugf("[engine] no dvms for kind %d\n", event.Kind)
					continue
				}

				nip90Input, err := nostr.Nip90InputFromJobRequestEvent(event)
				if err != nil {
					e.log.Errorf("[engine] nip90Input from event  %+v\n", err)
					continue
				}

				// if the inputs are asking for events/jobs, we fetch them here before proceeding
				var wg sync.WaitGroup
				for inputIdx := range nip90Input.Inputs {
					if nip90Input.Inputs[inputIdx].Type == nostr.InputTypeEvent ||
						nip90Input.Inputs[inputIdx].Type == nostr.InputTypeJob {
						wg.Add(1)
						go func(input *nostr.Input) {
							defer wg.Done()

							// TODO: must handle when the event is not found, only when the input type is "event".
							//       When input type is "job", we have to wait no matter what, because it could
							//       be a job that is completed in the future.
							waitCh, err := e.nostrSvc.FetchEvent(ctx, input.Value)
							if err != nil {
								e.log.Errorf("[engine] fetch event for job input %+v", err)
								return
							}
							input.Event = <-waitCh

							e.log.Tracef("[engine] fetched event for job input")
						}(nip90Input.Inputs[inputIdx])
					}
				}
				wg.Wait()

				e.log.Tracef("[engine] finished waiting for input events")

				for i := range dvmsForKind {
					go func(dvm domain.Dvmer, input *nostr.Nip90Input) {
						if dvm.AcceptJob(input) {
							e.runDvm(ctx, dvm, input)
						}
					}(dvmsForKind[i], nip90Input)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (e *Engine) runDvm(ctx context.Context, dvm domain.Dvmer, input *nostr.Nip90Input) {
	chanToDvm, chanToEngine, chanErr := dvm.Run(ctx, input)

	for {
		select {
		case update := <-chanToEngine:
			if update.Status == domain.StatusPaymentRequired {
				invoice, err := e.addInvoiceAndTrack(ctx, chanToDvm, int64(update.AmountSats))
				if err != nil {
					e.log.Tracef("[nostr] addInvoice %+v\n", err)
					return
				}

				update.PaymentRequest = invoice.PayReq
				if err := e.sendFeedbackEvent(
					ctx,
					dvm,
					input,
					update,
				); err != nil {
					e.log.Errorf("[nostr] sendEventFeedback %+v\n", err)
				}
			} else if update.Status == domain.StatusProcessing {
				if err := e.sendFeedbackEvent(
					ctx,
					dvm,
					input,
					update,
				); err != nil {
					e.log.Errorf("[nostr] sendEventFeedback %+v\n", err)
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
				return
			} else if update.Status == domain.StatusSuccessWithPayment {
				if err := e.sendFeedbackEvent(
					ctx,
					dvm,
					input,
					update,
				); err != nil {
					e.log.Tracef("[nostr] sendEventFeedback %+v\n", err)
				}

				invoice, err := e.lnSvc.AddInvoice(ctx, int64(update.AmountSats))
				if err != nil {
					e.log.Tracef("[nostr] StatusSuccessWithPayment addInvoice %+v\n", err)
					return
				}
				update.PaymentRequest = invoice.PayReq

				if err := e.sendJobResultEvent(
					ctx,
					dvm,
					input,
					update,
				); err != nil {
					e.log.Errorf("[nostr] sendEventFeedback %+v\n", err)
				}

				e.log.Tracef("[engine] job completed %+v", update)
				return
			}

		case err := <-chanErr:
			if err != nil {
				e.log.Tracef("[engine] job failed %+v", err)
				return
			}
		case <-ctx.Done():
			e.log.Tracef("[engine] job context canceled")
			return
		}
	}
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
				dvms[i].Version(),
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

func (e *Engine) addInvoiceAndTrack(
	ctx context.Context,
	chanToDvm chan *domain.JobUpdate,
	amountSats int64,
) (*lightning.Invoice, error) {
	invoice, err := e.lnSvc.AddInvoice(ctx, amountSats)
	if err != nil {
		chanToDvm <- &domain.JobUpdate{
			Status: domain.StatusError,
		}
		return nil, err
	}

	go func() {
		u, e := e.lnSvc.TrackInvoice(ctx, invoice)
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
	}()

	return invoice, nil
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
		input.Relays...,
	)
}

func (e *Engine) sendJobResultEvent(
	ctx context.Context,
	dvm domain.Dvmer,
	input *nostr.Nip90Input,
	update *domain.JobUpdate,
) error {
	tags := goNostr.Tags{
		{"request", input.JobRequestEventJSON},
		{"e", input.JobRequestId},
		{"p", input.CustomerPubkey},
	}

	for i := range input.Inputs {
		tag := goNostr.Tag{
			"i",
			input.Inputs[i].Value,
		}

		if input.Inputs[i].Type != "" {
			tag = append(tag, input.Inputs[i].Type)
		}

		if input.Inputs[i].Relay != "" {
			tag = append(tag, input.Inputs[i].Relay)
		}

		if input.Inputs[i].Marker != "" {
			tag = append(tag, input.Inputs[i].Marker)
		}

		tags = append(tags, tag)
	}

	if update.Status == domain.StatusSuccessWithPayment && update.PaymentRequest != "" {
		tags = append(
			tags,
			goNostr.Tag{
				"amount",
				fmt.Sprintf("%d", update.AmountSats*1000),
				update.PaymentRequest,
			},
		)
	}

	jobResultEvent := &goNostr.Event{
		PubKey:    dvm.Pk(),
		CreatedAt: goNostr.Now(),
		Kind:      input.ResultKind,
		Content:   update.Result,
		Tags:      tags,
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
		input.Relays...,
	)
}

func (e *Engine) saveDvmWaitingForEvent(id string, waitCh chan *goNostr.Event) {
	if _, exist := e.waitingForEvent[id]; !exist {
		e.waitingForEvent[id] = make([]chan *goNostr.Event, 0, 1)
	}

	e.waitingForEvent[id] = append(e.waitingForEvent[id], waitCh)
}

func (e *Engine) getKindsSupported() []int {
	kinds := make([]int, 0, len(e.dvmsByKind))
	for kindKey, _ := range e.dvmsByKind {
		kinds = append(kinds, kindKey)
	}

	return kinds
}
