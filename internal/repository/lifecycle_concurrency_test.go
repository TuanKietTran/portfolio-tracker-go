package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
)

func TestDistinctConcurrentPortfolioCreationsRemainIsolated(t *testing.T) {
	ctx := context.Background()
	n := 0
	s, _ := NewInMemoryStateWithSources(ClockFunc(func() time.Time { return time.Unix(100, 0) }), func() (uuid.UUID, error) {
		n++
		return uuid.MustParse(fmt.Sprintf("00000000-0000-4000-8000-%012d", n)), nil
	})
	results := make(chan Portfolio, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, e := s.CreatePortfolio(ctx, "JPY", domain.FIFO, fmt.Sprintf("create-%d", i))
			if e != nil {
				t.Error(e)
				return
			}
			results <- p
		}(i)
	}
	wg.Wait()
	close(results)
	seen := map[uuid.UUID]bool{}
	for p := range results {
		if seen[p.ID] {
			t.Fatal("duplicate identity")
		}
		seen[p.ID] = true
		rows, e := s.History(ctx, p.ID, 0, 100)
		if e != nil || len(rows) != 1 || rows[0].Sequence != 1 {
			t.Fatal(rows, e)
		}
		got, e := s.GetPortfolio(ctx, p.ID)
		if e != nil || got != p {
			t.Fatal(got, e)
		}
	}
	if n != 100 || len(seen) != 100 {
		t.Fatal(n, len(seen))
	}
}
