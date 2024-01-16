package domain

import (
	"context"
	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sebdeveloper6952/go-dvm/nostr"
)

type Dvmer interface {
	Pk() string
	Sign(e *goNostr.Event) error
	KindSupported() int
	AcceptJob(input *nostr.Nip90Input) bool
	Run(ctx context.Context, input *nostr.Nip90Input) (chan *JobUpdate, chan *JobUpdate, chan error)
}

type Dvm struct {
	sk   string
	Pk   string
	Name string
	Kind int
}
