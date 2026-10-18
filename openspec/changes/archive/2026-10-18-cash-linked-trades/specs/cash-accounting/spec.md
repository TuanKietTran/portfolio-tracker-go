## ADDED Requirements

### Requirement: Atomic cash and position changes
Live trade writes MUST include a supported canonical currency. Successful first trade binds a symbol's currency globally in the local state; failed writes MUST NOT bind symbols, consume keys or alter cash/holdings. Position cost method MUST NOT change. BUY/FEE debits and SELL/DIVIDEND credits MUST commit atomically with positions. Insufficient funds MUST return 409 with no effect. Trade key payload identity includes currency.

#### Scenario: Concurrent buys and withdrawals
- **GIVEN** cash of 100 and 100 concurrent requests each debiting 2, divided between buys and withdrawals
- **WHEN** all requests complete
- **THEN** exactly 50 succeed, cash is zero, and position cost basis equals the successful buys' total cost

### Requirement: Exact bounded settlement
Cash/notional settlement MUST use RejectFractionalMinorUnits: USD/EUR/GBP scale 2, JPY scale 0; unsupported/lowercase currencies and fractional minor units MUST be rejected. No implicit conversion/rounding is performed. Input decimals MUST have exponent -18 through 18 and at most 30 coefficient digits before arithmetic or replay comparison.

#### Scenario: Fractional unit price
- **WHEN** the quantity times unit price is USD 0.001
- **THEN** the trade is rejected without cash or holding changes

### Requirement: Original cash replay and external fail-closed boundary
Cash same-payload replay MUST return the original balance result without any new effect, even if current cash has since changed. Header/body keys MUST agree. An external payment port without recovery-safe outbox support MUST return unavailable without attempting a transfer.

#### Scenario: Replay after spending
- **GIVEN** a keyed funding of 20 followed by a buy spending all 20
- **WHEN** the funding is replayed
- **THEN** its response reports original balance 20 while stored cash remains zero
