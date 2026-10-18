## ADDED Requirements

### Requirement: Unified production holdings
The production router MUST mount one `/v1` router. Both trade paths MUST use the same keyed accounting state. The named trade path MUST retain an empty 201 response body; identical replays MUST have no further effect, conflicts MUST return 409, missing keys/unsupported sides/oversells MUST return 400.

#### Scenario: Trade then value through either view
- **GIVEN** a keyed buy of two units at 10 via `/trades` and a mark of 15
- **WHEN** position and summary queries use the mark's currency and timestamp
- **THEN** both report quantity 2 and the summary reports value 30 and unrealized gain 10
- **AND** a subsequent sell through the position route changes the same summary holdings

### Requirement: Explicit marked summary
Summary MUST require currency, positive integer as_of and non-negative integer max_age. It MUST use one isolated holdings snapshot, sort positions by symbol, and fail with 400 for missing/stale/invalid marks instead of returning placeholder zeros. Empty holdings MUST return an empty array and zero security value. TotalValue is security market value only, not cash-inclusive NAV.

#### Scenario: Missing valuation semantics
- **WHEN** summary lacks explicit as_of or max_age, or a required mark is absent or stale
- **THEN** it returns 400, not an invented zero valuation
