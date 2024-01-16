package nostr

import (
	"context"
	"errors"
	goNostr "github.com/nbd-wtf/go-nostr"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type Service interface {
	Run(
		ctx context.Context,
		dvmSupportedKinds []int,
		initialRelays []string,
	) error
	JobRequestEvents() chan *goNostr.Event
	InputEvents() chan *goNostr.Event
	PublishEvent(
		ctx context.Context,
		e goNostr.Event,
		additionalRelays ...string,
	) error
	FetchEvent(ctx context.Context, id string) error
}

type svc struct {
	relays           []*goNostr.Relay
	jobRequestEvents chan *goNostr.Event
	inputEvents      chan *goNostr.Event
	supportedKinds   []int
	seenEvents       map[string]struct{}
	log              *logrus.Logger
}

func NewNostr(
	log *logrus.Logger,
) (Service, error) {
	return &svc{
		jobRequestEvents: make(chan *goNostr.Event),
		inputEvents:      make(chan *goNostr.Event),
		seenEvents:       make(map[string]struct{}),
		log:              log,
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
		var (
			now                     = goNostr.Timestamp(time.Now().Unix())
			filters goNostr.Filters = []goNostr.Filter{
				{
					Kinds: s.supportedKinds,
					Since: &now,
				},
			}
		)

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
						if event != nil {
							if _, exist := s.seenEvents[event.ID]; exist {
								continue
							}
							s.log.Tracef("[nostr] received event %+v\n", event)
							s.seenEvents[event.ID] = struct{}{}
							s.jobRequestEvents <- event
						}
					case <-ctx.Done():
						return
					}
				}
			}(s.relays[i])

		}
	}()

	return nil
}

func (s *svc) JobRequestEvents() chan *goNostr.Event {
	return s.jobRequestEvents
}

func (s *svc) InputEvents() chan *goNostr.Event {
	return s.inputEvents
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

func (s *svc) FetchEvent(ctx context.Context, id string) error {
	var (
		filters = []goNostr.Filter{
			{IDs: []string{id}},
		}
		wg                sync.WaitGroup
		subCtx, cancelCtx = context.WithCancel(ctx)
	)

	wg.Add(len(s.relays))
	for i := range s.relays {
		go func(relay *goNostr.Relay) {
			defer wg.Done()

			sub, err := relay.Subscribe(subCtx, filters)
			if err != nil {
				s.log.Errorf("[nostr] %+v\n", err)
				return
			}

			for {
				select {
				case event := <-sub.Events:
					if event != nil {
						s.log.Tracef("[nostr] received requested event %s %+v\n", relay.URL, event)
						s.inputEvents <- event
						sub.Close()
						cancelCtx()
						return
					}
				case <-subCtx.Done():
					return
				}
			}
		}(s.relays[i])
	}

	wg.Wait()
	cancelCtx()

	return nil
}
