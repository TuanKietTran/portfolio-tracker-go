# Local instrument and historical-price management

### Requirement: Immutable instrument denomination
Instrument registration and first successful trade auto-registration MUST share one denomination map and lock. Registration MUST require a key, replay the original same-payload result, and reject changed payload or denomination with409. Symbols MUST be case-sensitive path-safe ASCII identifiers1..32 characters. Registration keys are global within the registry, separate from cash/trade/price namespaces.

#### Scenario: Existing trade client
- **GIVEN** a symbol auto-registered by a successful USD buy
- **WHEN** explicit registration attempts EUR for that symbol
- **THEN** it returns409 without changing holdings or denomination

### Requirement: Deterministic historical marks
Publication MUST require a registered instrument, matching supported currency, nonnegative bounded decimal amount, positive timestamp and key. Symbol/currency/timestamp tuples MUST be immutable. Out-of-order marks MAY be inserted without replacing newer marks. Same-key semantic-decimal replay MUST return the original mark. Failed publication MUST NOT consume keys. Price keys are global within the feed.

#### Scenario: Future mark does not hide historical price
- **GIVEN** marks10 at100 and20 at200
- **WHEN** price or portfolio valuation asks for cutoff150 and max_age50
- **THEN** it selects mark10 at100, never future mark20

### Requirement: Missing/stale semantics and feed compatibility
Management lookup MUST require currency/as_of/max_age; unknown instrument or no mark at/before cutoff returns404, invalid/mismatched query or stale mark returns400. Existing portfolio missing-price errors remain400. Read-only injected feeds MUST continue supporting valuation; publication is unavailable503. All registry/price state remains memory-only.

#### Scenario: Read-only feed
- **GIVEN** an injected price function without publication support
- **WHEN** a valid registered mark is published
- **THEN** it returns503 without publication
