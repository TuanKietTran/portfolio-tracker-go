## ADDED Requirements

### Requirement: Immutable instrument denomination
Instrument registration and first successful trade auto-registration MUST share one denomination map and lock. Registration MUST require an idempotency key, replay the original same-payload result and reject changed payload or denomination with409. Symbols MUST be case-sensitive, path-safe ASCII identifiers1..32 characters. Registration keys are global within the registry and separate from other command namespaces.

#### Scenario: Existing trade client
- **GIVEN** a symbol auto-registered by a successful USD buy
- **WHEN** explicit registration attempts EUR for the same symbol
- **THEN** it returns409 without changing holdings or denomination

### Requirement: Deterministic historical price management
Price publication MUST require a registered instrument, supported matching currency, nonnegative bounded decimal amount, positive timestamp and key. A symbol/currency/timestamp tuple MUST be immutable. Out-of-order marks MAY be inserted without replacing newer marks. Same-key semantic-decimal replay MUST return the original mark. Failed publication MUST NOT consume keys. Price keys are global within the feed.

#### Scenario: Future mark does not hide historical price
- **GIVEN** marks10 at100 and20 at200
- **WHEN** price/portfolio valuation asks for cutoff150 and max_age50
- **THEN** it selects mark10 at100, never the future mark20

### Requirement: Explicit missing/stale semantics
Management lookup MUST require currency/as_of/max_age and return404 for unknown instrument or no mark at/before cutoff,400 for invalid/mismatched query or stale selected mark. Existing portfolio missing-price errors remain400. Read-only injected feeds MUST continue supporting valuation, with management writes unavailable503. Price management and registry remain memory-only.

#### Scenario: Read-only feed
- **GIVEN** an injected price function without publication support
- **WHEN** a valid registered mark is published
- **THEN** it returns503 without any publication
