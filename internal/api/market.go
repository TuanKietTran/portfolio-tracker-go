package api

import (
	"encoding/json"
	"net/http"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/go-chi/chi/v5"
)

func writeMarket(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func (h *PortfolioHandlers) registerInstrument(w http.ResponseWriter, r *http.Request) {
	var req struct {
		domain.Instrument
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
	instrument, err := h.service.RegisterInstrument(r.Context(), req.Instrument, key)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeMarket(w, 201, instrument)
}
func (h *PortfolioHandlers) instrument(w http.ResponseWriter, r *http.Request) {
	instrument, err := h.service.Instrument(r.Context(), chi.URLParam(r, "symbol"))
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeMarket(w, 200, instrument)
}
func (h *PortfolioHandlers) publishPrice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		domain.Price
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
	price, err := h.service.PublishPrice(r.Context(), req.Price, key)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeMarket(w, 201, price)
}
func (h *PortfolioHandlers) price(w http.ResponseWriter, r *http.Request) {
	asOf, maxAge, err := valuationTimes(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	price, err := h.service.Price(r.Context(), chi.URLParam(r, "symbol"), r.URL.Query().Get("currency"), asOf, maxAge)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeMarket(w, 200, price)
}
