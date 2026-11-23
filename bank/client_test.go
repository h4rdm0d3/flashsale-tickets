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
)

func TestChargeSendsTheRequestAndReadsTheCharge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChargeRequest
		if r.URL.Path != "/charges" || json.NewDecoder(r.Body).Decode(&req) != nil || req.Account != 42 || req.IdempotencyKey != "booking-7" {
			t.Errorf("unexpected request %s %+v", r.URL.Path, req)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Charge{ID: "ch_1", Account: req.Account, AmountCents: req.AmountCents})
	}))
	defer srv.Close()
	ch, err := New(srv.URL, time.Second).Charge(context.Background(), ChargeRequest{IdempotencyKey: "booking-7", Account: 42, AmountCents: 4500})
	if err != nil || ch.ID != "ch_1" || ch.AmountCents != 4500 {
		t.Fatalf("got %+v, %v", ch, err)
	}
}

func TestChargeIsDeclined(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer srv.Close()
	if _, err := New(srv.URL, time.Second).Charge(context.Background(), ChargeRequest{Account: 1, AmountCents: 1}); !errors.Is(err, ErrDeclined) {
		t.Fatalf("err = %v, want ErrDeclined", err)
	}
}

func TestChargeRetriesWhenTheBankFails(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if _, err := New(srv.URL, time.Second).Charge(context.Background(), ChargeRequest{Account: 1, AmountCents: 1}); err == nil {
		t.Fatal("expected an error")
	}
	if n := calls.Load(); n != chargeAttempts {
		t.Fatalf("%d attempts, want %d", n, chargeAttempts)
	}
}
