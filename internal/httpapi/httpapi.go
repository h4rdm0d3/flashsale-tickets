// Package httpapi is the service's HTTP transport: it decodes requests,
// calls the booking service, and turns its answers into responses. No
// business rules live here.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/h4rdm0d3/flashsale-tickets/internal/booking"
)

// Booker sells tickets (booking.Service).
type Booker interface {
	Book(ctx context.Context, eventID, buyer int64) (booking.Ticket, error)
}

// Pinger reports whether a dependency (the database) answers.
type Pinger interface {
	Ping(ctx context.Context) error
}

// New returns the service's routes. Each booking may take requestTimeout.
func New(booker Booker, health Pinger, requestTimeout time.Duration, log *slog.Logger) http.Handler {
	h := &handler{booker: booker, health: health, timeout: requestTimeout, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events/{id}/bookings", h.book)
	mux.HandleFunc("GET /healthz", h.healthz)
	return mux
}

type handler struct {
	booker  Booker
	health  Pinger
	timeout time.Duration
	log     *slog.Logger
}

type bookRequest struct {
	Buyer int64 `json:"buyer"`
}

func (h *handler) book(w http.ResponseWriter, r *http.Request) {
	eventID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req bookRequest
	if err != nil || json.NewDecoder(r.Body).Decode(&req) != nil || req.Buyer <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	ticket, err := h.booker.Book(ctx, eventID, req.Buyer)
	if err != nil {
		status, code := statusOf(err)
		if status == http.StatusInternalServerError {
			h.log.ErrorContext(ctx, "booking failed", "event", eventID, "buyer", req.Buyer, "err", err)
		}
		writeError(w, status, code)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"ticket": ticket.ID})
}

// statusOf maps the booking domain's outcomes to HTTP.
func statusOf(err error) (int, string) {
	switch {
	case errors.Is(err, booking.ErrSoldOut):
		return http.StatusConflict, "sold_out"
	case errors.Is(err, booking.ErrLimitReached):
		return http.StatusConflict, "limit_reached"
	case errors.Is(err, booking.ErrDeclined):
		return http.StatusPaymentRequired, "declined"
	case errors.Is(err, booking.ErrNotOnSale):
		return http.StatusNotFound, "not_on_sale"
	}
	return http.StatusInternalServerError, "internal"
}

func (h *handler) healthz(w http.ResponseWriter, r *http.Request) {
	if err := h.health.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
