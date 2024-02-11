package lnd

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/invoices"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"

	"github.com/sebdeveloper6952/godvm/lightning"
)

type lnd struct {
	address     string
	grpcPort    string
	httpPort    string
	macaroonHex string
	svc         *lndclient.GrpcLndServices
}

type lndInvoice struct {
	Settled bool `json:"settled"`
}

func New(
	address string,
	grpcPort string,
	httpPort string,
	macaroonHex string,
	tlsData string,
	network lndclient.Network,
) (lightning.Service, error) {
	svc, err := lndclient.NewLndServices(&lndclient.LndServicesConfig{
		LndAddress:        fmt.Sprintf("%s:%s", address, grpcPort),
		Network:           network,
		CustomMacaroonHex: macaroonHex,
		TLSData:           tlsData,
	})
	if err != nil {
		return nil, err
	}

	return &lnd{
		address,
		grpcPort,
		httpPort,
		macaroonHex,
		svc,
	}, nil
}

func (l *lnd) AddInvoice(
	ctx context.Context,
	amountSats int64,
) (*lightning.Invoice, error) {
	preimage := &lntypes.Preimage{}
	if _, err := rand.Read(preimage[:]); err != nil {
		return nil, err
	}

	hash, req, err := l.svc.Client.AddInvoice(
		ctx,
		&invoicesrpc.AddInvoiceData{
			Value:    lnwire.MilliSatoshi(amountSats * 1000),
			Preimage: preimage,
		},
	)
	if err != nil {
		return nil, err
	}

	return &lightning.Invoice{
		Hash:   hash,
		PayReq: req,
	}, nil

}

func (l *lnd) TrackInvoice(
	ctx context.Context,
	invoice *lightning.Invoice,
) (chan *lightning.InvoiceUpdate, chan error) {
	return l.trackInvoiceGrpc(ctx, invoice)
}

func (l *lnd) trackInvoiceGrpc(
	ctx context.Context,
	invoice *lightning.Invoice,
) (chan *lightning.InvoiceUpdate, chan error) {
	updates := make(chan *lightning.InvoiceUpdate)
	errors := make(chan error)

	go func() {
		defer close(updates)
		defer close(errors)

		u, errs, err := l.svc.Invoices.SubscribeSingleInvoice(
			ctx,
			invoice.Hash,
		)
		if err != nil {
			errors <- err
			return
		}

		for {
			select {
			case update := <-u:
				if update.State == invoices.ContractSettled {
					updates <- &lightning.InvoiceUpdate{
						Settled: true,
					}
					return
				}
			case err := <-errs:
				errors <- err
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return updates, errors
}

func (l *lnd) trackInvoiceHttp(
	ctx context.Context,
	invoice *lightning.Invoice,
) (chan *lightning.InvoiceUpdate, chan error) {
	updates := make(chan *lightning.InvoiceUpdate)
	errors := make(chan error)

	go func() {
		defer close(updates)
		defer close(errors)

		req, err := http.NewRequest(
			http.MethodGet,
			fmt.Sprintf("https://%s:%s/v2/invoices/subscribe/%s", l.address, l.httpPort, invoice.Hash.String()),
			http.NoBody,
		)
		if err != nil {
			errors <- err
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Grpc-Metadata-macaroon", l.macaroonHex)

		for {
			select {
			case <-time.After(time.Second):
				http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					errors <- err
					return
				}

				target := &lndInvoice{}
				b, err := io.ReadAll(res.Body)
				log.Printf("%s\n", b)
				if err != nil {
					errors <- err
					res.Body.Close()
					return
				}
				if err := json.NewDecoder(res.Body).Decode(target); err != nil {
					errors <- err
					res.Body.Close()
					return
				}

				log.Println(target)

				if target.Settled {
					updates <- &lightning.InvoiceUpdate{
						Settled: true,
					}
					res.Body.Close()
					return
				}
			case <-ctx.Done():
				return
			}
		}

	}()

	return updates, errors
}
