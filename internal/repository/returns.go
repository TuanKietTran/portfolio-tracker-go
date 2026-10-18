package repository

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"sort"
	"time"
)

type ReturnWindow struct {
	Start PortfolioSnapshot
	End   PortfolioSnapshot
	Flows []HistoryRecord
}

// ReturnWindow captures one consistent committed holdings/flow view. Boundaries
// include all same-time records; external flows are selected from (from,to].
func (s *InMemoryState) ReturnWindow(ctx context.Context, id uuid.UUID, from, to time.Time) (ReturnWindow, error) {
	if err := ctx.Err(); err != nil {
		return ReturnWindow{}, err
	}
	from, to = from.UTC(), to.UTC()
	if id == uuid.Nil || from.Unix() <= 0 || to.Year() > 9999 || !to.After(from) {
		return ReturnWindow{}, errors.New("valid portfolio and increasing return window required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := s.history[id]
	start := sort.Search(len(rows), func(i int) bool { return rows[i].OccurredAt.After(from) }) - 1
	end := sort.Search(len(rows), func(i int) bool { return rows[i].OccurredAt.After(to) }) - 1
	if start < 0 || end < 0 {
		return ReturnWindow{}, ErrNotFound
	}
	if end-start > 4096 {
		return ReturnWindow{}, errors.New("return window exceeds4096 portfolio commands")
	}
	result := ReturnWindow{Start: rows[start].Snapshot.clone(), End: rows[end].Snapshot.clone(), Flows: []HistoryRecord{}}
	for i := start + 1; i <= end; i++ {
		if !rows[i].InvestorFlow.IsZero() {
			if len(result.Flows) >= 256 {
				return ReturnWindow{}, errors.New("return window exceeds256 external flows")
			}
			result.Flows = append(result.Flows, rows[i].clone())
		}
	}
	return result, nil
}
