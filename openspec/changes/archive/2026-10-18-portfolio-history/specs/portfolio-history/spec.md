## ADDED Requirements

### Requirement: Atomic ordered portfolio history
Successful FUND/WITHDRAW, trades and corporate actions MUST append one record with positive UTC timestamp, per-portfolio sequence starting1, command metadata, signed cash delta and independent full holdings/cash snapshot. History, projections and replay identity MUST commit together. Replays, conflicts and failed commands MUST NOT append or consume sequence. Instrument/price metadata writes are not portfolio command history. Raw request keys MUST NOT be included in returned records.

#### Scenario: Concurrent same-time commands
- **GIVEN** a fixed clock,100 identical funding requests and100 distinct buys2 against funded cash100
- **WHEN** all complete
- **THEN** funding has one effect,50 buys succeed and51 ordered records exist at the same timestamp
- **AND** each snapshot conserves cash plus basis100 without partial effects

### Requirement: Investor-perspective flow classification
Only FUND/WITHDRAW MUST produce nonzero InvestorFlow: funding negative, withdrawal positive. Buy/sell/dividend/fee/split effects MUST NOT be classified as investor contributions or withdrawals.

#### Scenario: Investment income and rebalance
- **WHEN** a dividend, fee, split and sale are recorded
- **THEN** their signed cash/holdings effects are retained while investor flow is zero

### Requirement: Clock failure rollback
Invalid positive-time/year range or clock regression for a portfolio MUST return unavailable503 before state/key/history mutation. Equal timestamps MUST be accepted and resolved by sequence. Successful replays MUST work even while the clock is unavailable.

#### Scenario: Clock recovers
- **GIVEN** valid holdings and a regressing clock
- **WHEN** a cash/trade/corporate command is rejected then retried with the same key after recovery
- **THEN** it commits once and has exactly one next history record

### Requirement: Isolated historical repository reads
History pages MUST use exclusive sequence cursors and limits1..1000. SnapshotAt MUST return the final committed snapshot at/before cutoff, greatest sequence on ties, or not-found if no eligible record. Returned maps/lots MUST NOT alias internal or later state. These Go ports MUST NOT imply implemented HTTP history/return routes, durable recovery or historical NAV computation.

#### Scenario: Current holdings differ
- **GIVEN** a held position later fully sold
- **WHEN** SnapshotAt asks for the earlier buy timestamp
- **THEN** it returns earlier holdings/cash, not current closed holdings
