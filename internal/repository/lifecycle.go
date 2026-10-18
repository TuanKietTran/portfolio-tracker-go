package repository

import (
	"context"
	"errors"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrPortfolioConflict = errors.New("immutable portfolio metadata conflict")
var ErrPortfolioIdentity = errors.New("portfolio identity unavailable or already in use")

// IDSource is invoked under the state lock, only for a new valid create command.
// Sources must not call back into the store.
type IDSource func() (uuid.UUID, error)

type Portfolio struct {
	ID         uuid.UUID         `json:"portfolio_id"`
	Currency   string            `json:"currency"`
	CostMethod domain.CostMethod `json:"cost_method"`
	CreatedAt  time.Time         `json:"created_at"`
}

type portfolioCreation struct {
	currency string
	method   domain.CostMethod
	result   Portfolio
}

func (s *InMemoryState) CreatePortfolio(ctx context.Context, currency string, method domain.CostMethod, key string) (Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return Portfolio{}, err
	}
	if !validManagementKey(key) {
		return Portfolio{}, errors.New("valid idempotency key required")
	}
	if err := domain.ValidateCashAmount(decimal.Zero, currency); err != nil {
		return Portfolio{}, err
	}
	if method != domain.FIFO && method != domain.AverageCost {
		return Portfolio{}, errors.New("unsupported accounting method")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Portfolio{}, err
	}
	if prior, ok := s.portfolioKeys[key]; ok {
		if prior.currency != currency || prior.method != method {
			return Portfolio{}, ErrIdempotencyConflict
		}
		return prior.result, nil
	}
	id, err := s.ids()
	if err != nil || id == uuid.Nil {
		return Portfolio{}, ErrPortfolioIdentity
	}
	if _, ok := s.portfolios[id]; ok {
		return Portfolio{}, ErrPortfolioIdentity
	}
	// All successful legacy mutations have history; never adopt their UUIDs.
	if len(s.history[id]) != 0 {
		return Portfolio{}, ErrPortfolioIdentity
	}
	record, err := s.prepareHistoryLocked(id, HistoryRecord{Kind: "CREATE", Currency: currency})
	if err != nil {
		return Portfolio{}, err
	}
	result := Portfolio{ID: id, Currency: currency, CostMethod: method, CreatedAt: record.OccurredAt}
	s.portfolios[id] = result
	s.portfolioKeys[key] = portfolioCreation{currency: currency, method: method, result: result}
	s.appendHistoryLocked(id, record)
	return result, nil
}

func (s *InMemoryState) GetPortfolio(ctx context.Context, id uuid.UUID) (Portfolio, error) {
	if err := ctx.Err(); err != nil {
		return Portfolio{}, err
	}
	if id == uuid.Nil {
		return Portfolio{}, errors.New("nonzero portfolio UUID required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.portfolios[id]
	if !ok {
		return Portfolio{}, ErrNotFound
	}
	return p, nil
}

func (s *InMemoryState) validatePortfolioLocked(id uuid.UUID, currency string, method domain.CostMethod) error {
	if p, ok := s.portfolios[id]; ok && (p.Currency != currency || method != "" && p.CostMethod != method) {
		return ErrPortfolioConflict
	}
	return nil
}
