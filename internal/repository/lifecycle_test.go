package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPortfolioCreationConcurrentReplayAndImmutableMetadata(t *testing.T) {
	ctx := context.Background()
	id := uuid.MustParse("9113183e-08e9-455b-bfc8-a522c352bd20")
	now := time.Unix(100, 123).In(time.FixedZone("fixture", 3600))
	calls := 0
	s, err := NewInMemoryStateWithSources(ClockFunc(func() time.Time { return now }), func() (uuid.UUID, error) { calls++; return id, nil })
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := s.CreatePortfolio(ctx, "USD", domain.AverageCost, "create")
			if e != nil || p.ID != id || p.CostMethod != domain.AverageCost || p.CreatedAt != now.UTC() {
				t.Errorf("p=%+v err=%v", p, e)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("identity calls=%d", calls)
	}
	p, _ := s.GetPortfolio(ctx, id)
	p.Currency = "EUR"
	original, _ := s.GetPortfolio(ctx, id)
	if original.Currency != "USD" {
		t.Fatal("metadata aliased")
	}
	rows, _ := s.History(ctx, id, 0, 100)
	if len(rows) != 1 || rows[0].Kind != "CREATE" || rows[0].Sequence != 1 || !rows[0].InvestorFlow.IsZero() || len(rows[0].Snapshot.Positions) != 0 || len(rows[0].Snapshot.Cash) != 0 {
		t.Fatalf("history=%+v", rows)
	}
	if _, e := s.SnapshotAt(ctx, id, now.Add(-time.Nanosecond)); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	snap, e := s.SnapshotAt(ctx, id, now)
	if e != nil || len(snap.Cash) != 0 {
		t.Fatal(snap, e)
	}
	for _, tc := range []struct {
		currency string
		method   domain.CostMethod
	}{{"EUR", domain.AverageCost}, {"USD", domain.FIFO}} {
		if _, e := s.CreatePortfolio(ctx, tc.currency, tc.method, "create"); !errors.Is(e, ErrIdempotencyConflict) {
			t.Fatal(e)
		}
	}
	if _, e := s.MoveCash(ctx, id, "EUR", "FUND", decimal.NewFromInt(10), "cash"); !errors.Is(e, ErrPortfolioConflict) {
		t.Fatal(e)
	}
	if _, e := s.CashBalance(ctx, id, "EUR"); !errors.Is(e, ErrPortfolioConflict) {
		t.Fatal(e)
	}
	if _, e := s.ApplyTrade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(1)}, domain.FIFO, "buy"); !errors.Is(e, ErrPortfolioConflict) {
		t.Fatal(e)
	}
	if _, e := s.ApplyCorporateAction(ctx, id, "ACME", domain.CorporateAction{Kind: domain.Fee, Currency: "EUR", Amount: decimal.NewFromInt(1)}, "fee"); !errors.Is(e, ErrPortfolioConflict) {
		t.Fatal(e)
	}
	if _, e := s.MoveCash(ctx, id, "USD", "FUND", decimal.NewFromInt(10), "cash"); e != nil {
		t.Fatal(e)
	}
	now = time.Time{} // original replay must not consult clock or identity.
	replay, e := s.CreatePortfolio(ctx, "USD", domain.AverageCost, "create")
	if e != nil || replay != original || calls != 1 {
		t.Fatal(replay, e, calls)
	}
	rows, _ = s.History(ctx, id, 0, 100)
	if len(rows) != 2 {
		t.Fatal(len(rows))
	}
}

func TestPortfolioCreationFailuresDoNotConsumeKeyOrAdoptLegacy(t *testing.T) {
	ctx := context.Background()
	now := time.Time{}
	id := uuid.New()
	identityErr := errors.New("synthetic identity failure")
	s, _ := NewInMemoryStateWithSources(ClockFunc(func() time.Time { return now }), func() (uuid.UUID, error) { return id, identityErr })
	if _, e := NewInMemoryStateWithSources(ClockFunc(time.Now), nil); e == nil {
		t.Fatal("nil identity source")
	}
	for _, tc := range []struct {
		currency string
		method   domain.CostMethod
		key      string
	}{{"usd", domain.FIFO, "k"}, {"USD", "BAD", "k"}, {"USD", domain.FIFO, " "}, {"XXX", domain.FIFO, "k"}} {
		if _, e := s.CreatePortfolio(ctx, tc.currency, tc.method, tc.key); e == nil {
			t.Fatal(tc)
		}
	}
	if _, e := s.CreatePortfolio(ctx, "USD", domain.FIFO, "k"); !errors.Is(e, ErrPortfolioIdentity) {
		t.Fatal(e)
	}
	identityErr = nil
	id = uuid.Nil
	if _, e := s.CreatePortfolio(ctx, "USD", domain.FIFO, "k"); !errors.Is(e, ErrPortfolioIdentity) {
		t.Fatal(e)
	}
	id = uuid.New()
	if _, e := s.CreatePortfolio(ctx, "USD", domain.FIFO, "k"); !errors.Is(e, ErrHistoryClock) {
		t.Fatal(e)
	}
	if _, e := s.GetPortfolio(ctx, id); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	now = time.Unix(100, 0)
	if _, e := s.MoveCash(ctx, id, "USD", "FUND", decimal.NewFromInt(3), "legacy"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.GetPortfolio(ctx, id); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := s.CreatePortfolio(ctx, "USD", domain.FIFO, "k"); !errors.Is(e, ErrPortfolioIdentity) {
		t.Fatal(e)
	}
	legacy := id
	id = uuid.New()
	p, e := s.CreatePortfolio(ctx, "USD", domain.FIFO, "k")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := s.CreatePortfolio(ctx, "USD", domain.FIFO, "new-key"); !errors.Is(e, ErrPortfolioIdentity) {
		t.Fatal(e)
	}
	if _, e := s.GetPortfolio(ctx, uuid.Nil); e == nil {
		t.Fatal("nil ID")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := s.CreatePortfolio(cancelled, "USD", domain.FIFO, "k"); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := s.GetPortfolio(cancelled, p.ID); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	cash, _ := s.CashBalance(ctx, legacy, "USD")
	if !cash.Equal(decimal.NewFromInt(3)) {
		t.Fatal(cash)
	}
	rows, _ := s.History(ctx, p.ID, 0, 100)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	now = time.Unix(99, 0)
	if _, e := s.MoveCash(ctx, p.ID, "USD", "FUND", decimal.NewFromInt(1), "later"); !errors.Is(e, ErrHistoryClock) {
		t.Fatal(e)
	}
	now = time.Unix(101, 0)
	if _, e := s.MoveCash(ctx, p.ID, "USD", "FUND", decimal.NewFromInt(1), "later"); e != nil {
		t.Fatal(e)
	}
}
