package repository

import (
	"context"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
)

type PortfolioRepository interface {
	SaveTrade(ctx context.Context, t domain.Trade) error
	GetPositions(ctx context.Context, id domain.PortfolioID) ([]domain.Position, error)
}
