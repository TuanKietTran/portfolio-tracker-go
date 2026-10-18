package command

import (
	"context"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
)

func TestSummaryDeterministicSortedMarkedSnapshot(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	state := repository.NewInMemoryState()
	feed := repository.NewStaticPriceFeed()
	svc, _ := NewPortfolioService(state, feed, nil, domain.FIFO)
	for _, symbol := range []string{"ZED", "ACME"} {
		feed.Set(domain.Price{Symbol: symbol, Currency: "USD", Amount: cd("12"), AsOf: 100})
		if _, err := svc.Trade(ctx, id, symbol, domain.AccountingEvent{Kind: domain.Buy, Quantity: cd("2"), UnitPrice: cd("10")}, symbol); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := svc.Summary(ctx, id, "USD", 100, 0)
	if err != nil || len(summary.Positions) != 2 || summary.Positions[0].Symbol != "ACME" || summary.Positions[1].Symbol != "ZED" || !summary.TotalValue.Equal(cd("48")) || !summary.UnrealizedPnL.Equal(cd("8")) || !summary.Positions[0].AverageCost.Equal(cd("10")) {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	if _, err := svc.Summary(ctx, id, "EUR", 100, 0); err == nil {
		t.Fatal("missing currency mark accepted")
	}
	if _, err := svc.Summary(ctx, id, "USD", 101, 0); err == nil {
		t.Fatal("stale mark accepted")
	}
	positions, _ := state.ListAccountPositions(ctx, id)
	positions["ACME"].Lots[0].Quantity = cd("999")
	delete(positions, "ZED")
	p, _ := state.GetAccountPosition(ctx, id, "ACME")
	if !p.Lots[0].Quantity.Equal(cd("2")) {
		t.Fatal("listed position aliases stored lots")
	}
	fresh, _ := state.ListAccountPositions(ctx, id)
	if len(fresh) != 2 {
		t.Fatal("listing map aliases state")
	}
}

func TestSummaryEmptyAndValidation(t *testing.T) {
	svc, _ := NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	ctx := context.Background()
	id := uuid.New()
	empty, err := svc.Summary(ctx, id, "USD", 100, 0)
	if err != nil || empty.Positions == nil || len(empty.Positions) != 0 || !empty.TotalValue.IsZero() {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
	for _, tc := range []struct {
		currency     string
		asOf, maxAge int64
	}{{"bad-code", 100, 0}, {"USD", 0, 0}, {"USD", 100, -1}} {
		if _, err := svc.Summary(ctx, id, tc.currency, tc.asOf, tc.maxAge); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}
