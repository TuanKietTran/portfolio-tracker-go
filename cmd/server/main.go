package main

import (
	"context"
	"log"
	"net/http"

	"github.com/example/portfolio-tracker/internal/api"
	"github.com/example/portfolio-tracker/internal/command"
	"github.com/example/portfolio-tracker/internal/domain"
	"github.com/example/portfolio-tracker/internal/query"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shopspring/decimal"
)

type memRepo struct{}

func (m *memRepo) SaveTrade(ctx context.Context, t domain.Trade) error { return nil }
func (m *memRepo) GetPositions(ctx context.Context, id domain.PortfolioID) ([]domain.Position, error) {
	return []domain.Position{{Symbol: "AAPL", Quantity: decimal.NewFromInt(10), AverageCost: decimal.NewFromFloat(150)}}, nil
}

func main() {
	repo := &memRepo{}
	recH := command.NewRecordTradeHandler(repo)
	sumH := query.NewGetSummaryHandler(repo)
	handlers := api.NewHandlers(recH, sumH)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Mount("/v1", handlers.Routes())

	log.Println("Portfolio API listening on :8083")
	log.Fatal(http.ListenAndServe(":8083", r))
}
