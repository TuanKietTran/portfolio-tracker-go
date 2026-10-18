# Tasks

- [x] Require header/body idempotency key with disagreement rejection on accounting trades.
- [x] Persist command identity and original position snapshot atomically in memory.
- [x] Reject changed payloads, compare decimals semantically and allow failed-request retry.
- [x] Prove concurrent duplicates apply once and returned snapshots cannot mutate stored state.
- [x] Prove domain sell/split operations do not alias input lots, including failed sells.
- [x] Run full gates and archive this bounded change with checkpoint evidence.
