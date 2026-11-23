// Package booking is the domain of the tickets service: selling a seat of
// an event to a buyer, and taking their payment for it.
//
// It knows nothing about HTTP or SQL. It needs two things, described by
// the interfaces below: somewhere to keep events and tickets (Store), and a
// way to take money (Payments). main wires real ones in; tests use fakes.
package booking

import (
	"context"
	"errors"
	"fmt"
)

// Outcomes a buyer can be told about. Anything else is a failure of the
// service.
var (
	ErrNotOnSale    = errors.New("booking: event not on sale")
	ErrSoldOut      = errors.New("booking: sold out")
	ErrLimitReached = errors.New("booking: buyer limit reached")
	ErrDeclined     = errors.New("booking: payment declined")
)

// Event is an event on sale.
type Event struct {
	ID         int64
	Capacity   int64
	PriceCents int64
}

// Ticket is a seat sold to a buyer.
type Ticket struct {
	ID      int64
	EventID int64
	Buyer   int64
}

// Store keeps events and tickets.
type Store interface {
	// EventOnSale returns the event, or ErrNotOnSale.
	EventOnSale(ctx context.Context, eventID int64) (Event, error)
	// SoldCount is how many tickets the event has sold.
	SoldCount(ctx context.Context, eventID int64) (int64, error)
	// BuyerCount is how many tickets of the event the buyer holds.
	BuyerCount(ctx context.Context, eventID, buyer int64) (int64, error)
	// CreateTicket records a ticket, paid for by a charge.
	CreateTicket(ctx context.Context, eventID, buyer int64, chargeID string) (Ticket, error)
}

// Payment is a request to take money from a buyer.
type Payment struct {
	Buyer       int64
	AmountCents int64
}

// Payments takes money. Charge returns the payment's ID, or ErrDeclined.
type Payments interface {
	Charge(ctx context.Context, p Payment) (chargeID string, err error)
}

// Service sells tickets.
type Service struct {
	store           Store
	payments        Payments
	ticketsPerBuyer int64
}

// NewService returns a Service. ticketsPerBuyer is the most tickets one
// buyer may hold per event.
func NewService(store Store, payments Payments, ticketsPerBuyer int64) *Service {
	return &Service{store: store, payments: payments, ticketsPerBuyer: ticketsPerBuyer}
}

// Book sells one seat of an event to a buyer and charges them for it.
func (s *Service) Book(ctx context.Context, eventID, buyer int64) (Ticket, error) {
	event, err := s.store.EventOnSale(ctx, eventID)
	if err != nil {
		return Ticket{}, err
	}

	sold, err := s.store.SoldCount(ctx, eventID)
	if err != nil {
		return Ticket{}, fmt.Errorf("counting tickets sold: %w", err)
	}
	if sold >= event.Capacity {
		return Ticket{}, ErrSoldOut
	}

	held, err := s.store.BuyerCount(ctx, eventID, buyer)
	if err != nil {
		return Ticket{}, fmt.Errorf("counting the buyer's tickets: %w", err)
	}
	if held >= s.ticketsPerBuyer {
		return Ticket{}, ErrLimitReached
	}

	// Take the payment, then record the ticket it paid for.
	chargeID, err := s.payments.Charge(ctx, Payment{Buyer: buyer, AmountCents: event.PriceCents})
	if err != nil {
		return Ticket{}, fmt.Errorf("charging the buyer: %w", err)
	}
	ticket, err := s.store.CreateTicket(ctx, eventID, buyer, chargeID)
	if err != nil {
		return Ticket{}, fmt.Errorf("recording the ticket: %w", err)
	}
	return ticket, nil
}
