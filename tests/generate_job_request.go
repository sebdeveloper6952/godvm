package main

import (
	"context"
	"log"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		ctx, cancel = context.WithCancel(context.Background())
	)
	defer cancel()

	sk := "42de75ac949ec44903f8d329d888a92d531bbc646aece510916dcadeeb08ff80"
	pk, _ := nostr.GetPublicKey(sk)

	ev := nostr.Event{
		PubKey:    pk,
		CreatedAt: nostr.Now(),
		Kind:      5100,
		Tags: nostr.Tags{
			{"i", "prompt here"},
		},
	}
	ev.Sign(sk)

	relay, err := nostr.RelayConnect(ctx, "wss://nostr-pub.wellorder.net")
	if err != nil {
		log.Fatal(err)
	}

	relay.Publish(ctx, ev)

	logrus.Debug("event sent")
}
