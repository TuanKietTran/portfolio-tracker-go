# Portfolio Tracker (CQRS)

Synthetic investment accounting in Go: FIFO/average-cost domain lots, realized/unrealized P&L, deterministic price marks and local cash. No real money or credentials.

## Run and verify

```sh
GOTOOLCHAIN=local go run ./cmd/server  # localhost:8105
GOTOOLCHAIN=local go test -p 2 ./... -race -count=1 -cover
```

Production trade routes share one accounting store. `POST /v1/portfolios/{id}/trades` accepts `symbol`, `side` (BUY/SELL), decimal-string `quantity` and `price`, plus `Idempotency-Key` or body `idempotency_key`. Same-payload replay is 201; changed payload is 409. Position trades use `/positions/{symbol}/trades`. Both reject missing keys and oversells.

`GET /v1/portfolios/{id}/summary?currency=USD&as_of=100&max_age=0` and `/positions/{symbol}` use explicit Unix-second cutoff and maximum price age. Marks are injected through the Go price-feed port; the server currently has no mark-seeding route/config. Missing/stale marks fail rather than inventing zero values. Summary `TotalValue` is securities only, not cash-inclusive NAV. See [`api/openapi.yaml`](api/openapi.yaml).

## Measured status

On 2026-10-18, macOS arm64, Go 1.27.1: build/vet/race gates passed; domain coverage 92.5%, weighted non-cmd coverage **63.55% (319/502 statements)**. Coverage remains below the required 80% overall bar. Actual server-router regression: `TestRuntimeRouterSharesTradePositionAndMarkedSummary`. The original in-process 1,000-request playback still passes against the legacy adapter; it does not prove the production router or a real-process 10,000-request target.

Incomplete: trade/cash linkage and currency binding, instrument/price management, corporate-action/returns HTTP, payment-backed funding/outbox, Postgres parity, operational hardening and complete production documentation. Local cash is an in-memory simulation; no durable recovery or integrated S3 claim is made.
