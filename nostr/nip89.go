package nostr

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	goNostr "github.com/nbd-wtf/go-nostr"
	"math"
)

const (
	KindHandlerInformation = 31990
)

func randomBase16String(l int) string {
	buff := make([]byte, int(math.Ceil(float64(l)/2)))
	rand.Read(buff)
	str := hex.EncodeToString(buff)
	return str[:l] // strip 1 extra character we get from odd length results
}

func NewHandlerInformationEvent(
	pk string,
	profile *ProfileMetadata,
	supportedEventKinds []int,
) *goNostr.Event {
	e := &goNostr.Event{
		PubKey:    pk,
		CreatedAt: goNostr.Now(),
		Kind:      KindHandlerInformation,
	}

	profileBytes, _ := json.Marshal(profile)
	e.Content = string(profileBytes)

	tags := goNostr.Tags{
		{"d", randomBase16String(16)},
	}

	for i := range supportedEventKinds {
		tags = append(tags, goNostr.Tag{"k", fmt.Sprintf("%d", supportedEventKinds[i])})
	}

	e.Tags = tags

	return e
}
