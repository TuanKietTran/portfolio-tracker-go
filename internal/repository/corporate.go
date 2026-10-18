package repository

import (
	"context"
	"errors"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CorporateResult struct {
	Position    domain.AccountPosition `json:"position"`
	CashBalance decimal.Decimal        `json:"cash_balance"`
	Currency    string                 `json:"currency"`
}

func (r CorporateResult) clone() CorporateResult { r.Position = r.Position.Clone(); return r }

type corporateCommand struct {
	portfolio uuid.UUID
	symbol    string
	action    domain.CorporateAction
	result    CorporateResult
}

func (c corporateCommand) matches(other corporateCommand) bool {
	return c.portfolio == other.portfolio && c.symbol == other.symbol && c.action.Kind == other.action.Kind && c.action.Currency == other.action.Currency && c.action.Numerator == other.action.Numerator && c.action.Denominator == other.action.Denominator && c.action.Amount.Equal(other.action.Amount)
}

func (s *InMemoryState) ApplyCorporateAction(_ context.Context, portfolio uuid.UUID, symbol string, action domain.CorporateAction, idempotencyKey string) (CorporateResult, error) {
	if !validManagementKey(idempotencyKey) {
		return CorporateResult{}, errors.New("valid idempotency key required")
	}
	if portfolio == uuid.Nil {
		return CorporateResult{}, errors.New("portfolio required")
	}
	if err := (domain.Instrument{Symbol: symbol, Currency: action.Currency}).Validate(); err != nil {
		return CorporateResult{}, err
	}
	if err := action.Validate(); err != nil {
		return CorporateResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd := corporateCommand{portfolio: portfolio, symbol: symbol, action: action}
	if previous, ok := s.corporateKeys[idempotencyKey]; ok {
		if !previous.matches(cmd) {
			return CorporateResult{}, ErrIdempotencyConflict
		}
		return previous.result.clone(), nil
	}
	currency, ok := s.instrumentCurrencies[symbol]
	if !ok {
		return CorporateResult{}, ErrNotFound
	}
	if currency != action.Currency {
		return CorporateResult{}, ErrInstrumentConflict
	}
	key := stateKey{portfolio, symbol}
	position, ok := s.positions[key]
	if !ok {
		return CorporateResult{}, ErrNotFound
	}
	updated, delta, err := domain.ApplyCorporateAction(position, action)
	if err != nil {
		return CorporateResult{}, err
	}
	cashKey := cashKey{portfolio, action.Currency}
	balance := s.cash[cashKey].Add(delta)
	if balance.IsNegative() {
		return CorporateResult{}, ErrInsufficientCash
	}
	record, err := s.prepareHistoryLocked(portfolio, HistoryRecord{Kind: string(action.Kind), Symbol: symbol, Currency: action.Currency, Amount: action.Amount, Numerator: action.Numerator, Denominator: action.Denominator, CashDelta: delta})
	if err != nil {
		return CorporateResult{}, err
	}
	result := CorporateResult{Position: updated, CashBalance: balance, Currency: action.Currency}
	s.positions[key] = updated
	s.cash[cashKey] = balance
	cmd.result = result.clone()
	s.corporateKeys[idempotencyKey] = cmd
	s.appendHistoryLocked(portfolio, record)
	return result.clone(), nil
}
