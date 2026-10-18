package domain

import (
	"github.com/shopspring/decimal"
	"math"
	"strings"
	"testing"
)

func irrOptions() IRROptions {
	return IRROptions{RateTolerance: d("0.0000000001"), NPVTolerance: d("0.00000001"), MaxIterations: 128}
}
func TestGuardedIRRAnalyticConvergenceAndSameTimeAggregation(t *testing.T) {
	flows := []CashFlow{{Year: 1, Amount: d("110")}, {Year: 0, Amount: d("-100")}}
	solution, err := SolveConventionalIRR(flows, irrOptions())
	if err != nil || solution.Rate.Sub(d("0.1")).Abs().GreaterThan(d("0.00000001")) || solution.Residual.GreaterThan(irrOptions().NPVTolerance) || solution.BracketWidth.GreaterThan(irrOptions().RateTolerance) || flows[0].Year != 1 {
		t.Fatalf("solution=%+v err=%v", solution, err)
	}
	merged := []CashFlow{{Year: 0, Amount: d("-100")}, {Year: 0.5, Amount: d("-10")}, {Year: 0.5, Amount: d("10")}, {Year: 1, Amount: d("110")}}
	equal, err := SolveConventionalIRR(merged, irrOptions())
	if err != nil || !equal.Rate.Equal(solution.Rate) {
		t.Fatalf("aggregation=%+v err=%v", equal, err)
	}
	for _, options := range []IRROptions{
		{RateTolerance: d("0.001"), NPVTolerance: d("0.000000000001"), MaxIterations: 20},
		{RateTolerance: d("0.000000000001"), NPVTolerance: d("0.01"), MaxIterations: 20},
		{RateTolerance: d("0.0000000001"), NPVTolerance: d("0.00000001"), MaxIterations: 1},
	} {
		if _, err := SolveConventionalIRR(flows, options); err == nil {
			t.Fatal("one convergence criterion or iteration limit was ignored")
		}
	}
	high, err := SolveConventionalIRR([]CashFlow{{Year: 0, Amount: d("-100")}, {Year: 1, Amount: d("1100")}}, irrOptions())
	if err != nil || !high.Rate.Equal(d("10")) || high.Iterations != 0 {
		t.Fatalf("upper root=%+v err=%v", high, err)
	}
	lower := d("-0.999999999999")
	discount := decimal.NewFromFloat(math.Pow(1+lower.InexactFloat64(), 1))
	terminal := decimal.RequireFromString(discount.Mul(d("1000000000000000")).String())
	low, err := SolveConventionalIRR([]CashFlow{{Year: 0, Amount: d("-1000000000000000")}, {Year: 1, Amount: terminal}}, irrOptions())
	if err != nil || !low.Rate.Equal(lower) {
		t.Fatalf("lower root=%+v err=%v", low, err)
	}
}
func TestGuardedIRRRejectsAmbiguityUnboundedInputsAndInvalidDiscounts(t *testing.T) {
	for _, flows := range [][]CashFlow{
		nil, {{Year: 0, Amount: d("-1")}}, make([]CashFlow, 259),
		{{Year: 0, Amount: d("-100")}, {Year: 1, Amount: d("2000")}},
		{{Year: 0, Amount: d("-100")}, {Year: 0.3, Amount: d("200")}, {Year: 0.6, Amount: d("-200")}, {Year: 1, Amount: d("110")}},
		{{Year: 0, Amount: d("-100")}, {Year: 0, Amount: d("100")}},
		{{Year: 0, Amount: d("100")}, {Year: 1, Amount: d("110")}},
		{{Year: 0, Amount: d("-100")}, {Year: 1, Amount: d("0")}},
		{{Year: math.NaN(), Amount: d("-100")}, {Year: 1, Amount: d("110")}},
		{{Year: math.Inf(1), Amount: d("-100")}, {Year: 1, Amount: d("110")}},
		{{Year: -1, Amount: d("-100")}, {Year: 1, Amount: d("110")}},
		{{Year: 0, Amount: d("-100")}, {Year: 11, Amount: d("110")}},
		{{Year: 0, Amount: d("-1e999999")}, {Year: 1, Amount: d("110")}},
	} {
		if _, err := SolveConventionalIRR(flows, irrOptions()); err == nil {
			t.Fatalf("accepted flows=%+v", flows)
		}
	}
	for _, options := range []IRROptions{
		{}, {RateTolerance: d("1e999999"), NPVTolerance: d("0.0001"), MaxIterations: 100},
		{RateTolerance: d("0.0001"), NPVTolerance: d("1e999999"), MaxIterations: 100},
		{RateTolerance: d("0.1"), NPVTolerance: d("0.0001"), MaxIterations: 100},
		{RateTolerance: d("0.0001"), NPVTolerance: d("0.1"), MaxIterations: 100},
		{RateTolerance: d("0.0001"), NPVTolerance: d("0.0001"), MaxIterations: 257},
	} {
		if err := options.Validate(); err == nil {
			t.Fatalf("accepted options=%+v", options)
		}
	}
	for _, tc := range []struct {
		year float64
		rate string
	}{{0.5, "-2"}, {1, "-1"}, {10000, "10"}} {
		if _, err := boundedNPV([]CashFlow{{Year: tc.year, Amount: d("1")}}, d(tc.rate)); err == nil {
			t.Fatal("invalid discount factor accepted")
		}
	}
	if _, err := SolveConventionalIRR([]CashFlow{{Year: 0, Amount: d("-100")}, {Year: 0.5, Amount: d("10")}, {Year: 0.75, Amount: d("-10")}, {Year: 1, Amount: d("110")}}, irrOptions()); err == nil || !strings.Contains(err.Error(), "nonconventional") {
		t.Fatalf("ambiguity=%v", err)
	}
}
func FuzzGuardedIRROneYearAnalytic(f *testing.F) {
	f.Add(int64(100))
	f.Add(int64(-100))
	f.Add(int64(0))
	f.Fuzz(func(t *testing.T, scaled int64) {
		if scaled < -500 || scaled > 500 {
			return
		}
		rate := decimal.NewFromInt(scaled).Div(decimal.NewFromInt(1000))
		terminal := d("100").Mul(d("1").Add(rate))
		solution, err := SolveConventionalIRR([]CashFlow{{Year: 0, Amount: d("-100")}, {Year: 1, Amount: terminal}}, irrOptions())
		if err != nil || solution.Rate.Sub(rate).Abs().GreaterThan(d("0.00000001")) || solution.Residual.GreaterThan(irrOptions().NPVTolerance) || solution.BracketWidth.GreaterThan(irrOptions().RateTolerance) {
			t.Fatalf("rate=%s solution=%+v err=%v", rate, solution, err)
		}
	})
}
