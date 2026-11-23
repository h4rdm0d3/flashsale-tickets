// Package bank talks to the bank: a third party, so its state is its own,
// and it's as slow and unreliable as real payment providers. Client is its
// HTTP API as the bank defines it. Payments adapts it to the booking
// domain.
//
//	POST /charges          {"idempotency_key": "...", "account": 42, "amount_cents": 4500}
//	  201 {"id": "ch_...", ...}   charged
//	  200 {"id": "ch_...", ...}   already charged under this idempotency key
//	  402                         declined: not enough allowance
//	  5xx, or no answer           failed: it may or may not have charged
//	GET  /charges/{idempotency_key}
//	  200 {"id": ...}, or 404 if there's no charge under that key
//	POST /charges/{id}/refund
//	  200 {"id": ..., "refunded": true}
//
// An idempotency key is optional, but without one a repeated request is a
// second charge.
package bank

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrDeclined: the bank refused the charge (not enough allowance).
var ErrDeclined = errors.New("bank: declined")

// Client is the bank's HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient returns a client whose requests give up after timeout.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: timeout}}
}

// ChargeRequest is a payment to take.
type ChargeRequest struct {
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Account        int64  `json:"account"`
	AmountCents    int64  `json:"amount_cents"`
}

// Charge is a payment the bank took.
type Charge struct {
	ID          string `json:"id"`
	Account     int64  `json:"account"`
	AmountCents int64  `json:"amount_cents"`
	Refunded    bool   `json:"refunded"`
}

// CreateCharge takes a payment: one request, no retries.
func (c *Client) CreateCharge(ctx context.Context, req ChargeRequest) (Charge, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Charge{}, err
	}
	var ch Charge
	status, err := c.do(ctx, http.MethodPost, "/charges", body, &ch)
	switch {
	case err != nil:
		return Charge{}, err
	case status == http.StatusPaymentRequired:
		return Charge{}, ErrDeclined
	case status != http.StatusCreated && status != http.StatusOK:
		return Charge{}, fmt.Errorf("bank: charge failed: HTTP %d", status)
	}
	return ch, nil
}

// LookupCharge finds the charge taken under an idempotency key; ok is
// false if there is none.
func (c *Client) LookupCharge(ctx context.Context, idempotencyKey string) (ch Charge, ok bool, err error) {
	status, err := c.do(ctx, http.MethodGet, "/charges/"+idempotencyKey, nil, &ch)
	switch {
	case err != nil:
		return Charge{}, false, err
	case status == http.StatusNotFound:
		return Charge{}, false, nil
	case status != http.StatusOK:
		return Charge{}, false, fmt.Errorf("bank: lookup failed: HTTP %d", status)
	}
	return ch, true, nil
}

// Refund gives a charge back.
func (c *Client) Refund(ctx context.Context, chargeID string) error {
	status, err := c.do(ctx, http.MethodPost, "/charges/"+chargeID+"/refund", nil, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("bank: refund failed: HTTP %d", status)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("bank: %w", err)
	}
	defer res.Body.Close()
	if out != nil && (res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated) {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return res.StatusCode, fmt.Errorf("bank: reading the response: %w", err)
		}
	}
	return res.StatusCode, nil
}
