# Portfolio tracker

CQRS portfolio simulation. Production named/position trades share exact FIFO accounting state, and explicit as-of summary queries compute marked security value and P&L. The older adapter and playback are retained as historical regression fixtures, not server wiring. Cash and trades commit atomically in the local simulation; external payments remain unavailable until outbox/recovery support exists.

## Stack

Go 1.22, Chi, `shopspring/decimal`, Google UUID. In-memory only.

## Public API

- `POST /v1/portfolios/{id}/trades`
- `GET /v1/portfolios/{id}/summary`

## Checkpoints

| Date | Change | Evidence |
| --- | --- | --- |
| 2026-10-17 | Baseline spec recorded from the shipped service | `08b5e3c`. No handler tests and no Postgres repository yet. |
| 2026-10-17 | 1000 requests in one minute, retry, playback, coverage | `internal/playback`. 1000 keyed buys plus one 503 retry each finished in 78ms. Replay did not change quantity. A missing key still applies every post. Statement coverage 67.4%. |
| 2026-10-17 | Definition of done and playback storyboard | `openspec/dod.md` and `openspec/gifs/playback-flow.gif`. GIF review: retry stays at the pre-write state, playback shows the unchanged result. |
| 2026-10-18 | Accounting trade replay and independent lot snapshots | `e3c0cd9`; `go test -p 2 ./... -race -count=1 -cover`; domain 92.5%, weighted non-cmd 64.52%. |
| 2026-10-18 | Unified runtime trades/summary and marked totals | `366081e`; `go test -p 2 ./... -race -count=1 -coverprofile=<ctl logs>/unified-runtime.cover -cover`; domain 92.5%, weighted non-cmd 319/502 = 63.55%. `TestRuntimeRouterSharesTradePositionAndMarkedSummary` exercises actual server router. |

### 2026-10-18 accounting trade replay checkpoint

The `/positions/{symbol}/trades` route now requires a key and returns the original snapshot on replay; changed payloads return 409. Its store is still separate from legacy trades/summary. `TestAccountingTradeHTTPReplayConflictAndRequiredKey`, `TestConcurrentTradeReplayAppliesOnce` and `TestAccountingEventsDoNotMutateInputLots` prove the bounded change. On macOS arm64 Go 1.27.1, `go test -p 2 ./... -race -count=1 -coverprofile=<ctl logs>/trade-replay.cover -cover` passed; domain 92.5%, weighted non-cmd 280/434 = 64.52%. Build, vet, empty gofmt and module tidy gates passed. Exact commit is recorded in the append-only lane STATUS (this checkpoint is part of that commit).

### 2026-10-18 exact residual-basis checkpoint

Commit recorded in STATUS. `go test -p 2 ./... -race -count=1 -cover` passed: domain 94.9%. `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=<ctl logs>/basis-residuals-all.cover` passed and measures aggregate non-cmd 506/587 = 86.20% unique statements across all test binaries; default package-local profile is 410/587 = 69.85%. Build/vet/empty-gofmt/tidy gates passed on macOS arm64 Go 1.27.1. Added regressions first failed against `1b99131` (exact residuals logged in ctl `residual-before.log`) and now pass. [ADR 0002](../docs/adr/0002-authoritative-lot-basis.md) defines authoritative basis, 18-place partial-allocation rounding and split representability.

### 2026-10-18 local cash-linked trade checkpoint

`go test -p 2 ./... -race -count=1 -coverprofile=<ctl logs>/cash-trades.cover -cover` passed on macOS arm64 Go 1.27.1: domain 92.6%, weighted non-cmd 380/559 = 67.98%. Build/vet/empty-gofmt/tidy gates passed. `GOMAXPROCS=2 go test ./internal/domain -run='^$' -fuzz=FuzzSettlementAmount -fuzztime=3s` passed 164,686 executions. `TestConcurrentBuysAndWithdrawalsShareOneCashBoundary` proves local insufficient-funds concurrency; `TestCashLinkedTradeHTTPAndHeaderReplay` proves live route effects/replay. `1b99131`. This does not prove payment-backed funding or restart durability; the next checkpoint adds repeating-ratio lot conservation regressions.

## Out of scope until implemented

- Cash-inclusive NAV, instrument/price management over HTTP.
- External payment-backed funding and durable persistence.
- A portfolio row. Any UUID is accepted. `migrations/001_init.sql` is not used by the server.

## Definition of done

See `openspec/dod.md`. The playback storyboard is `openspec/gifs/playback-flow.gif`.
