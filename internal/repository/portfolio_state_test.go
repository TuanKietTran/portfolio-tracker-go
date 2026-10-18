package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func pd(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func TestStateAccountingCashIdempotencyAndConflict(t *testing.T) {
	s := NewInMemoryState()
	ctx := context.Background()
	id := uuid.New()
	p, err := s.ApplyTrade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Quantity: pd("2"), UnitPrice: pd("10")}, domain.FIFO, "buy-1")
	if err != nil || !p.CostBasis.Equal(pd("20")) {
		t.Fatalf("position=%+v err=%v", p, err)
	}
	balance, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "fund-1")
	if err != nil || !balance.Equal(pd("100")) {
		t.Fatalf("balance=%s err=%v", balance, err)
	}
	balance, err = s.MoveCash(ctx, id, "USD", "FUND", pd("100.0"), "fund-1")
	if err != nil || !balance.Equal(pd("100")) {
		t.Fatalf("idempotent replay balance=%s err=%v", balance, err)
	}
	if _, err = s.MoveCash(ctx, id, "USD", "FUND", pd("101"), "fund-1"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	if _, err = s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("101"), "withdraw-1"); !errors.Is(err, ErrInsufficientCash) {
		t.Fatalf("insufficient err=%v", err)
	}
}
func TestConcurrentWithdrawalsNeverOverdraw(t *testing.T) {
	s := NewInMemoryState()
	id := uuid.New()
	ctx := context.Background()
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "initial"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	successes := 0
	var mu sync.Mutex
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("2"), fmt.Sprintf("w-%d", i)); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, ErrInsufficientCash) {
				t.Errorf("unexpected: %v", err)
			}
		}(i)
	}
	wg.Wait()
	balance, _ := s.CashBalance(ctx, id, "USD")
	if successes != 50 || !balance.IsZero() {
		t.Fatalf("successes=%d balance=%s", successes, balance)
	}
}
