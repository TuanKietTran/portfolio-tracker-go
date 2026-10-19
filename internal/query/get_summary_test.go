package query

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
)

type summaryRepository struct {
	positions []domain.Position
	err       error
}

func (r summaryRepository) SaveTrade(context.Context, domain.Trade) error { return nil }
func (r summaryRepository) GetPositions(context.Context, domain.PortfolioID) ([]domain.Position, error) {
	return r.positions, r.err
}

func TestGetSummaryReturnsRepositoryPositions(t *testing.T) {
	want := []domain.Position{{Symbol: "ACME"}}
	handler := NewGetSummaryHandler(summaryRepository{positions: want})
	got, err := handler.Handle(context.Background(), GetSummaryQuery{PortfolioID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Positions, want) {
		t.Fatalf("positions = %#v, want %#v", got.Positions, want)
	}
	if !got.TotalValue.IsZero() || !got.UnrealizedPnL.IsZero() {
		t.Fatalf("unvalued summary totals = %s/%s, want zero", got.TotalValue, got.UnrealizedPnL)
	}
}

func TestGetSummaryPropagatesRepositoryFailureWithoutPartialResult(t *testing.T) {
	failure := errors.New("repository unavailable")
	handler := NewGetSummaryHandler(summaryRepository{positions: []domain.Position{{Symbol: "SHOULD_NOT_ESCAPE"}}, err: failure})
	got, err := handler.Handle(context.Background(), GetSummaryQuery{})
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if len(got.Positions) != 0 || !got.TotalValue.IsZero() || !got.UnrealizedPnL.IsZero() {
		t.Fatalf("partial summary escaped on error: %+v", got)
	}
}
