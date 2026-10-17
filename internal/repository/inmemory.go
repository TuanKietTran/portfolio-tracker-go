package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
)

type InMemoryPortfolio struct {
	mu          sync.RWMutex
	trades      map[domain.PortfolioID][]domain.Trade
	positions   map[domain.PortfolioID]map[string]*domain.Position
	idempotency map[string]uuid.UUID
}

func NewInMemoryPortfolio() *InMemoryPortfolio {
	return &InMemoryPortfolio{
		trades:      make(map[domain.PortfolioID][]domain.Trade),
		positions:   make(map[domain.PortfolioID]map[string]*domain.Position),
		idempotency: make(map[string]uuid.UUID),
	}
}

func (r *InMemoryPortfolio) SaveTrade(ctx context.Context, t domain.Trade) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.IdempotencyKey != "" {
		if existing, ok := r.idempotency[t.IdempotencyKey]; ok {
			return fmt.Errorf("idempotency key already used: %s", existing)
		}
		r.idempotency[t.IdempotencyKey] = t.ID
	}
	r.trades[t.PortfolioID] = append(r.trades[t.PortfolioID], t)

	if r.positions[t.PortfolioID] == nil {
		r.positions[t.PortfolioID] = make(map[string]*domain.Position)
	}
	pos := r.positions[t.PortfolioID][t.Symbol]
	if pos == nil {
		pos = &domain.Position{Symbol: t.Symbol}
		r.positions[t.PortfolioID][t.Symbol] = pos
	}

	if t.Side == "BUY" {
		newQty := pos.Quantity.Add(t.Quantity)
		if newQty.IsPositive() {
			totalCost := pos.AverageCost.Mul(pos.Quantity).Add(t.Price.Mul(t.Quantity))
			pos.AverageCost = totalCost.Div(newQty)
		}
		pos.Quantity = newQty
	} else if t.Side == "SELL" {
		realized := t.Price.Sub(pos.AverageCost).Mul(t.Quantity)
		pos.RealizedPnL = pos.RealizedPnL.Add(realized)
		pos.Quantity = pos.Quantity.Sub(t.Quantity)
	}
	return nil
}

func (r *InMemoryPortfolio) GetPositions(ctx context.Context, id domain.PortfolioID) ([]domain.Position, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.positions[id]
	out := make([]domain.Position, 0, len(m))
	for _, p := range m {
		out = append(out, *p)
	}
	return out, nil
}
