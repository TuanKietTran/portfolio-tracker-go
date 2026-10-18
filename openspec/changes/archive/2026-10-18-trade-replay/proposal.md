# Accounting trade replay and snapshot isolation

The accounting-position trade route currently applies duplicate requests twice, and shared FIFO lot slices can mutate prior results. Require an idempotency key on this route, compare semantic decimal payloads, replay the original response snapshot, and reject changed payloads with HTTP 409. Clone lots at pure-domain and persistence boundaries.

The legacy `/trades` and `/summary` route store remains unchanged in this bounded change; its playback tests remain baseline regression evidence, not evidence of the corrected accounting-route semantics. Unification and cash-backed buys remain required production-portfolio work.
