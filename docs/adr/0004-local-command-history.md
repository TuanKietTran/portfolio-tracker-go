# ADR 0004: Atomic timestamped local command snapshots

Status: accepted as a return-query foundation, not a durable event store.

Portfolio cash/trade/corporate writes append one record under the same memory lock as projections and replay identity. A record has an injected UTC wall timestamp, per-portfolio sequence, command metadata, signed cash delta and full isolated holdings/cash snapshot. Sequence is the deterministic local event identity. Raw request keys are not exposed in returned history records. Instrument/price metadata writes are outside portfolio command history.

Only FUND and WITHDRAW are external investor flows: a contribution is negative from the investor perspective, withdrawal positive. Buy/sell rebalancing, dividend income, fee expense and splits are internal investment effects and have zero InvestorFlow. This prevents treating investment income as a new contribution when computing future performance.

Clock values must have positive Unix seconds and a UTC year through9999. Equal timestamps are accepted and ordered by sequence. A regressing or invalid clock rejects a new command with503 before any balance, lot, identity or sequence mutation. Successful replays bypass the clock and append nothing; a rejected command can retry its original key after clock recovery.

Repository history pages use exclusive sequence cursors with limits1..1000. SnapshotAt selects the greatest sequence at/before cutoff, including timestamp ties. Maps and lot slices are copied on storage/read to isolate historical projections from later commands and callers. No eligible record is not-found, not fabricated zero holdings.

Full snapshots deliberately favor auditable local correctness over scale: memory grows with command count times holdings/lot size. History is lost on restart, is not cryptographically tamper-evident and has no retention/compaction design. This is not evidence of Postgres parity or durable outbox recovery. No new history/returns HTTP route is advertised. Current position/summary as_of still chooses price marks for current holdings; future historical NAV/returns must combine actual historical snapshots and appropriately selected marks rather than reuse current positions.
