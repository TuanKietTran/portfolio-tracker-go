package domain

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PortfolioID = uuid.UUID

type Trade struct {
	ID             uuid.UUID
	PortfolioID    PortfolioID
	Symbol         string
	Side           string // BUY or SELL
	Quantity       decimal.Decimal
	Price          decimal.Decimal
	IdempotencyKey string
}

type Position struct {
	Symbol      string
	Quantity    decimal.Decimal
	AverageCost decimal.Decimal
	RealizedPnL decimal.Decimal
}
