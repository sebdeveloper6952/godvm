package godvm

import (
	"encoding/json"
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

	KindResTextExtraction    = 6000
	KindResTextSummarization = 6001
	KindResTextTranslation   = 6002
	KindResTextGeneration    = 6050
	KindResImageGeneration   = 6100
	KindResVideoConversion   = 6200
	KindResVideoTranslation  = 6201
	KindResTextToSpeech      = 6250
	KindResContentDiscovery  = 6300
	KindResNpubDiscovery     = 6301
	KindResNostrEventCount   = 6400
	KindResMalwareScan       = 6500
	KindResAppAnalysis       = 6501
	KindResEventTimestamping = 6900
	KindResBitcoinOpReturn   = 6901

	KindJobFeedback = 7000

	InputTypeText  = "text"
	InputTypeURL   = "url"
	InputTypeEvent = "event"
	InputTypeJob   = "job"
)

var (
	reqToRes = map[int]int{
		KindReqImageGeneration: KindResImageGeneration,
		KindReqMalwareScan:     KindResMalwareScan,
	}
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
	ResultKind          int
	TaggedPubkeys       map[string]struct{}
}

func Nip90InputFromJobRequestEvent(e *goNostr.Event) (*Nip90Input, error) {
	input := &Nip90Input{
		JobRequestId:   e.ID,
		CustomerPubkey: e.PubKey,
		Params:         make([][2]string, 0),
		Event:          e,
		ResultKind:     responseKind(e.Kind),
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

func responseKind(requestKind int) int {
	if res, ok := reqToRes[requestKind]; ok {
		return res
	}

	return 0
}
