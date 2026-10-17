package command

import (
	"context"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type RecordTradeCommand struct {
	PortfolioID    domain.PortfolioID
	Symbol         string
	Side           string
	Quantity       decimal.Decimal
	Price          decimal.Decimal
	IdempotencyKey string
}

type RecordTradeHandler struct {
	repo repository.PortfolioRepository
}

func NewRecordTradeHandler(repo repository.PortfolioRepository) *RecordTradeHandler {
	return &RecordTradeHandler{repo: repo}
}

func (h *RecordTradeHandler) Handle(ctx context.Context, cmd RecordTradeCommand) error {
	trade := domain.Trade{
		ID:             uuid.New(),
		PortfolioID:    cmd.PortfolioID,
		Symbol:         cmd.Symbol,
		Side:           cmd.Side,
		Quantity:       cmd.Quantity,
		Price:          cmd.Price,
		IdempotencyKey: cmd.IdempotencyKey,
	}
	return h.repo.SaveTrade(ctx, trade)
}
