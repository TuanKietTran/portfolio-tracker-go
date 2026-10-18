package repository

import (
	"context"
	"errors"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Clock interface{ Now() time.Time }
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

var ErrHistoryClock = errors.New("history clock unavailable or regressed")

// PortfolioSnapshot is an isolated full holdings/cash projection, not a price or
// NAV snapshot. Decimal amounts are immutable; mutable maps/lots are deep-copied.
type PortfolioSnapshot struct {
	Positions map[string]domain.AccountPosition `json:"positions"`
	Cash      map[string]decimal.Decimal        `json:"cash"`
}

func (s PortfolioSnapshot) clone() PortfolioSnapshot {
	out := PortfolioSnapshot{Positions: map[string]domain.AccountPosition{}, Cash: map[string]decimal.Decimal{}}
	for symbol, p := range s.Positions {
		out.Positions[symbol] = p.Clone()
	}
	for currency, balance := range s.Cash {
		out.Cash[currency] = balance
	}
	return out
}

// HistoryRecord orders successful portfolio commands by per-portfolio sequence.
// InvestorFlow is investor-perspective: contribution negative, withdrawal positive.
// Trade/dividend/fee/split effects are NOT investor contributions/withdrawals.
type HistoryRecord struct {
	Sequence     uint64            `json:"sequence"`
	OccurredAt   time.Time         `json:"occurred_at"`
	Kind         string            `json:"kind"`
	Symbol       string            `json:"symbol,omitempty"`
	Currency     string            `json:"currency"`
	Quantity     decimal.Decimal   `json:"quantity"`
	UnitPrice    decimal.Decimal   `json:"unit_price"`
	Amount       decimal.Decimal   `json:"amount"`
	Numerator    int64             `json:"numerator,omitempty"`
	Denominator  int64             `json:"denominator,omitempty"`
	CashDelta    decimal.Decimal   `json:"cash_delta"`
	InvestorFlow decimal.Decimal   `json:"investor_flow"`
	Snapshot     PortfolioSnapshot `json:"snapshot"`
}

func (r HistoryRecord) clone() HistoryRecord { r.Snapshot = r.Snapshot.clone(); return r }

// prepareHistoryLocked must run after business validation but before ANY mutation.
// Equal timestamps are valid and ordered by sequence. Clock regression/invalid
// time rejects the whole command without consuming its key or history sequence.
func (s *InMemoryState) prepareHistoryLocked(portfolio uuid.UUID, record HistoryRecord) (HistoryRecord, error) {
	now := s.clock.Now().UTC()
	if now.Unix() <= 0 || now.Year() > 9999 {
		return HistoryRecord{}, ErrHistoryClock
	}
	history := s.history[portfolio]
	if len(history) > 0 && now.Before(history[len(history)-1].OccurredAt) {
		return HistoryRecord{}, ErrHistoryClock
	}
	record.Sequence = uint64(len(history)) + 1
	record.OccurredAt = now
	return record, nil
}
func (s *InMemoryState) appendHistoryLocked(portfolio uuid.UUID, record HistoryRecord) {
	record.Snapshot = PortfolioSnapshot{Positions: map[string]domain.AccountPosition{}, Cash: map[string]decimal.Decimal{}}
	for key, p := range s.positions {
		if key.portfolio == portfolio {
			record.Snapshot.Positions[key.symbol] = p.Clone()
		}
	}
	for key, balance := range s.cash {
		if key.portfolio == portfolio {
			record.Snapshot.Cash[key.currency] = balance
		}
	}
	s.history[portfolio] = append(s.history[portfolio], record)
}

// History returns a bounded page after an exclusive sequence cursor. No HTTP
// route is exposed yet; these are the repository ports for future return queries.
func (s *InMemoryState) History(ctx context.Context, portfolio uuid.UUID, after uint64, limit int) ([]HistoryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if portfolio == uuid.Nil || limit < 1 || limit > 1000 {
		return nil, errors.New("portfolio and history limit1..1000 required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.history[portfolio]
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	result := []HistoryRecord{}
	// Sequences start at1, so after is also the next zero-based offset.
	for i := after; i < uint64(len(rows)) && len(result) < limit; i++ {
		result = append(result, rows[i].clone())
	}
	return result, nil
}

// SnapshotAt returns the final command snapshot at/before the UTC cutoff;
// timestamp ties resolve to the greatest committed sequence, never current state.
func (s *InMemoryState) SnapshotAt(ctx context.Context, portfolio uuid.UUID, asOf time.Time) (PortfolioSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return PortfolioSnapshot{}, err
	}
	asOf = asOf.UTC()
	if portfolio == uuid.Nil || asOf.Unix() <= 0 || asOf.Year() > 9999 {
		return PortfolioSnapshot{}, errors.New("portfolio and valid historical cutoff required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.history[portfolio]
	for i := len(rows) - 1; i >= 0; i-- {
		if !rows[i].OccurredAt.After(asOf) {
			return rows[i].Snapshot.clone(), nil
		}
	}
	return PortfolioSnapshot{}, ErrNotFound
}
