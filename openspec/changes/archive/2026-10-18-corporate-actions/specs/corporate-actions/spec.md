## ADDED Requirements

### Requirement: Atomic keyed corporate effects
Corporate actions MUST require a valid portfolio UUID, registered instrument, existing position, explicit matching currency and key. DIVIDEND/FEE amounts are positive total cash amounts in exact supported minor units, not per-share rates. SPLIT uses positive integer numerator/denominator and zero cash amount. Cash actions MUST have no nonzero ratio. SPLIT/DIVIDEND require current shares; FEE MAY apply after a previously held position is closed.

#### Scenario: Worked actions
- **GIVEN** funding100 and two units bought at10
- **WHEN** dividend4, fee1 and a two-for-one split are applied
- **THEN** quantity4, basis20, realized gain3 and cash83 are reported
- **AND** a new mark5 values securities at20 without automatic price-history rewriting

### Requirement: Original snapshot replay and rollback
Position/cash/key/original result MUST commit atomically. Same-key semantic-payload replay MUST return the original snapshot after later changes; changed portfolio/symbol/action/currency/amount/ratio MUST return409. Corporate keys are global in a separate namespace. Insufficient fee cash or invalid/inexact splits MUST leave cash/holdings/keys unchanged.

#### Scenario: Concurrent fees
- **GIVEN** cash80 and100 concurrent distinct fees of2
- **WHEN** all complete
- **THEN** exactly40 succeed, cash is zero and realized expense equals80

#### Scenario: Replay split after cash actions
- **GIVEN** a successful split followed by dividend and fee
- **WHEN** the split key is replayed
- **THEN** its original cash and holdings snapshot is returned and current state does not change

### Requirement: Explicit local action limitations
Split ratios MUST preserve exact total basis and representable aggregate/per-lot share quantities; unsupported repeating fractions fail without mutation. No cash-in-lieu, historical ownership replay, automatic mark adjustment, external posting or durable recovery is implied.

#### Scenario: Inexact reverse split
- **GIVEN** two held shares
- **WHEN** a one-for-three split would require repeating fractional shares
- **THEN** the action returns400 without consuming its key
