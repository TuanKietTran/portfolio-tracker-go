# Record trade

## Purpose

Append a trade and update that portfolio's position for the symbol.

## Requirements

### Requirement: Accept a trade
The system SHALL accept a trade for any portfolio UUID and return no body.

#### Scenario: Valid trade
- **WHEN** a client posts `symbol`, `side`, `quantity`, and `price` to `/v1/portfolios/{id}/trades` with a UUID id
- **THEN** the response is HTTP 201
- **AND** the body is empty

### Requirement: Buy updates quantity and average cost
The system SHALL add a buy's quantity and set average cost to the total cost divided by the new quantity when that quantity is positive.

#### Scenario: Two buys
- **GIVEN** a portfolio has no position in `ACME`
- **WHEN** the client buys 10 at 100 and then buys 10 at 200
- **THEN** the `ACME` quantity is 20
- **AND** the average cost is 150
- **AND** realized P&L is 0

### Requirement: Sell updates quantity and realized P&L
The system SHALL subtract a sell's quantity and add `(price - average cost) * quantity` to realized P&L. Average cost stays unchanged.

#### Scenario: Sell after the two buys
- **GIVEN** `ACME` has quantity 20 and average cost 150
- **WHEN** the client sells 5 at 180
- **THEN** the quantity is 15
- **AND** the average cost is 150
- **AND** realized P&L is 150

### Requirement: Other sides do not move the position
The system SHALL store a trade whose side is neither `BUY` nor `SELL` and SHALL leave quantity, average cost, and realized P&L unchanged, creating a zero position if the symbol is new.

#### Scenario: Side is not buy or sell
- **WHEN** the side is `HOLD`
- **THEN** the response is HTTP 201
- **AND** quantity, average cost, and realized P&L stay at their previous values, or zero if the symbol is new

### Requirement: Reject a malformed trade
The system SHALL reject a bad id or a bad body before changing positions.

#### Scenario: Portfolio id is not a UUID
- **WHEN** the path id is not a UUID
- **THEN** the response is HTTP 400 and the body contains `invalid portfolio id`

#### Scenario: Invalid JSON
- **WHEN** the body is not JSON
- **THEN** the response is HTTP 400 and the body contains `invalid json`
