package command

import (
	"context"
	"errors"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
)

var ErrPriceManagementUnavailable = errors.New("configured price feed does not support publication")

type historicalPriceFeed interface {
	PriceAt(context.Context, string, string, int64) (domain.Price, error)
}
type pricePublisher interface {
	Publish(context.Context, domain.Price, string) (domain.Price, error)
}

func (s *PortfolioService) priceAt(ctx context.Context, symbol, currency string, asOf int64) (domain.Price, error) {
	if feed, ok := s.prices.(historicalPriceFeed); ok {
		return feed.PriceAt(ctx, symbol, currency, asOf)
	}
	return s.prices.Price(ctx, symbol, currency)
}
func (s *PortfolioService) RegisterInstrument(ctx context.Context, instrument domain.Instrument, key string) (domain.Instrument, error) {
	return s.state.RegisterInstrument(ctx, instrument, key)
}
func (s *PortfolioService) Instrument(ctx context.Context, symbol string) (domain.Instrument, error) {
	return s.state.GetInstrument(ctx, symbol)
}
func (s *PortfolioService) PublishPrice(ctx context.Context, price domain.Price, key string) (domain.Price, error) {
	instrument, err := s.state.GetInstrument(ctx, price.Symbol)
	if err != nil {
		return domain.Price{}, err
	}
	if instrument.Currency != price.Currency {
		return domain.Price{}, repository.ErrInstrumentConflict
	}
	publisher, ok := s.prices.(pricePublisher)
	if !ok {
		return domain.Price{}, ErrPriceManagementUnavailable
	}
	return publisher.Publish(ctx, price, key)
}
func (s *PortfolioService) Price(ctx context.Context, symbol, currency string, asOf, maxAge int64) (domain.Price, error) {
	if asOf <= 0 || maxAge < 0 {
		return domain.Price{}, errors.New("positive as_of and nonnegative max_age required")
	}
	instrument, err := s.state.GetInstrument(ctx, symbol)
	if err != nil {
		return domain.Price{}, err
	}
	if instrument.Currency != currency {
		return domain.Price{}, errors.New("instrument currency mismatch")
	}
	price, err := s.priceAt(ctx, symbol, currency, asOf)
	if errors.Is(err, repository.ErrPriceUnavailable) {
		return domain.Price{}, repository.ErrNotFound
	}
	if err != nil {
		return domain.Price{}, err
	}
	if _, err := domain.ValuePosition(domain.AccountPosition{}, symbol, currency, price, asOf, maxAge); err != nil {
		return domain.Price{}, err
	}
	return price, nil
}
