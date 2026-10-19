# Platform 04: Outbox and resilience

Status: Payment-backed cash adapter implemented; durable portfolio outbox deferred.

Local mode remains the default. Explicit loopback payments mode derives a stable payment idempotency key from portfolio and caller key (direction, accounts, currency and amount remain in the downstream payload); creates/processes the payment, confirms SETTLED plus LedgerTxID, and requires clean reconciliation before changing local cash. On ambiguous outcomes, retrying the original portfolio key resolves the same downstream intent. Cash-affecting operations serialize within one in-memory service instance. The portfolio-side intent is not persisted, so process restart recovery and durable outbox guarantees remain unimplemented.
