package bank

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/h4rdm0d3/flashsale-tickets/internal/booking"
)

func server(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, time.Second)
}

func TestCreateChargeSendsTheRequestAndReadsTheCharge(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		var req ChargeRequest
		if r.Method != http.MethodPost || r.URL.Path != "/charges" || json.NewDecoder(r.Body).Decode(&req) != nil || req.IdempotencyKey != "k1" || req.Account != 42 {
			t.Errorf("unexpected request %s %s %+v", r.Method, r.URL.Path, req)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Charge{ID: "ch_1", Account: req.Account, AmountCents: req.AmountCents})
	})
	ch, err := c.CreateCharge(context.Background(), ChargeRequest{IdempotencyKey: "k1", Account: 42, AmountCents: 4500})
	if err != nil || ch.ID != "ch_1" || ch.AmountCents != 4500 {
		t.Fatalf("got %+v, %v", ch, err)
	}
}

func TestLookupChargeNotFound(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	if _, ok, err := c.LookupCharge(context.Background(), "k1"); ok || err != nil {
		t.Fatalf("ok = %v, err = %v; want a clean miss", ok, err)
	}
}

func TestPaymentsMapsADeclineToTheDomain(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusPaymentRequired) })
	if _, err := NewPayments(c, 1).Charge(context.Background(), booking.Payment{Buyer: 1, AmountCents: 1}); !errors.Is(err, booking.ErrDeclined) {
		t.Fatalf("err = %v, want booking.ErrDeclined", err)
	}
}

func TestPaymentsTriesAgainWhenTheBankFails(t *testing.T) {
	const attempts = 3
	var calls atomic.Int32
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	})
	if _, err := NewPayments(c, attempts).Charge(context.Background(), booking.Payment{Buyer: 1, AmountCents: 1}); err == nil {
		t.Fatal("expected an error")
	}
	if n := calls.Load(); n != attempts {
		t.Fatalf("%d attempts, want %d", n, attempts)
	}
}
