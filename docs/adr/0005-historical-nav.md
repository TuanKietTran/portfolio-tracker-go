# ADR 0005: Historical holdings plus marks, not current holdings with old prices

Status: accepted for local historical NAV queries.

The new `/v1/portfolios/{id}/valuation` route reads the final committed holdings/cash snapshot at or before cutoff, then selects historical marks. Cash plus securities value is NAV. Decimal values are never converted through float. Any position with a different denomination or nonzero other-currency cash is rejected; FX/cross-currency aggregation is unsupported. A closed zero-quantity/basis position retains realized PnL but has zero market value and a null price timestamp. Cash-only books need no price feed call.

HTTP `as_of` is a positive Unix second at its **exact beginning**, through9999-12-31T23:59:59Z. A command at110 seconds plus750ns is excluded by as_of110 and included by as_of111. The Go query accepts exact time.Time (including nanoseconds), normalizes UTC and returns the cutoff as canonical UTC date-time. Timestamp ties select the last committed sequence. This avoids end-of-second rounding or silently including later commands.

Positive-share marks must match symbol/currency, be nonnegative/bounded, have positive timestamps at/before cutoff and satisfy required max_age seconds. Any missing/future/stale/invalid mark fails the whole query, not just one holding. No eligible holdings snapshot is404 rather than invented zero holdings; invalid query/FX/snapshot/price is400. Prices are the latest currently known marks by event timestamp; a newly published older mark can revise a past query. This is not a frozen publication-time price audit.

Existing summary/position routes intentionally retain current-holdings price-cutoff semantics, with securities-only Summary.TotalValue. They must not be reused as historical NAV. Worked regression: fund100, buy two at10, mark15, later sell all at20. The earlier book is cash80 plus securities30 = NAV110, not later cash120. `TestHistoricalNAVUsesEarlierHoldingsAndCashAfterFullSale` and the HTTP regression prove this distinction, fractional cutoffs and zero-share behavior.

This adds historical local NAV only, not TWR/MWR endpoints, raw-history HTTP, portfolio lifecycle, external payment backing, durable storage or integrated S3 evidence. Return calculations still need ordered external-flow boundary valuation and a numeric convergence contract.
