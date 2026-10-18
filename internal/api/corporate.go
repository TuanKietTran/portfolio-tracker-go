package api

import (
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"net/http"
)

func (h *PortfolioHandlers) corporateAction(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	var req struct {
		domain.CorporateAction
		Symbol         string `json:"symbol"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	key, err := requestKey(r, req.IdempotencyKey)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	result, err := h.service.CorporateAction(r.Context(), id, req.Symbol, req.CorporateAction, key)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeMarket(w, 201, result)
}
