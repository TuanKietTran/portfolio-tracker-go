# Definition of done

Met means implemented and asserted by named tests. This is a synthetic local-memory service, not production-ready. Historical playback/storyboard describes its preserved legacy adapter, not the current runtime's safety guarantees.

## Met

| Done when | Evidence |
| --- | --- |
| Both live trade paths share holdings and actual marked summary | `TestRuntimeRouterSharesTradePositionAndMarkedSummary` |
| Keyed trades replay original snapshots and reject changed payloads | `TestTradeReplayReturnsOriginalSnapshotAndRejectsChangedPayload`, `TestAccountingTradeHTTPReplayConflictAndRequiredKey` |
| Unfunded buys/withdrawals cannot overdraw local cash under concurrency | `TestConcurrentBuysAndWithdrawalsShareOneCashBoundary`, `TestConcurrentWithdrawalsNeverOverdraw` |
| Local settlement has explicit currency, precision and minor-unit rules | `TestCashAndTradeRejectUnsupportedCurrencyAndFractionalMinorUnits`, `TestRejectFractionalMinorUnitsPolicy` |
| FIFO/average basis survives repeating splits/allocations and full sale | `TestPartialAllocationsCarryResidualUntilFinalSale`, `TestAverageFullLiquidationConsumesExactRemainingBasis` |
| Positions require valid as-of price marks instead of zero placeholders | `TestSummaryDeterministicSortedMarkedSnapshot`, `TestPortfolioServiceTradesValuationAndLocalCash` |
| Keyed atomic corporate split/dividend/fee commands with original snapshot replay | `TestCorporateWorkedDividendFeeAndSplit`, `TestCorporateHTTPConcurrentReplayCashAndWorkedValuation`, `TestCorporateConcurrentReplayOriginalSnapshotAndAtomicCash` |
| Corporate fee contention cannot overdraw or partially apply failed commands | `TestConcurrentCorporateFeesCannotOverdrawAndFailureDoesNotConsumeKey`, `TestCorporateInvalidFieldsAndClosedHoldings` |
| Keyed immutable instruments and historical marks with explicit cutoffs | `TestInstrumentAndPriceReplayImmutableHistoryAndConcurrency`, `TestMarketHTTPManagementAndHistoricalPortfolioValuation`, `TestReadOnlyFeedCompatibilityAndPublicationUnavailable` |
| Operational config rejects unsupported modes/adapters | `TestValidatedRuntimeConfiguration` |
| Health/readiness/metrics, request IDs, uniform errors and panic recovery | `TestOperationalRequestsRecoveryReadinessAndMetrics` |
| Mutation bodies are bounded and reject unknown/trailing JSON | `TestStrictBoundedBodiesAndUniformErrors` |
| Shutdown drains active work or forces bounded closure | `TestServeCancellationDrainsActiveRequest`, `TestServeErrorsAndForcedShutdown`; repeated race count10 passed |
| Parsed OpenAPI paths/statuses match registered runtime routes | `TestOpenAPIParsesAndDeclaresImplementedRoutesAndStatuses` |
| Domain >=90% and aggregate non-cmd >=80% statement coverage | 2026-10-18 macOS arm64 Go1.27.1: domain95.7%; `go test -p 2 ./... -race -count=1 -coverpkg=./... -coverprofile=coverage.out` aggregate859/939=91.48% unique non-cmd statements. Package-local instrumentation773/939=82.32%; not an unweighted mean. |

## Partial

| Capability | Remaining scope |
| --- | --- |
| Returns | Pure TWR and bounded MWR methods tested; no HTTP or stored investor cash-flow history. |
| Documentation | README and two ADRs exist; full glossary/API examples/CI and milestone closure remain. |

## Unmet

| Capability | Limitation |
| --- | --- |
| Payment-backed cash/S3 | Local simulation only; external port fails closed pending recoverable reservations/outbox and posted-payment evidence. No direct duplicate ledger postings. |
| Postgres parity and restart recovery | No durable adapter/outbox; memory loses all state on process restart. |
| Portfolio lifecycle | No explicit portfolio resource yet; nonzero UUID validation only. |
| Real-process 10,000 request integration/load | Manager-owned S1/S2/S3/load evidence pending; preserved 1000-request in-process legacy playback is not proof. |
| CI/Docker execution | No workflow/container run claimed. |

## Historical regression evidence

The original playback tests remain unchanged, including legacy missing-key and repeated-key behavior, against `NewHandlers`/`NewInMemoryPortfolio`. These are not production server wiring. Production now rejects missing keys and accepts same-payload replay without another effect. The original storyboard is `openspec/gifs/playback-flow.gif`; it is not a native process capture.
