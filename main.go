package main

import (
	"context"
	"github.com/lightninglabs/lndclient"
	"github.com/sebdeveloper6952/go-dvm/dvm"
	"github.com/sebdeveloper6952/go-dvm/engine"
	"github.com/sebdeveloper6952/go-dvm/lightning/lnd"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancelCtx := context.WithCancel(context.Background())

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetLevel(logrus.TraceLevel)

	tlsBytes, err := os.ReadFile(os.Getenv("LND_TLS_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	lnSvc, err := lnd.New(
		os.Getenv("LND_ADDR"),
		os.Getenv("LND_GRPC_PORT"),
		os.Getenv("LND_HTTP_PORT"),
		os.Getenv("LND_MAC_HEX"),
		string(tlsBytes),
		lndclient.NetworkRegtest,
	)
	if err != nil {
		log.Fatal(err)
	}

	imageGenDvm := &dvm.DvmImageGen{}
	imageGenDvm.SetSk(os.Getenv("DVM_SK"))

	e, err := engine.NewEngine()
	if err != nil {
		log.Fatal(err)
	}
	e.SetLnService(lnSvc)

	e.RegisterDVM(imageGenDvm)

	e.Run(ctx)

	log.Println("running...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	for {
		select {
		case sig := <-sigChan:
			if sig == os.Interrupt {
				cancelCtx()
				log.Println("bye")
				return
			}
		}
	}
}
