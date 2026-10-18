# Explicit local portfolio lifecycle

Add keyed POST /v1/portfolios and GET /v1/portfolios/{id} with generated identity, injected clock/identity sources and immutable currency/cost method. Creation atomically records metadata, original replay result and an empty CREATE snapshot. New portfolios enforce denomination and accounting method across writes; configured method remains the legacy default only.

Compatibility: existing arbitrary-UUID commands remain supported as unregistered legacy state. Lookup returns404 for those UUIDs; creation never adopts or overwrites existing state. No automatic migration of potentially mixed-currency/method holdings. Durable migration remains future work. No payments/database/durability claim.
