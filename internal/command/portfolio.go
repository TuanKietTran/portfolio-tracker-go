package command

import (
	"context"
	"errors"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PriceFeed is injected so tests and local runs use deterministic marks.
type PriceFeed interface {
	Price(context.Context, string, string) (domain.Price, error)
}

type PriceFunc func(context.Context, string, string) (domain.Price, error)

func (f PriceFunc) Price(ctx context.Context, symbol, currency string) (domain.Price, error) {
	return f(ctx, symbol, currency)
}

type PaymentCashPort interface {
	Move(context.Context, uuid.UUID, string, string, decimal.Decimal, string) error
}

type PortfolioService struct {
	state    repository.PortfolioState
	prices   PriceFeed
	payments PaymentCashPort
	method   domain.CostMethod
}

func NewPortfolioService(state repository.PortfolioState, prices PriceFeed, payments PaymentCashPort, method domain.CostMethod) (*PortfolioService, error) {
	if state == nil || prices == nil {
		return nil, errors.New("portfolio state and price feed are required")
	}
	if method != domain.FIFO && method != domain.AverageCost {
		return nil, errors.New("unsupported accounting method")
	}
	return &PortfolioService{state: state, prices: prices, payments: payments, method: method}, nil
}

func (s *PortfolioService) Trade(ctx context.Context, portfolio uuid.UUID, symbol string, event domain.AccountingEvent) (domain.AccountPosition, error) {
	if symbol == "" {
		return domain.AccountPosition{}, errors.New("symbol is required")
	}
	return s.state.ApplyTrade(ctx, portfolio, symbol, event, s.method)
}

func (s *PortfolioService) Position(ctx context.Context, portfolio uuid.UUID, symbol, currency string, asOf, maxAge int64) (domain.AccountPosition, domain.PositionValuation, error) {
	p, err := s.state.GetAccountPosition(ctx, portfolio, symbol)
	if err != nil {
		return domain.AccountPosition{}, domain.PositionValuation{}, err
	}
	price, err := s.prices.Price(ctx, symbol, currency)
	if err != nil {
		return domain.AccountPosition{}, domain.PositionValuation{}, err
	}
	v, err := domain.ValuePosition(p, symbol, currency, price, asOf, maxAge)
	return p, v, err
}

func (s *PortfolioService) Cash(ctx context.Context, portfolio uuid.UUID, currency string) (decimal.Decimal, error) {
	if _, err := domain.NewMoney(decimal.Zero, currency); err != nil {
		return decimal.Zero, err
	}
	return s.state.CashBalance(ctx, portfolio, currency)
}

// MoveCash invokes the injected payment boundary before reflecting a successful transfer locally.
// Its idempotency key must be honored by that boundary. The local in-memory adapter is not a durable outbox.
func (s *PortfolioService) MoveCash(ctx context.Context, portfolio uuid.UUID, currency, direction string, amount decimal.Decimal, key string) (decimal.Decimal, error) {
	if key == "" || amount.Sign() <= 0 {
		return decimal.Zero, errors.New("positive amount and idempotency key required")
	}
	if direction != "FUND" && direction != "WITHDRAW" {
		return decimal.Zero, errors.New("unsupported cash direction")
	}
	if direction == "WITHDRAW" {
		balance, err := s.state.CashBalance(ctx, portfolio, currency)
		if err != nil {
			return decimal.Zero, err
		}
		if balance.LessThan(amount) {
			return balance, repository.ErrInsufficientCash
		}
	}
	if s.payments != nil {
		if err := s.payments.Move(ctx, portfolio, currency, direction, amount, key); err != nil {
			return decimal.Zero, err
		}
	}
	return s.state.MoveCash(ctx, portfolio, currency, direction, amount, key)
}

func (s *PortfolioService) Split(ctx context.Context, portfolio uuid.UUID, symbol string, numerator, denominator int64) (domain.AccountPosition, error) {
	return s.state.ApplySplit(ctx, portfolio, symbol, numerator, denominator)
}
