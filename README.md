# Portfolio Tracker (CQRS)

Synthetic investment accounting in Go: FIFO/average-cost domain lots, realized/unrealized P&L, deterministic price marks and local cash. No real money or credentials.

See the [architecture decision index](docs/adr/README.md).

## Run and verify

```sh
GOTOOLCHAIN=local HTTP_ADDR=127.0.0.1:8105 CASH_MODE=local go run ./cmd/server
curl --fail http://127.0.0.1:8105/readyz
GOTOOLCHAIN=local go test -p 2 ./... -race -count=1 -cover
```

Configuration: `HTTP_ADDR` defaults to `127.0.0.1:8105`; `CASH_MODE=local` is the default and uses local simulated funding. Explicit `CASH_MODE=payments` requires loopback `PAYMENTS_URL`, `PAYMENTS_SOURCE_ACCOUNT`, and `PAYMENTS_DEST_ACCOUNT`; fund/withdraw requests create an idempotent payment intent, process it, confirm `SETTLED` plus `LedgerTxID`, and require clean payment reconciliation before local cash changes. Retries derive a stable payment key from the portfolio and request key, allowing replay after an ambiguous response; direction, accounts, currency and amount remain part of the payment payload, so reusing a key for a different operation conflicts. This pending intent is not persisted in the portfolio service; restart durability remains unmet. Payment API currencies are USD/EUR/GBP only. No direct ledger posting occurs. `DATABASE_URL` fails startup because persistence is not implemented. `COST_METHOD` supports `FIFO` (default) and `AVERAGE_COST`; explicitly created portfolios select their own immutable method. State is lost on restart. `/healthz`, `/readyz` and `/metrics` report in-memory initialization, not external dependency readiness. `/metrics` also includes heap bytes/objects, goroutines, active HTTP connections and latest GC pause; `/debug/pprof/` exposes standard Go profiles on the loopback listener only; CPU and trace captures are bounded to 10 seconds.

For an explicitly configured synthetic payments mode, start the payments service first and set:

```sh
HTTP_ADDR=127.0.0.1:8105 CASH_MODE=payments PAYMENTS_URL=http://127.0.0.1:8102 PAYMENTS_SOURCE_ACCOUNT=synthetic-investor-clearing PAYMENTS_DEST_ACCOUNT=synthetic-portfolio-clearing go run ./cmd/server
```

These account identifiers are synthetic fixture names, not credentials or real accounts.

```mermaid
flowchart LR
  HTTP[HTTP + operational middleware] --> UseCases[Portfolio use cases]
  UseCases --> Domain[Exact lot and return domain]
  UseCases --> State[Atomic in-memory holdings/cash]
  UseCases --> Prices[Injectable deterministic price feed]
  UseCases --> PaymentClient[Optional payments HTTP adapter]
  PaymentClient --> Payments[Payment orchestration API; loopback only]
```

Production trade routes share one accounting store. `POST /v1/portfolios/{id}/trades` accepts `symbol`, `side` (BUY/SELL), decimal-string `quantity` and `price`, explicit `currency` (USD/EUR/GBP/JPY), plus `Idempotency-Key` or body `idempotency_key`. Same-payload replay is 201; changed payload is 409. Position trades use `/positions/{symbol}/trades`. Both reject missing keys and oversells. Buys atomically debit portfolio cash and sells credit it; insufficient funds return 409. Fund with `/cash/fund` first. The default is local simulation; optional payments mode gates local cash changes on settled-payment and reconciliation evidence. Settlement rejects fractional minor units rather than rounding; see [ADR 0001](docs/adr/0001-exact-local-settlement.md).

`GET /v1/portfolios/{id}/summary?currency=USD&as_of=100&max_age=0` and `/positions/{symbol}` use explicit Unix-second cutoff and maximum price age. Register instruments with `POST /v1/instruments` and inject simulated historical marks with `POST /v1/prices`; lookup routes are `/v1/instruments/{symbol}` and `/v1/prices/{symbol}?currency=USD&as_of=100&max_age=0`. These writes use the same header/body key rules. Symbols are case-sensitive path-safe ASCII identifiers up to32 characters; denomination and each symbol/currency/timestamp mark are immutable. Out-of-order mark publication is supported; all valuation queries select the latest mark at/before cutoff, never a future mark. The Go price-feed port remains injectable. Holdings are the current snapshot: `as_of` chooses historical price marks only, not historical holdings/trade replay. No FX conversion or cross-currency aggregation is implemented. Missing/stale marks fail rather than inventing zero values. Summary `TotalValue` is securities only, not cash-inclusive NAV. See [`api/openapi.yaml`](api/openapi.yaml).

