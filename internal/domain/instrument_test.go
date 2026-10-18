package domain

import (
	"strings"
	"testing"
)

func TestInstrumentSymbolsRemainPathSafeAndCaseSensitive(t *testing.T) {
	for _, symbol := range []string{"ACME", "abc", "BRK.B", "A_B", "A-1", "7"} {
		if err := (Instrument{Symbol: symbol, Currency: "USD"}).Validate(); err != nil {
			t.Fatalf("%s: %v", symbol, err)
		}
	}
	for _, symbol := range []string{"", strings.Repeat("A", 33), "A B", " A", "A/B", "A?B", "株"} {
		if err := (Instrument{Symbol: symbol, Currency: "USD"}).Validate(); err == nil {
			t.Fatalf("accepted %q", symbol)
		}
	}
}

func FuzzInstrumentSymbol(f *testing.F) {
	f.Add("ACME")
	f.Add("bad/symbol")
	f.Add("")
	f.Fuzz(func(t *testing.T, symbol string) {
		if err := (Instrument{Symbol: symbol, Currency: "USD"}).Validate(); err == nil {
			if len(symbol) < 1 || len(symbol) > 32 || strings.ContainsAny(symbol, "/ ?\t\n") {
				t.Fatalf("accepted unsafe symbol %q", symbol)
			}
		}
	})
}
