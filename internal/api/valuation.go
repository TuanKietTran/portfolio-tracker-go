package api

import (
	"net/http"
	"time"
)

func (h *PortfolioHandlers) valuation(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	asOf, maxAge, err := valuationTimes(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	result, err := h.service.Valuation(r.Context(), id, r.URL.Query().Get("currency"), time.Unix(asOf, 0), maxAge)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeMarket(w, 200, result)
}
