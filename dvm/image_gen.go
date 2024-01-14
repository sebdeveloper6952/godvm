package dvm

import (
	"context"
	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sebdeveloper6952/go-dvm/domain"
	"github.com/sebdeveloper6952/go-dvm/nostr"
)

type ImageGen struct {
	sk string
	pk string
}

type res struct {
	Result string
}

func (i *ImageGen) SetSk(sk string) error {
	i.sk = sk
	pk, err := goNostr.GetPublicKey(i.sk)
	if err != nil {
		return err
	}
	i.pk = pk

	return nil
}

func (i *ImageGen) Pk() string {
	return i.pk
}

func (i *ImageGen) Sign(e *goNostr.Event) error {
	return e.Sign(i.sk)
}

func (i *ImageGen) Profile() *nostr.ProfileMetadata {
	return &nostr.ProfileMetadata{
		Name:    "Test Image Gen",
		About:   "Just testing out some stuff, dont' use me yet.",
		Picture: "https://t3.ftcdn.net/jpg/01/63/58/38/360_F_163583838_5OjOIdaCH47G3hthwBzuTfdwq12hL0IG.jpg",
	}
}

func (i *ImageGen) KindSupported() int {
	return 5100
}

func (i *ImageGen) AcceptJob(input *nostr.Nip90Input) bool {
	_, err := nostr.ImageGenerationInputFromNip90Input(input)
	if err != nil {
		return false
	}

	if input.InputType != nostr.InputTypeText {
		return false
	}

	return true
}

func (i *ImageGen) Run(ctx context.Context, input *nostr.Nip90Input) (chan *domain.JobUpdate, chan *domain.JobUpdate, chan error) {
	chanToDvm := make(chan *domain.JobUpdate)
	chanToEngine := make(chan *domain.JobUpdate)
	chanErr := make(chan error)

	go func() {
		defer func() {
			close(chanToDvm)
			close(chanToEngine)
			close(chanErr)
		}()

		_, err := nostr.ImageGenerationInputFromNip90Input(input)
		if err != nil {
			chanErr <- err
			return
		}

		chanToEngine <- &domain.JobUpdate{
			Status:     domain.StatusPaymentRequired,
			AmountSats: 1,
		}

		for {
			select {
			case update := <-chanToDvm:
				if update.Status == domain.StatusPaymentCompleted {
					res, err := run(ctx)
					if err != nil {
						chanErr <- err
						return
					}

					chanToEngine <- &domain.JobUpdate{
						Status: domain.StatusSuccess,
						Result: res.Result,
					}
				} else if update.Status == domain.StatusError {
					return
				}
			case <-ctx.Done():
				chanToEngine <- &domain.JobUpdate{
					Status:     domain.StatusError,
					FailureMsg: "job context canceled",
				}
				return
			}
		}
	}()

	return chanToDvm, chanToEngine, chanErr
}

func run(ctx context.Context) (*res, error) {
	return &res{
		Result: "https://nosta.me/_nuxt/baby-nostrich_2x.3a1fac03.jpg",
	}, nil
}
