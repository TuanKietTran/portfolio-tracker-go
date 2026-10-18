# Tasks

- [x] Require explicit supported currency on live accounting events and bind symbol denomination only on success.
- [x] Atomically debit/credit cash with position updates and preserve immutable cost method.
- [x] Prove conservation, concurrent buy/withdraw contention, insufficient-funds rollback, replay snapshots and currency conflicts.
- [x] Specify and test RejectFractionalMinorUnits and bounded decimal arithmetic.
- [x] Return original cash replay results even after spending and support agreeing header/body keys.
- [x] Fail external transfers closed until recovery-safe outbox implementation.
- [x] Run gates/fuzz, record measured coverage and archive this bounded change.
