package godvm

import (
	"context"

	goNostr "github.com/nbd-wtf/go-nostr"
)

type Dvmer interface {
	Pk() string
	Sign(e *goNostr.Event) error
	KindSupported() int
	Version() string
	Run(
		ctx context.Context,
		input *Nip90Input,
		chanToDvm <-chan *JobUpdate,
		chanToEngine chan<- *JobUpdate,
	) bool
	Profile() *ProfileMetadata
}

type Dvm struct {
	sk   string
	Pk   string
	Name string
	Kind int
}
