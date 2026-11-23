// Command tickets is the booking service of the flash-sale lab: buyers book
// seats over HTTP, the service records them in Postgres and charges the
// buyer through the bank.
//
//	POST /events/{id}/bookings  {"buyer": 42}
//	  201 {"ticket": 7}                 booked and paid
//	  409 {"error": "sold_out"}         no seats left
//	  409 {"error": "limit_reached"}    the buyer has the most tickets allowed
//	  402 {"error": "declined"}         the bank refused the payment
//	  404 {"error": "not_on_sale"}
//	  500 {"error": "..."}              anything else
//	GET /healthz                        200 once the database answers
//
// Configuration is the environment: the standard PG* variables for
// Postgres, and the variables in config below.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/h4rdm0d3/flashsale-tickets/bank"
)

type config struct {
	listenAddr      string
	bankURL         string
	bankTimeout     time.Duration
	requestTimeout  time.Duration
	ticketsPerBuyer int
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	cfg, err := load()
	if err != nil {
		log.Fatalf("tickets: %v", err)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, "") // PG* environment variables
	if err != nil {
		log.Fatalf("tickets: %v", err)
	}
	b := &Booker{
		db:              db,
		bank:            bank.New(cfg.bankURL, cfg.bankTimeout),
		ticketsPerBuyer: cfg.ticketsPerBuyer,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /events/{id}/bookings", func(w http.ResponseWriter, r *http.Request) {
		eventID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		var req struct {
			Buyer int64 `json:"buyer"`
		}
		if err != nil || json.NewDecoder(r.Body).Decode(&req) != nil || req.Buyer <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad_request"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), cfg.requestTimeout)
		defer cancel()
		ticket, err := b.Book(ctx, eventID, req.Buyer)
		switch {
		case err == nil:
			writeJSON(w, http.StatusCreated, map[string]int64{"ticket": ticket})
		case errors.Is(err, ErrSoldOut):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "sold_out"})
		case errors.Is(err, ErrLimitReached):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "limit_reached"})
		case errors.Is(err, bank.ErrDeclined):
			writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "declined"})
		case errors.Is(err, ErrNotOnSale):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_on_sale"})
		default:
			log.Printf("booking event %d for buyer %d: %v", eventID, req.Buyer, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})

	log.Printf("tickets listening on %s", cfg.listenAddr)
	log.Fatal(http.ListenAndServe(cfg.listenAddr, mux))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func load() (config, error) {
	var c config
	var errs []error
	str := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
		return v
	}
	dur := func(name string) time.Duration {
		d, err := time.ParseDuration(os.Getenv(name))
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s: a positive duration is required", name))
		}
		return d
	}
	c.listenAddr = str("LISTEN_ADDR")
	c.bankURL = str("BANK_URL")
	c.bankTimeout = dur("BANK_TIMEOUT")
	c.requestTimeout = dur("REQUEST_TIMEOUT")
	n, err := strconv.Atoi(os.Getenv("TICKETS_PER_BUYER"))
	if err != nil || n <= 0 {
		errs = append(errs, errors.New("TICKETS_PER_BUYER: a positive integer is required"))
	}
	c.ticketsPerBuyer = n
	return c, errors.Join(errs...)
}
