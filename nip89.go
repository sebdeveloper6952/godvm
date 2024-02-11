package godvm

import (
	"encoding/json"
	"fmt"

	goNostr "github.com/nbd-wtf/go-nostr"
)

const (
	KindHandlerInformation = 31990
)

func NewHandlerInformationEvent(
	pk string,
	profile *ProfileMetadata,
	supportedEventKinds []int,
	dTagVersion string,
) *goNostr.Event {
	e := &goNostr.Event{
		PubKey:    pk,
		CreatedAt: goNostr.Now(),
		Kind:      KindHandlerInformation,
	}

	profileBytes, _ := json.Marshal(profile)
	e.Content = string(profileBytes)

	tags := goNostr.Tags{
		{"d", dTagVersion},
	}

	for i := range supportedEventKinds {
		tags = append(tags, goNostr.Tag{"k", fmt.Sprintf("%d", supportedEventKinds[i])})
	}

	e.Tags = tags

	return e
}
