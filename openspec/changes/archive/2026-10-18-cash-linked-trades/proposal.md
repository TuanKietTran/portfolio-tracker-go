# Atomic local cash-linked trades

Both live trade paths must reject unfunded buys and atomically persist cash and holdings. Bind each symbol's currency on its first successful trade and make position accounting method immutable. Replay must precede current-balance checks and return original results without any second effect.

Local settlement uses explicit USD/EUR/GBP (2 minor-unit decimals) or JPY (0). RejectFractionalMinorUnits rejects non-integral settlement amounts rather than silently rounding; quantities/prices may be fractional if their product settles exactly. Bound decimal exponent/digit work before arithmetic or replay comparisons.

The old external-cash path called payments before committing local validation and had no recovery/ambiguous-commit protection. Fail closed when such a boundary is supplied; keep local simulation available. Payment-backed funding remains blocked on an actual posted-payment contract and a safe outbox/reservation implementation. No ledger duplication or durable/S3 claim.
