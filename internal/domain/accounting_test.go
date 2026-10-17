package domain

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestMoneyRequiresAndPreservesCurrency(t *testing.T) {
	m, err := NewMoney(d("1.25"), " usd ")
	if err != nil || m.Currency != "USD" || !m.Amount.Equal(d("1.25")) {
		t.Fatalf("money=%+v err=%v", m, err)
	}
	if _, err := NewMoney(d("1"), "US"); err == nil {
		t.Fatal("accepted invalid currency")
	}
	if _, err := m.Add(Money{Amount: d("1"), Currency: "EUR"}); err == nil {
		t.Fatal("added unlike currencies")
	}
}

func TestFIFOWorkedLotsAndRealizedPnL(t *testing.T) {
	p := AccountPosition{}
	var err error
	p, err = ApplyEvent(p, AccountingEvent{Kind: Buy, Quantity: d("10"), UnitPrice: d("10")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplyEvent(p, AccountingEvent{Kind: Buy, Quantity: d("10"), UnitPrice: d("20")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplyEvent(p, AccountingEvent{Kind: Sell, Quantity: d("12"), UnitPrice: d("30")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Quantity.Equal(d("8")) || !p.CostBasis.Equal(d("160")) || !p.RealizedPnL.Equal(d("220")) {
		t.Fatalf("unexpected FIFO result: %+v", p)
	}
	if len(p.Lots) != 1 || !p.Lots[0].Quantity.Equal(d("8")) || !p.Lots[0].UnitCost.Equal(d("20")) {
		t.Fatalf("unexpected remaining lots: %+v", p.Lots)
	}
}

func TestAverageCostWorkedExample(t *testing.T) {
	p := AccountPosition{}
	for _, e := range []AccountingEvent{{Kind: Buy, Quantity: d("10"), UnitPrice: d("10")}, {Kind: Buy, Quantity: d("10"), UnitPrice: d("20")}, {Kind: Sell, Quantity: d("5"), UnitPrice: d("30")}} {
		var err error
		p, err = ApplyEvent(p, e, AverageCost)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !p.Quantity.Equal(d("15")) || !p.CostBasis.Equal(d("225")) || !p.RealizedPnL.Equal(d("75")) {
		t.Fatalf("unexpected average-cost result: %+v", p)
	}
}

func TestOversellAndInvalidEventsDoNotMutateInput(t *testing.T) {
	original := AccountPosition{Quantity: d("2"), CostBasis: d("4"), Lots: []Lot{{Quantity: d("2"), UnitCost: d("2")}}}
	got, err := ApplyEvent(original, AccountingEvent{Kind: Sell, Quantity: d("3"), UnitPrice: d("3")}, FIFO)
	if err == nil || !got.Quantity.Equal(original.Quantity) || !original.Lots[0].Quantity.Equal(d("2")) {
		t.Fatalf("oversell mutated: %+v err=%v", got, err)
	}
	if _, err := ApplyEvent(original, AccountingEvent{Kind: Buy, Quantity: d("0"), UnitPrice: d("1")}, FIFO); err == nil {
		t.Fatal("accepted zero quantity")
	}
	if _, err := ApplyEvent(original, AccountingEvent{Kind: Buy, Quantity: d("1"), UnitPrice: d("1")}, "LIFO"); err == nil {
		t.Fatal("accepted unsupported method")
	}
}

func TestDividendFeeAndSplitConservation(t *testing.T) {
	p := AccountPosition{Quantity: d("10"), CostBasis: d("100"), Lots: []Lot{{Quantity: d("10"), UnitCost: d("10")}}}
	var err error
	p, err = ApplyEvent(p, AccountingEvent{Kind: Dividend, Amount: d("5")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplyEvent(p, AccountingEvent{Kind: Fee, Amount: d("2")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	p, err = ApplySplit(p, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Quantity.Equal(d("15")) || !p.CostBasis.Equal(d("100")) || !p.RealizedPnL.Equal(d("3")) {
		t.Fatalf("corporate action invariant failed: %+v", p)
	}
	if _, err := ApplySplit(p, 0, 1); err == nil {
		t.Fatal("accepted invalid split")
	}
	if _, err := ApplyEvent(p, AccountingEvent{Kind: Fee, Amount: d("-1")}, FIFO); err == nil {
		t.Fatal("accepted negative fee")
	}
}

func TestInstrumentValidation(t *testing.T) {
	if (Instrument{Symbol: "ACME", Currency: "USD"}).Validate() != nil {
		t.Fatal("valid instrument rejected")
	}
	if (Instrument{Symbol: " ", Currency: "USD"}).Validate() == nil {
		t.Fatal("empty symbol accepted")
	}
}

func FuzzNewMoneyCurrency(f *testing.F) {
	f.Add("USD")
	f.Add("usd")
	f.Add("")
	f.Add("US1")
	f.Fuzz(func(t *testing.T, currency string) {
		m, err := NewMoney(decimal.Zero, currency)
		if err == nil && (len(m.Currency) != 3 || m.Currency != strings.ToUpper(m.Currency)) {
			t.Fatalf("invalid accepted currency %q", m.Currency)
		}
	})
}
