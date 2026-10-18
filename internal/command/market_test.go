package command

import (
	"context"
	"errors"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
)

func TestReadOnlyFeedCompatibilityAndPublicationUnavailable(t *testing.T) {
	ctx := context.Background()
	state := repository.NewInMemoryState()
	feed := PriceFunc(func(context.Context, string, string) (domain.Price, error) {
		return domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("10"), AsOf: 100}, nil
	})
	svc, _ := NewPortfolioService(state, feed, nil, domain.FIFO)
	if _, err := svc.RegisterInstrument(ctx, domain.Instrument{Symbol: "ACME", Currency: "USD"}, "i"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Instrument(ctx, "ACME"); err != nil {
		t.Fatal(err)
	}
	price, err := svc.Price(ctx, "ACME", "USD", 100, 0)
	if err != nil || price.AsOf != 100 {
		t.Fatalf("price=%+v err=%v", price, err)
	}
	if _, err := svc.PublishPrice(ctx, price, "p"); !errors.Is(err, ErrPriceManagementUnavailable) {
		t.Fatal(err)
	}
	if _, err := svc.Price(ctx, "ACME", "USD", 0, 0); err == nil {
		t.Fatal("invalid cutoff accepted")
	}
	if _, err := svc.Price(ctx, "MISSING", "USD", 100, 0); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
	broken, _ := NewPortfolioService(state, PriceFunc(func(context.Context, string, string) (domain.Price, error) {
		return domain.Price{}, errors.New("feed failure")
	}), nil, domain.FIFO)
	if _, err := broken.Price(ctx, "ACME", "USD", 100, 0); err == nil {
		t.Fatal("feed error lost")
	}
}
