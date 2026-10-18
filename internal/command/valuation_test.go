package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestHistoricalNAVUsesEarlierHoldingsAndCashAfterFullSale(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	now := time.Unix(100, 123)
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
	prices := repository.NewStaticPriceFeed()
	svc, _ := NewPortfolioService(state, prices, nil, domain.FIFO)
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("100"), "fund"); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(110, 123)
	if _, err := svc.Trade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: cd("2"), UnitPrice: cd("10")}, "buy"); err != nil {
		t.Fatal(err)
	}
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("15"), AsOf: 110})
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("25"), AsOf: 200})
	now = time.Unix(160, 123)
	if _, err := svc.Trade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Sell, Currency: "USD", Quantity: cd("2"), UnitPrice: cd("20")}, "sell"); err != nil {
		t.Fatal(err)
	}
	value, err := svc.Valuation(ctx, id, "USD", time.Unix(150, 0), 40)
	if err != nil || !value.Cash.Equal(cd("80")) || !value.SecuritiesValue.Equal(cd("30")) || !value.NetAssetValue.Equal(cd("110")) || !value.CostBasis.Equal(cd("20")) || !value.UnrealizedPnL.Equal(cd("10")) || len(value.Positions) != 1 || !value.Positions[0].Quantity.Equal(cd("2")) || value.Positions[0].PriceAsOf == nil || *value.Positions[0].PriceAsOf != 110 {
		t.Fatalf("past NAV=%+v err=%v", value, err)
	}
	if _, err := svc.Valuation(ctx, id, "USD", time.Unix(151, 0), 40); err == nil {
		t.Fatal("stale mark accepted")
	}
	if _, err := svc.Valuation(ctx, id, "USD", time.Unix(100, 0), 0); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("nanosecond cutoff leaked future funding: %v", err)
	}
	calls := 0
	noPrices, _ := NewPortfolioService(state, PriceFunc(func(context.Context, string, string) (domain.Price, error) {
		calls++
		return domain.Price{}, errors.New("must not fetch a price")
	}), nil, domain.FIFO)
	cashOnly, err := noPrices.Valuation(ctx, id, "USD", time.Unix(110, 0), 0)
	if err != nil || !cashOnly.NetAssetValue.Equal(cd("100")) || cashOnly.Positions == nil || len(cashOnly.Positions) != 0 {
		t.Fatalf("cash-only=%+v err=%v", cashOnly, err)
	}
	closed, err := noPrices.Valuation(ctx, id, "USD", time.Unix(161, 0), 0)
	if err != nil || !closed.Cash.Equal(cd("120")) || !closed.NetAssetValue.Equal(cd("120")) || !closed.SecuritiesValue.IsZero() || !closed.RealizedPnL.Equal(cd("20")) || len(closed.Positions) != 1 || closed.Positions[0].PriceAsOf != nil || calls != 0 {
		t.Fatalf("closed=%+v price calls=%d err=%v", closed, calls, err)
	}
	// A nanosecond-exact Go cutoff includes the command at that instant.
	exact, err := svc.Valuation(ctx, id, "USD", time.Unix(110, 123), 0)
	if err != nil || !exact.NetAssetValue.Equal(cd("110")) {
		t.Fatalf("exact cutoff=%+v err=%v", exact, err)
	}
}

func TestHistoricalNAVSortedPositionsAndFutureCashIsolation(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	now := time.Unix(100, 0)
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
	prices := repository.NewStaticPriceFeed()
	svc, _ := NewPortfolioService(state, prices, nil, domain.FIFO)
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("100"), "fund"); err != nil {
		t.Fatal(err)
	}
	for i, tc := range []struct{ symbol, price, mark string }{{"ZED", "5", "7"}, {"ACME", "10", "15"}} {
		now = time.Unix(int64(110+i*10), 0)
		if _, err := svc.Trade(ctx, id, tc.symbol, domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: cd("1"), UnitPrice: cd(tc.price)}, tc.symbol); err != nil {
			t.Fatal(err)
		}
		prices.Set(domain.Price{Symbol: tc.symbol, Currency: "USD", Amount: cd(tc.mark), AsOf: 130})
	}
	now = time.Unix(200, 0)
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("50"), "later"); err != nil {
		t.Fatal(err)
	}
	value, err := svc.Valuation(ctx, id, "USD", time.Unix(150, 0).In(time.FixedZone("synthetic", 7*3600)), 20)
	if err != nil || !value.Cash.Equal(cd("85")) || !value.NetAssetValue.Equal(cd("107")) || len(value.Positions) != 2 || value.Positions[0].Symbol != "ACME" || value.Positions[1].Symbol != "ZED" || value.AsOf.Location() != time.UTC {
		t.Fatalf("value=%+v err=%v", value, err)
	}
	now = time.Unix(210, 0)
	if _, err := svc.MoveCash(ctx, id, "EUR", "FUND", cd("1"), "eur"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Valuation(ctx, id, "USD", now, 1000); err == nil {
		t.Fatal("cross-currency cash silently ignored")
	}
}

