## ADDED Requirements

### Requirement: Authoritative remaining basis
Each newly created FIFO lot MUST retain exact total cost basis independently of its display unit cost. Average-cost and FIFO sales MUST consume the exact remaining basis on full liquidation. Partial-sale basis is allocated at 18 fractional decimal places, half away from zero; any residual MUST remain in the unsold lot/position. Realized P&L MUST equal sale proceeds minus allocated basis.

#### Scenario: Repeating average
- **GIVEN** one unit bought at 1 and two units bought at 2 using average cost
- **WHEN** all three units are sold at 3
- **THEN** realized gain is exactly 4 and remaining basis is zero

#### Scenario: Repeating split unit cost
- **GIVEN** one unit bought at 10 and split three for one
- **WHEN** each of the three resulting units is sold at 10
- **THEN** total gain is exactly 20 and sale proceeds plus remaining basis minus realized gain remains exactly 10 after each sale

### Requirement: Exact representable split shares
Splits MUST multiply quantity by the numerator before dividing by denominator. Aggregate and individual lot share quantities MUST be exactly representable at 18 fractional decimal places; otherwise the action MUST fail without mutating input. Display unit costs may be rounded but MUST NOT determine basis allocation.

#### Scenario: Reverse split
- **GIVEN** three shares
- **WHEN** a one-for-three reverse split is applied
- **THEN** resulting quantity is exactly one

#### Scenario: Unsupported fractional share
- **GIVEN** one share
- **WHEN** a one-for-three reverse split would require repeating fractional shares
- **THEN** it is rejected because cash-in-lieu is unsupported
