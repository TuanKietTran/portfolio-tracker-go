# Historical cash-inclusive NAV

### Requirement: Historical holdings and cash
Valuation MUST use the final committed holdings/cash snapshot at/before cutoff, not current positions, and sum exact cash plus securities marks. Positions MUST be sorted. Other-denomination positions or nonzero other-currency cash MUST fail without implicit FX. Closed zero-share/basis positions retain realized PnL with zero value and null price timestamp; cash-only books need no prices.

#### Scenario: Later sale
- **GIVEN** funding100, two shares bought at10, mark15 and later full sale at20
- **WHEN** the cutoff is between buy and sale
- **THEN** cash80 plus securities30 gives NAV110, not current cash120

### Requirement: Exact cutoff and marks
HTTP as_of MUST be the exact start of a positive Unix second through year9999. Go valuation accepts exact UTC-normalized time.Time. Timestamp ties select greatest sequence. Marks MUST be matching, nonnegative, bounded, positive-time, at/before cutoff and fresh within max_age. No snapshot returns404; invalid/FX/mark errors return400 with no partial result. Late historical marks may revise valuations.

#### Scenario: Fractional command timestamp
- **GIVEN** a buy at110s+750ns
- **WHEN** HTTP as_of110 is requested
- **THEN** that buy is excluded, while as_of111 includes it

### Requirement: Existing routes remain distinct
Summary/position routes MUST retain current-holdings semantics and securities-only Summary.TotalValue. New historical NAV MUST NOT imply return rates, raw history HTTP, external cash or durable recovery.

#### Scenario: Closed current position
- **GIVEN** current shares are fully sold
- **WHEN** earlier historical NAV and current summary are queried
- **THEN** they intentionally differ because historical NAV reconstructs past holdings/cash
