package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/h4rdm0d3/flashsale-tickets/bank"
)

// Outcomes of a booking that aren't failures of the service.
var (
	ErrSoldOut      = errors.New("sold out")
	ErrLimitReached = errors.New("buyer limit reached")
	ErrNotOnSale    = errors.New("event not on sale")
)

// Booker books seats: one ticket per call, paid through the bank.
type Booker struct {
	db              *pgxpool.Pool
	bank            *bank.Client
	ticketsPerBuyer int
}

// Book sells one seat of an event to a buyer and charges them for it.
func (b *Booker) Book(ctx context.Context, eventID, buyer int64) (int64, error) {
	var capacity, priceCents int64
	err := b.db.QueryRow(ctx,
		`select capacity, price_cents from events where id = $1 and on_sale`, eventID,
	).Scan(&capacity, &priceCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotOnSale
	}
	if err != nil {
		return 0, fmt.Errorf("reading the event: %w", err)
	}

	// Any seats left?
	var sold int64
	if err := b.db.QueryRow(ctx,
		`select count(*) from bookings where event_id = $1`, eventID,
	).Scan(&sold); err != nil {
		return 0, fmt.Errorf("counting bookings: %w", err)
	}
	if sold >= capacity {
		return 0, ErrSoldOut
	}

	// Under the buyer's limit?
	var mine int64
	if err := b.db.QueryRow(ctx,
		`select count(*) from bookings where event_id = $1 and buyer = $2`, eventID, buyer,
	).Scan(&mine); err != nil {
		return 0, fmt.Errorf("counting the buyer's bookings: %w", err)
	}
	if mine >= int64(b.ticketsPerBuyer) {
		return 0, ErrLimitReached
	}

	var ticket int64
	if err := b.db.QueryRow(ctx,
		`insert into bookings (event_id, buyer) values ($1, $2) returning id`, eventID, buyer,
	).Scan(&ticket); err != nil {
		return 0, fmt.Errorf("recording the booking: %w", err)
	}

	// Take the payment.
	charge, err := b.bank.Charge(ctx, bank.ChargeRequest{Account: buyer, AmountCents: priceCents})
	if err != nil {
		return 0, fmt.Errorf("charging the buyer: %w", err)
	}
	if _, err := b.db.Exec(ctx,
		`update bookings set charge_id = $1 where id = $2`, charge.ID, ticket,
	); err != nil {
		return 0, fmt.Errorf("recording the payment: %w", err)
	}
	return ticket, nil
}
