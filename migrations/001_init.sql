CREATE TABLE portfolios (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT,
    base_currency TEXT NOT NULL DEFAULT 'USD',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE trades (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL REFERENCES portfolios(id),
    symbol TEXT NOT NULL,
    side TEXT NOT NULL CHECK (side IN ('BUY', 'SELL')),
    quantity NUMERIC(20,8) NOT NULL,
    price NUMERIC(20,8) NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE positions (
    portfolio_id UUID NOT NULL REFERENCES portfolios(id),
    symbol TEXT NOT NULL,
    quantity NUMERIC(20,8) NOT NULL DEFAULT 0,
    average_cost NUMERIC(20,8) NOT NULL DEFAULT 0,
    realized_pnl NUMERIC(20,8) NOT NULL DEFAULT 0,
    PRIMARY KEY (portfolio_id, symbol)
);
