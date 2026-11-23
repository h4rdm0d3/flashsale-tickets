-- The schema the lab creates before the sale. For reference: the lab
-- applies it, not this file. Put your own changes in later files here.

create table events (
	id bigint generated always as identity primary key,
	name text not null,
	capacity integer not null check (capacity > 0),
	price_cents bigint not null check (price_cents > 0),
	on_sale boolean not null default false
);

create table bookings (
	id bigint generated always as identity primary key,
	event_id bigint not null references events (id),
	buyer bigint not null,
	-- The bank's charge for this ticket, once paid.
	charge_id text,
	booked_at timestamptz not null default now()
);
create index bookings_event_id on bookings (event_id);
