# Recoverable payments-backed portfolio cash

## Why

Local simulation is intentionally the default, but an explicit payments mode can use the existing payment orchestration contract without posting directly to the ledger or claiming local-store durability.

## Scope

Create payments through POST /v1/payments using a deterministic idempotency key; process pending intents, re-read payment status, require SETTLED plus LedgerTxID, and verify GET /v1/reconciliation before applying local cash. Preserve local mode by default. Replay ambiguous calls with the same downstream intent key and serialize payment-backed cash-affecting portfolio writes.

## Out of scope

Durable portfolio-side outbox/pending records, restart recovery for local portfolio state, Postgres, and claims of cross-process S3 platform verification.
