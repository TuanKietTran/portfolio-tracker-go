package command

import (
	"context"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func cd(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func TestPortfolioServiceTradesValuationAndLocalCash(t *testing.T) {
	ctx := context.Background()
	state := repository.NewInMemoryState()
	feed := repository.NewStaticPriceFeed()
	feed.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("15"), AsOf: 50})
	svc, err := NewPortfolioService(state, feed, nil, domain.FIFO)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if _, err = svc.MoveCash(ctx, id, "USD", "FUND", cd("20"), "seed"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Trade(ctx, id, "ACME", domain.AccountingEvent{Currency: "USD", Kind: domain.Buy, Quantity: cd("2"), UnitPrice: cd("10")}, "buy-1"); err != nil {
		t.Fatal(err)
	}
	p, v, err := svc.Position(ctx, id, "ACME", "USD", 55, 5)
	if err != nil || !p.CostBasis.Equal(cd("20")) || !v.MarketValue.Equal(cd("30")) || !v.UnrealizedPnL.Equal(cd("10")) {
		t.Fatalf("p=%+v v=%+v err=%v", p, v, err)
	}
	if _, _, err = svc.Position(ctx, id, "ACME", "EUR", 55, 5); err == nil {
		t.Fatal("accepted stale/mismatched price")
	}
	if _, err = svc.MoveCash(ctx, id, "USD", "FUND", cd("50"), "fund-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.MoveCash(ctx, id, "USD", "WITHDRAW", cd("51"), "withdraw-1"); err == nil {
		t.Fatal("accepted overdraw")
	}
	bal, err := svc.MoveCash(ctx, id, "USD", "WITHDRAW", cd("20"), "withdraw-2")
	if err != nil || !bal.Equal(cd("30")) {
		t.Fatalf("balance=%s err=%v", bal, err)
	}
	if _, err = NewPortfolioService(state, feed, nil, "bad"); err == nil {
		t.Fatal("accepted bad cost method")
	}
}
