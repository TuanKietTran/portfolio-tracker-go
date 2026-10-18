package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
)

func TestTimestampedCommandHistoryFlowsSnapshotsAndIsolation(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	now := time.Unix(100, 123).In(time.FixedZone("synthetic", 7*3600))
	s, err := NewInMemoryStateWithClock(ClockFunc(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "fund"); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(110, 123)
	buy := domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("2"), UnitPrice: pd("10")}
	if _, err = s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	for i, action := range []domain.CorporateAction{{Kind: domain.Dividend, Currency: "USD", Amount: pd("4")}, {Kind: domain.Fee, Currency: "USD", Amount: pd("1")}, {Kind: domain.Split, Currency: "USD", Numerator: 2, Denominator: 1}} {
		now = time.Unix(int64(120+i*10), 123)
		if _, err = s.ApplyCorporateAction(ctx, id, "ACME", action, fmt.Sprint("action", i)); err != nil {
			t.Fatal(err)
		}
	}
	now = time.Unix(150, 123)
	if _, err = s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("10"), "withdraw"); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(160, 123)
	if _, err = s.ApplyTrade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Sell, Currency: "USD", Quantity: pd("4"), UnitPrice: pd("5")}, domain.FIFO, "sell"); err != nil {
		t.Fatal(err)
	}
	rows, err := s.History(ctx, id, 0, 1000)
	if err != nil || len(rows) != 7 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for i, kind := range []string{"FUND", "BUY", "DIVIDEND", "FEE", "SPLIT", "WITHDRAW", "SELL"} {
		row := rows[i]
		if row.Kind != kind || row.Sequence != uint64(i+1) || row.OccurredAt.Location() != time.UTC || row.OccurredAt.Unix() != int64(100+i*10) || row.OccurredAt.Nanosecond() != 123 {
			t.Fatalf("row=%+v", row)
		}
		if i != 0 && i != 5 && !row.InvestorFlow.IsZero() {
			t.Fatalf("internal command classified as external flow: %+v", row)
		}
	}
	if !rows[0].InvestorFlow.Equal(pd("-100")) || !rows[5].InvestorFlow.Equal(pd("10")) || !rows[1].CashDelta.Equal(pd("-20")) || !rows[2].CashDelta.Equal(pd("4")) || !rows[3].CashDelta.Equal(pd("-1")) || !rows[4].CashDelta.IsZero() || !rows[6].CashDelta.Equal(pd("20")) {
		t.Fatal("cash-flow signs/classification wrong")
	}
	if !rows[4].Snapshot.Positions["ACME"].Quantity.Equal(pd("4")) || rows[4].Numerator != 2 || rows[4].Denominator != 1 {
		t.Fatal("split history payload/snapshot wrong")
	}
	for _, tc := range []struct {
		at             int64
		quantity, cash string
	}{{100, "0", "100"}, {115, "2", "80"}, {140, "4", "83"}, {160, "0", "93"}} {
		snapshot, err := s.SnapshotAt(ctx, id, time.Unix(tc.at, 123))
		if err != nil || !snapshot.Positions["ACME"].Quantity.Equal(pd(tc.quantity)) || !snapshot.Cash["USD"].Equal(pd(tc.cash)) {
			t.Fatalf("cutoff=%d snapshot=%+v err=%v", tc.at, snapshot, err)
		}
	}
	page, err := s.History(ctx, id, 2, 2)
	if err != nil || len(page) != 2 || page[0].Sequence != 3 || page[1].Sequence != 4 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	empty, err := s.History(ctx, id, 9999, 2)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty=%v err=%v", empty, err)
	}
	rows[1].Snapshot.Positions["ACME"].Lots[0].Quantity = pd("999")
	rows[0].Snapshot.Cash["USD"] = pd("999")
	delete(rows[4].Snapshot.Positions, "ACME")
	fresh, _ := s.History(ctx, id, 0, 1000)
	if !fresh[1].Snapshot.Positions["ACME"].Lots[0].Quantity.Equal(pd("2")) || !fresh[0].Snapshot.Cash["USD"].Equal(pd("100")) || len(fresh[4].Snapshot.Positions) != 1 {
		t.Fatal("history reads alias internal snapshots")
	}
	snapshot, _ := s.SnapshotAt(ctx, id, time.Unix(115, 123))
	snapshot.Positions["ACME"].Lots[0].Quantity = pd("999")
	snapshot.Cash["USD"] = pd("999")
	snapshot, _ = s.SnapshotAt(ctx, id, time.Unix(115, 123))
	if !snapshot.Positions["ACME"].Lots[0].Quantity.Equal(pd("2")) || !snapshot.Cash["USD"].Equal(pd("80")) {
		t.Fatal("historical snapshot aliases state")
	}
	current, _ := s.GetAccountPosition(ctx, id, "ACME")
	cash, _ := s.CashBalance(ctx, id, "USD")
	if !current.Quantity.IsZero() || !cash.Equal(pd("93")) {
		t.Fatal("history caller changed live state")
	}
}

