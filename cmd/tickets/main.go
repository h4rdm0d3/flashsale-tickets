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
	"github.com/h4rdm0d3/flashsale-tickets/internal/profiling"
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

	dbConfig, err := pgxpool.ParseConfig("") // the standard PG* variables
	if err != nil {
		return err
	}
	if cfg.DBMaxConns > 0 {
		dbConfig.MaxConns = cfg.DBMaxConns
	}
	db, err := pgxpool.NewWithConfig(ctx, dbConfig)
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
	// The profiles, on a port of their own (the lab's profiler reads them).
	prof := &http.Server{Addr: cfg.PprofAddr, Handler: profiling.Handler(), ReadHeaderTimeout: cfg.RequestTimeout}
	errc := make(chan error, 2)
	go func() {
		log.Info("tickets listening", "addr", cfg.ListenAddr)
		errc <- srv.ListenAndServe()
	}()
	go func() {
		log.Info("profiles listening", "addr", cfg.PprofAddr)
		errc <- prof.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	// Profiles first: they can't hold up requests that are finishing.
	_ = prof.Close()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
