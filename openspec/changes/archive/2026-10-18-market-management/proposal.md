# Instrument registry and deterministic historical marks

Expose keyed instrument registration/lookup and keyed simulated price publication/as-of lookup. Share denomination state/lock with first-successful-trade auto-registration so existing clients remain compatible and no registration can redenominate acquired holdings. Symbols become path-safe case-sensitive ASCII identifiers of 1..32 letters/digits/dot/underscore/hyphen.

Retain historical marks and select the latest timestamp at/before an explicit cutoff; enforce maximum age. Future marks must not hide valid older marks. Price tuples are immutable, out-of-order insertion is supported, and same-key semantic-decimal replays return original results. Validate bounded decimal work before replay comparison.

Use separate global idempotency namespaces for instruments and prices. Keep original trusted Go feed setter only for fixtures/bootstrap; HTTP writes use immutable keyed publication. Existing read-only injected feed ports continue to work for valuation but reject price publication as unavailable. All state remains in memory; no external market, durable or S3 claim.
