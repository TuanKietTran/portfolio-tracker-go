package main

import (
	"context"
	"fmt"
	"log"

	"github.com/example/portfolio-tracker/internal/command"
	"github.com/example/portfolio-tracker/internal/domain"
	"github.com/example/portfolio-tracker/internal/query"
	"github.com/example/portfolio-tracker/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type memRepo struct{}

func (m *memRepo) SaveTrade(ctx context.Context, t domain.Trade) error {
	fmt.Printf("Recorded trade %s %s %s @ %s\n", t.Side, t.Quantity, t.Symbol, t.Price)
	return nil
}
func (m *memRepo) GetPositions(ctx context.Context, id domain.PortfolioID) ([]domain.Position, error) {
	return []domain.Position{{
		Symbol:      "AAPL",
		Quantity:    decimal.NewFromInt(10),
		AverageCost: decimal.NewFromFloat(150),
	}}, nil
}

func main() {
	repo := &memRepo{}
	recH := command.NewRecordTradeHandler(repo)
	sumH := query.NewGetSummaryHandler(repo)

	pid := uuid.New()
	err := recH.Handle(context.Background(), command.RecordTradeCommand{
		PortfolioID: pid,
		Symbol:      "AAPL",
		Side:        "BUY",
		Quantity:    decimal.NewFromInt(10),
		Price:       decimal.NewFromFloat(150.5),
	})
	if err != nil {
		log.Fatal(err)
	}

	s, err := sumH.Handle(context.Background(), query.GetSummaryQuery{PortfolioID: pid})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Positions: %+v\n", s.Positions)
}
