package godvm

import (
	"encoding/json"

	goNostr "github.com/nbd-wtf/go-nostr"
)

const (
	KindProfileMetadata = 0
)

type ProfileMetadata struct {
	Name    string `json:"name"`
	About   string `json:"about"`
	Picture string `json:"picture"`
}

func NewProfileMetadataEvent(
	pk string,
	profile *ProfileMetadata,
) *goNostr.Event {
	e := &goNostr.Event{
		PubKey:    pk,
		CreatedAt: goNostr.Now(),
		Kind:      KindProfileMetadata,
	}

	profileBytes, _ := json.Marshal(profile)
	e.Content = string(profileBytes)

	return e
}
