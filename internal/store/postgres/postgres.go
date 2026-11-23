// Package postgres is the booking Store, in Postgres.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/h4rdm0d3/flashsale-tickets/internal/booking"
)

// Store implements booking.Store.
type Store struct {
	db *pgxpool.Pool
}

// New returns a Store on a connection pool.
func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

var _ booking.Store = (*Store)(nil)

func (s *Store) EventOnSale(ctx context.Context, eventID int64) (booking.Event, error) {
	e := booking.Event{ID: eventID}
	err := s.db.QueryRow(ctx,
		`select capacity, price_cents from events where id = $1 and on_sale`, eventID,
	).Scan(&e.Capacity, &e.PriceCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return booking.Event{}, booking.ErrNotOnSale
	}
	return e, err
}

func (s *Store) SoldCount(ctx context.Context, eventID int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(ctx, `select count(*) from bookings where event_id = $1`, eventID).Scan(&n)
	return n, err
}

func (s *Store) BuyerCount(ctx context.Context, eventID, buyer int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(ctx,
		`select count(*) from bookings where event_id = $1 and buyer = $2`, eventID, buyer,
	).Scan(&n)
	return n, err
}

func (s *Store) CreateTicket(ctx context.Context, eventID, buyer int64, chargeID string) (booking.Ticket, error) {
	t := booking.Ticket{EventID: eventID, Buyer: buyer}
	err := s.db.QueryRow(ctx,
		`insert into bookings (event_id, buyer, charge_id) values ($1, $2, $3) returning id`, eventID, buyer, chargeID,
	).Scan(&t.ID)
	return t, err
}

// Ping reports whether the database answers.
func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }
