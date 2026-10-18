# Historical cash-inclusive NAV query

Expose GET /v1/portfolios/{id}/valuation backed by actual SnapshotAt holdings/cash and PriceAt marks, not current holdings plus past prices. Sum exact decimal cash and securities value with explicit supported currency; no FX. Reject any differently denominated position or nonzero other-currency cash. Closed positions retain realized PnL but require no mark for zero quantity/basis; cash-only books need none.

The Go query accepts exact time.Time cutoffs, normalized UTC. HTTP as_of is a positive Unix second interpreted at its exact beginning (zero nanoseconds); response echoes canonical UTC date-time. Highest history sequence wins on timestamp ties. Marks use latest known timestamp at/before that second with required max_age. No snapshot is404; invalid query/FX/snapshot/missing or stale/future marks are400, never invented zeros. Late historical mark insertion can revise query results.

Preserve existing summary and single-position current-holdings semantics and securities-only Summary.TotalValue. No return rate, raw history route, durable storage, external payments or S3 claim is added.
