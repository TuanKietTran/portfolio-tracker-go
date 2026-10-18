package command

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func decimalFromRate(value float64) decimal.Decimal { return decimal.NewFromFloat(value) }

func TestHistoricalReturnsWorkedFlowAndAnalyticIRR(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	start := time.Unix(100, 0)
	end := start.Add(time.Duration(SecondsPerYear) * time.Second)
	now := start
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
	prices := repository.NewStaticPriceFeed()
	svc, _ := NewPortfolioService(state, prices, nil, domain.FIFO)
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("100"), "start"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Trade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: cd("10"), UnitPrice: cd("10")}, "buy"); err != nil {
		t.Fatal(err)
	}
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("10"), AsOf: start.Unix()})
	now = start.Add(time.Duration(SecondsPerYear/2) * time.Second)
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("12"), AsOf: now.Unix()})
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("120"), "flow"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Trade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: cd("10"), UnitPrice: cd("12")}, "rebalance"); err != nil {
		t.Fatal(err)
	}
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("15"), AsOf: end.Unix()})
	twr, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "TWR", MaxAge: SecondsPerYear})
	if err != nil || !twr.Rate.Equal(cd("0.5")) || twr.Annualized || twr.ExternalFlows != 1 || twr.Subperiods != 2 || !twr.StartNAV.Equal(cd("100")) || !twr.EndNAV.Equal(cd("300")) {
		t.Fatalf("TWR=%+v err=%v", twr, err)
	}
	mwr, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "MWR", MaxAge: SecondsPerYear, IRR: DefaultIRROptions()})
	x := (math.Sqrt(336) - 6) / 10
	reference := decimalFromRate(x*x - 1)
	if err != nil || !mwr.Annualized || mwr.Rate.Sub(reference).Abs().GreaterThan(cd("0.00000001")) || mwr.Solver == nil || mwr.Solver.NPVResidual.GreaterThan(DefaultIRROptions().NPVTolerance) || mwr.Solver.RateBracketWidth.GreaterThan(DefaultIRROptions().RateTolerance) {
		t.Fatalf("MWR=%+v reference=%s err=%v", mwr, reference, err)
	}
	limited := DefaultIRROptions()
	limited.MaxIterations = 1
	if _, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "MWR", MaxAge: SecondsPerYear, IRR: limited}); err == nil {
		t.Fatal("nonconvergence hidden")
	}
	now = end.Add(time.Second)
	if _, err := svc.Trade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Sell, Currency: "USD", Quantity: cd("20"), UnitPrice: cd("15")}, "later-sell"); err != nil {
		t.Fatal(err)
	}
	replay, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "TWR", MaxAge: SecondsPerYear})
	if err != nil || !replay.Rate.Equal(twr.Rate) {
		t.Fatal("current holdings leaked into historical returns", err)
	}
}

func TestSameTimeFlowsUseIndividualSnapshotsAndFeesAreInternal(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	start := time.Unix(100, 0)
	end := start.Add(time.Duration(SecondsPerYear) * time.Second)
	now := start
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
	prices := repository.NewStaticPriceFeed()
	svc, _ := NewPortfolioService(state, prices, nil, domain.FIFO)
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("100"), "initial"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Trade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: cd("1"), UnitPrice: cd("0")}, "free-holding"); err != nil {
		t.Fatal(err)
	}
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("0"), AsOf: start.Unix()})
	now = start.Add(time.Duration(SecondsPerYear/2) * time.Second)
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("100"), "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CorporateAction(ctx, id, "ACME", domain.CorporateAction{Kind: domain.Fee, Currency: "USD", Amount: cd("20")}, "fee"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("100"), "second"); err != nil {
		t.Fatal(err)
	}
	twr, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "TWR", MaxAge: SecondsPerYear})
	if err != nil || !twr.Rate.Equal(cd("-0.1")) || twr.ExternalFlows != 2 || twr.Subperiods != 3 || !twr.EndNAV.Equal(cd("280")) {
		t.Fatalf("same-time TWR=%+v err=%v", twr, err)
	}
	mwr, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "MWR", MaxAge: SecondsPerYear, IRR: DefaultIRROptions()})
	x := (math.Sqrt(380) - 10) / 10
	if err != nil || mwr.Rate.Sub(decimalFromRate(x*x-1)).Abs().GreaterThan(cd("0.00000001")) {
		t.Fatalf("same-time MWR=%+v err=%v", mwr, err)
	}
}

type changingPriceSnapshot struct {
	*repository.StaticPriceFeed
	change func()
}