type valuationState struct {
	repository.PortfolioState
	snapshot repository.PortfolioSnapshot
	err      error
}

func (s valuationState) SnapshotAt(context.Context, uuid.UUID, time.Time) (repository.PortfolioSnapshot, error) {
	return s.snapshot, s.err
}

func TestHistoricalNAVRejectsInvalidDataAndMarksWithoutPartialResults(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	position := domain.AccountPosition{Currency: "USD", Quantity: cd("1"), CostBasis: cd("10")}
	validMark := domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("15"), AsOf: 100}
	for _, tc := range []struct {
		name string
		mark domain.Price
		err  error
	}{
		{"missing", domain.Price{}, repository.ErrPriceUnavailable},
		{"future", domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("1"), AsOf: 151}, nil},
		{"stale", domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("1"), AsOf: 99}, nil},
		{"wrong-symbol", domain.Price{Symbol: "OTHER", Currency: "USD", Amount: cd("1"), AsOf: 100}, nil},
		{"wrong-currency", domain.Price{Symbol: "ACME", Currency: "EUR", Amount: cd("1"), AsOf: 100}, nil},
		{"negative", domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("-1"), AsOf: 100}, nil},
		{"zero-time", domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("1"), AsOf: 0}, nil},
		{"unbounded", domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("1e999999"), AsOf: 100}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := valuationState{snapshot: repository.PortfolioSnapshot{Positions: map[string]domain.AccountPosition{"ACME": position}}}
			svc, _ := NewPortfolioService(state, PriceFunc(func(context.Context, string, string) (domain.Price, error) { return tc.mark, tc.err }), nil, domain.FIFO)
			if value, err := svc.Valuation(ctx, id, "USD", time.Unix(150, 0), 50); err == nil || value.Positions != nil {
				t.Fatalf("accepted/partial=%+v err=%v", value, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		p    domain.AccountPosition
		cash map[string]decimal.Decimal
	}{
		{"position-fx", domain.AccountPosition{Currency: "EUR", Quantity: cd("1")}, nil},
		{"negative-quantity", domain.AccountPosition{Currency: "USD", Quantity: cd("-1")}, nil},
		{"negative-basis", domain.AccountPosition{Currency: "USD", Quantity: cd("1"), CostBasis: cd("-1")}, nil},
		{"closed-basis", domain.AccountPosition{Currency: "USD", CostBasis: cd("1")}, nil},
		{"negative-cash", position, map[string]decimal.Decimal{"USD": cd("-1")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := valuationState{snapshot: repository.PortfolioSnapshot{Positions: map[string]domain.AccountPosition{"ACME": tc.p}, Cash: tc.cash}}
			svc, _ := NewPortfolioService(state, PriceFunc(func(context.Context, string, string) (domain.Price, error) { return validMark, nil }), nil, domain.FIFO)
			if _, err := svc.Valuation(ctx, id, "USD", time.Unix(150, 0), 50); err == nil {
				t.Fatal("invalid data accepted")
			}
		})
	}
	svc, _ := NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	for _, tc := range []struct {
		currency string
		at       time.Time
		age      int64
	}{{"ZZZ", time.Unix(100, 0), 0}, {"USD", time.Time{}, 0}, {"USD", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), 0}, {"USD", time.Unix(100, 0), -1}} {
		if _, err := svc.Valuation(ctx, id, tc.currency, tc.at, tc.age); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := svc.Valuation(canceled, id, "USD", time.Unix(100, 0), 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
