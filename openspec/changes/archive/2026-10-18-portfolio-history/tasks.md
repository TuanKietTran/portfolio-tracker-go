# Tasks

- [x] Inject clock and assign per-portfolio UTC timestamp/sequence to successful commands.
- [x] Atomically append cloned holdings/cash snapshots and cash/command metadata with cash/trade/corporate replay identity.
- [x] Classify investor cash flows only for fund/withdraw with explicit signs.
- [x] Add repository paging and historical snapshot reads with isolation/cutoff/tie rules.
- [x] Fail invalid/regressing clocks closed before effect/key/sequence consumption while preserving successful replays.
- [x] Add deterministic concurrent ordering, rollback/recovery, isolation, business-failure and HTTP503 regressions.
- [x] Run full gates, update measured docs/current specs, archive and commit.
