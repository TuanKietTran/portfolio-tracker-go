# Portfolio Tracker (CQRS)

Synthetic investment accounting in Go: FIFO/average-cost domain lots, realized/unrealized P&L, deterministic price marks and local cash. No real money or credentials.

## Run and verify

```sh
GOTOOLCHAIN=local HTTP_ADDR=127.0.0.1:8105 CASH_MODE=local go run ./cmd/server
curl --fail http://127.0.0.1:8105/readyz
GOTOOLCHAIN=local go test -p 2 ./... -race -count=1 -cover
```

Configuration: `HTTP_ADDR` defaults to `:8105`; `CASH_MODE` supports only `local`; `COST_METHOD` supports `FIFO` (default) and `AVERAGE_COST`. Unsupported `PAYMENTS_URL` or `DATABASE_URL` fails startup rather than silently simulating external funding. State is lost on restart. `/healthz`, `/readyz` and `/metrics` report local-memory initialization and per-process request/error/panic counters, not external dependency readiness.

```mermaid
flowchart LR
  HTTP[HTTP + operational middleware] --> UseCases[Portfolio use cases]
  UseCases --> Domain[Exact lot and return domain]
  UseCases --> State[Atomic in-memory holdings/cash]
  UseCases --> Prices[Injectable deterministic price feed]
  Payments[Payments/ledger: not yet connected]
```

Production trade routes share one accounting store. `POST /v1/portfolios/{id}/trades` accepts `symbol`, `side` (BUY/SELL), decimal-string `quantity` and `price`, explicit `currency` (USD/EUR/GBP/JPY), plus `Idempotency-Key` or body `idempotency_key`. Same-payload replay is 201; changed payload is 409. Position trades use `/positions/{symbol}/trades`. Both reject missing keys and oversells. Buys atomically debit local cash and sells credit it; insufficient funds return 409. Fund the local simulation with `/cash/fund` first. Settlement rejects fractional minor units rather than rounding; see [ADR 0001](docs/adr/0001-exact-local-settlement.md).

`GET /v1/portfolios/{id}/summary?currency=USD&as_of=100&max_age=0` and `/positions/{symbol}` use explicit Unix-second cutoff and maximum price age. Register instruments with `POST /v1/instruments` and inject simulated historical marks with `POST /v1/prices`; lookup routes are `/v1/instruments/{symbol}` and `/v1/prices/{symbol}?currency=USD&as_of=100&max_age=0`. These writes use the same header/body key rules. Symbols are case-sensitive path-safe ASCII identifiers up to32 characters; denomination and each symbol/currency/timestamp mark are immutable. Out-of-order mark publication is supported; all valuation queries select the latest mark at/before cutoff, never a future mark. The Go price-feed port remains injectable. Holdings are the current snapshot: `as_of` chooses historical price marks only, not historical holdings/trade replay. No FX conversion or cross-currency aggregation is implemented. Missing/stale marks fail rather than inventing zero values. Summary `TotalValue` is securities only, not cash-inclusive NAV. See [`api/openapi.yaml`](api/openapi.yaml).

## Local synthetic example

With the server running, publish a mark, fund local cash, buy two units and value them:

```sh
base=http://127.0.0.1:8105/v1
id=9113183e-08e9-455b-bfc8-a522c352bd20
curl --fail -H 'Content-Type: application/json' -H 'Idempotency-Key: instrument-acme' "$base/instruments" -d '{"symbol":"ACME","currency":"USD"}'
curl --fail -H 'Content-Type: application/json' -H 'Idempotency-Key: mark-acme-100' "$base/prices" -d '{"symbol":"ACME","currency":"USD","amount":"10","as_of":100}'
curl --fail -H 'Content-Type: application/json' -H 'Idempotency-Key: local-fund' "$base/portfolios/$id/cash/fund" -d '{"currency":"USD","amount":"50"}'
curl --fail -H 'Content-Type: application/json' -H 'Idempotency-Key: local-buy' "$base/portfolios/$id/trades" -d '{"symbol":"ACME","currency":"USD","side":"BUY","quantity":"2","price":"5"}'
curl --fail "$base/portfolios/$id/summary?currency=USD&as_of=100&max_age=0"
```

This is local simulation, not funding through payments or ledger. Any nonzero portfolio UUID currently selects local state; no portfolio lifecycle API is implemented yet.

## Measured status

On 2026-10-18, macOS arm64, Go 1.27.1: build/vet/race gates passed; domain package coverage **95.0%**. Aggregate non-cmd coverage with `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage.out` is **89.64% (770/859 unique statements)**, including cross-package handler/repository execution from preserved playback. Default package-local instrumentation reports **79.74% (685/859)**; it does not instrument dependencies used by other packages' tests. Actual server-router regression: `TestRuntimeRouterSharesTradePositionAndMarkedSummary`. The original in-process 1,000-request playback still passes against the legacy adapter; it does not prove the production router or a real-process 10,000-request target.

Operational middleware uses UUID request IDs and JSON errors, structured logs without bodies/keys, 16 KiB strict JSON mutation bodies and panic recovery. Server read-header/read/write/idle timeouts are 5/10/15/60 seconds; SIGTERM drains requests for up to five seconds then forces closure. Tests: `TestOperationalRequestsRecoveryReadinessAndMetrics`, `TestStrictBoundedBodiesAndUniformErrors`, `TestServeCancellationDrainsActiveRequest` and `TestServeErrorsAndForcedShutdown`. A native process smoke on port8105 verified health/readiness/metrics and clean SIGTERM exit on 2026-10-18; it is not an integrated S3 test.

Incomplete: portfolio lifecycle, corporate-action/returns HTTP, payment-backed funding/outbox, Postgres parity, CI and complete production documentation. Existing trade clients still auto-register denominations on their first successful trade; explicit registration cannot change those denominations. Management replay/concurrency and historical selection are proved by `TestInstrumentAndPriceReplayImmutableHistoryAndConcurrency` and `TestMarketHTTPManagementAndHistoricalPortfolioValuation`. Repeating average-cost/split residuals are preserved by authoritative lot basis and exact final allocation; [ADR 0002](docs/adr/0002-authoritative-lot-basis.md) states the fixed allocation precision and unsupported fractional-share split policy. Local cash is an in-memory simulation; no durable recovery or integrated S3 claim is made.
