# Unified production portfolio runtime

The production server mounted two chi routers on the same `/v1` prefix, and its named trades/summary routes used a separate legacy store from positions. Mount a single router and serve both trade paths from the accounting service. Summary values a consistent, symbol-sorted holdings snapshot using explicit currency/as_of/max_age inputs; missing or stale marks fail rather than invent zeros.

The original legacy constructors and playback remain unchanged as historical regression fixtures, not production wiring. Runtime migration intentionally rejects missing keys/unsupported sides/oversells, replays valid trades with 201, and reports changed payloads with 409. Summary now requires explicit valuation inputs and returns real marks rather than zero placeholders. Existing PascalCase response names and empty `/trades` 201 body remain.

Cash-linked trades, instrument persistence, feed configuration and external funding remain follow-up production-portfolio work; no integrated S3 claim is made.
