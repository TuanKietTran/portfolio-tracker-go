## MODIFIED Requirements

### Requirement: Original cash replay and external fail-closed boundary
Cash same-payload replay MUST return the original balance result without any new local effect, even if current cash has since changed. Header/body keys MUST agree. Local mode MUST remain the default and use local simulated cash. In explicit payments mode, funding/withdrawal MUST create a payment intent using an idempotency key deterministically derived from portfolio and request key, with direction/accounts/currency/amount in the payload; process the payment; re-read and confirm `SETTLED` with nonempty `LedgerTxID`; and verify GET `/v1/reconciliation` before changing local cash. The payments mode MUST require loopback payment API configuration and distinct synthetic source/destination accounts. Portfolio MUST NOT post directly to a ledger. Ambiguous create/process outcomes MUST be retried/re-read using the same intent key. Any unavailable/failed downstream confirmation MUST leave local cash unchanged and return an unavailable response; the caller can recover by retrying the original portfolio idempotency key. Payments-mode cash-affecting trades, cash movements, and corporate actions MUST serialize through the same service instance so pending withdrawals cannot race local debits. This in-memory serialization and intent derivation do not provide portfolio restart durability.

#### Scenario: Replay after spending
- **GIVEN** a keyed funding of 20 followed by a buy spending all 20
- **WHEN** the funding is replayed
- **THEN** its response reports original balance 20 while stored cash remains zero

#### Scenario: Ambiguous response recovers one settled funding intent
- **GIVEN** payment creation or processing committed but its response was lost
- **WHEN** the caller retries the same keyed portfolio funding request
- **THEN** the same payment intent is retrieved by stable idempotency identity
- **AND** settlement status and ledger transaction identity are confirmed
- **AND** reconciliation is clean before local cash changes exactly once

#### Scenario: Payment failure does not change local cash
- **GIVEN** the payment API or reconciliation is unavailable or reports a discrepancy
- **WHEN** a keyed funding or withdrawal is submitted
- **THEN** the API returns 503 and local cash remains unchanged
- **AND** retrying with the original key may recover an already committed payment

#### Scenario: Insufficient local withdrawal is rejected before payment
- **GIVEN** local cash is below a requested payment-backed withdrawal amount
- **WHEN** the withdrawal is submitted
- **THEN** the API returns 409 without creating a payment intent
