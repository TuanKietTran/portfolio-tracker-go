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

func TestTradesCashAndBasisConserveValueAndReplayOriginal(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	s := NewInMemoryState()
	fundState(t, s, id)
	buy := domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("2"), UnitPrice: pd("10")}
	if _, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	cash, _ := s.CashBalance(ctx, id, "USD")
	if !cash.Equal(pd("80")) {
		t.Fatalf("cash=%s", cash)
	}
	sell := domain.AccountingEvent{Kind: domain.Sell, Currency: "USD", Quantity: pd("1"), UnitPrice: pd("15")}
	p, err := s.ApplyTrade(ctx, id, "ACME", sell, domain.FIFO, "sell")
	cash, _ = s.CashBalance(ctx, id, "USD")
	if err != nil || !cash.Equal(pd("95")) || !cash.Add(p.CostBasis).Sub(p.RealizedPnL).Equal(pd("100")) {
		t.Fatalf("cash=%s position=%+v err=%v", cash, p, err)
	}
	if _, err = s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	after, _ := s.CashBalance(ctx, id, "USD")
	if !after.Equal(cash) {
		t.Fatal("replay changed cash")
	}
	// Cash replay returns the original result, not the balance after later trades.
	replay, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "initial")
	if err != nil || !replay.Equal(pd("100")) {
		t.Fatalf("fund replay=%s err=%v", replay, err)
	}
	if _, err = s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("95"), "withdraw"); err != nil {
		t.Fatal(err)
	}
	replay, err = s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("95"), "withdraw")
	if err != nil || !replay.IsZero() {
		t.Fatalf("withdraw replay=%s err=%v", replay, err)
	}
}

func TestInsufficientTradeIsAtomicAndDoesNotBindInstrumentOrKey(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	s := NewInMemoryState()
	buy := domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("2"), UnitPrice: pd("10")}
	if _, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy"); !errors.Is(err, ErrInsufficientCash) {
		t.Fatalf("err=%v", err)
	}
	positions, _ := s.ListAccountPositions(ctx, id)
	if len(positions) != 0 || len(s.instrumentCurrencies) != 0 || len(s.tradeKeys) != 0 {
		t.Fatal("failed trade persisted state")
	}
	fundState(t, s, id)
	if _, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	euro := buy
	euro.Currency = "EUR"
	if _, err := s.MoveCash(ctx, id, "EUR", "FUND", pd("100"), "eur"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyTrade(ctx, id, "ACME", euro, domain.FIFO, "eur-buy"); err == nil {
		t.Fatal("changed instrument currency")
	}
	if _, err := s.ApplyTrade(ctx, id, "ACME", euro, domain.FIFO, "buy"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("currency replay=%v", err)
	}
	if _, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.AverageCost, "method"); err == nil {
		t.Fatal("changed cost method")
	}
	cash, _ := s.CashBalance(ctx, id, "USD")
	euroCash, _ := s.CashBalance(ctx, id, "EUR")
	if !cash.Equal(pd("80")) || !euroCash.Equal(pd("100")) {
		t.Fatalf("cash=%s EUR=%s", cash, euroCash)
	}
}

func TestConcurrentBuysAndWithdrawalsShareOneCashBoundary(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	s := NewInMemoryState()
	fundState(t, s, id)
	var wg sync.WaitGroup
	var mu sync.Mutex
	buys, withdrawals := 0, 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				_, err = s.ApplyTrade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("1"), UnitPrice: pd("2")}, domain.FIFO, fmt.Sprintf("b%d", i))
			} else {
				_, err = s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("2"), fmt.Sprintf("w%d", i))
			}
			if err == nil {
				mu.Lock()
				if i%2 == 0 {
					buys++
				} else {
					withdrawals++
				}
				mu.Unlock()
			} else if !errors.Is(err, ErrInsufficientCash) {
				t.Errorf("err=%v", err)
			}
		}(i)
	}
	wg.Wait()
	cash, _ := s.CashBalance(ctx, id, "USD")
	p, _ := s.GetAccountPosition(ctx, id, "ACME")
	if buys+withdrawals != 50 || !cash.IsZero() || !p.Quantity.Equal(pd(fmt.Sprint(buys))) || !p.CostBasis.Equal(pd(fmt.Sprint(buys*2))) {
		t.Fatalf("buys=%d withdrawals=%d cash=%s position=%+v", buys, withdrawals, cash, p)
	}
}

func TestCashAndTradeRejectUnsupportedCurrencyAndFractionalMinorUnits(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	s := NewInMemoryState()
	fundState(t, s, id)
	for _, tc := range []struct{ currency, amount string }{{"usd", "1"}, {"ZZZ", "1"}, {"USD", "0.001"}, {"JPY", "0.1"}} {
		if _, err := s.MoveCash(ctx, id, tc.currency, "FUND", pd(tc.amount), tc.currency+tc.amount); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	for _, event := range []domain.AccountingEvent{{Kind: domain.Buy, Currency: "USD", Quantity: pd("1"), UnitPrice: pd("0.001")}, {Kind: domain.Buy, Quantity: pd("1"), UnitPrice: pd("10")}} {
		if _, err := s.ApplyTrade(ctx, id, "ACME", event, domain.FIFO, "bad"); err == nil {
			t.Fatalf("accepted %+v", event)
		}
	}
	cash, _ := s.CashBalance(ctx, id, "USD")
	if !cash.Equal(pd("100")) {
		t.Fatal("invalid request moved cash")
	}
	valid := domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("1"), UnitPrice: pd("10")}
	if _, err := s.ApplyTrade(ctx, id, "ACME", valid, domain.FIFO, "valid"); err != nil {
		t.Fatal(err)
	}
	valid.Quantity = pd("1e999999")
	if _, err := s.ApplyTrade(ctx, id, "ACME", valid, domain.FIFO, "valid"); err == nil {
		t.Fatal("unbounded exponent reached replay arithmetic")
	}

}
