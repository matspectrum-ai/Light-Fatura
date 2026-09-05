package supabase

import (
	"context"
	"net/url"

	"github.com/matspectrum-ai/Light-Fatura/internal/payment"
)

func (c *Client) LatestInvoiceTransaction(ctx context.Context, invoiceID string) (*payment.ConfirmationTransaction, error) {
	tx, found, err := c.confirmationTransaction(ctx, url.Values{
		"fatura_id": {"eq." + invoiceID},
		"order":     {"created_at.desc"},
		"limit":     {"1"},
	})
	if err != nil || !found {
		return nil, err
	}
	return &tx, nil
}
