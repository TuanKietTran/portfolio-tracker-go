package command

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PriceFeed is injected so tests and local runs use deterministic marks.
type PriceFeed = repository.PriceFeed

type PriceFunc func(context.Context, string, string) (domain.Price, error)

func (f PriceFunc) Price(ctx context.Context, symbol, currency string) (domain.Price, error) {
	return f(ctx, symbol, currency)
}

var ErrPaymentCashUnavailable = errors.New("payment-backed cash operation unavailable")

type PaymentCashPort interface {
	Move(context.Context, uuid.UUID, string, string, decimal.Decimal, string) error
}

type PortfolioService struct {
	state    repository.PortfolioState
	prices   PriceFeed
	payments PaymentCashPort
	method   domain.CostMethod
	cashMu   *sync.Mutex
}

func NewPortfolioService(state repository.PortfolioState, prices PriceFeed, payments PaymentCashPort, method domain.CostMethod) (*PortfolioService, error) {
	if state == nil || prices == nil {
		return nil, errors.New("portfolio state and price feed are required")
	}
	if method != domain.FIFO && method != domain.AverageCost {
		return nil, errors.New("unsupported accounting method")
	}
	return &PortfolioService{state: state, prices: prices, payments: payments, method: method, cashMu: &sync.Mutex{}}, nil
}

func (s *PortfolioService) Trade(ctx context.Context, portfolio uuid.UUID, symbol string, event domain.AccountingEvent, key string) (domain.AccountPosition, error) {
	if s.payments != nil {
		s.cashMu.Lock()
		defer s.cashMu.Unlock()
	}
	if symbol == "" || strings.TrimSpace(symbol) != symbol {
		return domain.AccountPosition{}, errors.New("symbol is required")
	}
	method := s.method
	p, err := s.state.GetPortfolio(ctx, portfolio)
	if err == nil {
		method = p.CostMethod
	} else if !errors.Is(err, repository.ErrNotFound) {
		return domain.AccountPosition{}, err
	}
	return s.state.ApplyTrade(ctx, portfolio, symbol, event, method, key)
}

func (s *PortfolioService) Position(ctx context.Context, portfolio uuid.UUID, symbol, currency string, asOf, maxAge int64) (domain.AccountPosition, domain.PositionValuation, error) {
	p, err := s.state.GetAccountPosition(ctx, portfolio, symbol)
	if err != nil {
		return domain.AccountPosition{}, domain.PositionValuation{}, err
	}
	price, err := s.priceAt(ctx, symbol, currency, asOf)
	if err != nil {
		return domain.AccountPosition{}, domain.PositionValuation{}, err
	}
	v, err := domain.ValuePosition(p, symbol, currency, price, asOf, maxAge)
	return p, v, err
}

type SummaryPosition struct {
	Symbol        string
	Quantity      decimal.Decimal
	AverageCost   decimal.Decimal
	CostBasis     decimal.Decimal
	RealizedPnL   decimal.Decimal
	MarketValue   decimal.Decimal
	UnrealizedPnL decimal.Decimal
	PriceAsOf     int64
}

type PortfolioSummary struct {
	Currency      string
	AsOf          int64
	TotalValue    decimal.Decimal
	UnrealizedPnL decimal.Decimal
	Positions     []SummaryPosition
}

