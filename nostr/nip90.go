package nostr

import (
	"encoding/json"
	goNostr "github.com/nbd-wtf/go-nostr"
	"strconv"
)

type Nip90Input struct {
	JobRequestId        string
	CustomerPubkey      string
	Input               string
	InputType           string
	Relay               string
	Marker              string
	Output              string
	Params              [][2]string
	BidMillisats        int
	Relays              []string
	JobRequestEventJSON string
}

func Nip90InputFromEvent(e *goNostr.Event) (*Nip90Input, error) {
	input := &Nip90Input{
		JobRequestId:   e.ID,
		CustomerPubkey: e.PubKey,
		Params:         make([][2]string, 0),
	}

	eventJson, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	input.JobRequestEventJSON = string(eventJson)

	for i := range e.Tags {
		if len(e.Tags[i]) > 1 {
			if e.Tags[i][0] == "i" {
				input.Input = e.Tags[i][1]
				if len(e.Tags[i]) > 2 {
					input.InputType = e.Tags[i][2]
				}
				if len(e.Tags[i]) > 3 {
					input.Relay = e.Tags[i][3]
				}
				if len(e.Tags[i]) > 4 {
					input.Marker = e.Tags[i][4]
				}
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
			}
		}
	}

	return input, nil
}
