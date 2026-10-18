package domain

import (
	"errors"
	"math"
	"sort"

	"github.com/shopspring/decimal"
)

type IRROptions struct {
	RateTolerance decimal.Decimal
	NPVTolerance  decimal.Decimal
	MaxIterations int
}
type IRRSolution struct {
	Rate         decimal.Decimal
	Iterations   int
	Residual     decimal.Decimal
	BracketWidth decimal.Decimal
}

func (o IRROptions) Validate() error {
	if err := ValidateDecimalPrecision(o.RateTolerance); err != nil {
		return err
	}
	if err := ValidateDecimalPrecision(o.NPVTolerance); err != nil {
		return err
	}
	min := decimal.RequireFromString("0.000000000001")
	if o.RateTolerance.LessThan(min) || o.RateTolerance.GreaterThan(decimal.RequireFromString("0.001")) || o.NPVTolerance.LessThan(min) || o.NPVTolerance.GreaterThan(decimal.RequireFromString("0.01")) || o.MaxIterations < 1 || o.MaxIterations > 256 {
		return errors.New("IRR tolerances out of bounds or iterations not1..256")
	}
	return nil
}

// SolveConventionalIRR is the guarded HTTP-query solver, separate from the legacy
// pure primitive. It rejects ambiguous sign patterns, limits horizon/flow count,
// and requires BOTH computed NPV residual and rate-bracket tolerance. Discount
// factors use math.Pow; cash amounts never pass through float64. These diagnostics
// bound the computed numerical model, not a formal real-arithmetic Pow error.
func SolveConventionalIRR(input []CashFlow, options IRROptions) (IRRSolution, error) {
	if err := options.Validate(); err != nil {
		return IRRSolution{}, err
	}
	if len(input) < 2 || len(input) > 258 {
		return IRRSolution{}, errors.New("IRR requires2..258 cash flows")
	}
	flows := append([]CashFlow(nil), input...)
	for _, flow := range flows {
		if math.IsNaN(flow.Year) || math.IsInf(flow.Year, 0) || flow.Year < 0 || flow.Year > 10 {
			return IRRSolution{}, errors.New("IRR ACT/365 horizon must be0..10 years")
		}
		if err := ValidateDecimalPrecision(flow.Amount); err != nil {
			return IRRSolution{}, err
		}
	}
	sort.Slice(flows, func(i, j int) bool { return flows[i].Year < flows[j].Year })
	merged := []CashFlow{}
	for _, flow := range flows {
		if len(merged) > 0 && merged[len(merged)-1].Year == flow.Year {
			merged[len(merged)-1].Amount = merged[len(merged)-1].Amount.Add(flow.Amount)
		} else {
			merged = append(merged, flow)
		}
	}
	flows = nil
	for _, flow := range merged {
		if !flow.Amount.IsZero() {
			flows = append(flows, flow)
		}
	}
	if len(flows) < 2 || !flows[0].Amount.IsNegative() || !flows[len(flows)-1].Amount.IsPositive() || flows[0].Year == flows[len(flows)-1].Year {
		return IRRSolution{}, errors.New("IRR requires a negative-to-positive conventional flow pattern")
	}
	positive := false
	for _, flow := range flows {
		if flow.Amount.IsPositive() {
			positive = true
		} else if positive {
			return IRRSolution{}, errors.New("nonconventional flow pattern may have multiple IRRs")
		}
	}
	low, high := decimal.RequireFromString("-0.999999999999"), decimal.NewFromInt(10)
	fLow, err := boundedNPV(flows, low)
	if err != nil {
		return IRRSolution{}, err
	}
	fHigh, err := boundedNPV(flows, high)
	if err != nil {
		return IRRSolution{}, err
	}
	if fLow.IsZero() {
		return IRRSolution{Rate: low}, nil
	}
	if fHigh.IsZero() {
		return IRRSolution{Rate: high}, nil
	}
	if fLow.Sign() == fHigh.Sign() {
		return IRRSolution{}, errors.New("IRR root outside supported bracket(-1,10]")
	}
	for iteration := 1; iteration <= options.MaxIterations; iteration++ {
		mid := low.Add(high).DivRound(decimal.NewFromInt(2), 18)
		residual, err := boundedNPV(flows, mid)
		if err != nil {
			return IRRSolution{}, err
		}
		width := high.Sub(low)
		if residual.IsZero() {
			width = decimal.Zero
		}
		if residual.Abs().LessThanOrEqual(options.NPVTolerance) && width.LessThanOrEqual(options.RateTolerance) {
			return IRRSolution{Rate: mid, Iterations: iteration, Residual: residual.Abs(), BracketWidth: width}, nil
		}
		if residual.Sign() == fLow.Sign() {
			low = mid
			fLow = residual
		} else {
			high = mid
		}
	}
	return IRRSolution{}, errors.New("IRR did not satisfy both residual and rate tolerance within iteration bound")
}
func boundedNPV(flows []CashFlow, rate decimal.Decimal) (decimal.Decimal, error) {
	total := decimal.Zero
	for _, flow := range flows {
		discount := math.Pow(1+rate.InexactFloat64(), flow.Year)
		if discount <= 0 || math.IsNaN(discount) || math.IsInf(discount, 0) {
			return decimal.Zero, errors.New("IRR discount factor is not finite positive")
		}
		total = total.Add(flow.Amount.DivRound(decimal.NewFromFloat(discount), 18))
	}
	return total, nil
}