// Summary values one consistent holdings snapshot with explicit as-of marks.
// Missing/stale prices fail the entire query; zero placeholder marks are never used.
func (s *PortfolioService) Summary(ctx context.Context, portfolio uuid.UUID, currency string, asOf, maxAge int64) (PortfolioSummary, error) {
	money, err := domain.NewMoney(decimal.Zero, currency)
	if err != nil {
		return PortfolioSummary{}, err
	}
	if asOf <= 0 || maxAge < 0 {
		return PortfolioSummary{}, errors.New("positive as_of and non-negative max_age required")
	}
	positions, err := s.state.ListAccountPositions(ctx, portfolio)
	if err != nil {
		return PortfolioSummary{}, err
	}
	symbols := make([]string, 0, len(positions))
	for symbol := range positions {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	result := PortfolioSummary{Currency: money.Currency, AsOf: asOf, Positions: []SummaryPosition{}}
	for _, symbol := range symbols {
		p := positions[symbol]
		price, err := s.priceAt(ctx, symbol, money.Currency, asOf)
		if err != nil {
			return PortfolioSummary{}, err
		}
		v, err := domain.ValuePosition(p, symbol, money.Currency, price, asOf, maxAge)
		if err != nil {
			return PortfolioSummary{}, err
		}
		average := decimal.Zero
		if p.Quantity.IsPositive() {
			average = p.CostBasis.Div(p.Quantity)
		}
		result.Positions = append(result.Positions, SummaryPosition{Symbol: symbol, Quantity: p.Quantity, AverageCost: average, CostBasis: p.CostBasis, RealizedPnL: p.RealizedPnL, MarketValue: v.MarketValue, UnrealizedPnL: v.UnrealizedPnL, PriceAsOf: v.AsOf})
		result.TotalValue = result.TotalValue.Add(v.MarketValue)
		result.UnrealizedPnL = result.UnrealizedPnL.Add(v.UnrealizedPnL)
	}
	return result, nil
}

func (s *PortfolioService) Cash(ctx context.Context, portfolio uuid.UUID, currency string) (decimal.Decimal, error) {
	if err := domain.ValidateCashAmount(decimal.Zero, currency); err != nil {
		return decimal.Zero, err
	}
	return s.state.CashBalance(ctx, portfolio, currency)
}

// MoveCash supports local simulation only until a durable payment outbox can
// reserve cash and recover ambiguous downstream commits. A configured external
// boundary fails closed rather than transferring before local validation.
func (s *PortfolioService) MoveCash(ctx context.Context, portfolio uuid.UUID, currency, direction string, amount decimal.Decimal, key string) (decimal.Decimal, error) {
	if key == "" || amount.Sign() <= 0 {
		return decimal.Zero, errors.New("positive amount and idempotency key required")
	}
	if direction != "FUND" && direction != "WITHDRAW" {
		return decimal.Zero, errors.New("unsupported cash direction")
	}
	if err := domain.ValidateCashAmount(amount, currency); err != nil {
		return decimal.Zero, err
	}
	if s.payments != nil {
		s.cashMu.Lock()
		defer s.cashMu.Unlock()
		balance, err := s.state.CashBalance(ctx, portfolio, currency)
		if err != nil {
			return decimal.Zero, err
		}
		if direction == "WITHDRAW" && balance.LessThan(amount) {
			return balance, repository.ErrInsufficientCash
		}
		if err := s.payments.Move(ctx, portfolio, currency, direction, amount, key); err != nil {
			var intentConflict interface{ PaymentIntentConflict() bool }
			if errors.As(err, &intentConflict) && intentConflict.PaymentIntentConflict() {
				return decimal.Zero, repository.ErrIdempotencyConflict
			}
			return decimal.Zero, ErrPaymentCashUnavailable
		}
	}
	return s.state.MoveCash(ctx, portfolio, currency, direction, amount, key)
}

func (s *PortfolioService) CreatePortfolio(ctx context.Context, currency string, method domain.CostMethod, key string) (repository.Portfolio, error) {
	return s.state.CreatePortfolio(ctx, currency, method, key)
}

func (s *PortfolioService) Portfolio(ctx context.Context, id uuid.UUID) (repository.Portfolio, error) {
	return s.state.GetPortfolio(ctx, id)
}

func (s *PortfolioService) CorporateAction(ctx context.Context, portfolio uuid.UUID, symbol string, action domain.CorporateAction, key string) (repository.CorporateResult, error) {
	if s.payments != nil {
		s.cashMu.Lock()
		defer s.cashMu.Unlock()
	}
	return s.state.ApplyCorporateAction(ctx, portfolio, symbol, action, key)
}
