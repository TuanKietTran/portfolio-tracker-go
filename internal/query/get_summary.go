package query

import (
	"context"

	"github.com/example/portfolio-tracker/internal/domain"
	"github.com/example/portfolio-tracker/internal/repository"
	"github.com/shopspring/decimal"
)

type GetSummaryQuery struct {
	PortfolioID domain.PortfolioID
}

type Summary struct {
	TotalValue    decimal.Decimal
	UnrealizedPnL decimal.Decimal
	Positions     []domain.Position
}

type GetSummaryHandler struct {
	repo repository.PortfolioRepository
}

func NewGetSummaryHandler(repo repository.PortfolioRepository) *GetSummaryHandler {
	return &GetSummaryHandler{repo: repo}
}

func (h *GetSummaryHandler) Handle(ctx context.Context, q GetSummaryQuery) (Summary, error) {
	positions, err := h.repo.GetPositions(ctx, q.PortfolioID)
	if err != nil {
		return Summary{}, err
	}
	return Summary{
		TotalValue:    decimal.NewFromInt(0),
		UnrealizedPnL: decimal.NewFromInt(0),
		Positions:     positions,
	}, nil
}
