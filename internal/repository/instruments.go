package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/shopspring/decimal"
)

var ErrNotFound = errors.New("resource not found")
var ErrInstrumentConflict = errors.New("instrument currency is immutable")
var ErrPriceConflict = errors.New("price at this timestamp is immutable")

func validManagementKey(key string) bool { return strings.TrimSpace(key) != "" && len(key) <= 200 }

// RegisterInstrument shares the denomination map and lock with first-trade
// auto-registration. Existing trade clients remain compatible, without permitting
// a registry write to redenominate previously acquired holdings.
func (s *InMemoryState) RegisterInstrument(_ context.Context, instrument domain.Instrument, key string) (domain.Instrument, error) {
	if !validManagementKey(key) {
		return domain.Instrument{}, errors.New("valid idempotency key required")
	}
	if err := instrument.Validate(); err != nil {
		return domain.Instrument{}, err
	}
	if err := domain.ValidateCashAmount(decimal.Zero, instrument.Currency); err != nil {
		return domain.Instrument{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.instrumentKeys[key]; ok {
		if old != instrument {
			return domain.Instrument{}, ErrIdempotencyConflict
		}
		return old, nil
	}
	if currency, ok := s.instrumentCurrencies[instrument.Symbol]; ok && currency != instrument.Currency {
		return domain.Instrument{}, ErrInstrumentConflict
	}
	s.instrumentCurrencies[instrument.Symbol] = instrument.Currency
	s.instrumentKeys[key] = instrument
	return instrument, nil
}
func (s *InMemoryState) GetInstrument(_ context.Context, symbol string) (domain.Instrument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	currency, ok := s.instrumentCurrencies[symbol]
	if !ok {
		return domain.Instrument{}, ErrNotFound
	}
	return domain.Instrument{Symbol: symbol, Currency: currency}, nil
}
