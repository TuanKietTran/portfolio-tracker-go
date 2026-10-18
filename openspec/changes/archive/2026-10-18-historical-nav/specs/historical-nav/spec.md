## ADDED Requirements

### Requirement: Historical cash-inclusive NAV
Valuation MUST use the final committed holdings/cash snapshot at/before cutoff, not current positions, and sum cash plus historical securities marks exactly. Positions MUST be sorted. Explicit currency MUST be supported; any other-denomination position or nonzero other-currency cash MUST fail instead of implicit FX. Zero-share/basis closed positions MUST retain realized PnL with zero value and null price timestamp; cash-only books need no prices.

#### Scenario: Later sale does not erase past holdings
- **GIVEN** cash100, then two shares bought at10 and marked15, then later fully sold at20
- **WHEN** valuation asks for a cutoff between buy and sale
- **THEN** historical cash80 plus securities30 gives NAV110, not current cash120

### Requirement: Exact cutoff and price semantics
HTTP as_of MUST be a positive Unix second at its exact beginning (zero fractional nanoseconds), through year9999. Go valuation accepts exact UTC-normalized time.Time. Highest sequence wins on history timestamp ties. Positive-share marks MUST be nonnegative, bounded, matching currency/instrument, at/before cutoff and within max_age. No eligible snapshot returns404; invalid/FX/price errors return400 with no partial valuation. Late historical mark insertion may revise results.

#### Scenario: Fractional command time
- **GIVEN** a buy committed at second110 plus750ns
- **WHEN** HTTP as_of110 is queried
- **THEN** the buy is excluded, while as_of111 includes it

#### Scenario: Missing mark
- **GIVEN** held shares with only a future mark
- **WHEN** the historical book is valued before that mark
- **THEN** it fails400, not zero or future-price substitution

### Requirement: Existing routes remain distinct
Existing summary/position routes MUST retain current-holdings price-cutoff semantics and securities-only summary TotalValue. The new route MUST NOT imply return rates, raw history HTTP, durable recovery or payment integration.

#### Scenario: Closed current position
- **GIVEN** the current position has been fully sold
- **WHEN** current summary and earlier historical valuation are requested
- **THEN** they may intentionally differ because only the historical route reconstructs past holdings and cash
