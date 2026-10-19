# Portfolio tracker

CQRS portfolio simulation. Production named/position trades share exact FIFO accounting state, and explicit as-of summary queries compute marked security value and P&L. The older adapter and playback are retained as historical regression fixtures, not server wiring. Cash and trades commit atomically in local mode. Explicit payments mode confirms a stable, idempotent payment intent and reconciliation before local cash changes; portfolio-side restart durability remains unavailable.

## Stack

Go module1.22; historical coverage evidence used Go1.27.1 macOS arm64; current gates use Go1.27.2 Windows/amd64. Chi, `shopspring/decimal`, Google UUID. In-memory only; unsupported database configuration fails startup. HTTP defaults 127.0.0.1:8105 with selectable FIFO/average accounting, bounded lifecycle, local default cash and optional loopback payments mode; readiness reports local initialization only.

## Public API

- `POST /v1/portfolios/{id}/trades`
- `GET /v1/portfolios/{id}/summary`
- `POST /v1/portfolios` and `GET /v1/portfolios/{id}` (immutable explicit local metadata)

## Checkpoints

| Date | Change | Evidence |
| --- | --- | --- |
| 2026-10-19 | Explicit immutable keyed local portfolio lifecycle | Parent `3f36d1e`; exact milestone hash in the project history. `go test -p 2 ./... -race -count=1 -coverprofile=coverage-lifecycle.cover -cover`; domain95.0%, package-local non-cmd1218/1389=87.69%, cross-package1303/1389=93.81%. Historical verification evidence was collected for this milestone. |
| 2026-10-17 | Baseline spec recorded from the shipped service | `08b5e3c`. No handler tests and no Postgres repository yet. |
| 2026-10-17 | 1000 requests in one minute, retry, playback, coverage | `internal/playback`. 1000 keyed buys plus one 503 retry each finished in 78ms. Replay did not change quantity. A missing key still applies every post. Statement coverage 67.4%. |
| 2026-10-17 | Definition of done and playback storyboard | `openspec/dod.md` and `openspec/gifs/playback-flow.gif`. GIF review: retry stays at the pre-write state, playback shows the unchanged result. |
| 2026-10-18 | Accounting trade replay and independent lot snapshots | `e3c0cd9`; `go test -p 2 ./... -race -count=1 -cover`; domain 92.5%, weighted non-cmd 64.52%. |
| 2026-10-18 | Unified runtime trades/summary and marked totals | `366081e`; `go test -p 2 ./... -race -count=1 -coverprofile=coverage-unified-runtime.cover -cover`; domain 92.5%, weighted non-cmd 319/502 = 63.55%. `TestRuntimeRouterSharesTradePositionAndMarkedSummary` exercises actual server router. |

### 2026-10-19 explicit portfolio lifecycle checkpoint

Build/vet/empty-gofmt/default and cross-package race/tidy/diff gates passed on Go1.27.1 macOS arm64. `TestPortfolioCreationConcurrentReplayAndImmutableMetadata` proves100-way duplicate creation and isolated original metadata; `TestDistinctConcurrentPortfolioCreationsRemainIsolated` proves100 distinct identities with independent initial history. Failure/legacy-collision tests prove no adoption or consumed key, and HTTP tests prove per-portfolio AVERAGE_COST on a FIFO-default server (worked realized gain15, remaining basis15), denomination409 and source503 recovery. Parsed OpenAPI tests check new routes/statuses and metadata schemas. Creation establishes a zero-investor-flow empty CREATE snapshot; no cash transfer. ADR0007 records immutable metadata and explicit legacy404 compatibility, not a durable migration. Source unchanged after full gates; later archive/docs only. Exact commit recorded in STATUS.

### 2026-10-18 accounting trade replay checkpoint

The `/positions/{symbol}/trades` route now requires a key and returns the original snapshot on replay; changed payloads return 409. Its store is still separate from legacy trades/summary. `TestAccountingTradeHTTPReplayConflictAndRequiredKey`, `TestConcurrentTradeReplayAppliesOnce` and `TestAccountingEventsDoNotMutateInputLots` prove the bounded change. On macOS arm64 Go 1.27.1, `go test -p 2 ./... -race -count=1 -coverprofile=coverage-trade-replay.cover -cover` passed; domain 92.5%, weighted non-cmd 280/434 = 64.52%. Build, vet, empty gofmt and module tidy gates passed. The checkpoint commit records this milestone.

### 2026-10-19 bounded portfolio-return checkpoint

Full build/vet/empty-gofmt/race/tidy/diff gates passed on Go1.27.1 macOS arm64. Domain95.0%; default non-cmd1127/1295=87.03%; cross-package unique non-cmd1209/1295=93.36% with `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage-returns-all.cover`. `GOMAXPROCS=2 go test ./internal/domain -run='^$' -fuzz=FuzzGuardedIRROneYearAnalytic -fuzztime=3s` passed26156 executions. Named tests prove analytic/same-time-fee returns, frozen views, work/precision limits and explicit numerical failure. ADR0006 documents supported periods/capital/sign patterns and computed-model diagnostics, not formal Pow error certification.

### 2026-10-18 historical NAV checkpoint

