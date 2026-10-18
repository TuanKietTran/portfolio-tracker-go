package api

import (
	"encoding/json"
	"net/http"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
)

func (h *PortfolioHandlers) createPortfolio(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Currency       string            `json:"currency"`
		CostMethod     domain.CostMethod `json:"cost_method"`
		IdempotencyKey string            `json:"idempotency_key"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	key, err := requestKey(r, req.IdempotencyKey)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	p, err := h.service.CreatePortfolio(r.Context(), req.Currency, req.CostMethod, key)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/v1/portfolios/"+p.ID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

func (h *PortfolioHandlers) getPortfolio(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	p, err := h.service.Portfolio(r.Context(), id)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}
