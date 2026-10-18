package repository

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/shopspring/decimal"
)

type PriceFeed interface {
	Price(context.Context, string, string) (domain.Price, error)
}
type PriceSnapshotter interface{ SnapshotPrices() PriceFeed }

// SnapshotPrices freezes all currently known marks for a consistent query view.
func (f *StaticPriceFeed) SnapshotPrices() PriceFeed {
	f.mu.RLock()
	defer f.mu.RUnlock()
	copy := NewStaticPriceFeed()
	for _, history := range f.prices {
		for _, price := range history {
			copy.set(price)
		}
	}
	return copy
}

var ErrPriceUnavailable = errors.New("price unavailable at or before requested timestamp")

// StaticPriceFeed retains deterministic historical marks. It is memory-only.
type StaticPriceFeed struct {
	mu     sync.RWMutex
	prices map[string]map[int64]domain.Price
	keys   map[string]domain.Price
}

func NewStaticPriceFeed() *StaticPriceFeed {
	return &StaticPriceFeed{prices: map[string]map[int64]domain.Price{}, keys: map[string]domain.Price{}}
}
func (f *StaticPriceFeed) set(price domain.Price) {
	key := price.Currency + "/" + price.Symbol
	if f.prices[key] == nil {
		f.prices[key] = map[int64]domain.Price{}
	}
	f.prices[key][price.AsOf] = price
}

// Set is trusted test/bootstrap injection, not the HTTP publication path.
func (f *StaticPriceFeed) Set(price domain.Price) { f.mu.Lock(); defer f.mu.Unlock(); f.set(price) }

func (f *StaticPriceFeed) Publish(_ context.Context, price domain.Price, key string) (domain.Price, error) {
	if !validManagementKey(key) {
		return domain.Price{}, errors.New("valid idempotency key required")
	}
	if err := (domain.Instrument{Symbol: price.Symbol, Currency: price.Currency}).Validate(); err != nil {
		return domain.Price{}, err
	}
	if err := domain.ValidateCashAmount(decimal.Zero, price.Currency); err != nil {
		return domain.Price{}, err
	}
	if err := domain.ValidateDecimalPrecision(price.Amount); err != nil {
		return domain.Price{}, err
	}
	if price.Amount.IsNegative() || price.AsOf <= 0 {
		return domain.Price{}, errors.New("nonnegative price and positive timestamp required")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	equal := func(a, b domain.Price) bool {
		return a.Symbol == b.Symbol && a.Currency == b.Currency && a.AsOf == b.AsOf && a.Amount.Equal(b.Amount)
	}
	if previous, ok := f.keys[key]; ok {
		if !equal(previous, price) {
			return domain.Price{}, ErrIdempotencyConflict
		}
		return previous, nil
	}
	if old, ok := f.prices[price.Currency+"/"+price.Symbol][price.AsOf]; ok && !equal(old, price) {
		return domain.Price{}, ErrPriceConflict
	}
	f.set(price)
	f.keys[key] = price
	return price, nil
}
func (f *StaticPriceFeed) Price(ctx context.Context, symbol, currency string) (domain.Price, error) {
	return f.PriceAt(ctx, symbol, currency, math.MaxInt64)
}
func (f *StaticPriceFeed) PriceAt(_ context.Context, symbol, currency string, asOf int64) (domain.Price, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var latest domain.Price
	found := false
	for timestamp, price := range f.prices[currency+"/"+symbol] {
		if timestamp <= asOf && (!found || timestamp > latest.AsOf) {
			latest = price
			found = true
		}
	}
	if !found {
		return domain.Price{}, ErrPriceUnavailable
	}
	return latest, nil
}
