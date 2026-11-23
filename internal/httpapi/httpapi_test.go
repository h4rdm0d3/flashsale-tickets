package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/h4rdm0d3/flashsale-tickets/internal/booking"
)

type fakeBooker struct{ err error }

func (f fakeBooker) Book(_ context.Context, eventID, buyer int64) (booking.Ticket, error) {
	if f.err != nil {
		return booking.Ticket{}, f.err
	}
	return booking.Ticket{ID: 7, EventID: eventID, Buyer: buyer}, nil
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func call(t *testing.T, h http.Handler, method, path, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec.Code, strings.TrimSpace(rec.Body.String())
}

func TestBookAnswers(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	cases := map[string]struct {
		err      error
		body     string
		status   int
		response string
	}{
		"booked":        {nil, `{"buyer": 42}`, http.StatusCreated, `{"ticket":7}`},
		"sold out":      {booking.ErrSoldOut, `{"buyer": 42}`, http.StatusConflict, `{"error":"sold_out"}`},
		"limit reached": {booking.ErrLimitReached, `{"buyer": 42}`, http.StatusConflict, `{"error":"limit_reached"}`},
		"declined":      {booking.ErrDeclined, `{"buyer": 42}`, http.StatusPaymentRequired, `{"error":"declined"}`},
		"not on sale":   {booking.ErrNotOnSale, `{"buyer": 42}`, http.StatusNotFound, `{"error":"not_on_sale"}`},
		"broken":        {errors.New("database down"), `{"buyer": 42}`, http.StatusInternalServerError, `{"error":"internal"}`},
		"no buyer":      {nil, `{}`, http.StatusBadRequest, `{"error":"bad_request"}`},
	}
	for name, c := range cases {
		h := New(fakeBooker{err: c.err}, fakePinger{}, time.Second, quiet)
		status, body := call(t, h, http.MethodPost, "/events/1/bookings", c.body)
		if status != c.status || body != c.response {
			t.Errorf("%s: %d %s, want %d %s", name, status, body, c.status, c.response)
		}
	}
}

func TestHealthzFollowsTheDatabase(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if status, _ := call(t, New(fakeBooker{}, fakePinger{}, time.Second, quiet), http.MethodGet, "/healthz", ""); status != http.StatusOK {
		t.Errorf("healthy: %d", status)
	}
	if status, _ := call(t, New(fakeBooker{}, fakePinger{err: errors.New("down")}, time.Second, quiet), http.MethodGet, "/healthz", ""); status != http.StatusServiceUnavailable {
		t.Errorf("database down: %d", status)
	}
}
