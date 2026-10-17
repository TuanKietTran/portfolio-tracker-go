package domain

import (
	"errors"
	"math"

	"github.com/shopspring/decimal"
)

type Price struct {
	Symbol   string
	Currency string
	Amount   decimal.Decimal
	AsOf     int64
}

type PositionValuation struct {
	MarketValue   decimal.Decimal
	UnrealizedPnL decimal.Decimal
	Currency      string
	AsOf          int64
}

// ValuePosition requires a price no older than maxAge at asOf and enforces currency equality.
func ValuePosition(position AccountPosition, symbol, currency string, price Price, asOf, maxAge int64) (PositionValuation, error) {
	if symbol != price.Symbol || currency != price.Currency {
		return PositionValuation{}, errors.New("price instrument or currency mismatch")
	}
	if price.Amount.IsNegative() {
		return PositionValuation{}, errors.New("price cannot be negative")
	}
	if price.AsOf > asOf || maxAge < 0 || asOf-price.AsOf > maxAge {
		return PositionValuation{}, errors.New("price is stale or from the future")
	}
	value := position.Quantity.Mul(price.Amount)
	return PositionValuation{MarketValue: value, UnrealizedPnL: value.Sub(position.CostBasis), Currency: currency, AsOf: price.AsOf}, nil
}

// TimeWeightedReturn geometrically links period returns. Returns must be finite and above -100%.
func TimeWeightedReturn(periodReturns []decimal.Decimal) (decimal.Decimal, error) {
	factor := decimal.NewFromInt(1)
	for _, r := range periodReturns {
		if r.LessThanOrEqual(decimal.NewFromInt(-1)) {
			return decimal.Zero, errors.New("period return must exceed -100 percent")
		}
		factor = factor.Mul(decimal.NewFromInt(1).Add(r))
	}
	return factor.Sub(decimal.NewFromInt(1)), nil
}

// MoneyWeightedReturn solves NPV=0 for dated cash flows using bounded bisection.
// years are non-negative, cash flows are investor-perspective (contribution negative).
func MoneyWeightedReturn(cashFlows []CashFlow, tolerance decimal.Decimal, maxIterations int) (decimal.Decimal, int, error) {
	if len(cashFlows) < 2 || tolerance.Sign() <= 0 || maxIterations <= 0 {
		return decimal.Zero, 0, errors.New("invalid IRR inputs")
	}
	minYear, maxYear := cashFlows[0].Year, cashFlows[0].Year
	hasPositive, hasNegative := false, false
	for _, cf := range cashFlows {
		if cf.Year < 0 || math.IsNaN(cf.Year) || math.IsInf(cf.Year, 0) {
			return decimal.Zero, 0, errors.New("cash flow year must be finite and non-negative")
		}
		if cf.Year < minYear {
			minYear = cf.Year
		}
		if cf.Year > maxYear {
			maxYear = cf.Year
		}
		if cf.Amount.IsPositive() {
			hasPositive = true
		}
		if cf.Amount.IsNegative() {
			hasNegative = true
		}
	}
	if !hasPositive || !hasNegative || maxYear == minYear {
		return decimal.Zero, 0, errors.New("cash flows do not define a return")
	}
	low, high := decimal.NewFromInt(-1).Add(tolerance), decimal.NewFromInt(10)
	fLow, fHigh := npv(cashFlows, low), npv(cashFlows, high)
	if fLow.IsZero() {
		return low, 0, nil
	}
	if fHigh.IsZero() {
		return high, 0, nil
	}
	if fLow.Sign() == fHigh.Sign() {
		return decimal.Zero, 0, errors.New("IRR root is not bracketed")
	}
	for i := 1; i <= maxIterations; i++ {
		mid := low.Add(high).Div(decimal.NewFromInt(2))
		fMid := npv(cashFlows, mid)
		if fMid.Abs().LessThanOrEqual(tolerance) || high.Sub(low).LessThanOrEqual(tolerance) {
			return mid, i, nil
		}
		if fMid.Sign() == fLow.Sign() {
			low, fLow = mid, fMid
		} else {
			high = mid
		}
	}
	return decimal.Zero, maxIterations, errors.New("IRR did not converge within iteration bound")
}

type CashFlow struct {
	Year   float64
	Amount decimal.Decimal
}

func npv(flows []CashFlow, rate decimal.Decimal) decimal.Decimal {
	total := decimal.Zero
	for _, cf := range flows {
		factor := math.Pow(1+rate.InexactFloat64(), cf.Year)
		total = total.Add(cf.Amount.Div(decimal.NewFromFloat(factor)))
	}
	return total
}
