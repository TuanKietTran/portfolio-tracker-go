package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
)

// StaticPriceFeed is a deterministic injectable price adapter for local/test use.
type StaticPriceFeed struct {
	mu     sync.RWMutex
	prices map[string]domain.Price
}

func NewStaticPriceFeed() *StaticPriceFeed {
	return &StaticPriceFeed{prices: map[string]domain.Price{}}
}
func (f *StaticPriceFeed) Set(price domain.Price) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prices[price.Currency+"/"+price.Symbol] = price
}
func (f *StaticPriceFeed) Price(_ context.Context, symbol, currency string) (domain.Price, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	p, ok := f.prices[currency+"/"+symbol]
	if !ok {
		return domain.Price{}, errors.New("price unavailable")
	}
	return p, nil
}
