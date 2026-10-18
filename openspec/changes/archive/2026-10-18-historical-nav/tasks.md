# Tasks

- [x] Add exact historical holdings/cash/mark valuation and sorted snake_case result.
- [x] Enforce supported single currency, price freshness, all-or-error data invariants and no future-state leakage.
- [x] Define exact start-of-second HTTP cutoff with UTC response; preserve nanosecond precision in Go port.
- [x] Expose valuation GET route and parsed OpenAPI paths/status/schema.
- [x] Test old holdings after later sale/cash changes, cash-only/closed positions, cutoff fractions, invalid/FX/missing/stale/future marks and canceled reads.
- [x] Run full gates, update measured docs/current specs, archive and commit.
