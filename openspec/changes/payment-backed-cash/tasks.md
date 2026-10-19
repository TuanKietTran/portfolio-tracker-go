# Tasks

- [x] Read and implement only the published payment orchestration API/OpenAPI contract.
- [x] Add explicit loopback-only payments configuration; retain local cash as default.
- [x] Create stable idempotent payment intents, process/re-read settled status and ledger identity, and verify reconciliation.
- [x] Apply local cash only after successful downstream confirmation; preflight withdrawals and serialize cash-affecting commands.
- [x] Test ambiguous create/process responses and original-key replay through the production cash route.
- [ ] Complete module gates, update current specs/README/OpenAPI/ADR/DoD/project checkpoint, archive and gated-push.
