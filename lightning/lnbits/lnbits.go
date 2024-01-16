package lnbits

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/sebdeveloper6952/go-dvm/lightning"
	"net/http"
	"time"
)

type lnbits struct {
	url string
	key string
}

type payment struct {
	Out    bool `json:"out"`
	Amount int  `json:"amount"`
}

type paymentResponse struct {
	PaymentHash    string `json:"payment_hash"`
	PaymentRequest string `json:"payment_request"`
	Paid           bool   `json:"paid"`
}

func New(
	url string,
	key string,
) (lightning.Service, error) {
	return &lnbits{
		url: url,
		key: key,
	}, nil
}

func (l lnbits) AddInvoice(ctx context.Context, amountSats int64) (*lightning.Invoice, error) {
	body := &payment{
		Out:    false,
		Amount: int(amountSats),
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		l.url+"/api/v1/payments",
		bytes.NewBuffer(bodyBytes),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", l.key)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	target := &paymentResponse{}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}

	hash, err := lntypes.MakeHashFromStr(target.PaymentHash)
	if err != nil {
		return nil, err
	}

	return &lightning.Invoice{
		Hash:   hash,
		PayReq: target.PaymentRequest,
	}, nil
}

func (l lnbits) TrackInvoice(ctx context.Context, invoice *lightning.Invoice) (chan *lightning.InvoiceUpdate, chan error) {
	updates := make(chan *lightning.InvoiceUpdate)
	errors := make(chan error)

	go func() {
		defer close(updates)
		defer close(errors)

		req, err := http.NewRequest(
			http.MethodGet,
			l.url+"/api/v1/payments/"+invoice.Hash.String(),
			http.NoBody,
		)
		if err != nil {
			errors <- err
			return
		}

		req.Header.Set("X-Api-Key", l.key)

		for {
			select {
			case <-time.After(time.Second):
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					errors <- err
					return
				}

				target := &paymentResponse{}
				if err := json.NewDecoder(res.Body).Decode(target); err != nil {
					errors <- err
					res.Body.Close()
					return
				}

				if target.Paid {
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
