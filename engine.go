package godvm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sebdeveloper6952/godvm/lightning"
)

type Engine struct {
	dvmsByKind      map[int][]Dvmer
	nostrSvc        NostrService
	lnSvc           lightning.Service
	log             *log.Logger
	waitingForEvent map[string][]chan *goNostr.Event
}

func NewEngine() (*Engine, error) {
	logger := log.New(os.Stderr, "[godvm] ", log.LstdFlags)

	nostrSvc, err := NewNostrService(
		logger,
	)
	if err != nil {
		return nil, err
	}

	e := &Engine{
		dvmsByKind:      make(map[int][]Dvmer),
		waitingForEvent: make(map[string][]chan *goNostr.Event),
		nostrSvc:        nostrSvc,
		log:             logger,
	}

	return e, nil
}

func (e *Engine) RegisterDVM(dvm Dvmer) {
	kindSupported := dvm.KindSupported()
	if _, ok := e.dvmsByKind[kindSupported]; !ok {
		e.dvmsByKind[kindSupported] = make([]Dvmer, 0, 2)
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
			e.log.Printf("run nostr service %+v", err)
		}

		e.advertiseDvms(ctx)
	}()

	go func() {
		for {
			select {
			case event := <-e.nostrSvc.JobRequestEvents():
				dvmsForKind, ok := e.dvmsByKind[event.Kind]
				if !ok {
					e.log.Printf("no dvms for kind %d\n", event.Kind)
					continue
				}

				nip90Input, err := Nip90InputFromJobRequestEvent(event)
				if err != nil {
					e.log.Printf("nip90Input from event  %+v\n", err)
					continue
				}

				// if the inputs are asking for events/jobs, we fetch them here before proceeding
				var wg sync.WaitGroup
				for inputIdx := range nip90Input.Inputs {
					if nip90Input.Inputs[inputIdx].Type == InputTypeEvent ||
						nip90Input.Inputs[inputIdx].Type == InputTypeJob {
						wg.Add(1)
						go func(input *Input) {
							defer wg.Done()

							// TODO: must handle when the event is not found, only when the input type is "event".
							//       When input type is "job", we have to wait no matter what, because it could
							//       be a job that is completed in the future.
							waitCh, err := e.nostrSvc.FetchEvent(ctx, input.Value)
							if err != nil {
								e.log.Printf("fetch event for job input %+v", err)
								return
							}
							input.Event = <-waitCh

							e.log.Printf("fetched event for job input")
						}(nip90Input.Inputs[inputIdx])
					}
				}
				wg.Wait()

				e.log.Printf("finished waiting for input events")

				for i := range dvmsForKind {
					go func(dvm Dvmer, input *Nip90Input) {
						if err := e.runDvm(ctx, dvm, input); err != nil {
							e.log.Println(err)
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

func (e *Engine) runDvm(ctx context.Context, dvm Dvmer, input *Nip90Input) error {
	chanToDvm := make(chan *JobUpdate)
	chanToEngine := make(chan *JobUpdate)

	defer func() {
		close(chanToDvm)
	}()

	if !dvm.Run(ctx, input, chanToDvm, chanToEngine) {
		return errors.New("job not accepted by DVM")
	}

	for {
		select {
		case update := <-chanToEngine:
			if update.Status == StatusPaymentRequired || update.Status == StatusSuccessWithPayment {
				invoice, err := e.addInvoiceAndTrack(ctx, chanToDvm, int64(update.AmountSats))
				if err != nil {
					return err
				}
				update.PaymentRequest = invoice.PayReq
			}

			if err := e.sendFeedbackEvent(
				ctx,
				dvm,
				input,
				update,
			); err != nil {
				return err
			}

			if update.Status == StatusSuccess || update.Status == StatusSuccessWithPayment {
				if err := e.sendJobResultEvent(
					ctx,
					dvm,
					input,
					update,
				); err != nil {
					return err
				}

				// if success status, exit this goroutine to free resources
				return nil
			}
		case <-ctx.Done():
			e.log.Printf("job context canceled")
			return nil
		}
	}
}

// advertiseDvms publishes two events:
// - kind 31990 for nip-89 handler information
// - kind 0 for nip-01 profile metadata
func (e *Engine) advertiseDvms(ctx context.Context) {
	for kind, dvms := range e.dvmsByKind {
		for i := range dvms {
			ev := NewHandlerInformationEvent(
				dvms[i].PublicKeyHex(),
				dvms[i].Profile(),
				[]int{kind},
				dvms[i].Version(),
			)
			dvms[i].Sign(ev)
			if err := e.nostrSvc.PublishEvent(ctx, *ev); err != nil {
				e.log.Printf("publish nip-89 %s %+v", dvms[i].PublicKeyHex(), err)
			}

			profileEv := NewProfileMetadataEvent(
				dvms[i].PublicKeyHex(),
				dvms[i].Profile(),
			)
			dvms[i].Sign(profileEv)
			if err := e.nostrSvc.PublishEvent(ctx, *profileEv); err != nil {
				e.log.Printf("publish profile %s %+v", dvms[i].PublicKeyHex(), err)
			}
		}
	}
}

func (e *Engine) addInvoiceAndTrack(
	ctx context.Context,
	chanToDvm chan<- *JobUpdate,
	amountSats int64,
) (*lightning.Invoice, error) {
	invoice, err := e.lnSvc.AddInvoice(ctx, amountSats)
	if err != nil {
		chanToDvm <- &JobUpdate{
			Status: StatusError,
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
					chanToDvm <- &JobUpdate{
						Status: StatusPaymentCompleted,
					}
					break trackInvoiceLoop
				}
			case <-e:
				chanToDvm <- &JobUpdate{
					Status: StatusError,
				}
				return
			}
		}
	}()

	return invoice, nil
}

func (e *Engine) sendFeedbackEvent(
	ctx context.Context,
	dvm Dvmer,
	input *Nip90Input,
	update *JobUpdate,
) error {
	feedbackEvent := &goNostr.Event{
		PubKey:    dvm.PublicKeyHex(),
		CreatedAt: goNostr.Now(),
		Kind:      KindJobFeedback,
		Tags: goNostr.Tags{
			{"e", input.JobRequestId},
			{"p", input.CustomerPubkey},
			{"status", JobStatusToString[update.Status]},
		},
	}

	if update.ExtraTags != nil && len(update.ExtraTags) > 0 {
		for i := range update.ExtraTags {
			feedbackEvent.Tags = append(feedbackEvent.Tags, update.ExtraTags[i])
		}
	}

	if update.Status == StatusPaymentRequired {
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
	dvm Dvmer,
	input *Nip90Input,
	update *JobUpdate,
) error {
	tags := goNostr.Tags{
		{"request", input.JobRequestEventJSON},
		{"e", input.JobRequestId},
		{"p", input.CustomerPubkey},
	}

	if update.ExtraTags != nil && len(update.ExtraTags) > 0 {
		for i := range update.ExtraTags {
			tags = append(tags, update.ExtraTags[i])
		}
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

	if update.Status == StatusSuccessWithPayment && update.PaymentRequest != "" {
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
		PubKey:    dvm.PublicKeyHex(),
		CreatedAt: goNostr.Now(),
		Kind:      input.ResultKind,
		Content:   update.Result,
		Tags:      tags,
	}

	if update.Status == StatusPaymentRequired {
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
