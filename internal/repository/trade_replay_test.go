package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
)

func TestTradeReplayReturnsOriginalSnapshotAndRejectsChangedPayload(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryState()
	id := uuid.New()
	buy := domain.AccountingEvent{Kind: domain.Buy, Quantity: pd("2"), UnitPrice: pd("10")}
	original, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy")
	if err != nil {
		t.Fatal(err)
	}
	original.Lots[0].Quantity = pd("999") // Caller cannot mutate stored lots or replay results.
	sell := domain.AccountingEvent{Kind: domain.Sell, Quantity: pd("1"), UnitPrice: pd("15")}
	sold, err := s.ApplyTrade(ctx, id, "ACME", sell, domain.FIFO, "sell")
	if err != nil || !sold.Quantity.Equal(pd("1")) {
		t.Fatalf("sold=%+v err=%v", sold, err)
	}
	replay, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy")
	if err != nil || !replay.Quantity.Equal(pd("2")) || !replay.Lots[0].Quantity.Equal(pd("2")) {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	replay.Lots[0].Quantity = pd("100")
	replay, _ = s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy")
	if !replay.Lots[0].Quantity.Equal(pd("2")) {
		t.Fatal("replay aliases cached snapshot")
	}
	equal := buy
	equal.Quantity = pd("2.000")
	if _, err = s.ApplyTrade(ctx, id, "ACME", equal, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		id     uuid.UUID
		symbol string
		event  domain.AccountingEvent
		method domain.CostMethod
	}{
		{"portfolio", uuid.New(), "ACME", buy, domain.FIFO},
		{"symbol", id, "OTHER", buy, domain.FIFO},
		{"side", id, "ACME", sell, domain.FIFO},
		{"quantity", id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Quantity: pd("3"), UnitPrice: pd("10")}, domain.FIFO},
		{"price", id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Quantity: pd("2"), UnitPrice: pd("11")}, domain.FIFO},
		{"method", id, "ACME", buy, domain.AverageCost},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.ApplyTrade(ctx, tc.id, tc.symbol, tc.event, tc.method, "buy"); !errors.Is(err, ErrIdempotencyConflict) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	current, _ := s.GetAccountPosition(ctx, id, "ACME")
	if !current.Quantity.Equal(pd("1")) || !current.RealizedPnL.Equal(pd("5")) {
		t.Fatalf("current=%+v", current)
	}
}

func TestConcurrentTradeReplayAppliesOnce(t *testing.T) {
	s := NewInMemoryState()
	id := uuid.New()
	ctx := context.Background()
	buy := domain.AccountingEvent{Kind: domain.Buy, Quantity: pd("2"), UnitPrice: pd("10")}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "same")
			if err != nil || !p.Quantity.Equal(pd("2")) {
				t.Errorf("position=%+v err=%v", p, err)
			}
		}()
	}
	wg.Wait()
	p, _ := s.GetAccountPosition(ctx, id, "ACME")
	if !p.Quantity.Equal(pd("2")) || len(p.Lots) != 1 {
		t.Fatalf("position=%+v", p)
	}
}

func TestFailedTradeDoesNotConsumeKey(t *testing.T) {
	s := NewInMemoryState()
	id := uuid.New()
	ctx := context.Background()
	event := domain.AccountingEvent{Kind: domain.Sell, Quantity: pd("1"), UnitPrice: pd("10")}
	if _, err := s.ApplyTrade(ctx, id, "ACME", event, domain.FIFO, "retry"); err == nil {
		t.Fatal("oversell accepted")
	}
	event.Kind = domain.Buy
	if _, err := s.ApplyTrade(ctx, id, "ACME", event, domain.FIFO, "retry"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id          uuid.UUID
		symbol, key string
	}{{id, "ACME", ""}, {uuid.Nil, "ACME", "k"}, {id, "", "k"}} {
		if _, err := s.ApplyTrade(ctx, tc.id, tc.symbol, event, domain.FIFO, tc.key); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}
