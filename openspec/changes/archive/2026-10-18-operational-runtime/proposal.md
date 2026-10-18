# Operational local runtime

Make the memory-backed portfolio simulation launchable with validated HTTP_ADDR/CASH_MODE/COST_METHOD settings, structured request logs and request IDs, health/readiness/metrics, bounded strict JSON, common error envelopes and panic recovery. Reject unsupported PAYMENTS_URL/DATABASE_URL instead of silently ignoring external configuration. Use bounded server/header/idle/write/shutdown timeouts and graceful context-driven shutdown.

Readiness promises only initialization of the local simulation, not payment-backed cash, durable persistence or complete price coverage. Recovered panic details and request bodies/keys are not logged or returned. Errors after a response has already started cannot be rewritten; recovery logs/counts the panic without claiming a new status.

Preserve legacy playback constructors and assertions as historical fixtures while testing the production router and actual HTTP lifecycle separately. Publish only implemented operational routes. CI/docs and missing domain/persistence/payment capabilities remain follow-up cards.