func TestHistoryClockFailureRollbackReplayAndRecovery(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	now := time.Unix(100, 0)
	calls := 0
	s, _ := NewInMemoryStateWithClock(ClockFunc(func() time.Time { calls++; return now }))
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "fund"); err != nil {
		t.Fatal(err)
	}
	buy := domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("2"), UnitPrice: pd("10")}
	now = time.Unix(110, 0)
	if _, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(90, 0)
	sell := domain.AccountingEvent{Kind: domain.Sell, Currency: "USD", Quantity: pd("1"), UnitPrice: pd("15")}
	fee := domain.CorporateAction{Kind: domain.Fee, Currency: "USD", Amount: pd("1")}
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("10"), "more"); !errors.Is(err, ErrHistoryClock) {
		t.Fatal(err)
	}
	if _, err := s.ApplyTrade(ctx, id, "ACME", sell, domain.FIFO, "sell"); !errors.Is(err, ErrHistoryClock) {
		t.Fatal(err)
	}
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", fee, "fee"); !errors.Is(err, ErrHistoryClock) {
		t.Fatal(err)
	}
	if balance, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "fund"); err != nil || !balance.Equal(pd("100")) {
		t.Fatal("original replay depends on clock")
	}
	if _, err := s.ApplyTrade(ctx, id, "ACME", buy, domain.FIFO, "buy"); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.History(ctx, id, 0, 1000)
	cash, _ := s.CashBalance(ctx, id, "USD")
	p, _ := s.GetAccountPosition(ctx, id, "ACME")
	if len(rows) != 2 || !cash.Equal(pd("80")) || !p.Quantity.Equal(pd("2")) || !p.RealizedPnL.IsZero() || calls != 5 {
		t.Fatalf("partial commit/clock replay: rows=%d cash=%s p=%+v clock calls=%d", len(rows), cash, p, calls)
	}
	now = time.Unix(120, 0)
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("10"), "more"); err != nil {
		t.Fatal("clock failure consumed cash key", err)
	}
	if _, err := s.ApplyTrade(ctx, id, "ACME", sell, domain.FIFO, "sell"); err != nil {
		t.Fatal("clock failure consumed trade key", err)
	}
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", fee, "fee"); err != nil {
		t.Fatal("clock failure consumed action key", err)
	}
	rows, _ = s.History(ctx, id, 0, 1000)
	last, err := s.SnapshotAt(ctx, id, now)
	if err != nil || len(rows) != 5 || !last.Cash["USD"].Equal(pd("104")) || !last.Positions["ACME"].RealizedPnL.Equal(pd("4")) {
		t.Fatalf("tie ordering snapshot=%+v rows=%d err=%v", last, len(rows), err)
	}
	for _, invalid := range []time.Time{{}, time.Unix(-1, 0), time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)} {
		now = invalid
		if _, err := s.ApplyCorporateAction(ctx, id, "ACME", fee, "bad-time"); !errors.Is(err, ErrHistoryClock) {
			t.Fatal(err)
		}
	}
	now = time.Unix(130, 0)
	if _, err := s.ApplyCorporateAction(ctx, id, "ACME", fee, "bad-time"); err != nil {
		t.Fatal(err)
	}
	rows, _ = s.History(ctx, id, 0, 1000)
	if len(rows) != 6 || rows[5].Sequence != 6 {
		t.Fatal("failed clock consumed sequence/key")
	}
}

