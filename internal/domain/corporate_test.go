package domain

import (
	"github.com/shopspring/decimal"
	"testing"
)

func corporatePosition(t *testing.T, method CostMethod) AccountPosition {
	t.Helper()
	p, err := ApplyEvent(AccountPosition{}, AccountingEvent{Kind: Buy, Currency: "USD", Quantity: d("2"), UnitPrice: d("10")}, method)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestCorporateWorkedDividendFeeAndSplit(t *testing.T) {
	for _, method := range []CostMethod{FIFO, AverageCost} {
		original := corporatePosition(t, method)
		p, dividend, err := ApplyCorporateAction(original, CorporateAction{Kind: Dividend, Currency: "USD", Amount: d("4")})
		if err != nil || !dividend.Equal(d("4")) {
			t.Fatalf("dividend=%s err=%v", dividend, err)
		}
		p, fee, err := ApplyCorporateAction(p, CorporateAction{Kind: Fee, Currency: "USD", Amount: d("1")})
		if err != nil || !fee.Equal(d("-1")) {
			t.Fatalf("fee=%s err=%v", fee, err)
		}
		p, splitCash, err := ApplyCorporateAction(p, CorporateAction{Kind: Split, Currency: "USD", Numerator: 2, Denominator: 1})
		if err != nil || !splitCash.IsZero() || !p.Quantity.Equal(d("4")) || !p.CostBasis.Equal(d("20")) || !p.RealizedPnL.Equal(d("3")) {
			t.Fatalf("position=%+v err=%v", p, err)
		}
		if !original.Quantity.Equal(d("2")) || !original.RealizedPnL.IsZero() {
			t.Fatal("action changed original input")
		}
		if method == FIFO && (!p.Lots[0].CostBasis.Equal(d("20")) || !p.Lots[0].UnitCost.Equal(d("5")) || !original.Lots[0].Quantity.Equal(d("2"))) {
			t.Fatal("split lot conservation/isolation failed")
		}
	}
}
func TestCorporateInvalidFieldsAndClosedHoldings(t *testing.T) {
	p := corporatePosition(t, FIFO)
	for _, a := range []CorporateAction{
		{Kind: "UNKNOWN", Currency: "USD"}, {Kind: Split, Currency: "USD", Numerator: 0, Denominator: 1},
		{Kind: Split, Currency: "USD", Numerator: 1, Denominator: 1, Amount: d("1")},
		{Kind: Dividend, Currency: "USD", Amount: d("1"), Numerator: 1},
		{Kind: Fee, Currency: "USD", Amount: d("0")}, {Kind: Fee, Currency: "USD", Amount: d("-1")},
		{Kind: Fee, Currency: "USD", Amount: d("0.001")}, {Kind: Dividend, Currency: "USD", Amount: d("1e999999")},
		{Kind: Dividend, Currency: "EUR", Amount: d("1")}, {Kind: Dividend, Currency: "usd", Amount: d("1")},
	} {
		if _, _, err := ApplyCorporateAction(p, a); err == nil {
			t.Fatalf("accepted %+v", a)
		}
	}
	closed, err := ApplyEvent(p, AccountingEvent{Kind: Sell, Currency: "USD", Quantity: d("2"), UnitPrice: d("10")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []CorporateAction{{Kind: Split, Currency: "USD", Numerator: 2, Denominator: 1}, {Kind: Dividend, Currency: "USD", Amount: d("1")}} {
		if _, _, err := ApplyCorporateAction(closed, a); err == nil {
			t.Fatal("action on no shares accepted")
		}
	}
	if _, delta, err := ApplyCorporateAction(closed, CorporateAction{Kind: Fee, Currency: "USD", Amount: d("1")}); err != nil || !delta.Equal(d("-1")) {
		t.Fatalf("closed-position fee delta=%s err=%v", delta, err)
	}
	broken := closed
	broken.Method = "UNKNOWN"
	if _, _, err := ApplyCorporateAction(broken, CorporateAction{Kind: Fee, Currency: "USD", Amount: d("1")}); err == nil {
		t.Fatal("invalid cost method accepted")
	}
}

func FuzzCorporateSplitConservation(f *testing.F) {
	f.Add(int64(2), int64(1))
	f.Add(int64(1), int64(3))
	f.Add(int64(0), int64(1))
	f.Fuzz(func(t *testing.T, n, den int64) {
		if n < -1000 || n > 1000 || den < -1000 || den > 1000 {
			return
		}
		p := AccountPosition{Currency: "USD", Method: FIFO, Quantity: d("12"), CostBasis: d("120"), Lots: []Lot{{Quantity: d("12"), UnitCost: d("10"), CostBasis: d("120")}}}
		updated, delta, err := ApplyCorporateAction(p, CorporateAction{Kind: Split, Currency: "USD", Numerator: n, Denominator: den})
		if !p.Lots[0].Quantity.Equal(d("12")) {
			t.Fatal("input mutated")
		}
		if err == nil {
			if n <= 0 || den <= 0 || !delta.IsZero() || !updated.CostBasis.Equal(p.CostBasis) || !updated.Lots[0].CostBasis.Equal(p.CostBasis) {
				t.Fatalf("invalid successful split: %+v", updated)
			}
			if !updated.Quantity.Mul(decimal.NewFromInt(den)).Equal(p.Quantity.Mul(decimal.NewFromInt(n))) {
				t.Fatal("share ratio not exact")
			}
		}
	})
}
