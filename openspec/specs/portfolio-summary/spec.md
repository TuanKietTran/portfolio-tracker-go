# Portfolio summary

## Purpose

Return the positions produced by recorded trades. Market value is not priced.

## Requirements

### Requirement: Return positions and zero market figures
The system SHALL return every position for the portfolio, with `TotalValue` 0 and `UnrealizedPnL` 0.

#### Scenario: Summary after trades
- **GIVEN** trades have built one or more positions
- **WHEN** a client gets `/v1/portfolios/{id}/summary`
- **THEN** the response is HTTP 200
- **AND** `Positions` includes each symbol's quantity, average cost, and realized P&L
- **AND** `TotalValue` is 0
- **AND** `UnrealizedPnL` is 0

#### Scenario: No trades
- **WHEN** the portfolio id has no trades
- **THEN** `Positions` is empty
- **AND** `TotalValue` and `UnrealizedPnL` are 0

### Requirement: Position order is not significant
The system SHALL NOT promise an order of `Positions`.

#### Scenario: More than one symbol
- **GIVEN** the portfolio has positions in two symbols
- **WHEN** the client reads the summary twice
- **THEN** both responses contain the same positions
- **AND** the array order may differ

### Requirement: Invalid portfolio id
The system SHALL reject a summary id that is not a UUID.

#### Scenario: Not a UUID
- **WHEN** the path id is not a UUID
- **THEN** the response is HTTP 400 and the body contains `invalid portfolio id`