func TestConcurrentHistorySnapshotsAreOrderedAndAtomic(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	now := time.Unix(100, 0)
	calls := 0
	s, _ := NewInMemoryStateWithClock(ClockFunc(func() time.Time { calls++; return now }))
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("100"), "fund"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.ApplyTrade(ctx, id, "ACME", domain.AccountingEvent{Kind: domain.Buy, Currency: "USD", Quantity: pd("1"), UnitPrice: pd("2")}, domain.FIFO, fmt.Sprint("buy", i))
			if err != nil && !errors.Is(err, ErrInsufficientCash) {
				t.Error(err)
			}
			rows, err := s.History(ctx, id, 0, 1000)
			if err != nil {
				t.Error(err)
				return
			}
			for _, row := range rows {
				cost := row.Snapshot.Positions["ACME"].CostBasis
				if !row.Snapshot.Cash["USD"].Add(cost).Equal(pd("100")) {
					t.Errorf("partial snapshot at sequence%d", row.Sequence)
				}
			}
		}(i)
	}
	wg.Wait()
	rows, err := s.History(ctx, id, 0, 1000)
	if err != nil || len(rows) != 51 || calls != 51 {
		t.Fatalf("rows=%d clock calls=%d err=%v", len(rows), calls, err)
	}
	for i, row := range rows {
		if row.Sequence != uint64(i+1) || !row.OccurredAt.Equal(now) || !row.Snapshot.Positions["ACME"].Quantity.Equal(pd(fmt.Sprint(i))) {
			t.Fatalf("unordered/partial row=%+v", row)
		}
	}
	other := uuid.New()
	if _, err := s.MoveCash(ctx, other, "EUR", "FUND", pd("10"), "other"); err != nil {
		t.Fatal(err)
	}
	otherRows, _ := s.History(ctx, other, 0, 1000)
	if len(otherRows) != 1 || otherRows[0].Sequence != 1 || len(otherRows[0].Snapshot.Positions) != 0 || len(otherRows[0].Snapshot.Cash) != 1 {
		t.Fatal("portfolio history isolation failed")
	}
}

func TestHistoryReadValidationAndBusinessFailures(t *testing.T) {
	if _, err := NewInMemoryStateWithClock(nil); err == nil {
		t.Fatal("nil clock accepted")
	}
	if _, err := NewInMemoryStateWithClock(ClockFunc(nil)); err == nil {
		t.Fatal("nil clock function accepted")
	}
	ctx := context.Background()
	id := uuid.New()
	s, _ := NewInMemoryStateWithClock(ClockFunc(func() time.Time { return time.Unix(100, 0) }))
	if _, err := s.History(ctx, id, 0, 1); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.SnapshotAt(ctx, id, time.Unix(100, 0)); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for _, limit := range []int{0, 1001} {
		if _, err := s.History(ctx, id, 0, limit); err == nil {
			t.Fatal("bad limit accepted")
		}
	}
	if _, err := s.History(ctx, uuid.Nil, 0, 1); err == nil {
		t.Fatal("nil portfolio accepted")
	}
	for _, at := range []time.Time{{}, time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)} {
		if _, err := s.SnapshotAt(ctx, id, at); err == nil {
			t.Fatal("bad cutoff accepted")
		}
	}
	if _, err := s.SnapshotAt(ctx, uuid.Nil, time.Unix(100, 0)); err == nil {
		t.Fatal("nil portfolio cutoff accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.History(canceled, id, 0, 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.SnapshotAt(canceled, id, time.Unix(100, 0)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("10"), "fund"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MoveCash(ctx, id, "USD", "WITHDRAW", pd("11"), "bad"); !errors.Is(err, ErrInsufficientCash) {
		t.Fatal(err)
	}
	if _, err := s.MoveCash(ctx, id, "USD", "FUND", pd("11"), "fund"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	if _, err := s.SnapshotAt(ctx, id, time.Unix(99, 0)); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	rows, _ := s.History(ctx, id, 0, 1000)
	if len(rows) != 1 {
		t.Fatal("failed business command appended history")
	}
}
