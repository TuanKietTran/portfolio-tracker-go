## ADDED Requirements

### Requirement: Accounting trade replay
The accounting-position trade route MUST require a nonempty Idempotency-Key header or idempotency_key body field. Both values MUST agree when present. Key identity is global within the accounting trade store; portfolio, symbol, event kind, semantic decimal values and accounting method identify the payload. Same-payload replay MUST return the original position snapshot with HTTP 201; changed payload MUST return HTTP 409. Invalid requests MUST NOT consume keys.

#### Scenario: Concurrent duplicates
- **GIVEN** an empty position and a keyed BUY
- **WHEN** 100 requests submit that same trade concurrently
- **THEN** every response describes one buy and the stored quantity reflects one buy only

#### Scenario: Replay after a later sell
- **GIVEN** a keyed BUY followed by a different keyed SELL
- **WHEN** the original BUY is replayed
- **THEN** the response is the original BUY snapshot and current holdings do not change

### Requirement: Independent lot snapshots
Domain event/split operations and persistence results MUST NOT alias mutable lot slices held by callers, state or cached replay results.

#### Scenario: Failed FIFO consumption
- **GIVEN** an inconsistent position whose lot quantities are below its aggregate quantity
- **WHEN** a sell exhausts lots and fails validation
- **THEN** input lots remain unchanged