`a0fad91`. Full build/vet/empty-gofmt/race/tidy/diff gates passed on Go1.27.1 macOS arm64. Domain95.7%; default non-cmd908/1073=84.62%; cross-package unique non-cmd994/1073=92.64% with `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage-nav-all.cover`. Actual router/unit tests prove earlier holdings after later sale, exact fractional cutoff behavior, cash-only/closed books, sorted values, unsupported FX and missing/stale/future marks. ADR0005 defines precise cutoff and price-revision semantics. No return-rate/durable/external-cash claim.

### 2026-10-18 local command-history checkpoint

`28c2a5c`. Build/vet/empty-gofmt/race/tidy/diff gates passed on Go1.27.1 macOS arm64. Domain95.7%; default unique non-cmd846/1012=83.60%; cross-package unique non-cmd932/1012=92.09% with `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage-history-all.cover`. Historical verification evidence was collected for this milestone. Deterministic tests prove timestamp/sequence ordering, investor-flow signs, rollback/recovery, historical isolation, bounded paging and100-way same-clock concurrent commands without synchronization sleeps. [ADR0004](../docs/adr/0004-local-command-history.md) documents memory cost, clock policy and no durable/HTTP NAV/return claim.

### 2026-10-18 corporate-action checkpoint

`4c5061f`. Full build/vet/empty-gofmt/race/tidy/diff gates passed on Go1.27.1 macOS arm64. Domain95.7%; default package-local non-cmd773/939=82.32%; combined cross-package unique non-cmd859/939=91.48% with `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage-corporate-all.cover`. `GOMAXPROCS=2 go test ./internal/domain -run='^$' -fuzz=FuzzCorporateSplitConservation -fuzztime=3s` passed156521 executions. Historical verification evidence was collected; named DOD tests cover100-way HTTP/repository replay, fee contention/rollback and worked dividend/fee/split cash/valuation. [ADR0003](../docs/adr/0003-corporate-actions.md) states immediate/local/total-amount semantics and no price-history rewrite.

### 2026-10-18 market-management checkpoint

`4ac231f`. `go build ./...`, `go vet ./...`, empty `gofmt -l .`, race tests with default and cross-package coverage, tidy with no module diff and diff check passed on Go1.27.1 macOS arm64. Domain95.0%; weighted unique non-cmd770/859=89.64% with `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage-market-all.cover`; default package-local685/859=79.74%. `GOMAXPROCS=2 go test ./internal/domain -run='^$' -fuzz=FuzzInstrumentSymbol -fuzztime=3s` passed165954 executions. Historical verification evidence was collected. Immutable registry/marks,100-way duplicate concurrency and actual-router historical summary assertions are named in DOD. State remains memory-only.

### 2026-10-18 operational runtime checkpoint

`a56180c`. `go build ./...`, `go vet ./...`, empty `gofmt -l .`, race tests with default and `-coverpkg=./...` profiles, tidy with no module diff and diff check passed. Domain94.9%, server95.5%; aggregate unique non-cmd630/715=88.11%, package-local535/715=74.83%. Build, vet, formatting, race, coverage, and module hygiene were verified at the time; detailed process output is not part of this public repository. `go test ./internal/server -race -count=10` passed; lifecycle tests use channel synchronization. Native compiled process on127.0.0.1:8105 with AVERAGE_COST returned200 for readiness/health/metrics and exited0 on SIGTERM; structured process logging was observed. No external cash/persistence/integration claim.

### 2026-10-18 exact residual-basis checkpoint

`21e0842`. `go test -p 2 ./... -race -count=1 -cover` passed: domain 94.9%. `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage-basis-residuals-all.cover` passed and measures aggregate non-cmd 506/587 = 86.20% unique statements across all test binaries; default package-local profile is 410/587 = 69.85%. Build/vet/empty-gofmt/tidy gates passed on macOS arm64 Go 1.27.1. Added regressions first failed against `1b99131` (exact residuals reproduced against the prior revision) and now pass. [ADR 0002](../docs/adr/0002-authoritative-lot-basis.md) defines authoritative basis, 18-place partial-allocation rounding and split representability.

### 2026-10-18 local cash-linked trade checkpoint

`go test -p 2 ./... -race -count=1 -coverprofile=coverage-cash-trades.cover -cover` passed on macOS arm64 Go 1.27.1: domain 92.6%, weighted non-cmd 380/559 = 67.98%. Build/vet/empty-gofmt/tidy gates passed. `GOMAXPROCS=2 go test ./internal/domain -run='^$' -fuzz=FuzzSettlementAmount -fuzztime=3s` passed 164,686 executions. `TestConcurrentBuysAndWithdrawalsShareOneCashBoundary` proves local insufficient-funds concurrency; `TestCashLinkedTradeHTTPAndHeaderReplay` proves live route effects/replay. `1b99131`. This does not prove payment-backed funding or restart durability; the next checkpoint adds repeating-ratio lot conservation regressions.

## Out of scope until implemented

- Generalized zero-capital/multiple-IRR cases and raw-history HTTP.
- External payment-backed funding and durable persistence.
- Durable portfolio rows and audited migration of legacy UUID state. Explicit metadata now exists in memory; legacy arbitrary-UUID writes stay unregistered and lookup404. `migrations/001_init.sql` is not used by the server.

## Definition of done

See `openspec/dod.md`. The playback storyboard is `openspec/gifs/playback-flow.gif`.
