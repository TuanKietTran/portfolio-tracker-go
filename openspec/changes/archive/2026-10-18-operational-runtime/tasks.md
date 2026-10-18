# Tasks

- [x] Validate HTTP_ADDR, local CASH_MODE and selectable COST_METHOD; reject unsupported external adapters.
- [x] Add per-router metrics, UUID request IDs, slog and standard JSON errors/recovery.
- [x] Add /healthz, /readyz and /metrics with honest local-only semantics.
- [x] Bound mutation bodies to 16384 bytes; reject unknown fields, trailing values, invalid/blank/oversized keys and zero portfolio UUIDs.
- [x] Wire bounded HTTP timeouts and context-driven graceful shutdown with forced close/join on grace expiry.
- [x] Test readiness success/failure/panic, error envelopes, metrics, strict JSON, configuration and lifecycle without synchronization sleeps.
- [x] Run full gates/repeated race lifecycle tests, update docs/spec and archive.
