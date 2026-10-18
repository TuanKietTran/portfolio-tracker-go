package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestConsistentReturnWindowBoundaryTiesAndIsolation(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	now := time.Unix(100, 0)
	s, _ := NewInMemoryStateWithClock(ClockFunc(func() time.Time { return now }))
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "start"); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(150, 0)
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "flow1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("20"), "flow2"); err != nil {
		t.Fatal(err)
	}
	window, err := s.ReturnWindow(ctx, id, time.Unix(100, 0), time.Unix(200, 0))
	if err != nil || len(window.Flows) != 2 || !window.Start.Cash["USD"].Equal(pd("100")) || !window.End.Cash["USD"].Equal(pd("180")) || window.Flows[0].Sequence != 2 || window.Flows[1].Sequence != 3 || !window.Flows[0].Snapshot.Cash["USD"].Equal(pd("200")) {
		t.Fatalf("window=%+v err=%v", window, err)
	}
	window.Start.Cash["USD"] = pd("999")
	window.End.Cash["USD"] = pd("999")
	window.Flows[0].Snapshot.Cash["USD"] = pd("999")
	fresh, _ := s.ReturnWindow(ctx, id, time.Unix(100, 0), time.Unix(200, 0))
	if !fresh.Start.Cash["USD"].Equal(pd("100")) || !fresh.End.Cash["USD"].Equal(pd("180")) || !fresh.Flows[0].Snapshot.Cash["USD"].Equal(pd("200")) {
		t.Fatal("window aliases stored history")
	}
	tied, _ := s.ReturnWindow(ctx, id, time.Unix(150, 0), time.Unix(200, 0))
	if len(tied.Flows) != 0 || !tied.Start.Cash["USD"].Equal(pd("180")) {
		t.Fatal("start-time flows double counted")
	}
	if _, err := s.ReturnWindow(ctx, id, time.Unix(99, 0), time.Unix(200, 0)); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.ReturnWindow(ctx, uuid.Nil, time.Unix(100, 0), time.Unix(200, 0)); err == nil {
		t.Fatal("nil ID accepted")
	}
	if _, err := s.ReturnWindow(ctx, id, time.Unix(200, 0), time.Unix(100, 0)); err == nil {
		t.Fatal("reversed window accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.ReturnWindow(canceled, id, time.Unix(100, 0), time.Unix(200, 0)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestReturnWindowWorkBoundsAndFrozenPriceHistory(t *testing.T) {
	for _, count := range []int{257, 4097} {
		now := time.Unix(100, 0)
		s, _ := NewInMemoryStateWithClock(ClockFunc(func() time.Time { return now }))
		id := uuid.New()
		ctx := context.Background()
		if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("1"), "start"); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < count; i++ {
			now = now.Add(time.Second)
			if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("0.01"), fmt.Sprint(i)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.ReturnWindow(ctx, id, time.Unix(100, 0), now); err == nil {
			t.Fatalf("unbounded window count=%d accepted", count)
		}
	}
	feed := NewStaticPriceFeed()
	ctx := context.Background()
	feed.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: pd("10"), AsOf: 100})
	frozen := feed.SnapshotPrices().(*StaticPriceFeed)
	if _, err := feed.Publish(ctx, domain.Price{Symbol: "ACME", Currency: "USD", Amount: pd("20"), AsOf: 200}, "new"); err != nil {
		t.Fatal(err)
	}
	feed.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: pd("99"), AsOf: 100})
	price, err := frozen.PriceAt(ctx, "ACME", "USD", 200)
	if err != nil || !price.Amount.Equal(pd("10")) {
		t.Fatalf("price snapshot aliases source: %+v %v", price, err)
	}
}
