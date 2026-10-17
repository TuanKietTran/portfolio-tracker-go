# Portfolio tracker

CQRS portfolio. Callers record trades and read positions. Average cost and realized P&L are updated on the trade. Summary market value and unrealized P&L are always zero.

## Stack

Go 1.22, Chi, `shopspring/decimal`, Google UUID. In-memory only.

## Public API

- `POST /v1/portfolios/{id}/trades`
- `GET /v1/portfolios/{id}/summary`

## Checkpoints

| Date | Change | Evidence |
| --- | --- | --- |
| 2026-10-17 | Baseline spec recorded from the shipped service | `08b5e3c`. No handler tests and no Postgres repository yet. |

## Out of scope until implemented

- `GetPositions` over HTTP. Positions are nested in the summary.
- Mark-to-market `TotalValue` and `UnrealizedPnL`. Both are fixed at 0.
- A portfolio row. Any UUID is accepted. `migrations/001_init.sql` is not used by the server.
