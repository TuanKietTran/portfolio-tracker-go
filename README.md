# Portfolio Tracker (CQRS)

Synthetic investment accounting in Go: FIFO/average-cost domain lots, realized/unrealized P&L, deterministic price marks and local cash. No real money or credentials.

## Run and verify

```sh
GOTOOLCHAIN=local go run ./cmd/server  # localhost:8105
GOTOOLCHAIN=local go test -p 2 ./... -race -count=1 -cover
```

Production trade routes share one accounting store. `POST /v1/portfolios/{id}/trades` accepts `symbol`, `side` (BUY/SELL), decimal-string `quantity` and `price`, explicit `currency` (USD/EUR/GBP/JPY), plus `Idempotency-Key` or body `idempotency_key`. Same-payload replay is 201; changed payload is 409. Position trades use `/positions/{symbol}/trades`. Both reject missing keys and oversells. Buys atomically debit local cash and sells credit it; insufficient funds return 409. Fund the local simulation with `/cash/fund` first. Settlement rejects fractional minor units rather than rounding; see [ADR 0001](docs/adr/0001-exact-local-settlement.md).

`GET /v1/portfolios/{id}/summary?currency=USD&as_of=100&max_age=0` and `/positions/{symbol}` use explicit Unix-second cutoff and maximum price age. Marks are injected through the Go price-feed port; the server currently has no mark-seeding route/config. Missing/stale marks fail rather than inventing zero values. Summary `TotalValue` is securities only, not cash-inclusive NAV. See [`api/openapi.yaml`](api/openapi.yaml).

## Measured status

On 2026-10-18, macOS arm64, Go 1.27.1: build/vet/race gates passed; domain coverage 92.6%, weighted non-cmd coverage **67.98% (380/559 statements)**. Coverage remains below the required 80% overall bar. Actual server-router regression: `TestRuntimeRouterSharesTradePositionAndMarkedSummary`. The original in-process 1,000-request playback still passes against the legacy adapter; it does not prove the production router or a real-process 10,000-request target.

Incomplete: instrument/price management, corporate-action/returns HTTP, payment-backed funding/outbox, Postgres parity, operational hardening and complete production documentation. Instrument denominations currently bind on the first successful trade; no instrument registry API exists yet. Repeating average-cost/split residuals still need dedicated conservation regressions. Local cash is an in-memory simulation; no durable recovery or integrated S3 claim is made.