func (f changingPriceSnapshot) SnapshotPrices() repository.PriceFeed {
	copy := f.StaticPriceFeed.SnapshotPrices()
	f.change()
	return copy
}
func TestReturnQueryFreezesBookAndPriceViews(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	from := time.Unix(100, 0)
	to := from.Add(time.Duration(SecondsPerYear) * time.Second)
	now := from
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
	prices := repository.NewStaticPriceFeed()
	setup, _ := NewPortfolioService(state, prices, nil, domain.FIFO)
	if _, err := setup.MoveCash(ctx, id, "USD", "FUND", cd("100"), "fund"); err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Trade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: cd("10"), UnitPrice: cd("10")}, "buy"); err != nil {
		t.Fatal(err)
	}
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("10"), AsOf: from.Unix()})
	changing := changingPriceSnapshot{StaticPriceFeed: prices, change: func() {
		prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: cd("11"), AsOf: to.Unix()})
		now = to
		if _, err := setup.MoveCash(ctx, id, "USD", "FUND", cd("100"), "concurrent"); err != nil {
			t.Fatal(err)
		}
	}}
	svc, _ := NewPortfolioService(state, changing, nil, domain.FIFO)
	value, err := svc.Returns(ctx, id, "USD", from, to, ReturnOptions{Method: "TWR", MaxAge: SecondsPerYear})
	if err != nil || !value.Rate.IsZero() || value.ExternalFlows != 0 || !value.EndNAV.Equal(cd("100")) {
		t.Fatalf("mixed query views=%+v err=%v", value, err)
	}
}

func TestReturnUndefinedCapitalSignPatternAndFX(t *testing.T) {
	ctx := context.Background()
	start := time.Unix(100, 0)
	end := start.Add(time.Duration(SecondsPerYear) * time.Second)
	for _, scenario := range []string{"zero-start", "withdraw-all", "nonconventional", "fx"} {
		t.Run(scenario, func(t *testing.T) {
			now := start
			state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
			svc, _ := NewPortfolioService(state, repository.NewStaticPriceFeed(), nil, domain.FIFO)
			id := uuid.New()
			if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("100"), "fund"); err != nil {
				t.Fatal(err)
			}
			if scenario == "zero-start" {
				if _, err := svc.MoveCash(ctx, id, "USD", "WITHDRAW", cd("100"), "empty"); err != nil {
					t.Fatal(err)
				}
			}
			now = start.Add(time.Duration(SecondsPerYear/2) * time.Second)
			switch scenario {
			case "withdraw-all":
				if _, err := svc.MoveCash(ctx, id, "USD", "WITHDRAW", cd("100"), "withdraw"); err != nil {
					t.Fatal(err)
				}
			case "nonconventional":
				if _, err := svc.MoveCash(ctx, id, "USD", "WITHDRAW", cd("10"), "withdraw"); err != nil {
					t.Fatal(err)
				}
				now = now.Add(24 * time.Hour)
				if _, err := svc.MoveCash(ctx, id, "USD", "FUND", cd("10"), "refill"); err != nil {
					t.Fatal(err)
				}
			case "fx":
				if _, err := svc.MoveCash(ctx, id, "EUR", "FUND", cd("1"), "eur"); err != nil {
					t.Fatal(err)
				}
				now = now.Add(time.Second)
				if _, err := svc.MoveCash(ctx, id, "EUR", "WITHDRAW", cd("1"), "remove-eur"); err != nil {
					t.Fatal(err)
				}
			}
			twr, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "TWR", MaxAge: 0})
			if scenario == "nonconventional" {
				if err != nil || !twr.Rate.IsZero() {
					t.Fatalf("TWR=%+v err=%v", twr, err)
				}
			} else if err == nil {
				t.Fatal("undefined capital/FX accepted")
			}
			mwr, err := svc.Returns(ctx, id, "USD", start, end, ReturnOptions{Method: "MWR", MaxAge: 0, IRR: DefaultIRROptions()})
			if scenario == "withdraw-all" {
				if err != nil || mwr.Rate.Abs().GreaterThan(cd("0.00000001")) {
					t.Fatalf("withdrawn MWR=%+v err=%v", mwr, err)
				}
			} else if err == nil {
				t.Fatal("unsupported MWR accepted")
			}
			if scenario == "nonconventional" && !strings.Contains(err.Error(), "nonconventional") {
				t.Fatal(err)
			}
		})
	}
}

type returnState struct {
	repository.PortfolioState
	window repository.ReturnWindow
	err    error
}

