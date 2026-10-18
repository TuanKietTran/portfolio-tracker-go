package command

import (
	"context"
	"errors"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type unsafeCashBoundary struct{ calls int }

func (p *unsafeCashBoundary) Move(context.Context, uuid.UUID, string, string, decimal.Decimal, string) error {
	p.calls++
	return nil
}

func TestLocalWithdrawalReplayAfterBalanceSpent(t *testing.T) {
	svc, _ := NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	ctx := context.Background()
	id := uuid.New()
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("10"), "fund"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		balance, err := svc.MoveCash(ctx, id, "USD", "WITHDRAW", cd("10"), "withdraw")
		if err != nil || !balance.IsZero() {
			t.Fatalf("attempt %d balance=%s err=%v", i, balance, err)
		}
	}
}

func TestPaymentCashFailsClosedWithoutRecoverableOutbox(t *testing.T) {
	state := repository.NewInMemoryState()
	boundary := &unsafeCashBoundary{}
	svc, _ := NewPortfolioService(state, repository.NewStaticPriceFeed(), boundary, domain.FIFO)
	id := uuid.New()
	ctx := context.Background()
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("10"), "fund"); !errors.Is(err, ErrPaymentCashUnavailable) {
		t.Fatalf("error=%v", err)
	}
	cash, _ := state.CashBalance(ctx, id, "USD")
	if boundary.calls != 0 || !cash.IsZero() {
		t.Fatal("unsafe transfer attempted")
	}
}
