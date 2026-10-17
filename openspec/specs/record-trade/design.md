# Portfolio design

- `PortfolioRepository` stores trades and positions in memory, keyed by portfolio UUID. No portfolio row is created.
- A non-empty `IdempotencyKey` is stored before the trade is applied. A repeat returns `idempotency key already used: <trade uuid>` and does not change the position. An empty key is applied every time.
- Buy: if the new quantity is positive, average cost becomes `(old average * old quantity + price * quantity) / new quantity`. Quantity is always set to the new quantity.
- Sell: realized P&L gains `(price - average cost) * quantity`, then quantity decreases. A sell of a new symbol starts from a zero average cost, so realized P&L becomes `price * quantity` and quantity goes negative.
- `GetSummary` copies positions and hard-codes `TotalValue` and `UnrealizedPnL` to 0. It does not look up a market price.
- `migrations/001_init.sql` defines `portfolios`, `trades`, and `positions`. The server does not open Postgres.
