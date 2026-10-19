# Platform 04: Outbox and resilience

Status: Deferred for external cash movement.

No downstream call currently follows a portfolio write: `PAYMENTS_URL` is rejected and no outbox is implemented. Local cash mutations are atomic with portfolio state, but cannot claim payment-backed S3 recovery. A future adapter must persist a stable intent identity and recover/reconcile ambiguous payment outcomes before enabling external funding.
