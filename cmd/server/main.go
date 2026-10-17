package main

import (
	"log"
	"net/http"

	"github.com/example/portfolio-tracker/internal/api"
	"github.com/example/portfolio-tracker/internal/command"
	"github.com/example/portfolio-tracker/internal/query"
	"github.com/example/portfolio-tracker/internal/repository"
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
