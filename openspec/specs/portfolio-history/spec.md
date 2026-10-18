# Local portfolio command/snapshot history

### Requirement: Atomic ordered portfolio history
Successful FUND/WITHDRAW, trades and corporate actions MUST append one record with positive UTC timestamp, per-portfolio sequence starting1, metadata, signed cash delta and independent full holdings/cash snapshot. History, projections and replay identity MUST commit together. Replays/conflicts/failures MUST NOT append or consume sequence. Instrument/price metadata writes are outside this history. Raw request keys MUST NOT be in returned records.

#### Scenario: Concurrent same-time commands
- **GIVEN** a fixed clock,100 identical funding requests and100 distinct buys2 against funded cash100
- **WHEN** all complete
- **THEN** funding has one effect,50 buys succeed and51 ordered records exist at one timestamp
- **AND** every snapshot conserves cash plus basis100 without partial effects

### Requirement: Investor-perspective flows
Only FUND/WITHDRAW MUST produce nonzero InvestorFlow: funding negative, withdrawal positive. Buy/sell/dividend/fee/split MUST NOT be classified as investor contributions or withdrawals.

#### Scenario: Investment effects
- **WHEN** dividend, fee, split and sale are recorded
- **THEN** their cash/holdings effects remain while investor flow is zero

### Requirement: Clock rollback
Invalid positive-time/year range or per-portfolio regression MUST return503 before state/key/history mutation. Equal timestamps MUST be ordered by sequence. Successful replay MUST work while clock is unavailable.

#### Scenario: Recovery
- **GIVEN** valid holdings and a regressing clock
- **WHEN** cash/trade/corporate commands are rejected and retried with original keys after recovery
- **THEN** each commits once with one next history record

### Requirement: Isolated historical reads
History pages MUST use exclusive sequence cursors and limits1..1000. SnapshotAt MUST select the final snapshot at/before cutoff, greatest sequence on ties, or not-found. Returned maps/lots MUST NOT alias internal/later state. These repository ports MUST NOT imply HTTP history/return routes, durability or historical NAV computation.

#### Scenario: Sold position queried earlier
- **GIVEN** held shares later fully sold
- **WHEN** SnapshotAt asks for the earlier buy timestamp
- **THEN** it returns earlier holdings/cash, not current closed holdings
