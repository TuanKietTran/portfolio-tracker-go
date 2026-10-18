# ADR 0007: Explicit immutable local portfolios alongside legacy UUID state

## Decision

`POST /v1/portfolios` requires currency (USD/EUR/GBP/JPY), cost_method (FIFO/AVERAGE_COST) and an idempotency key. The store generates a UUID through an injected source and samples its injected clock under the same mutex as metadata, replay identity and an empty CREATE history snapshot. `GET /v1/portfolios/{id}` returns only explicitly created immutable metadata. Replays return original201, UUID, UTC created_at and Location without sampling either source. Different currency/method under that key conflicts409. Creation keys are a dedicated global namespace, separate from cash/trade/corporate/instrument/price keys.

No caller-selected identity, metadata update, rename, delete or owner/authentication model is implied. Source errors, zero or colliding IDs return503 without consuming the key or mutating portfolio state. There is no retry loop around a failing generator; retrying the command may use a recovered source. Clock errors return503. Sources run under the state mutex and must not call back into the store. Default sources are uuid.NewRandom and time.Now; tests inject fixed sources without sleeps. Clock observations/generator calls themselves are not rolled back on failure.

Explicit portfolios enforce immutable denomination and accounting method at the atomic persistence boundary. Service trade dispatch uses the portfolio's method, not COST_METHOD. Cash balance with a different supported denomination returns409. Existing position/summary/valuation/returns retain their no-FX validation semantics. CREATE has zero investor flow and establishes an empty historical snapshot; it is not a contribution. Subsequent commands cannot predate creation. Zero-capital return windows remain unsupported by the bounded solver.

## Compatibility and migration

Previously any nonzero UUID selected auto-provisioned local state, potentially with multiple cash currencies and per-position methods. Existing command routes retain that behavior only for unregistered UUIDs, using COST_METHOD as their trade default. Reads never auto-register. Lifecycle lookup returns404 for legacy state even when cash/holdings exist. New creation never adopts it: an injected/generated UUID with any prior successful command history is rejected503. Current successful writes always append history atomically, so this checks existing local state without guessing its currency or method. A generated UUID matching explicit metadata is likewise rejected.

There is deliberately no in-place migration endpoint: historical mixed-currency/method holdings cannot be truthfully assigned new immutable metadata by inference. Clients needing explicit lifecycle create a fresh empty resource; this card does not transfer legacy balances or holdings. No previous test is weakened, legacy playback stays separate, and stored lots/history are never rewritten. A durable migration policy requires a future explicit audited design.

## Limits

All metadata, keys, history and cash remain process-local memory, lost on restart. This does not implement payment-backed funding, durable outbox/reservations, Postgres, authentication or integrated S3. No additional dependency is introduced. Deterministic tests prove concurrent duplicate creation, isolated immutable metadata, source/clock failure recovery, legacy collision protection, method selection and original-result replay after later activity.
