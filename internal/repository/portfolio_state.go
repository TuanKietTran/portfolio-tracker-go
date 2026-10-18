package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrInsufficientCash = errors.New("insufficient cash")
var ErrIdempotencyConflict = errors.New("idempotency key payload conflict")

// PortfolioState is the atomic persistence boundary for positions and portfolio cash.
type PortfolioState interface {
	ApplyTrade(context.Context, uuid.UUID, string, domain.AccountingEvent, domain.CostMethod, string) (domain.AccountPosition, error)
	GetAccountPosition(context.Context, uuid.UUID, string) (domain.AccountPosition, error)
	ListAccountPositions(context.Context, uuid.UUID) (map[string]domain.AccountPosition, error)
	CashBalance(context.Context, uuid.UUID, string) (decimal.Decimal, error)
	MoveCash(context.Context, uuid.UUID, string, string, decimal.Decimal, string) (decimal.Decimal, error)
	ApplySplit(context.Context, uuid.UUID, string, int64, int64) (domain.AccountPosition, error)
}

type stateKey struct {
	portfolio uuid.UUID
	symbol    string
}
type cashKey struct {
	portfolio uuid.UUID
	currency  string
}
type cashCommand struct {
	portfolio           uuid.UUID
	currency, direction string
	amount              decimal.Decimal
}

type tradeCommand struct {
	portfolio uuid.UUID
	symbol    string
	event     domain.AccountingEvent
	method    domain.CostMethod
	result    domain.AccountPosition
}

func (c tradeCommand) matches(other tradeCommand) bool {
	return c.portfolio == other.portfolio && c.symbol == other.symbol && c.method == other.method &&
		c.event.Kind == other.event.Kind && c.event.Quantity.Equal(other.event.Quantity) &&
		c.event.UnitPrice.Equal(other.event.UnitPrice) && c.event.Amount.Equal(other.event.Amount)
}

// InMemoryState serializes position and cash changes. It is not durable across restarts.
type InMemoryState struct {
	mu        sync.RWMutex
	positions map[stateKey]domain.AccountPosition
	cash      map[cashKey]decimal.Decimal
	cashKeys  map[string]cashCommand
	tradeKeys map[string]tradeCommand
}

func NewInMemoryState() *InMemoryState {
	return &InMemoryState{positions: map[stateKey]domain.AccountPosition{}, cash: map[cashKey]decimal.Decimal{}, cashKeys: map[string]cashCommand{}, tradeKeys: map[string]tradeCommand{}}
}

func (s *InMemoryState) ApplyTrade(_ context.Context, portfolio uuid.UUID, symbol string, event domain.AccountingEvent, method domain.CostMethod, idempotencyKey string) (domain.AccountPosition, error) {
	if idempotencyKey == "" {
		return domain.AccountPosition{}, errors.New("idempotency key required")
	}
	if portfolio == uuid.Nil || symbol == "" {
		return domain.AccountPosition{}, errors.New("portfolio and symbol required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd := tradeCommand{portfolio: portfolio, symbol: symbol, event: event, method: method}
	if previous, ok := s.tradeKeys[idempotencyKey]; ok {
		if !previous.matches(cmd) {
			return domain.AccountPosition{}, ErrIdempotencyConflict
		}
		return previous.result.Clone(), nil
	}
	key := stateKey{portfolio, symbol}
	updated, err := domain.ApplyEvent(s.positions[key], event, method)
	if err != nil {
		return domain.AccountPosition{}, err
	}
	s.positions[key] = updated
	cmd.result = updated.Clone()
	s.tradeKeys[idempotencyKey] = cmd
	return updated.Clone(), nil
}

func (s *InMemoryState) GetAccountPosition(_ context.Context, portfolio uuid.UUID, symbol string) (domain.AccountPosition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.positions[stateKey{portfolio, symbol}]
	return p.Clone(), nil
}

func (s *InMemoryState) ListAccountPositions(_ context.Context, portfolio uuid.UUID) (map[string]domain.AccountPosition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	positions := make(map[string]domain.AccountPosition)
	for key, p := range s.positions {
		if key.portfolio == portfolio {
			positions[key.symbol] = p.Clone()
		}
	}
	return positions, nil
}

func (s *InMemoryState) CashBalance(_ context.Context, portfolio uuid.UUID, currency string) (decimal.Decimal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cash[cashKey{portfolio, currency}], nil
}

// MoveCash executes one local atomic cash-ledger command. Same key and payload is a replay;
// changed payload conflicts. Withdrawal cannot make a currency balance negative.
func (s *InMemoryState) MoveCash(_ context.Context, portfolio uuid.UUID, currency, direction string, amount decimal.Decimal, idempotencyKey string) (decimal.Decimal, error) {
	if idempotencyKey == "" {
		return decimal.Zero, errors.New("idempotency key required")
	}
	if amount.Sign() <= 0 {
		return decimal.Zero, errors.New("cash amount must be positive")
	}
	if direction != "FUND" && direction != "WITHDRAW" {
		return decimal.Zero, errors.New("unsupported cash direction")
	}
	if _, err := domain.NewMoney(amount, currency); err != nil {
		return decimal.Zero, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd := cashCommand{portfolio: portfolio, currency: currency, direction: direction, amount: amount}
	if previous, ok := s.cashKeys[idempotencyKey]; ok {
		if previous.portfolio != cmd.portfolio || previous.currency != cmd.currency || previous.direction != cmd.direction || !previous.amount.Equal(cmd.amount) {
			return decimal.Zero, ErrIdempotencyConflict
		}
		return s.cash[cashKey{portfolio, currency}], nil
	}
	key := cashKey{portfolio, currency}
	balance := s.cash[key]
	if direction == "WITHDRAW" {
		if balance.LessThan(amount) {
			return balance, ErrInsufficientCash
		}
		balance = balance.Sub(amount)
	} else {
		balance = balance.Add(amount)
	}
	s.cash[key] = balance
	s.cashKeys[idempotencyKey] = cmd
	return balance, nil
}

func (s *InMemoryState) ApplySplit(_ context.Context, portfolio uuid.UUID, symbol string, numerator, denominator int64) (domain.AccountPosition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := stateKey{portfolio, symbol}
	position, err := domain.ApplySplit(s.positions[key], numerator, denominator)
	if err != nil {
		return domain.AccountPosition{}, fmt.Errorf("apply split: %w", err)
	}
	s.positions[key] = position
	return position.Clone(), nil
}
