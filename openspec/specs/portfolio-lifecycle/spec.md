# Explicit local portfolio lifecycle

### Requirement: Keyed immutable portfolio creation and lookup

#### Scenario: Concurrent creation and replay
- **GIVEN** a valid currency, explicit supported cost method and a nonblank key
- **WHEN** concurrent POST /v1/portfolios requests use the same payload and key
- **THEN** all return201 with the same generated portfolio_id, immutable currency/cost_method and UTC created_at
- **AND** GET /v1/portfolios/{id} returns200 with identical metadata
- **AND** metadata, key and a zero-flow empty CREATE snapshot commit exactly once
- **AND** later activity cannot change creation replay results
- **AND** changed currency or method with that key returns409 without effects.

#### Scenario: Source failure and rollback
- **GIVEN** a new valid creation command
- **WHEN** identity is unavailable, zero or already used, or the clock is invalid
- **THEN** return503 without consuming the key or committing metadata/history
- **AND** retrying after source recovery can succeed with the same key.

### Requirement: Metadata constrains explicit portfolio writes

#### Scenario: Selected cost method and denomination
- **GIVEN** an explicitly created AVERAGE_COST portfolio denominated in USD on a FIFO-default server
- **WHEN** trades execute
- **THEN** lots use AVERAGE_COST independently of the legacy configured default
- **AND** cash/trade/corporate writes with another supported denomination return409 before effect/key/history changes
- **AND** cash balance in another supported denomination returns409
- **AND** new successful commands cannot predate created_at.

### Requirement: Legacy compatibility without silent migration

#### Scenario: Existing UUID state remains unregistered
- **GIVEN** a legacy nonzero UUID with successful cash or holdings commands but no explicit metadata
- **WHEN** existing command routes are used
- **THEN** legacy accounting behavior remains supported
- **AND** lifecycle lookup returns404 without auto-registration
- **AND** generated creation identity colliding with that history returns503
- **AND** no currency, lot method, holdings or history is inferred, adopted or rewritten.

#### Scenario: Unknown and invalid lookup
- **GIVEN** a metadata lookup request
- **WHEN** the UUID is malformed or zero
- **THEN** return400
- **AND** an unknown nonzero UUID returns404 without creating state.
