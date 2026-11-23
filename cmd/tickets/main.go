// Command tickets is the booking service of the flash-sale lab. main only
// wires the parts together; the logic lives in internal/booking.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/h4rdm0d3/flashsale-tickets/internal/bank"
	"github.com/h4rdm0d3/flashsale-tickets/internal/booking"
	"github.com/h4rdm0d3/flashsale-tickets/internal/config"
	"github.com/h4rdm0d3/flashsale-tickets/internal/httpapi"
	"github.com/h4rdm0d3/flashsale-tickets/internal/store/postgres"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("tickets stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	// Stop on SIGTERM (a deploy) or Ctrl-C, letting requests finish.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, "") // the standard PG* variables
	if err != nil {
		return err
	}
	defer db.Close()

	store := postgres.New(db)
	payments := bank.NewPayments(bank.NewClient(cfg.BankURL, cfg.BankTimeout), cfg.BankAttempts)
	service := booking.NewService(store, payments, cfg.TicketsPerBuyer)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           httpapi.New(service, store, cfg.RequestTimeout, log),
		ReadHeaderTimeout: cfg.RequestTimeout,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("tickets listening", "addr", cfg.ListenAddr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
