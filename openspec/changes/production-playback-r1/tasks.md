# Tasks

- [x] Add bounded worker playback and retryable per-attempt timeouts.
- [x] Use capped exponential full jitter with a maximum of six attempts.
- [x] Preserve the legacy 1,000-call playback regression separately.
- [x] Exercise production api.NewRouter over a real loopback listener with 10,000 calls, fault injection, duplicate submissions, and direct cash/position assertions.
- [ ] Run full module gates, update current playback spec/design and project checkpoint, archive this change, commit and gated-push.