Corporate actions use keyed `POST /v1/portfolios/{id}/corporate-actions`: `symbol`, `currency` and `kind` (`SPLIT`, `DIVIDEND`, `FEE`). SPLIT takes positive integer `numerator`/`denominator`; DIVIDEND/FEE take a positive total cash `amount`, not a per-share rate. Shares/basis, cash and original replay response commit atomically. Splits/dividends require held shares; fees may charge a previously held closed position but cannot overdraw cash. Unsupported repeating split shares are rejected; no cash-in-lieu. Split actions do not rewrite price history: publish an appropriate post-action simulated mark explicitly. Corporate keys have their own global namespace. See [ADR 0003](docs/adr/0003-corporate-actions.md).

Portfolio mutations now append atomic UTC/sequence-ordered command and holdings/cash snapshots in memory. Only FUND/WITHDRAW are external investor flows (contributions negative, withdrawals positive); trades/dividends/fees/splits are internal investment effects. Clock regression or invalid time returns503 before effect/key/history changes; already successful replays remain available. Repository history pages and historical snapshot lookup are implemented and tested. Historical NAV is now exposed below; raw history HTTP remains unimplemented; bounded return-rate queries are now exposed below. Full-snapshot memory grows with commands times holdings/lot size and is lost on restart; see [ADR 0004](docs/adr/0004-local-command-history.md).

`GET /v1/portfolios/{id}/valuation?currency=USD&as_of=<unix-seconds>&max_age=<seconds>` reconstructs historical cash plus securities NAV, not current holdings. The cutoff is the exact START of the Unix second: a command at110s+750ns is excluded by110 and included by111. Response `as_of` is canonical UTC date-time. Closed positions retain realized PnL with zero market value and `price_as_of:null`; cash-only books need no marks. Missing snapshots return404; missing/stale/future marks or unsupported FX return400. The default command clock is actual UTC wall time, independent of the synthetic price timestamp100 in the example. See [ADR0005](docs/adr/0005-historical-nav.md).

`GET /v1/portfolios/{id}/returns?currency=USD&from=<seconds>&to=<seconds>&max_age=<seconds>&method=TWR` links historical external-flow boundaries using a consistent holdings/flow window and frozen price set. TWR is a nonannualized period rate; MWR is annualized ACT/365 fixed seconds with optional bounded `rate_tolerance`, `npv_tolerance` and `max_iterations`. Starting NAV is counted once; only flows in `(from,to]` are included, and same-time flows use individual sequence snapshots. Query limits are3650 days,4096 commands and256 external flows. MWR reports both computed residual and rate-bracket diagnostics; zero-capital TWR, potentially multiple-IRR patterns, unbracketed roots and nonconvergence fail explicitly. See [ADR0006](docs/adr/0006-bounded-portfolio-returns.md) for numeric/rounding/error limitations; decimal rate0.1 means10 percent.

`POST /v1/portfolios` requires `currency`, `cost_method` (`FIFO`/`AVERAGE_COST`) and the usual header/body idempotency key; it returns201 with generated `portfolio_id`, immutable metadata, UTC `created_at` and `Location`. Same-key replay returns the original metadata even after later activity; changed payload409. `GET /v1/portfolios/{id}` returns that metadata or404. Creation atomically establishes an empty historical snapshot, not funding. Explicit portfolios reject other-denomination cash/trade/corporate writes409 and use their stored cost method regardless of server default. Invalid identity/clock sources fail503 without consuming the key. See [ADR0007](docs/adr/0007-explicit-local-portfolios.md).

```sh
curl --fail -H 'Content-Type: application/json' -H 'Idempotency-Key: portfolio-demo' http://127.0.0.1:8105/v1/portfolios -d '{"currency":"USD","cost_method":"AVERAGE_COST"}'
# Use the returned portfolio_id in subsequent cash/trade/lookup routes.
```

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

This is local simulation, not funding through payments or ledger. This example retains the legacy arbitrary-UUID compatibility path. Legacy cash/holdings remain operable but are unregistered: lifecycle GET returns404, and creation never adopts or reinterprets them. For explicit metadata, create a new empty resource first and use its returned UUID; there is no automatic legacy balance/holdings migration.

