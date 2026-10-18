package domain

import (
	"github.com/shopspring/decimal"
	"testing"
)

func TestRejectFractionalMinorUnitsPolicy(t *testing.T) {
	for _, tc := range []struct {
		currency, amount string
		valid            bool
	}{
		{"USD", "1.00", true}, {"EUR", "0.01", true}, {"GBP", "2.50", true}, {"JPY", "10", true}, {"USD", "0", true},
		{"USD", "-1", false}, {"USD", "0.001", false}, {"USD", "1e999999", false}, {"USD", "0.0000000000000000001", false}, {"USD", "1111111111111111111111111111111", false}, {"JPY", "0.1", false}, {"usd", "1", false}, {"ZZZ", "1", false},
	} {
		if err := ValidateCashAmount(d(tc.amount), tc.currency); (err == nil) != tc.valid {
			t.Fatalf("%+v err=%v", tc, err)
		}
	}
}

func TestPositionCurrencyAndCostMethodCannotChange(t *testing.T) {
	p, err := ApplyEvent(AccountPosition{}, AccountingEvent{Kind: Buy, Currency: "USD", Quantity: d("2"), UnitPrice: d("10")}, FIFO)
	if err != nil || p.Currency != "USD" || p.Method != FIFO {
		t.Fatalf("position=%+v err=%v", p, err)
	}
	if _, err := ApplyEvent(p, AccountingEvent{Kind: Buy, Currency: "EUR", Quantity: d("1"), UnitPrice: d("1")}, FIFO); err == nil {
		t.Fatal("changed position currency")
	}
	if _, err := ApplyEvent(p, AccountingEvent{Kind: Buy, Currency: "USD", Quantity: d("1"), UnitPrice: d("1")}, AverageCost); err == nil {
		t.Fatal("changed accounting method")
	}
	if _, err := ApplyEvent(p, AccountingEvent{Kind: Buy, Currency: "bad-code", Quantity: d("1"), UnitPrice: d("1")}, FIFO); err == nil {
		t.Fatal("invalid currency")
	}
	if _, err := ValuePosition(p, "ACME", "EUR", Price{Symbol: "ACME", Currency: "EUR", Amount: d("10"), AsOf: 100}, 100, 0); err == nil {
		t.Fatal("implicit currency conversion")
	}
}

func FuzzSettlementAmount(f *testing.F) {
	f.Add("1.00", "USD")
	f.Add("0.001", "USD")
	f.Add("1", "JPY")
	f.Add("1e999999", "USD")
	f.Fuzz(func(t *testing.T, raw, currency string) {
		if len(raw) > 100 {
			return
		}
		amount, err := decimal.NewFromString(raw)
		if err != nil {
			return
		}
		// Validation must bound arithmetic even for very large parsed exponents.
		if err := ValidateCashAmount(amount, currency); err == nil {
			scale := int32(2)
			if currency == "JPY" {
				scale = 0
			}
			if amount.IsNegative() || !amount.Equal(amount.Truncate(scale)) {
				t.Fatalf("accepted amount=%s currency=%s", amount, currency)
			}
		}
	})
}
