# ADR 0001: Exact local settlement and fail-closed external funding

Status: accepted for the synthetic in-memory runtime.

Use decimal quantities, prices and amounts with explicit supported settlement currency. Cash uses **RejectFractionalMinorUnits**: USD/EUR/GBP have two decimal minor units and JPY has zero. Reject a trade whose quantity times unit price cannot settle exactly; do not round, infer a currency or convert currencies. Bound input decimals to 30 coefficient digits and exponent -18 through 18 before arithmetic/replay comparisons.

The synchronized memory adapter commits holdings, cash and command identity together. Its replay cache stores original result snapshots, not current balances. First successful trade binds a symbol's denomination; each position's cost method is immutable. This is a local simulation and loses all state on process restart.

The earlier payment port could perform an external movement before local validation/commit, allowing overspending or ambiguous outcomes. Until an outbox with durable reservations and posted-payment evidence is implemented, an injected external boundary fails closed with unavailable and no call is made. The default nil boundary explicitly selects local simulation. Portfolio never posts directly to ledger.

Consequences: no payment-backed/S3/durable-recovery claim; no implicit FX or sub-minor-unit settlement support. Pure lot calculations retain decimal precision. [ADR 0002](0002-authoritative-lot-basis.md) defines residual allocation and exact full-liquidation basis with repeating split/average-cost conservation regressions.
