package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
)

func TestInstrumentAndPriceReplayImmutableHistoryAndConcurrency(t *testing.T) {
	ctx := context.Background()
	state := NewInMemoryState()
	feed := NewStaticPriceFeed()
	instrument := domain.Instrument{Symbol: "ACME", Currency: "USD"}
	mark := domain.Price{Symbol: "ACME", Currency: "USD", Amount: pd("10"), AsOf: 100}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, err := state.RegisterInstrument(ctx, instrument, "instrument"); err != nil || got != instrument {
				t.Errorf("instrument=%+v err=%v", got, err)
			}
			if got, err := feed.Publish(ctx, mark, "mark"); err != nil || !got.Amount.Equal(mark.Amount) {
				t.Errorf("price=%+v err=%v", got, err)
			}
		}()
	}
	wg.Wait()
	if got, err := state.GetInstrument(ctx, "ACME"); err != nil || got != instrument {
		t.Fatalf("instrument=%+v err=%v", got, err)
	}
	euro := instrument
	euro.Currency = "EUR"
	if _, err := state.RegisterInstrument(ctx, euro, "instrument"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("replay err=%v", err)
	}
	if _, err := state.RegisterInstrument(ctx, euro, "other"); !errors.Is(err, ErrInstrumentConflict) {
		t.Fatalf("denomination err=%v", err)
	}
	if _, err := state.GetInstrument(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	newer := mark
	newer.Amount = pd("20")
	newer.AsOf = 200
	if _, err := feed.Publish(ctx, newer, "mark-2"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cutoff int64
		want   string
	}{{100, "10"}, {150, "10"}, {200, "20"}, {300, "20"}} {
		got, err := feed.PriceAt(ctx, "ACME", "USD", tc.cutoff)
		if err != nil || !got.Amount.Equal(pd(tc.want)) {
			t.Fatalf("cutoff=%d got=%+v err=%v", tc.cutoff, got, err)
		}
	}
	if _, err := feed.PriceAt(ctx, "ACME", "USD", 99); !errors.Is(err, ErrPriceUnavailable) {
		t.Fatalf("future-only err=%v", err)
	}
	if got, err := feed.Price(ctx, "ACME", "USD"); err != nil || got.AsOf != 200 {
		t.Fatalf("latest=%+v err=%v", got, err)
	}
	if got, err := feed.Publish(ctx, mark, "mark"); err != nil || got.AsOf != 100 {
		t.Fatalf("replay=%+v err=%v", got, err)
	}
	changed := mark
	changed.Amount = pd("11")
	if _, err := feed.Publish(ctx, changed, "mark"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	if _, err := feed.Publish(ctx, changed, "same-time"); !errors.Is(err, ErrPriceConflict) {
		t.Fatal(err)
	}
	equivalent := mark
	equivalent.Amount = pd("10.000")
	if _, err := feed.Publish(ctx, equivalent, "equivalent"); err != nil {
		t.Fatal(err)
	}
	// Out-of-order insertion does not replace the latest mark.
	earlier := mark
	earlier.AsOf = 50
	if _, err := feed.Publish(ctx, earlier, "earlier"); err != nil {
		t.Fatal(err)
	}
	if latest, _ := feed.Price(ctx, "ACME", "USD"); latest.AsOf != 200 {
		t.Fatal("older insertion replaced latest")
	}
}

func TestMarketValidationDoesNotConsumeKeys(t *testing.T) {
	ctx := context.Background()
	state := NewInMemoryState()
	feed := NewStaticPriceFeed()
	for _, instrument := range []domain.Instrument{{Symbol: "bad symbol", Currency: "USD"}, {Symbol: "ACME", Currency: "usd"}} {
		if _, err := state.RegisterInstrument(ctx, instrument, "retry"); err == nil {
			t.Fatal("invalid instrument accepted")
		}
	}
	if _, err := state.RegisterInstrument(ctx, domain.Instrument{Symbol: "ACME", Currency: "USD"}, "retry"); err != nil {
		t.Fatal(err)
	}
	if _, err := state.RegisterInstrument(ctx, domain.Instrument{Symbol: "ACME", Currency: "USD"}, ""); err == nil {
		t.Fatal("empty key accepted")
	}
	for _, price := range []domain.Price{
		{Symbol: "bad/symbol", Currency: "USD", Amount: pd("1"), AsOf: 100},
		{Symbol: "ACME", Currency: "ZZZ", Amount: pd("1"), AsOf: 100},
		{Symbol: "ACME", Currency: "USD", Amount: pd("1e999999"), AsOf: 100},
		{Symbol: "ACME", Currency: "USD", Amount: pd("-1"), AsOf: 100},
		{Symbol: "ACME", Currency: "USD", Amount: pd("1"), AsOf: 0},
	} {
		if _, err := feed.Publish(ctx, price, "retry"); err == nil {
			t.Fatalf("accepted %+v", price)
		}
	}
	valid := domain.Price{Symbol: "ACME", Currency: "USD", Amount: pd("0.001"), AsOf: 100}
	if _, err := feed.Publish(ctx, valid, "retry"); err != nil {
		t.Fatal(err)
	}
	if _, err := feed.Publish(ctx, valid, " "); err == nil {
		t.Fatal("blank key accepted")
	}
}
