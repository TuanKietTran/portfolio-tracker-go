package domain

import (
	"fmt"
	"testing"
)

func TestAverageFullLiquidationConsumesExactRemainingBasis(t *testing.T) {
	p := AccountPosition{}
	var err error
	for _, event := range []AccountingEvent{{Kind: Buy, Quantity: d("1"), UnitPrice: d("1")}, {Kind: Buy, Quantity: d("2"), UnitPrice: d("2")}, {Kind: Sell, Quantity: d("3"), UnitPrice: d("3")}} {
		p, err = ApplyEvent(p, event, AverageCost)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !p.RealizedPnL.Equal(d("4")) || !p.CostBasis.IsZero() || !p.Quantity.IsZero() {
		t.Fatalf("liquidation loses residual: %+v", p)
	}
}

func TestFIFOSplitFullLiquidationConsumesExactLotBasis(t *testing.T) {
	for n := int64(2); n <= 9; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			p, err := ApplyEvent(AccountPosition{}, AccountingEvent{Kind: Buy, Quantity: d("1"), UnitPrice: d("10")}, FIFO)
			if err != nil {
				t.Fatal(err)
			}
			p, err = ApplySplit(p, n, 1)
			if err != nil {
				t.Fatal(err)
			}
			p, err = ApplyEvent(p, AccountingEvent{Kind: Sell, Quantity: d(fmt.Sprint(n)), UnitPrice: d("10")}, FIFO)
			if err != nil {
				t.Fatal(err)
			}
			expected := d(fmt.Sprint(n*10 - 10))
			if !p.RealizedPnL.Equal(expected) || !p.CostBasis.IsZero() {
				t.Fatalf("split liquidation=%+v expected pnl=%s", p, expected)
			}
		})
	}
}

func TestPartialAllocationsCarryResidualUntilFinalSale(t *testing.T) {
	for _, method := range []CostMethod{FIFO, AverageCost} {
		for n := int64(2); n <= 9; n++ {
			p, err := ApplyEvent(AccountPosition{}, AccountingEvent{Kind: Buy, Quantity: d("1"), UnitPrice: d("10")}, method)
			if err != nil {
				t.Fatal(err)
			}
			p, err = ApplySplit(p, n, 1)
			if err != nil {
				t.Fatal(err)
			}
			for sold := int64(1); sold <= n; sold++ {
				p, err = ApplyEvent(p, AccountingEvent{Kind: Sell, Quantity: d("1"), UnitPrice: d("3")}, method)
				if err != nil {
					t.Fatal(err)
				}
				proceeds := d(fmt.Sprint(sold * 3))
				if !p.CostBasis.Add(proceeds).Sub(p.RealizedPnL).Equal(d("10")) {
					t.Fatalf("method=%s ratio=%d sold=%d loses basis: %+v", method, n, sold, p)
				}
			}
			if !p.CostBasis.IsZero() || !p.Quantity.IsZero() {
				t.Fatalf("not fully liquidated: %+v", p)
			}
		}
	}
}

func TestRepeatingSplitShareQuantitiesAreRejectedWithoutMutation(t *testing.T) {
	p, err := ApplyEvent(AccountPosition{}, AccountingEvent{Kind: Buy, Quantity: d("1"), UnitPrice: d("10")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySplit(p, 1, 3); err == nil {
		t.Fatal("unsupported repeating fractional share accepted")
	}
	if !p.Quantity.Equal(d("1")) || !p.Lots[0].CostBasis.Equal(d("10")) {
		t.Fatal("failed split changed input")
	}
	// Even if aggregate quantity divides exactly, each FIFO lot must be representable.
	p, err = ApplyEvent(p, AccountingEvent{Kind: Buy, Quantity: d("2"), UnitPrice: d("10")}, FIFO)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySplit(p, 1, 3); err == nil {
		t.Fatal("unrepresentable individual lots accepted")
	}
	if !p.Lots[0].Quantity.Equal(d("1")) || !p.Lots[1].Quantity.Equal(d("2")) {
		t.Fatal("failed lot split changed input")
	}
}

func TestReverseSplitMultipliesBeforeDividing(t *testing.T) {
	p := AccountPosition{Quantity: d("3"), CostBasis: d("10"), Lots: []Lot{{Quantity: d("3"), UnitCost: d("3.3333333333333333")}}}
	split, err := ApplySplit(p, 1, 3)
	if err != nil || !split.Quantity.Equal(d("1")) || !split.Lots[0].Quantity.Equal(d("1")) {
		t.Fatalf("reverse split=%+v err=%v", split, err)
	}
}
