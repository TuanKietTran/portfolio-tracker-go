# Portfolio tracker

CQRS portfolio simulation. Production named/position trades share exact FIFO accounting state, and explicit as-of summary queries compute marked security value and P&L. The older adapter and playback are retained as historical regression fixtures, not server wiring. Cash and trades commit atomically in local mode. Explicit payments mode confirms a stable, idempotent payment intent and reconciliation before local cash changes; portfolio-side restart durability remains unavailable.

## Stack

Go module1.22; historical coverage evidence used Go1.27.1 macOS arm64; current gates use Go1.27.2 Windows/amd64. Chi, `shopspring/decimal`, Google UUID. In-memory only; unsupported database configuration fails startup. HTTP defaults 127.0.0.1:8105 with selectable FIFO/average accounting, bounded lifecycle, local default cash and optional loopback payments mode; readiness reports local initialization only.

## Public API

- `POST /v1/portfolios/{id}/trades`
- `GET /v1/portfolios/{id}/summary`
- `POST /v1/portfolios` and `GET /v1/portfolios/{id}` (immutable explicit local metadata)

## Out of scope until implemented

- Generalized zero-capital/multiple-IRR cases and raw-history HTTP.
- External payment-backed funding and durable persistence.
- Durable portfolio rows and audited migration of legacy UUID state. Explicit metadata now exists in memory; legacy arbitrary-UUID writes stay unregistered and lookup404. `migrations/001_init.sql` is not used by the server.

## Definition of done

See `openspec/dod.md`. The playback storyboard is `openspec/gifs/playback-flow.gif`.
