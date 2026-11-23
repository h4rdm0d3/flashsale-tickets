# tickets

The booking service of the [rankeddebugging](https://rankeddebugging.com)
flash-sale course. Buyers book seats over HTTP. The service records each
ticket in Postgres and charges the buyer through the bank.

This is your copy to change. The lab runs whatever image you point it at.

## The API

The campaign, a crowd of buyers, calls one endpoint:

```
POST /events/{id}/bookings   {"buyer": 42}
  201 {"ticket": 7}                booked and paid
  409 {"error": "sold_out"}        no seats left
  409 {"error": "limit_reached"}   the buyer has the most tickets allowed
  402 {"error": "declined"}        the bank refused the payment
  404 {"error": "not_on_sale"}
  500                              anything else
GET /healthz                       200 once the database answers
```

Keep these answers. The campaign scores the sale by them.

## The database

```sql
events   (id, name, capacity, price_cents, on_sale)
bookings (id, event_id, buyer, charge_id, booked_at)
```

You can add tables, columns, indexes or functions from the lab's console
with `psql`. Changes you make there aren't in this repo, so keep them in
`migrations/` too.

## The bank

A third party. Its state is its own, and it's slow and sometimes fails.
See the comment at the top of `bank/client.go` for its API.

## Configuration

| Variable | What it is |
|---|---|
| `PGHOST`, `PGUSER`, `PGPASSWORD`, `PGDATABASE` | Postgres |
| `LISTEN_ADDR` | where the service listens, e.g. `:8080` |
| `BANK_URL` | the bank's address |
| `BANK_TIMEOUT` | how long to wait for the bank per request |
| `REQUEST_TIMEOUT` | how long a booking may take |
| `TICKETS_PER_BUYER` | the most tickets one buyer may hold per event |

The lab sets all of them.

## Deploying to the lab

1. Make your own copy: **Use this template** on GitHub, or clone it and
   push it to a repo of yours.
2. Push to `main`. The `image` workflow builds the service and pushes
   `ghcr.io/<you>/<repo>:<commit>` to your GitHub Container Registry. The
   run's summary shows the exact name.
3. Make the package public once (the package's settings on GitHub), so the
   lab can pull it.
4. In the lab's console, put the image in `~/values.yaml` (see
   `values.yaml.example`) and run `deploy`.

`deploy` waits until the new version answers `/healthz`. If it never does,
the old version keeps serving and the new one's logs say why.

## Building and testing

```
go build ./...
go test ./...
```
