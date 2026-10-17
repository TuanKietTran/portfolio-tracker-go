# Definition of done

A box is **Met** only when the code behaves that way and a test asserts it. **Partial** means the code does it, but no test asserts that scenario. **Unmet** means it is not implemented.

Playback storyboard: `openspec/gifs/playback-flow.gif`.

## Met

| Done when | Evidence |
| --- | --- |
| A buy with `idempotency_key` is HTTP 201 and increases quantity once | Playback. 1000 keyed buys end at quantity 1000. |
| Replaying those keys is HTTP 400 and does not change quantity | Playback. Quantity stays 1000. |
| A buy with no key applies every time | `TestTradeWithoutKeyAppliesTwice`. Quantity 2. |
| Summary after those buys reports the position | Playback reads the summary. |
| 1000 trades finish in under 60 seconds, with one HTTP 503 retried on each call | Playback test, 78ms. |
| HTTP 400 is not retried | Playback unit test. |
| Statement coverage is measured | 67.4%. |

## Partial

| Done when | Why it is not Met |
| --- | --- |
| Two buys recompute average cost | Implemented. The 1000-buy case is all at price 10, so the average does not have to change. No test buys at two prices. |
| A sell updates quantity and realized P&L | Implemented. No test sells. |
| A side other than BUY or SELL does not move the position | Implemented. No test sends one. |
| Summary `TotalValue` and `UnrealizedPnL` are 0 | Implemented. The playback test does not assert those two fields. |
| Invalid portfolio id or invalid JSON is HTTP 400 | Implemented. No test sends them. |

## Unmet

| Not done | Why |
| --- | --- |
| Mark-to-market value | `TotalValue` and `UnrealizedPnL` are fixed at 0. |
| A portfolio row, or Postgres | Any UUID is accepted. The migration is unused. |
| `GetPositions` as its own route | Positions are nested in the summary. |
| Coverage at a high bar | 67.4%. Sell, average-cost mix, and `cmd/server` are uncovered. No 80% gate. |

## GIF review

Good enough. Quantity stays 0 on the retry frame, becomes 1, then 1000, and stays 1000 on HTTP 400. The note on frame 6 says a missing key would still apply. Storyboard, not a screen capture.
