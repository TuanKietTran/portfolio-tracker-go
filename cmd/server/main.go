package main

import (
	"log"
	"net/http"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/api"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	state := repository.NewInMemoryState()
	prices := repository.NewStaticPriceFeed()
	portfolioService, err := command.NewPortfolioService(state, prices, nil, "FIFO")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Portfolio API (in-memory) listening on :8105")
	log.Fatal(http.ListenAndServe(":8105", newRouter(portfolioService)))
}

func newRouter(service *command.PortfolioService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Mount("/v1", api.NewPortfolioHandlers(service).Routes())
	return r
}
