package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrInsufficientCash = errors.New("insufficient cash")
var ErrIdempotencyConflict = errors.New("idempotency key payload conflict")

// PortfolioState is the atomic persistence boundary for positions and portfolio cash.
type PortfolioState interface {
	CreatePortfolio(context.Context, string, domain.CostMethod, string) (Portfolio, error)
	GetPortfolio(context.Context, uuid.UUID) (Portfolio, error)
	ReturnWindow(context.Context, uuid.UUID, time.Time, time.Time) (ReturnWindow, error)
	History(context.Context, uuid.UUID, uint64, int) ([]HistoryRecord, error)
	SnapshotAt(context.Context, uuid.UUID, time.Time) (PortfolioSnapshot, error)
	RegisterInstrument(context.Context, domain.Instrument, string) (domain.Instrument, error)
	GetInstrument(context.Context, string) (domain.Instrument, error)
	ApplyTrade(context.Context, uuid.UUID, string, domain.AccountingEvent, domain.CostMethod, string) (domain.AccountPosition, error)
	GetAccountPosition(context.Context, uuid.UUID, string) (domain.AccountPosition, error)
	ListAccountPositions(context.Context, uuid.UUID) (map[string]domain.AccountPosition, error)
	CashBalance(context.Context, uuid.UUID, string) (decimal.Decimal, error)
	MoveCash(context.Context, uuid.UUID, string, string, decimal.Decimal, string) (decimal.Decimal, error)
	ApplyCorporateAction(context.Context, uuid.UUID, string, domain.CorporateAction, string) (CorporateResult, error)
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
	result              decimal.Decimal
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
		c.event.Kind == other.event.Kind && c.event.Currency == other.event.Currency && c.event.Quantity.Equal(other.event.Quantity) &&
		c.event.UnitPrice.Equal(other.event.UnitPrice) && c.event.Amount.Equal(other.event.Amount)
}

// InMemoryState serializes position and cash changes. It is not durable across restarts.
type InMemoryState struct {
	mu                   sync.RWMutex
	portfolios           map[uuid.UUID]Portfolio
	portfolioKeys        map[string]portfolioCreation
	ids                  IDSource
	positions            map[stateKey]domain.AccountPosition
	cash                 map[cashKey]decimal.Decimal
	cashKeys             map[string]cashCommand
	tradeKeys            map[string]tradeCommand
	instrumentCurrencies map[string]string
	instrumentKeys       map[string]domain.Instrument
	corporateKeys        map[string]corporateCommand
	clock                Clock
	history              map[uuid.UUID][]HistoryRecord
}

func NewInMemoryState() *InMemoryState {
	state, _ := NewInMemoryStateWithClock(ClockFunc(time.Now))
	return state
}

func NewInMemoryStateWithClock(clock Clock) (*InMemoryState, error) {
	return NewInMemoryStateWithSources(clock, uuid.NewRandom)
}

func NewInMemoryStateWithSources(clock Clock, ids IDSource) (*InMemoryState, error) {
	if ids == nil {
		return nil, errors.New("identity source required")
	}
	if clock == nil {
		return nil, errors.New("clock required")
	}
	if f, ok := clock.(ClockFunc); ok && f == nil {
		return nil, errors.New("clock required")
	}
	return &InMemoryState{portfolios: map[uuid.UUID]Portfolio{}, portfolioKeys: map[string]portfolioCreation{}, ids: ids, positions: map[stateKey]domain.AccountPosition{}, cash: map[cashKey]decimal.Decimal{}, cashKeys: map[string]cashCommand{}, tradeKeys: map[string]tradeCommand{}, instrumentCurrencies: map[string]string{}, instrumentKeys: map[string]domain.Instrument{}, corporateKeys: map[string]corporateCommand{}, clock: clock, history: map[uuid.UUID][]HistoryRecord{}}, nil
}

