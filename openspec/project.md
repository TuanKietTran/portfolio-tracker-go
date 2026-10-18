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
| 2026-10-17 | 1000 requests in one minute, retry, playback, coverage | `internal/playback`. 1000 keyed buys plus one 503 retry each finished in 78ms. Replay did not change quantity. A missing key still applies every post. Statement coverage 67.4%. |
| 2026-10-17 | Definition of done and playback storyboard | `openspec/dod.md` and `openspec/gifs/playback-flow.gif`. GIF review: retry stays at the pre-write state, playback shows the unchanged result. |

### 2026-10-18 accounting trade replay checkpoint

The `/positions/{symbol}/trades` route now requires a key and returns the original snapshot on replay; changed payloads return 409. Its store is still separate from legacy trades/summary. `TestAccountingTradeHTTPReplayConflictAndRequiredKey`, `TestConcurrentTradeReplayAppliesOnce` and `TestAccountingEventsDoNotMutateInputLots` prove the bounded change. On macOS arm64 Go 1.27.1, `go test -p 2 ./... -race -count=1 -coverprofile=<ctl logs>/trade-replay.cover -cover` passed; domain 92.5%, weighted non-cmd 280/434 = 64.52%. Build, vet, empty gofmt and module tidy gates passed. Exact commit is recorded in the append-only lane STATUS (this checkpoint is part of that commit).

## Out of scope until implemented

- `GetPositions` over HTTP. Positions are nested in the summary.
- Mark-to-market `TotalValue` and `UnrealizedPnL`. Both are fixed at 0.
- A portfolio row. Any UUID is accepted. `migrations/001_init.sql` is not used by the server.

## Definition of done

See `openspec/dod.md`. The playback storyboard is `openspec/gifs/playback-flow.gif`.