func (s returnState) ReturnWindow(context.Context, uuid.UUID, time.Time, time.Time) (repository.ReturnWindow, error) {
	return s.window, s.err
}

type noPriceSnapshot struct{}

func (noPriceSnapshot) Price(context.Context, string, string) (domain.Price, error) {
	return domain.Price{}, errors.New("unused")
}
func (noPriceSnapshot) SnapshotPrices() repository.PriceFeed { return nil }

func TestReturnRequestAndAdapterErrors(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	from := time.Unix(100, 0)
	to := from.Add(time.Duration(SecondsPerYear) * time.Second)
	svc, _ := NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	for _, tc := range []struct {
		currency   string
		start, end time.Time
		options    ReturnOptions
	}{
		{"ZZZ", from, to, ReturnOptions{Method: "TWR"}}, {"USD", time.Time{}, to, ReturnOptions{Method: "TWR"}},
		{"USD", from, from, ReturnOptions{Method: "TWR"}}, {"USD", from, from.Add(time.Duration(11*SecondsPerYear) * time.Second), ReturnOptions{Method: "TWR"}},
		{"USD", from, to, ReturnOptions{Method: "OTHER"}}, {"USD", from, to, ReturnOptions{Method: "TWR", MaxAge: -1}},
		{"USD", from, to, ReturnOptions{Method: "MWR"}},
	} {
		if _, err := svc.Returns(ctx, id, tc.currency, tc.start, tc.end, tc.options); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if _, err := svc.Returns(ctx, id, "USD", from, to, ReturnOptions{Method: "TWR"}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal(err)
	}
	window := repository.ReturnWindow{Start: repository.PortfolioSnapshot{Cash: map[string]decimal.Decimal{"USD": cd("100")}}, End: repository.PortfolioSnapshot{Cash: map[string]decimal.Decimal{"USD": cd("100")}}}
	for _, feed := range []PriceFeed{PriceFunc(func(context.Context, string, string) (domain.Price, error) { return domain.Price{}, nil }), noPriceSnapshot{}} {
		reader, _ := NewPortfolioService(returnState{window: window}, feed, nil, domain.FIFO)
		if _, err := reader.Returns(ctx, id, "USD", from, to, ReturnOptions{Method: "TWR"}); !errors.Is(err, ErrReturnsUnavailable) {
			t.Fatal(err)
		}
	}
	validFlow := repository.HistoryRecord{Sequence: 1, OccurredAt: from.Add(time.Hour), Kind: "FUND", Currency: "USD", CashDelta: cd("100"), InvestorFlow: cd("-100"), Snapshot: repository.PortfolioSnapshot{Cash: map[string]decimal.Decimal{"USD": cd("200")}}}
	for _, change := range []func(*repository.HistoryRecord){
		func(f *repository.HistoryRecord) { f.Currency = "EUR" }, func(f *repository.HistoryRecord) { f.Sequence = 0 },
		func(f *repository.HistoryRecord) { f.OccurredAt = from }, func(f *repository.HistoryRecord) { f.Kind = "BUY" },
		func(f *repository.HistoryRecord) { f.InvestorFlow = cd("0") }, func(f *repository.HistoryRecord) {
			f.Snapshot = repository.PortfolioSnapshot{Cash: map[string]decimal.Decimal{"USD": cd("-1")}}
		},
		func(f *repository.HistoryRecord) {
			f.Snapshot = repository.PortfolioSnapshot{Cash: map[string]decimal.Decimal{"USD": cd("100")}}
		},
	} {
		flow := validFlow
		change(&flow)
		copy := window
		copy.Flows = []repository.HistoryRecord{flow}
		reader, _ := NewPortfolioService(returnState{window: copy}, repository.NewStaticPriceFeed(), nil, domain.FIFO)
		if _, err := reader.Returns(ctx, id, "USD", from, to, ReturnOptions{Method: "TWR"}); err == nil {
			t.Fatalf("accepted invalid flow=%+v", flow)
		}
	}
	for _, badStart := range []bool{true, false} {
		copy := window
		invalid := repository.PortfolioSnapshot{Cash: map[string]decimal.Decimal{"EUR": cd("1")}}
		if badStart {
			copy.Start = invalid
		} else {
			copy.End = invalid
		}
		reader, _ := NewPortfolioService(returnState{window: copy}, repository.NewStaticPriceFeed(), nil, domain.FIFO)
		if _, err := reader.Returns(ctx, id, "USD", from, to, ReturnOptions{Method: "TWR"}); err == nil {
			t.Fatal("invalid boundary snapshot accepted")
		}
	}
}
