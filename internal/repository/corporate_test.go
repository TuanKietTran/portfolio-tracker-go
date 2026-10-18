package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
)

func heldState(t *testing.T) (*InMemoryState, uuid.UUID) {
	t.Helper()
	s := NewInMemoryState()
	id := uuid.New()
	fundState(t, s, id)
	if _, err := s.ApplyTrade(context.Background(), id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("2"), UnitPrice: pd("10")}, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	return s, id
}
func TestCorporateConcurrentReplayOriginalSnapshotAndAtomicCash(t *testing.T) {
	ctx := context.Background()
	s, id := heldState(t)
	action := domain.CorporateAction{Kind: domain.Split, Currency: "USD", Numerator: 2, Denominator: 1}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := s.ApplyCorporateAction(ctx, id, "ACME", action, "split")
			if err != nil || !result.Position.Quantity.Equal(pd("4")) || !result.CashBalance.Equal(pd("80")) {
				t.Errorf("result=%+v err=%v", result, err)
			}
		}()
	}
	wg.Wait()
	first, err := s.ApplyCorporateAction(ctx, id, "ACME", action, "split")
	if err != nil {
		t.Fatal(err)
	}
	first.Position.Lots[0].Quantity = pd("999")
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", domain.CorporateAction{Kind: domain.Dividend, Currency: "USD", Amount: pd("4")}, "dividend"); err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyCorporateAction(ctx, id, "ACME", domain.CorporateAction{Kind: domain.Fee, Currency: "USD", Amount: pd("1")}, "fee")
	if err != nil || !result.CashBalance.Equal(pd("83")) || !result.Position.RealizedPnL.Equal(pd("3")) || !result.Position.CostBasis.Equal(pd("20")) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	original, err := s.ApplyCorporateAction(ctx, id, "ACME", action, "split")
	if err != nil || !original.CashBalance.Equal(pd("80")) || !original.Position.Lots[0].Quantity.Equal(pd("4")) || !original.Position.RealizedPnL.IsZero() {
		t.Fatalf("original=%+v err=%v", original, err)
	}
	cash, _ := s.CashBalance(ctx, id, "USD")
	if !cash.Equal(pd("83")) {
		t.Fatal("replay changed actual cash")
	}
	for _, tc := range []struct {
		id     uuid.UUID
		symbol string
		action domain.CorporateAction
	}{
		{uuid.New(), "ACME", action}, {id, "OTHER", action}, {id, "ACME", domain.CorporateAction{Kind: domain.Split, Currency: "USD", Numerator: 3, Denominator: 1}},
		{id, "ACME", domain.CorporateAction{Kind: domain.Split, Currency: "EUR", Numerator: 2, Denominator: 1}},
		{id, "ACME", domain.CorporateAction{Kind: domain.Fee, Currency: "USD", Amount: pd("1")}},
	} {
		if _, err := s.ApplyCorporateAction(ctx, tc.id, tc.symbol, tc.action, "split"); !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("conflict=%v", err)
		}
	}
	// Equivalent decimal cash action replays without another dividend.
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", domain.CorporateAction{Kind: domain.Dividend, Currency: "USD", Amount: pd("4.000")}, "dividend"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentCorporateFeesCannotOverdrawAndFailureDoesNotConsumeKey(t *testing.T) {
	ctx := context.Background()
	s, id := heldState(t) // Remaining cash80.
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	fee := domain.CorporateAction{Kind: domain.Fee, Currency: "USD", Amount: pd("2")}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.ApplyCorporateAction(ctx, id, "ACME", fee, fmt.Sprint("fee", i))
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, ErrInsufficientCash) {
				t.Errorf("err=%v", err)
			}
		}(i)
	}
	wg.Wait()
	p, _ := s.GetAccountPosition(ctx, id, "ACME")
	cash, _ := s.CashBalance(ctx, id, "USD")
	if successes != 40 || !cash.IsZero() || !p.RealizedPnL.Equal(pd("-80")) || !p.Quantity.Equal(pd("2")) {
		t.Fatalf("successes=%d cash=%s position=%+v", successes, cash, p)
	}
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", fee, "retry"); !errors.Is(err, ErrInsufficientCash) {
		t.Fatal(err)
	}
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("2"), "more"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", fee, "retry"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", domain.CorporateAction{Kind: domain.Split, Currency: "USD", Numerator: 1, Denominator: 3}, "bad-split"); err == nil {
		t.Fatal("repeating split accepted")
	}
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", domain.CorporateAction{Kind: domain.Split, Currency: "USD", Numerator: 2, Denominator: 1}, "bad-split"); err != nil {
		t.Fatal("failed split consumed key", err)
	}
}

func TestCorporateMissingPositionCurrencyAndInvalidCommand(t *testing.T) {
	ctx := context.Background()
	s, id := heldState(t)
	a := domain.CorporateAction{Kind: domain.Dividend, Currency: "USD", Amount: pd("1")}
	if _, err := s.ApplyCorporateAction(ctx, id, "MISSING", a, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.ApplyCorporateAction(ctx, uuid.New(), "ACME", a, "no-position"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	euro := a
	euro.Currency = "EUR"
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", euro, "eur"); !errors.Is(err, ErrInstrumentConflict) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id          uuid.UUID
		symbol, key string
		action      domain.CorporateAction
	}{{id, "ACME", "", a}, {uuid.Nil, "ACME", "nil", a}, {id, "bad/symbol", "bad", a}, {id, "ACME", "bad-kind", domain.CorporateAction{Kind: "BAD", Currency: "USD"}}} {
		if _, err := s.ApplyCorporateAction(ctx, tc.id, tc.symbol, tc.action, tc.key); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if len(s.corporateKeys) != 0 {
		t.Fatal("failed actions persisted idempotency records")
	}
}
