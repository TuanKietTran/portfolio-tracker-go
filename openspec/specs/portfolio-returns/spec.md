# Bounded historical portfolio returns

### Requirement: Consistent historical inputs
Queries MUST capture one holdings/flow window and a frozen price view. Investor flows are selected from(from,to], with start NAV counted once. Same-time flows MUST use individual sequence snapshots. Require supported currency, fresh eligible start/end/flow marks, increasing period<=3650 days,<=4096 portfolio commands and<=256 external flows.

#### Scenario: Same-time funding and fee
- **GIVEN** start NAV100, fund100, fee20 and fund100 at one mid-period timestamp, end280
- **WHEN** TWR is queried
- **THEN** individual snapshots yield-10 percent, fee internal and two external flows

### Requirement: Period TWR
TWR MUST link pre-flow NAV/previous post-flow NAV then ending NAV, with18-place half-away ratio/final rounding. It MUST be nonannualized and reject zero/total-loss capital boundaries rather than guess a neutral rate.

#### Scenario: Contribution and growth
- **GIVEN**100 grows to120, contribution120 makes240, then end300
- **WHEN** TWR is queried
- **THEN**1.2*1.25-1 equals50 percent

### Requirement: Guarded annualized MWR
MWR MUST use negative starting NAV, external investor flows and positive terminal NAV with ACT/365 fixed seconds. Equal model-time flows are netted; require a conventional negative-to-positive sign pattern. Bisection MUST use bracket[-0.999999999999,10],<=256 iterations and BOTH computed residual/rate-width tolerances. Rate tolerance1e-12..1e-3 and NPV tolerance1e-12..0.01 are the supported bounds. Cash rounds at18 places and remains decimal; float year fractions/discount factors define the numerical model, not formal real-arithmetic Pow certification.

#### Scenario: Analytic one-year return
- **GIVEN** starting100 and ending110 with no external period flows
- **WHEN** MWR is queried
- **THEN** rate approximates0.1 and both reported numeric bounds hold

### Requirement: Explicit unsupported/error behavior
Return200 only with a complete rate;400 for invalid/undefined/ambiguous/unbracketed/nonconvergent/FX/stale/limit failures;404 for absent starting history;503 for non-snapshot-capable feed. No lifecycle, raw history HTTP, durable cash or S3 claim is implied.

#### Scenario: Iteration cap
- **WHEN** the supplied solver cap cannot meet both tolerances
- **THEN**400 has no partial rate
