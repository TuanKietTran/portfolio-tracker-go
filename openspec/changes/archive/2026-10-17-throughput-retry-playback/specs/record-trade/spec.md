## ADDED Requirements

### Requirement: Optional idempotency key
The system SHALL apply a trade at most once when `idempotency_key` is set. A second post with that key SHALL NOT change the position. A post that omits the key SHALL apply every time.

#### Scenario: Keyed buy is not applied twice
- **GIVEN** a buy of 1 `ACME` with key `t-1` was accepted
- **WHEN** the same body is posted again
- **THEN** the response is HTTP 400
- **AND** the body contains `idempotency key already used`
- **AND** the `ACME` quantity is still 1

#### Scenario: Missing key applies again
- **WHEN** two buys of 1 `ACME` omit `idempotency_key`
- **THEN** both responses are HTTP 201
- **AND** the `ACME` quantity is 2
