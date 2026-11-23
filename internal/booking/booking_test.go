package booking

import (
	"context"
	"errors"
	"testing"
)

// memStore is an in-memory Store for tests.
type memStore struct {
	events  map[int64]Event
	tickets []Ticket
	paid    map[int64]string
}

func newMemStore(events ...Event) *memStore {
	s := &memStore{events: map[int64]Event{}, paid: map[int64]string{}}
	for _, e := range events {
		s.events[e.ID] = e
	}
	return s
}

func (s *memStore) EventOnSale(_ context.Context, id int64) (Event, error) {
	e, ok := s.events[id]
	if !ok {
		return Event{}, ErrNotOnSale
	}
	return e, nil
}

func (s *memStore) SoldCount(_ context.Context, eventID int64) (int64, error) {
	var n int64
	for _, t := range s.tickets {
		if t.EventID == eventID {
			n++
		}
	}
	return n, nil
}

func (s *memStore) BuyerCount(_ context.Context, eventID, buyer int64) (int64, error) {
	var n int64
	for _, t := range s.tickets {
		if t.EventID == eventID && t.Buyer == buyer {
			n++
		}
	}
	return n, nil
}

func (s *memStore) CreateTicket(_ context.Context, eventID, buyer int64, chargeID string) (Ticket, error) {
	t := Ticket{ID: int64(len(s.tickets) + 1), EventID: eventID, Buyer: buyer}
	s.tickets = append(s.tickets, t)
	s.paid[t.ID] = chargeID
	return t, nil
}

// fakePayments approves every charge unless told to decline.
type fakePayments struct {
	decline bool
	charges []Payment
}

func (p *fakePayments) Charge(_ context.Context, pay Payment) (string, error) {
	if p.decline {
		return "", ErrDeclined
	}
	p.charges = append(p.charges, pay)
	return "ch_1", nil
}

const ticketsPerBuyer = 2

func TestBookSellsAndCharges(t *testing.T) {
	store := newMemStore(Event{ID: 1, Capacity: 10, PriceCents: 4500})
	pay := &fakePayments{}
	ticket, err := NewService(store, pay, ticketsPerBuyer).Book(context.Background(), 1, 42)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.EventID != 1 || ticket.Buyer != 42 {
		t.Errorf("ticket = %+v", ticket)
	}
	if len(pay.charges) != 1 || pay.charges[0] != (Payment{Buyer: 42, AmountCents: 4500}) {
		t.Errorf("charges = %+v", pay.charges)
	}
	if store.paid[ticket.ID] != "ch_1" {
		t.Errorf("the ticket wasn't recorded with its charge: %v", store.paid)
	}
}

func TestBookRefusals(t *testing.T) {
	cases := map[string]struct {
		store *memStore
		pay   *fakePayments
		event int64
		want  error
	}{
		"not on sale": {newMemStore(), &fakePayments{}, 1, ErrNotOnSale},
		"sold out": {func() *memStore {
			s := newMemStore(Event{ID: 1, Capacity: 1, PriceCents: 1})
			s.tickets = []Ticket{{ID: 1, EventID: 1, Buyer: 7}}
			return s
		}(), &fakePayments{}, 1, ErrSoldOut},
		"limit reached": {func() *memStore {
			s := newMemStore(Event{ID: 1, Capacity: 10, PriceCents: 1})
			s.tickets = []Ticket{{ID: 1, EventID: 1, Buyer: 42}, {ID: 2, EventID: 1, Buyer: 42}}
			return s
		}(), &fakePayments{}, 1, ErrLimitReached},
		"declined": {newMemStore(Event{ID: 1, Capacity: 10, PriceCents: 1}), &fakePayments{decline: true}, 1, ErrDeclined},
	}
	for name, c := range cases {
		_, err := NewService(c.store, c.pay, ticketsPerBuyer).Book(context.Background(), c.event, 42)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}
