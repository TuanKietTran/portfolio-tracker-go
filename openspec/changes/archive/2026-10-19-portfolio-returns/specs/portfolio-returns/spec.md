## ADDED Requirements

### Requirement: Consistent historical return inputs
Return queries MUST capture one holdings/flow window and frozen price view. Period is(from,to] for external investor flows, with starting NAV counted once. Timestamp ties MUST use individual flow snapshots/sequence. Require supported single currency, eligible fresh marks at start/end/flow snapshots, increasing period<=3650 days,<=4096 portfolio commands and<=256 external flows.

#### Scenario: Same-time funding with fee between
- **GIVEN** starting NAV100, funding100, fee20 and funding100 at the same mid-period timestamp, ending NAV280
- **WHEN** TWR is computed
- **THEN** individual sequence snapshots yield exactly-10 percent, with fee internal and two external flows

### Requirement: Defined period TWR
TWR MUST link pre-flow NAV/previous post-flow NAV and final NAV. Ratios/final rate round half away from zero at18 places. TWR MUST be nonannualized and reject zero/total-loss capital boundaries rather than return a guessed neutral rate.

#### Scenario: Worked gain with contribution
- **GIVEN** NAV100 grows to120, then a120 contribution makes240, and ending NAV300
- **WHEN** TWR is queried
- **THEN**1.2 times1.25 minus1 gives50 percent

### Requirement: Guarded annualized MWR
MWR MUST use investor-perspective initial negative NAV, external flows and terminal positive NAV with ACT/365 fixed elapsed seconds. Equal model-time flows are netted. Require a conventional negative-to-positive sign pattern; potentially multiple-IRR patterns fail. Bisection MUST use bracket[-0.999999999999,10], at most256 iterations and both computed residual and rate-width tolerances. Rate tolerance1e-12..1e-3 and NPV tolerance1e-12..0.01 are required numeric bounds. Cash rounds at18 places for the solver; float64 year fractions/discount factors do not convert cash amounts through floats. Diagnostics MUST identify the computed model, not claim formal real-arithmetic Pow accuracy.

#### Scenario: One-year analytic return
- **GIVEN** initial NAV100 and final NAV110 with no period external flows
- **WHEN** annualized MWR is computed
- **THEN** rate approximates0.1 and both reported numeric tolerances hold

### Requirement: Honest failure/availability contract
Return200 only with a complete rate;400 for invalid/undefined/ambiguous/unbracketed/nonconvergent, FX, stale data or limits;404 for missing starting history;503 for non-snapshot-capable feed. New route MUST NOT imply portfolio lifecycle, raw history HTTP, durable payment backing or S3 proof.

#### Scenario: Solver iteration limit
- **WHEN** the supplied iteration bound cannot meet both tolerances
- **THEN**400 has no partial rate
