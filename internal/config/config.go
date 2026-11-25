// Package config reads the service's settings from the environment, all in
// one place, and refuses to start on a missing or bad one.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the service's settings. Postgres is configured separately, by
// the standard PG* variables.
type Config struct {
	// ListenAddr is where the HTTP server listens, e.g. ":8080".
	ListenAddr string
	// PprofAddr is where the Go runtime's profiles are served, e.g. ":6060",
	// for the lab's profiler. Keep it off the public port.
	PprofAddr string
	// BankURL is the bank's address; BankTimeout bounds each request to it,
	// and BankAttempts is how many times a charge is tried.
	BankURL      string
	BankTimeout  time.Duration
	BankAttempts int
	// RequestTimeout bounds one booking, start to finish.
	RequestTimeout time.Duration
	// ShutdownTimeout is how long in-flight requests get to finish when the
	// service is told to stop (as it is during every deploy).
	ShutdownTimeout time.Duration
	// TicketsPerBuyer is the most tickets one buyer may hold per event.
	TicketsPerBuyer int64
}

// FromEnv reads the Config from the environment.
func FromEnv() (Config, error) {
	r := reader{}
	c := Config{
		ListenAddr:      r.str("LISTEN_ADDR"),
		PprofAddr:       r.str("PPROF_ADDR"),
		BankURL:         r.str("BANK_URL"),
		BankTimeout:     r.duration("BANK_TIMEOUT"),
		BankAttempts:    int(r.positive("BANK_ATTEMPTS")),
		RequestTimeout:  r.duration("REQUEST_TIMEOUT"),
		ShutdownTimeout: r.duration("SHUTDOWN_TIMEOUT"),
		TicketsPerBuyer: r.positive("TICKETS_PER_BUYER"),
	}
	return c, errors.Join(r.errs...)
}

// reader collects every problem, so one start reports them all.
type reader struct{ errs []error }

func (r *reader) str(name string) string {
	v := os.Getenv(name)
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("%s is required", name))
	}
	return v
}

func (r *reader) duration(name string) time.Duration {
	d, err := time.ParseDuration(os.Getenv(name))
	if err != nil || d <= 0 {
		r.errs = append(r.errs, fmt.Errorf("%s: a positive duration is required, e.g. 2s", name))
	}
	return d
}

func (r *reader) positive(name string) int64 {
	n, err := strconv.ParseInt(os.Getenv(name), 10, 64)
	if err != nil || n <= 0 {
		r.errs = append(r.errs, fmt.Errorf("%s: a positive integer is required", name))
	}
	return n
}
