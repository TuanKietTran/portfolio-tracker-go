# ADR 0002: Total basis is authoritative, unit cost is a display value

Status: accepted.

A repeating average/split unit cost cannot be represented exactly by a finite decimal. Regression tests reproduced full-liquidation realized gains of 3.9999999999999999 instead of 4, and 20.0000000000000001 instead of 20. A one-for-three reverse split of three shares also became 0.9999999999999999 when dividing the ratio before multiplying quantity.

New FIFO lots therefore retain total `CostBasis`. Sales allocate partial basis at a fixed 18 fractional decimal places with half-away-from-zero rounding and retain any remainder in the remaining basis. A full sale consumes the entire exact remaining basis. Display `UnitCost` is derived but never drives new lot-basis allocation. Older pure fixtures lacking explicit lot basis fall back to their quantity times unit cost; there is no persisted legacy database migration because the runtime is memory-only.

Splits multiply numerator first and require resulting aggregate/per-lot share quantities to be exactly representable at 18 decimal places. Unsupported fractional shares are rejected, not silently manufactured; cash-in-lieu remains unsupported. Split basis is unchanged.

`TestAverageFullLiquidationConsumesExactRemainingBasis`, `TestFIFOSplitFullLiquidationConsumesExactLotBasis`, `TestPartialAllocationsCarryResidualUntilFinalSale` and `TestReverseSplitMultipliesBeforeDividing` provide worked/conservation evidence. Fixed allocation precision does not claim arbitrary-precision rational returns; cash settlement still follows ADR 0001.