func (s *InMemoryState) ApplyTrade(_ context.Context, portfolio uuid.UUID, symbol string, event domain.AccountingEvent, method domain.CostMethod, idempotencyKey string) (domain.AccountPosition, error) {
	if idempotencyKey == "" {
		return domain.AccountPosition{}, errors.New("idempotency key required")
	}
	if portfolio == uuid.Nil {
		return domain.AccountPosition{}, errors.New("portfolio required")
	}
	if err := (domain.Instrument{Symbol: symbol, Currency: event.Currency}).Validate(); err != nil {
		return domain.AccountPosition{}, err
	}
	for _, value := range []decimal.Decimal{event.Quantity, event.UnitPrice, event.Amount} {
		if err := domain.ValidateDecimalPrecision(value); err != nil {
			return domain.AccountPosition{}, err
		}
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
	if err := s.validatePortfolioLocked(portfolio, event.Currency, method); err != nil {
		return domain.AccountPosition{}, err
	}
	if err := domain.ValidateCashAmount(decimal.Zero, event.Currency); err != nil {
		return domain.AccountPosition{}, err
	}
	if currency, exists := s.instrumentCurrencies[symbol]; exists && currency != event.Currency {
		return domain.AccountPosition{}, errors.New("instrument currency is immutable")
	}
	key := stateKey{portfolio, symbol}
	updated, err := domain.ApplyEvent(s.positions[key], event, method)
	if err != nil {
		return domain.AccountPosition{}, err
	}
	notional := event.Quantity.Mul(event.UnitPrice)
	if event.Kind == domain.Dividend || event.Kind == domain.Fee {
		notional = event.Amount
	}
	if err := domain.ValidateCashAmount(notional, event.Currency); err != nil {
		return domain.AccountPosition{}, err
	}
	cashKey := cashKey{portfolio, event.Currency}
	balance := s.cash[cashKey]
	if event.Kind == domain.Buy || event.Kind == domain.Fee {
		if balance.LessThan(notional) {
			return domain.AccountPosition{}, ErrInsufficientCash
		}
		balance = balance.Sub(notional)
	} else {
		balance = balance.Add(notional)
	}
	record, err := s.prepareHistoryLocked(portfolio, HistoryRecord{Kind: string(event.Kind), Symbol: symbol, Currency: event.Currency, Quantity: event.Quantity, UnitPrice: event.UnitPrice, Amount: notional, CashDelta: balance.Sub(s.cash[cashKey])})
	if err != nil {
		return domain.AccountPosition{}, err
	}
	updated.Currency = event.Currency
	s.cash[cashKey] = balance
	s.instrumentCurrencies[symbol] = event.Currency
	s.positions[key] = updated
	cmd.result = updated.Clone()
	s.tradeKeys[idempotencyKey] = cmd
	s.appendHistoryLocked(portfolio, record)
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
	if err := domain.ValidateCashAmount(decimal.Zero, currency); err != nil {
		return decimal.Zero, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.validatePortfolioLocked(portfolio, currency, ""); err != nil {
		return decimal.Zero, err
	}
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
	if portfolio == uuid.Nil {
		return decimal.Zero, errors.New("portfolio required")
	}
	if err := domain.ValidateCashAmount(amount, currency); err != nil {
		return decimal.Zero, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd := cashCommand{portfolio: portfolio, currency: currency, direction: direction, amount: amount}
	if previous, ok := s.cashKeys[idempotencyKey]; ok {
		if previous.portfolio != cmd.portfolio || previous.currency != cmd.currency || previous.direction != cmd.direction || !previous.amount.Equal(cmd.amount) {
			return decimal.Zero, ErrIdempotencyConflict
		}
		return previous.result, nil
	}
	if err := s.validatePortfolioLocked(portfolio, currency, ""); err != nil {
		return decimal.Zero, err
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
	delta := balance.Sub(s.cash[key])
	record, err := s.prepareHistoryLocked(portfolio, HistoryRecord{Kind: direction, Currency: currency, Amount: amount, CashDelta: delta, InvestorFlow: delta.Neg()})
	if err != nil {
		return decimal.Zero, err
	}
	s.cash[key] = balance
	cmd.result = balance
	s.cashKeys[idempotencyKey] = cmd
	s.appendHistoryLocked(portfolio, record)
	return balance, nil
}
