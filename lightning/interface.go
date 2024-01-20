package lightning

import (
	"context"

	"github.com/lightningnetwork/lnd/lntypes"
)

type Invoice struct {
	Hash   lntypes.Hash
	PayReq string
}

type InvoiceUpdate struct {
	Settled bool
}

type Service interface {
	AddInvoice(ctx context.Context, amountSats int64) (*Invoice, error)
	TrackInvoice(ctx context.Context, invoice *Invoice) (chan *InvoiceUpdate, chan error)
}
