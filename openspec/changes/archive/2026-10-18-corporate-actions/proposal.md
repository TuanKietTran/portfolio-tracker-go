# Keyed atomic corporate actions

Expose POST /v1/portfolios/{id}/corporate-actions for immediate local SPLIT, DIVIDEND and FEE actions. Apply holdings, signed cash effect, idempotency identity and original response together under the memory-state lock. Split/dividend require held shares; fees may apply to a previously held closed position. Cash amounts are total amounts, not rates. Require matching immutable currency and exact minor units; reject irrelevant nonzero cash/ratio fields.

Retain exact split basis/18-place representability rules and require caller-published post-split marks rather than silently rewriting price history. Replays return the original position/cash snapshot after later actions; changed payload returns409. Failed fee debit or split must not consume keys or partially mutate lots/cash. Corporate keys use a separate global namespace. Replace unused unkeyed repository/service split mutation helpers; preserve all pure split tests and historical playback assertions.

No scheduled/retroactive ownership processing, cash-in-lieu, durable recovery or payment posting claim. Pure return methods and future cash-flow histories remain separate cards.
