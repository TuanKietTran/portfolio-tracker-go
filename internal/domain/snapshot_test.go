package domain

import (
	"github.com/shopspring/decimal"
	"testing"
)

func TestAccountingEventsDoNotMutateInputLots(t *testing.T) {
	d := decimal.RequireFromString
	input := AccountPosition{Quantity: d("2"), CostBasis: d("20"), Lots: []Lot{{Quantity: d("2"), UnitCost: d("10")}}}
	sold, err := ApplyEvent(input, AccountingEvent{Kind: Sell, Quantity: d("1"), UnitPrice: d("15")}, FIFO)
	if err != nil || !sold.Lots[0].Quantity.Equal(d("1")) {
		t.Fatalf("sold=%+v err=%v", sold, err)
	}
	split, err := ApplySplit(input, 2, 1)
	if err != nil || !split.Lots[0].Quantity.Equal(d("4")) {
		t.Fatalf("split=%+v err=%v", split, err)
	}
	if !input.Lots[0].Quantity.Equal(d("2")) || !input.Lots[0].UnitCost.Equal(d("10")) {
		t.Fatalf("input mutated: %+v", input)
	}
	// Inconsistent input must not be partially consumed before returning an error.
	broken := input.Clone()
	broken.Quantity = d("3")
	if _, err := ApplyEvent(broken, AccountingEvent{Kind: Sell, Quantity: d("3"), UnitPrice: d("15")}, FIFO); err == nil {
		t.Fatal("broken lot invariant accepted")
	}
	if !broken.Lots[0].Quantity.Equal(d("2")) {
		t.Fatal("failed sell mutated input")
	}
}
