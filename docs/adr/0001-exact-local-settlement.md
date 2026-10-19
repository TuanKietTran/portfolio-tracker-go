# ADR 0001: Exact local settlement and fail-closed external funding

Status: accepted for the synthetic in-memory runtime.

Use decimal quantities, prices and amounts with explicit supported settlement currency. Cash uses **RejectFractionalMinorUnits**: USD/EUR/GBP have two decimal minor units and JPY has zero. Reject a trade whose quantity times unit price cannot settle exactly; do not round, infer a currency or convert currencies. Bound input decimals to 30 coefficient digits and exponent -18 through 18 before arithmetic/replay comparisons.

The synchronized memory adapter commits holdings, cash and command identity together. Its replay cache stores original result snapshots, not current balances. First successful trade binds a symbol's denomination; each position's cost method is immutable. This is a local simulation and loses all state on process restart.

Explicit payments mode now uses a stable payment idempotency identity, confirms settled status and ledger transaction identity, and checks reconciliation before local cash mutation. Withdrawals preflight local balance and payment-backed cash/trade/corporate writes serialize in the service instance. The payment intent is not persisted by the portfolio service, so restart recovery and durable reservations/outbox remain unimplemented. Local mode remains the default. Portfolio never posts directly to ledger.

Consequences: no payment-backed/S3/durable-recovery claim; no implicit FX or sub-minor-unit settlement support. Pure lot calculations retain decimal precision. [ADR 0002](0002-authoritative-lot-basis.md) defines residual allocation and exact full-liquidation basis with repeating split/average-cost conservation regressions.
