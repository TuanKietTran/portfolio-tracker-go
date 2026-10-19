# Production-router 10k replay

## Why

The legacy playback exercised an obsolete adapter and did not prove the current production router, bounded concurrency, timeout retries, or effects under duplicate submissions.

## Scope

Implement bounded worker replay with exponential full jitter and per-attempt deadlines under a separately bounded replay-window context. Prove 10,000 requests through the production API router and a real loopback HTTP listener, with at least 5% injected 503, some 429, 1% timeouts, 2% same-key/body duplicates, and direct state assertions. Keep the legacy adapter regression separately.

## Out of scope

Persistent storage and downstream payment integration are separate follow-up work. No timing result is claimed from race-instrumented replay; default replay test measures the 60-second service target, and race tests continue to cover playback and route code.
