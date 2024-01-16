package nostr

import (
	"context"
	"errors"
	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sirupsen/logrus"
	"time"
)

type Service interface {
	Run(
		ctx context.Context,
		dvmSupportedKinds []int,
		initialRelays []string,
	) error
	Events() chan *goNostr.Event
	PublishEvent(
		ctx context.Context,
		e goNostr.Event,
		additionalRelays ...string,
	) error
}

type svc struct {
	relays         []*goNostr.Relay
	events         chan *goNostr.Event
	supportedKinds []int
	log            *logrus.Logger
}

func NewNostr(
	log *logrus.Logger,
) (Service, error) {
	return &svc{
		events: make(chan *goNostr.Event),
		log:    log,
	}, nil
}

func (s *svc) Run(
	ctx context.Context,
	dvmSupportedKinds []int,
	initialRelays []string,
) error {
	s.supportedKinds = dvmSupportedKinds

	if initialRelays == nil || len(initialRelays) == 0 {
		return errors.New("must provide at least one relay")
	}

	s.relays = make([]*goNostr.Relay, 0, len(initialRelays))
	for i := range initialRelays {
		relay, err := goNostr.RelayConnect(ctx, initialRelays[i])
		if err != nil {
			return err
		}
		s.relays = append(s.relays, relay)
	}

	go func() {
		var now = goNostr.Timestamp(time.Now().Unix())
		var filters goNostr.Filters = []goNostr.Filter{
			{
				Kinds: s.supportedKinds,
				Since: &now,
			},
		}

		for i := range s.relays {
			go func(relay *goNostr.Relay) {
				sub, err := relay.Subscribe(ctx, filters)
				if err != nil {
					s.log.Errorf("[nostr] %+v\n", err)
					return
				}

				for {
					select {
					case event := <-sub.Events:
						s.log.Tracef("[nostr] received event %+v\n", event)
						s.events <- event
					case <-ctx.Done():
						return
					}
				}
			}(s.relays[i])

		}

	}()

	return nil
}

func (s *svc) Events() chan *goNostr.Event {
	return s.events
}

func (s *svc) PublishEvent(
	ctx context.Context,
	e goNostr.Event,
	additionalRelays ...string,
) error {
	s.log.Tracef("[nostr] publish event %+v\n", e)

	for i := range s.relays {
		if err := s.relays[i].Publish(ctx, e); err != nil {
			s.log.Errorf("[nostr] publish to relay %s %+v", s.relays[i].URL, err)
		}
	}

	return nil
}
