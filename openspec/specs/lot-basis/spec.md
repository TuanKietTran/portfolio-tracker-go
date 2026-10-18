# Lot-basis allocation

### Requirement: Authoritative remaining basis
New FIFO lots MUST retain total cost basis independently of display unit cost. Both cost methods MUST consume exact remaining basis on full sale. Partial-sale basis is allocated at 18 fractional decimal places, half away from zero; the residual MUST remain in the unsold lot/position. Realized P&L MUST equal proceeds minus allocated basis.

#### Scenario: Repeating average
- **GIVEN** one unit bought at 1 and two units bought at 2 using average cost
- **WHEN** all three units are sold at 3
- **THEN** realized gain is exactly 4 and remaining basis is zero

#### Scenario: Repeating split unit cost
- **GIVEN** one unit bought at 10 and split three for one
- **WHEN** each resulting unit is sold at 10
- **THEN** total gain is exactly 20 and proceeds plus remaining basis minus realized gain equals exactly 10 after each sale

### Requirement: Exact representable split shares
Splits MUST multiply numerator before dividing denominator. Aggregate and per-lot quantities MUST be exactly representable at 18 fractional decimal places or fail without mutation. Display unit costs MUST NOT determine basis allocation.

#### Scenario: Reverse split
- **GIVEN** three shares
- **WHEN** a one-for-three reverse split is applied
- **THEN** quantity is exactly one

#### Scenario: Unsupported fractional share
- **GIVEN** one share
- **WHEN** a one-for-three reverse split would require repeating fractional shares
- **THEN** it is rejected because cash-in-lieu is unsupported
