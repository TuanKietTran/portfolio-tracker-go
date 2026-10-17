package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestValuePositionUsesAsOfAndCurrency(t *testing.T) {
	p := AccountPosition{Quantity: d("3"), CostBasis: d("20")}
	v, err := ValuePosition(p, "ACME", "USD", Price{Symbol: "ACME", Currency: "USD", Amount: d("10"), AsOf: 90}, 100, 10)
	if err != nil || !v.MarketValue.Equal(d("30")) || !v.UnrealizedPnL.Equal(d("10")) || v.AsOf != 90 {
		t.Fatalf("valuation=%+v err=%v", v, err)
	}
	for _, price := range []Price{{Symbol: "OTHER", Currency: "USD", Amount: d("10"), AsOf: 100}, {Symbol: "ACME", Currency: "EUR", Amount: d("10"), AsOf: 100}, {Symbol: "ACME", Currency: "USD", Amount: d("10"), AsOf: 80}, {Symbol: "ACME", Currency: "USD", Amount: d("10"), AsOf: 101}} {
		if _, err := ValuePosition(p, "ACME", "USD", price, 100, 10); err == nil {
			t.Fatalf("accepted invalid price %+v", price)
		}
	}
}

func TestTimeWeightedReturnGeometricallyLinksPeriods(t *testing.T) {
	got, err := TimeWeightedReturn([]decimal.Decimal{d("0.1"), d("-0.1")})
	if err != nil || !got.Equal(d("-0.01")) {
		t.Fatalf("got %s err=%v", got, err)
	}
	if _, err := TimeWeightedReturn([]decimal.Decimal{d("-1")}); err == nil {
		t.Fatal("accepted total loss")
	}
}

func TestMoneyWeightedReturnBisectionAndErrors(t *testing.T) {
	flows := []CashFlow{{Year: 0, Amount: d("-100")}, {Year: 1, Amount: d("110")}}
	got, iterations, err := MoneyWeightedReturn(flows, d("0.0000000001"), 200)
	if err != nil || iterations <= 0 || got.Sub(d("0.1")).Abs().GreaterThan(d("0.000001")) {
		t.Fatalf("IRR=%s iterations=%d err=%v", got, iterations, err)
	}
	if _, _, err := MoneyWeightedReturn(flows, d("0.0000000001"), 1); err == nil {
		t.Fatal("expected bounded convergence error")
	}
	for _, bad := range [][]CashFlow{nil, {{Year: 0, Amount: d("-1")}}, {{Year: 0, Amount: d("-1")}, {Year: 1, Amount: d("-2")}}, {{Year: 0, Amount: d("-1")}, {Year: 0, Amount: d("1")}}, {{Year: -1, Amount: d("-1")}, {Year: 1, Amount: d("1")}}} {
		if _, _, err := MoneyWeightedReturn(bad, d("0.0001"), 100); err == nil {
			t.Fatalf("accepted invalid flows %+v", bad)
		}
	}
	if _, _, err := MoneyWeightedReturn(flows, d("0"), 10); err == nil {
		t.Fatal("accepted zero tolerance")
	}
}
