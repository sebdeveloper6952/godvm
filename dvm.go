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
	AcceptJob(input *Nip90Input) bool
	Run(ctx context.Context, input *Nip90Input) (chan *JobUpdate, chan *JobUpdate, chan error)
	Profile() *ProfileMetadata
}

type Dvm struct {
	sk   string
	Pk   string
	Name string
	Kind int
}
