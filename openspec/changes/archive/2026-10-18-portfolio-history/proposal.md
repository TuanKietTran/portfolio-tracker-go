# Timestamped local command and holdings/cash history

Record every successful portfolio cash/trade/corporate command with an injected UTC clock, per-portfolio sequence and isolated full holdings/cash snapshot under the same lock as balances and replay identity. Retain command metadata/signed cash effect. Classify only FUND/WITHDRAW as investor-perspective external flows (negative contribution, positive withdrawal); trades, dividends, fees and splits are internal investment effects.

Clock timestamps must be positive Unix time within supported JSON years. Equal timestamps are allowed and ordered by sequence; invalid or regressing per-portfolio clock rejects a new command with503 before any mutation/key/sequence consumption. Successful-key replays do not consult the clock or append history. Deterministic sequence IDs avoid random event identities.

Expose bounded repository history paging and historical snapshot lookup for future return queries, not new HTTP routes. Snapshot lookup resolves ties to highest committed sequence, never current holdings. Preserve existing current-position/price-cutoff API semantics. Do not claim historical NAV/returns until historical holdings and marks are actually combined and tested.

Full snapshots trade memory for simple auditable local correctness and grow with commands times holdings. This is neither a durable event store nor a high-throughput storage design; Postgres/outbox/recovery and bounded retention remain future work. Instrument registration/price publication are market metadata, not investor portfolio-flow events. Raw keys are not included in returned history records.