## Measured status

On 2026-10-19, macOS arm64, Go 1.27.1: build/vet/race gates passed; domain package coverage **95.0%**. Aggregate non-cmd coverage with `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage.out` is **93.81% (1303/1389 unique statements)**, including cross-package handler/repository execution from preserved playback. Default package-local instrumentation reports **87.69% (1218/1389)**; it does not instrument dependencies used by other packages' tests. Actual server-router regression: `TestRuntimeRouterSharesTradePositionAndMarkedSummary`. The legacy 1,000-request playback remains a separate adapter regression. On 2026-10-19, Windows/amd64 with Go 1.27.2, `go test -p 2 ./internal/playback -run TestProductionRouterTenThousandRequestsWithFaultsAndDuplicates -count=1 -v` completed 10,000 requests in 9.283s: 9,800 unique trades, 200 duplicate submissions, 500 injected 503s, 100 injected 429s and 100 per-attempt timeouts; state quantity was 9,800 and cash 200. The httptest listener uses real loopback HTTP, not a separate OS process. Race-instrumented replay is excluded from this timing test; the package passes under `-race`.

Operational middleware uses UUID request IDs and JSON errors, structured logs without bodies/keys, 16 KiB strict JSON mutation bodies and panic recovery. Runtime metrics include heap, goroutines, active connections and latest GC pause; loopback-only `/debug/pprof/` exposes profiles for load diagnosis; CPU/trace captures are capped at 10 seconds. Server read-header/read/write/idle timeouts are 5/10/15/60 seconds; SIGTERM drains requests for up to five seconds then forces closure. Tests: `TestOperationalRequestsRecoveryReadinessAndMetrics`, `TestStrictBoundedBodiesAndUniformErrors`, `TestServeCancellationDrainsActiveRequest` and `TestServeErrorsAndForcedShutdown`. A native process smoke on port8105 verified health/readiness/metrics and clean SIGTERM exit on 2026-10-18; it is not an integrated S3 test.

Lifecycle proofs: `TestPortfolioCreationConcurrentReplayAndImmutableMetadata`, `TestPortfolioCreationFailuresDoNotConsumeKeyOrAdoptLegacy`, `TestPortfolioLifecycleHTTPConcurrentReplayAndSelectedMethod` and `TestPortfolioLifecycleHTTPUnavailableSourcesAndRecovery`. All metadata and replay keys are memory-only. Incomplete: generalized zero-capital/multiple-IRR handling, durable payment intent/outbox, Postgres parity, CI and complete production documentation. Bounded return cases are proved by `TestHistoricalReturnsWorkedFlowAndAnalyticIRR`, `TestSameTimeFlowsUseIndividualSnapshotsAndFeesAreInternal` and `TestReturnHTTPAnalyticRatesDiagnosticsValidationAndNoPartialFailure`. Numeric diagnostics bound the computed discount model, not formal real-arithmetic math.Pow error. Historical NAV is proved by `TestHistoricalNAVUsesEarlierHoldingsAndCashAfterFullSale` and `TestHistoricalNAVHTTPExactCutoffClosedHoldingsAndNoFutureLeakage`. Local history/clock proofs are `TestTimestampedCommandHistoryFlowsSnapshotsAndIsolation`, `TestConcurrentHistorySnapshotsAreOrderedAndAtomic` and `TestHistoryClockHTTPUnavailableHasNoEffectAndSameKeyCanRetry`. Corporate-action worked/concurrent proofs are `TestCorporateWorkedDividendFeeAndSplit`, `TestCorporateHTTPConcurrentReplayCashAndWorkedValuation` and `TestConcurrentCorporateFeesCannotOverdrawAndFailureDoesNotConsumeKey`. Existing trade clients still auto-register denominations on their first successful trade; explicit registration cannot change those denominations. Management replay/concurrency and historical selection are proved by `TestInstrumentAndPriceReplayImmutableHistoryAndConcurrency` and `TestMarketHTTPManagementAndHistoricalPortfolioValuation`. Repeating average-cost/split residuals are preserved by authoritative lot basis and exact final allocation; [ADR 0002](docs/adr/0002-authoritative-lot-basis.md) states the fixed allocation precision and unsupported fractional-share split policy. Local cash is an in-memory simulation; no durable recovery or integrated S3 claim is made.
