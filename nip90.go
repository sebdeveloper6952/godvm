package godvm

import (
	"encoding/json"
	"fmt"
	"strconv"

	goNostr "github.com/nbd-wtf/go-nostr"
)

const (
	KindReqTextExtraction    = 5000
	KindReqTextSummarization = 5001
	KindReqTextTranslation   = 5002
	KindReqTextGeneration    = 5050
	KindReqImageGeneration   = 5100
	KindReqVideoConversion   = 5200
	KindReqVideoTranslation  = 5201
	KindReqTextToSpeech      = 5250
	KindReqContentDiscovery  = 5300
	KindReqNpubDiscovery     = 5301
	KindReqNostrEventCount   = 5400
	KindReqMalwareScan       = 5500
	KindReqAppAnalysis       = 5501
	KindReqEventTimestamping = 5900
	KindReqBitcoinOpReturn   = 5901

	KindJobFeedback = 7000

	InputTypeText  = "text"
	InputTypeURL   = "url"
	InputTypeEvent = "event"
	InputTypeJob   = "job"
)

type Input struct {
	Value  string
	Type   string
	Relay  string
	Marker string
	Event  *goNostr.Event
}

type Nip90Input struct {
	JobRequestId        string
	CustomerPubkey      string
	Inputs              []*Input
	Relay               string
	Output              string
	Params              [][2]string
	BidMillisats        int
	Relays              []string
	JobRequestEventJSON string
	Event               *goNostr.Event
	TaggedPubkeys       map[string]struct{}
}

func Nip90InputFromJobRequestEvent(e *goNostr.Event) (*Nip90Input, error) {
	input := &Nip90Input{
		JobRequestId:   e.ID,
		CustomerPubkey: e.PubKey,
		Params:         make([][2]string, 0),
		Event:          e,
		Inputs:         make([]*Input, 0, 1),
		TaggedPubkeys:  make(map[string]struct{}),
	}

	eventJson, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	input.JobRequestEventJSON = string(eventJson)

	for i := range e.Tags {
		if len(e.Tags[i]) > 1 {
			if e.Tags[i][0] == "i" {
				newInput := &Input{
					Value: e.Tags[i][1],
				}

				if len(e.Tags[i]) > 2 {
					newInput.Type = e.Tags[i][2]
				}

				if len(e.Tags[i]) > 3 {
					newInput.Relay = e.Tags[i][3]
				}

				if len(e.Tags[i]) == 5 {
					newInput.Marker = e.Tags[i][4]
				}

				input.Inputs = append(input.Inputs, newInput)
			} else if e.Tags[i][0] == "output" {
				input.Output = e.Tags[i][1]
			} else if e.Tags[i][0] == "param" && len(e.Tags[i]) == 3 {
				input.Params = append(input.Params, [2]string{e.Tags[i][1], e.Tags[i][2]})
			} else if e.Tags[i][0] == "bid" {
				bidMillisats, err := strconv.Atoi(e.Tags[i][1])
				if err != nil {
					return nil, err
				}
				input.BidMillisats = bidMillisats
			} else if e.Tags[i][0] == "p" && len(e.Tags[i]) == 2 {
				input.TaggedPubkeys[e.Tags[i][1]] = struct{}{}
			}
		}
	}

	return input, nil
}

func Nip90JobFeedbackFromEngineUpdate(
	input *Nip90Input,
	update *JobUpdate,
) *goNostr.Event {
	feedbackEvent := &goNostr.Event{
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

	return feedbackEvent
}

func Nip90JobResultFromEngineUpdate(
	input *Nip90Input,
	update *JobUpdate,
) *goNostr.Event {
	jobResultEvent := &goNostr.Event{
		Kind:      input.Event.Kind + 1000,
		Content:   update.Result,
		CreatedAt: goNostr.Now(),
		Tags: goNostr.Tags{
			{"request", input.JobRequestEventJSON},
			{"e", input.JobRequestId},
			{"p", input.CustomerPubkey},
		},
	}

	if update.ExtraTags != nil && len(update.ExtraTags) > 0 {
		for i := range update.ExtraTags {
			jobResultEvent.Tags = append(jobResultEvent.Tags, update.ExtraTags[i])
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

		jobResultEvent.Tags = append(jobResultEvent.Tags, tag)
	}

	if update.Status == StatusSuccessWithPayment && update.PaymentRequest != "" {
		jobResultEvent.Tags = append(
			jobResultEvent.Tags,
			goNostr.Tag{
				"amount",
				fmt.Sprintf("%d", update.AmountSats*1000),
				update.PaymentRequest,
			},
		)
	}

	if update.Status == StatusPaymentRequired {
		tag := goNostr.Tag{
			"amount",
			fmt.Sprintf("%d", update.AmountSats*1000),
			update.PaymentRequest,
		}
		jobResultEvent.Tags = append(jobResultEvent.Tags, tag)
	}

	return jobResultEvent
}
