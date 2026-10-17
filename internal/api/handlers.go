package api

import (
	"encoding/json"
	"net/http"

	"github.com/example/portfolio-tracker/internal/command"
	"github.com/example/portfolio-tracker/internal/query"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Handlers struct {
	record  *command.RecordTradeHandler
	summary *query.GetSummaryHandler
}

func NewHandlers(rec *command.RecordTradeHandler, sum *query.GetSummaryHandler) *Handlers {
	return &Handlers{record: rec, summary: sum}
}

func (h *Handlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/portfolios/{id}/trades", h.recordTrade)
	r.Get("/portfolios/{id}/summary", h.getSummary)
	return r
}

type tradeReq struct {
	Symbol   string          `json:"symbol"`
	Side     string          `json:"side"`
	Quantity decimal.Decimal `json:"quantity"`
	Price    decimal.Decimal `json:"price"`
}

func (h *Handlers) recordTrade(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid portfolio id", http.StatusBadRequest)
		return
	}
	var req tradeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	err = h.record.Handle(r.Context(), command.RecordTradeCommand{
		PortfolioID: pid,
		Symbol:      req.Symbol,
		Side:        req.Side,
		Quantity:    req.Quantity,
		Price:       req.Price,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handlers) getSummary(w http.ResponseWriter, r *http.Request) {
	pid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid portfolio id", http.StatusBadRequest)
		return
	}
	s, err := h.summary.Handle(r.Context(), query.GetSummaryQuery{PortfolioID: pid})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}
