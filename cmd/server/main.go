package main

import (
	"log"
	"net/http"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/api"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/query"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	repo := repository.NewInMemoryPortfolio()
	recH := command.NewRecordTradeHandler(repo)
	sumH := query.NewGetSummaryHandler(repo)
	handlers := api.NewHandlers(recH, sumH)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Mount("/v1", handlers.Routes())

	log.Println("Portfolio API (in-memory) listening on :8083")
	log.Fatal(http.ListenAndServe(":8083", r))
}
