package nostr

import (
	"context"
	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sirupsen/logrus"
	"time"
)

type Service interface {
	Run(ctx context.Context, dvmSupportedKinds []int) error
	Events() chan *goNostr.Event
	SendEvent(ctx context.Context, e goNostr.Event) error
}

type svc struct {
	relay          *goNostr.Relay
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

func (s *svc) Run(ctx context.Context, dvmSupportedKinds []int) error {
	s.supportedKinds = dvmSupportedKinds

	relay, err := goNostr.RelayConnect(ctx, "wss://nostr-pub.wellorder.net")
	if err != nil {
		return err
	}
	s.relay = relay

	go func() {
		var now = goNostr.Timestamp(time.Now().Unix())
		var filters goNostr.Filters = []goNostr.Filter{
			{
				Kinds: s.supportedKinds,
				Since: &now,
			},
		}

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
	}()

	return nil
}

func (s *svc) Events() chan *goNostr.Event {
	return s.events
}

func (s *svc) SendEvent(
	ctx context.Context,
	e goNostr.Event,
) error {
	s.log.Tracef("[nostr] publish event %+v\n", e)
	return s.relay.Publish(ctx, e)
}
