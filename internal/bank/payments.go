package bank

import (
	"context"
	"errors"

	"github.com/h4rdm0d3/flashsale-tickets/internal/booking"
)

// Payments takes the booking domain's payments through the bank: it
// implements booking.Payments.
type Payments struct {
	client   *Client
	attempts int
}

var _ booking.Payments = (*Payments)(nil)

// NewPayments returns Payments that try each charge up to attempts times
// when the bank fails.
func NewPayments(client *Client, attempts int) *Payments {
	return &Payments{client: client, attempts: attempts}
}

// Charge takes a payment, trying again when the bank fails.
func (p *Payments) Charge(ctx context.Context, pay booking.Payment) (string, error) {
	req := ChargeRequest{Account: pay.Buyer, AmountCents: pay.AmountCents}
	var lastErr error
	for range p.attempts {
		ch, err := p.client.CreateCharge(ctx, req)
		switch {
		case err == nil:
			return ch.ID, nil
		case errors.Is(err, ErrDeclined):
			return "", booking.ErrDeclined
		}
		lastErr = err
	}
	return "", lastErr
}
