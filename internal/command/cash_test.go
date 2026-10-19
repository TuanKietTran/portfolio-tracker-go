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

type fakeCashBoundary struct {
	calls int
	err   error
}

type fakePaymentConflict struct{}

func (fakePaymentConflict) Error() string               { return "payment intent conflict" }
func (fakePaymentConflict) PaymentIntentConflict() bool { return true }

func (p *fakeCashBoundary) Move(context.Context, uuid.UUID, string, string, decimal.Decimal, string) error {
	p.calls++
	return p.err
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

func TestPaymentCashAppliesLocallyOnlyAfterBoundaryConfirms(t *testing.T) {
	state := repository.NewInMemoryState()
	boundary := &fakeCashBoundary{}
	svc, _ := NewPortfolioService(state, repository.NewStaticPriceFeed(), boundary, domain.FIFO)
	id := uuid.New()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		balance, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("10"), "fund")
		if err != nil || !balance.Equal(cd("10")) {
			t.Fatalf("replay %d balance=%s err=%v", i, balance, err)
		}
	}
	cash, _ := state.CashBalance(ctx, id, "USD")
	if boundary.calls != 2 || !cash.Equal(cd("10")) {
		t.Fatalf("calls=%d cash=%s", boundary.calls, cash)
	}
}

func TestPaymentWithdrawalPreflightDoesNotCallBoundaryWhenUnfunded(t *testing.T) {
	boundary := &fakeCashBoundary{}
	svc, _ := NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), boundary, domain.FIFO)
	if _, err := svc.MoveCash(context.Background(), uuid.New(), "USD", "WITHDRAW", cd("1"), "withdraw"); !errors.Is(err, repository.ErrInsufficientCash) {
		t.Fatalf("error=%v", err)
	}
	if boundary.calls != 0 {
		t.Fatalf("payments called %d times for unfunded withdrawal", boundary.calls)
	}
}

func TestPaymentIntentConflictMapsToIdempotencyConflict(t *testing.T) {
	svc, _ := NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), &fakeCashBoundary{err: fakePaymentConflict{}}, domain.FIFO)
	_, err := svc.MoveCash(context.Background(), uuid.New(), "USD", "FUND", cd("1"), "same-key")
	if !errors.Is(err, repository.ErrIdempotencyConflict) {
		t.Fatalf("error=%v", err)
	}
}

func TestPaymentCashFailureLeavesLocalCashUnchanged(t *testing.T) {
	state := repository.NewInMemoryState()
	boundary := &fakeCashBoundary{err: errors.New("payments unavailable")}
	svc, _ := NewPortfolioService(state, repository.NewStaticPriceFeed(), boundary, domain.FIFO)
	id := uuid.New()
	ctx := context.Background()
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("10"), "fund"); err == nil {
		t.Fatal("expected downstream failure")
	}
	cash, _ := state.CashBalance(ctx, id, "USD")
	if boundary.calls != 1 || !cash.IsZero() {
		t.Fatalf("calls=%d cash=%s", boundary.calls, cash)
	}
}
