package command

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type HistoricalHolding struct {
	Symbol        string          `json:"symbol"`
	Quantity      decimal.Decimal `json:"quantity"`
	CostBasis     decimal.Decimal `json:"cost_basis"`
	RealizedPnL   decimal.Decimal `json:"realized_pnl"`
	MarketValue   decimal.Decimal `json:"market_value"`
	UnrealizedPnL decimal.Decimal `json:"unrealized_pnl"`
	PriceAsOf     *int64          `json:"price_as_of"`
}

type HistoricalValuation struct {
	Currency        string              `json:"currency"`
	AsOf            time.Time           `json:"as_of"`
	Cash            decimal.Decimal     `json:"cash"`
	SecuritiesValue decimal.Decimal     `json:"securities_value"`
	NetAssetValue   decimal.Decimal     `json:"net_asset_value"`
	CostBasis       decimal.Decimal     `json:"cost_basis"`
	RealizedPnL     decimal.Decimal     `json:"realized_pnl"`
	UnrealizedPnL   decimal.Decimal     `json:"unrealized_pnl"`
	Positions       []HistoricalHolding `json:"positions"`
}

// Valuation combines actual historical holdings/cash with marks at/before cutoff.
// It never uses current positions, converts currency or invents a missing mark.
func (s *PortfolioService) Valuation(ctx context.Context, portfolio uuid.UUID, currency string, asOf time.Time, maxAge int64) (HistoricalValuation, error) {
	asOf = asOf.UTC()
	if err := domain.ValidateCashAmount(decimal.Zero, currency); err != nil {
		return HistoricalValuation{}, err
	}
	if asOf.Unix() <= 0 || asOf.Year() > 9999 || maxAge < 0 {
		return HistoricalValuation{}, errors.New("positive supported cutoff and nonnegative max_age required")
	}
	snapshot, err := s.state.SnapshotAt(ctx, portfolio, asOf)
	if err != nil {
		return HistoricalValuation{}, err
	}
	for denomination, balance := range snapshot.Cash {
		if balance.IsNegative() {
			return HistoricalValuation{}, errors.New("negative historical cash balance")
		}
		if denomination != currency && !balance.IsZero() {
			return HistoricalValuation{}, errors.New("cross-currency cash valuation requires unsupported FX conversion")
		}
	}
	result := HistoricalValuation{Currency: currency, AsOf: asOf, Cash: snapshot.Cash[currency], Positions: []HistoricalHolding{}}
	symbols := make([]string, 0, len(snapshot.Positions))
	for symbol := range snapshot.Positions {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	for _, symbol := range symbols {
		p := snapshot.Positions[symbol]
		if p.Currency != currency {
			return HistoricalValuation{}, errors.New("cross-currency position valuation requires unsupported FX conversion")
		}
		if p.Quantity.IsNegative() || p.CostBasis.IsNegative() || (p.Quantity.IsZero() && !p.CostBasis.IsZero()) {
			return HistoricalValuation{}, errors.New("invalid historical position quantity or basis")
		}
		holding := HistoricalHolding{Symbol: symbol, Quantity: p.Quantity, CostBasis: p.CostBasis, RealizedPnL: p.RealizedPnL}
		if p.Quantity.IsPositive() {
			price, err := s.priceAt(ctx, symbol, currency, asOf.Unix())
			if err != nil {
				return HistoricalValuation{}, err
			}
			if price.AsOf <= 0 {
				return HistoricalValuation{}, errors.New("positive mark timestamp required")
			}
			if err := domain.ValidateDecimalPrecision(price.Amount); err != nil {
				return HistoricalValuation{}, err
			}
			value, err := domain.ValuePosition(p, symbol, currency, price, asOf.Unix(), maxAge)
			if err != nil {
				return HistoricalValuation{}, err
			}
			holding.MarketValue = value.MarketValue
			holding.UnrealizedPnL = value.UnrealizedPnL
			timestamp := price.AsOf
			holding.PriceAsOf = &timestamp
		}
		result.Positions = append(result.Positions, holding)
		result.SecuritiesValue = result.SecuritiesValue.Add(holding.MarketValue)
		result.CostBasis = result.CostBasis.Add(p.CostBasis)
		result.RealizedPnL = result.RealizedPnL.Add(p.RealizedPnL)
		result.UnrealizedPnL = result.UnrealizedPnL.Add(holding.UnrealizedPnL)
	}
	result.NetAssetValue = result.Cash.Add(result.SecuritiesValue)
	return result, nil
}
