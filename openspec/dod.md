# Definition of done

Met means implemented and asserted by named tests. This is a synthetic local-memory service, not production-ready. Historical playback/storyboard describes its preserved legacy adapter, not the current runtime's safety guarantees.

## Met

| Done when | Evidence |
| --- | --- |
| Explicit immutable portfolio lifecycle, source failure recovery and legacy isolation | `TestPortfolioCreationConcurrentReplayAndImmutableMetadata`, `TestPortfolioCreationFailuresDoNotConsumeKeyOrAdoptLegacy`, `TestPortfolioLifecycleHTTPConcurrentReplayAndSelectedMethod`, `TestPortfolioLifecycleHTTPUnavailableSourcesAndRecovery` |
| Both live trade paths share holdings and actual marked summary | `TestRuntimeRouterSharesTradePositionAndMarkedSummary` |
| Keyed trades replay original snapshots and reject changed payloads | `TestTradeReplayReturnsOriginalSnapshotAndRejectsChangedPayload`, `TestAccountingTradeHTTPReplayConflictAndRequiredKey` |
| Unfunded buys/withdrawals cannot overdraw local cash under concurrency | `TestConcurrentBuysAndWithdrawalsShareOneCashBoundary`, `TestConcurrentWithdrawalsNeverOverdraw` |
| Local settlement has explicit currency, precision and minor-unit rules | `TestCashAndTradeRejectUnsupportedCurrencyAndFractionalMinorUnits`, `TestRejectFractionalMinorUnitsPolicy` |
| FIFO/average basis survives repeating splits/allocations and full sale | `TestPartialAllocationsCarryResidualUntilFinalSale`, `TestAverageFullLiquidationConsumesExactRemainingBasis` |
| Positions require valid as-of price marks instead of zero placeholders | `TestSummaryDeterministicSortedMarkedSnapshot`, `TestPortfolioServiceTradesValuationAndLocalCash` |
| Supported historical TWR/MWR windows preserve flow timing and computed convergence bounds | `TestHistoricalReturnsWorkedFlowAndAnalyticIRR`, `TestSameTimeFlowsUseIndividualSnapshotsAndFeesAreInternal`, `TestGuardedIRRAnalyticConvergenceAndSameTimeAggregation`, `TestReturnHTTPAnalyticRatesDiagnosticsValidationAndNoPartialFailure` |
| Return input windows/price views are frozen and bounded | `TestReturnQueryFreezesBookAndPriceViews`, `TestConsistentReturnWindowBoundaryTiesAndIsolation`, `TestReturnWindowWorkBoundsAndFrozenPriceHistory` |
| Historical cash-inclusive NAV uses historical holdings and fresh marks, not current state | `TestHistoricalNAVUsesEarlierHoldingsAndCashAfterFullSale`, `TestHistoricalNAVHTTPExactCutoffClosedHoldingsAndNoFutureLeakage`, `TestHistoricalNAVRejectsInvalidDataAndMarksWithoutPartialResults` |
| Atomic timestamped/sequence-ordered command/holdings/cash history with isolated reads | `TestTimestampedCommandHistoryFlowsSnapshotsAndIsolation`, `TestConcurrentHistorySnapshotsAreOrderedAndAtomic` |
| External investor flows exclude internal investment actions; clock failure/replay has no partial history | `TestHistoryClockFailureRollbackReplayAndRecovery`, `TestHistoryClockHTTPUnavailableHasNoEffectAndSameKeyCanRetry` |
| Keyed atomic corporate split/dividend/fee commands with original snapshot replay | `TestCorporateWorkedDividendFeeAndSplit`, `TestCorporateHTTPConcurrentReplayCashAndWorkedValuation`, `TestCorporateConcurrentReplayOriginalSnapshotAndAtomicCash` |
| Corporate fee contention cannot overdraw or partially apply failed commands | `TestConcurrentCorporateFeesCannotOverdrawAndFailureDoesNotConsumeKey`, `TestCorporateInvalidFieldsAndClosedHoldings` |
| Keyed immutable instruments and historical marks with explicit cutoffs | `TestInstrumentAndPriceReplayImmutableHistoryAndConcurrency`, `TestMarketHTTPManagementAndHistoricalPortfolioValuation`, `TestReadOnlyFeedCompatibilityAndPublicationUnavailable` |
| Operational config rejects unsupported modes/adapters | `TestValidatedRuntimeConfiguration` |
| Health/readiness/metrics, request IDs, uniform errors, panic recovery, process gauges and loopback pprof | `TestOperationalRequestsRecoveryReadinessAndMetrics`; `TestValidatedRuntimeConfiguration` verifies connection gauge transitions |
| Mutation bodies are bounded and reject unknown/trailing JSON | `TestStrictBoundedBodiesAndUniformErrors` |
| Shutdown drains active work or forces bounded closure | `TestServeCancellationDrainsActiveRequest`, `TestServeErrorsAndForcedShutdown`; repeated race count10 passed |
| Parsed OpenAPI paths/statuses match registered runtime routes | `TestOpenAPIParsesAndDeclaresImplementedRoutesAndStatuses` |
| Production-router R1: 10,000 calls, bounded retries, injected faults, duplicate effects and state invariants | `TestProductionRouterTenThousandRequestsWithFaultsAndDuplicates` (Windows/amd64 Go1.27.2: 10,000 completed in9.283s; 9800 unique/200 duplicates; 500 503,100 429,100 timeouts; position9800, cash200). Race-instrumented suite excludes the timed load test; standard test run proves the time target. |
| Domain >=90% and aggregate non-cmd >=80% statement coverage | Historical aggregate: 2026-10-19 macOS arm64 Go1.27.1; `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage.out` aggregate1303/1389=93.81% unique non-cmd statements. Current Windows Go1.27.2 package-local run: domain94.9%; package-local instrumentation is not an aggregate weighted profile. |

## Partial

| Capability | Remaining scope |
| --- | --- |
| Generalized returns | Bounded TWR/conventional MWR use cases/HTTP are tested; zero-capital/total-loss TWR and nonconventional IRR are explicitly rejected. No formal float-discount ULP certification claimed. |
| Payments-backed cash option | Explicit loopback payments mode creates/processes a stable intent, confirms SETTLED/LedgerTxID and clean reconciliation before local cash mutation; `TestPaymentsCashHTTPReplayRecoversAmbiguousSettlement`. Portfolio-side durable intent/outbox/restart recovery and cross-process S3 verification remain unmet. |
| Documentation | README, API examples and ADRs exist; full glossary/CI and milestone closure remain. |

## Unmet

| Capability | Limitation |
| --- | --- |
| Durable payment intent/outbox and restart recovery | Payment-backed mode requires payment settle/reconciliation before local effect, but portfolio intent state is memory-only. Integrated platform S3 has not been verified. |
| Postgres parity and restart recovery | No durable adapter/outbox; memory loses all state on process restart. |
| Cross-process/platform 10,000 request integration/load | Local production-router R1 is proved by the named 10k test; manager-owned platform S1/S2/S3/load evidence and soak remain pending. |
| CI/Docker execution | No workflow/container run claimed. |

## Historical regression evidence

The original playback tests remain unchanged, including legacy missing-key and repeated-key behavior, against `NewHandlers`/`NewInMemoryPortfolio`. These are not production server wiring. Production now rejects missing keys and accepts same-payload replay without another effect. The original storyboard is `openspec/gifs/playback-flow.gif`; it is not a native process capture.
