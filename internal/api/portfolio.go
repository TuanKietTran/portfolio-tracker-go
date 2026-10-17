package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PortfolioHandlers struct{ service *command.PortfolioService }

func NewPortfolioHandlers(service *command.PortfolioService) *PortfolioHandlers {
	return &PortfolioHandlers{service: service}
}
func (h *PortfolioHandlers) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/portfolios/{id}/positions/{symbol}", h.position)
	r.Post("/portfolios/{id}/positions/{symbol}/trades", h.trade)
	r.Get("/portfolios/{id}/cash", h.cash)
	r.Post("/portfolios/{id}/cash/fund", h.fund)
	r.Post("/portfolios/{id}/cash/withdraw", h.withdraw)
	return r
}

type positionResponse struct {
	Symbol        string `json:"symbol"`
	Quantity      string `json:"quantity"`
	CostBasis     string `json:"cost_basis"`
	RealizedPnL   string `json:"realized_pnl"`
	MarketValue   string `json:"market_value"`
	UnrealizedPnL string `json:"unrealized_pnl"`
	Currency      string `json:"currency"`
	PriceAsOf     int64  `json:"price_as_of"`
}

func portfolioID(r *http.Request) (uuid.UUID, error) { return uuid.Parse(chi.URLParam(r, "id")) }
func writePortfolioError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, repository.ErrInsufficientCash) || errors.Is(err, repository.ErrIdempotencyConflict) {
		status = http.StatusConflict
	}
	http.Error(w, err.Error(), status)
}
func (h *PortfolioHandlers) position(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		http.Error(w, "invalid portfolio id", 400)
		return
	}
	currency := r.URL.Query().Get("currency")
	asOf, _ := strconv.ParseInt(r.URL.Query().Get("as_of"), 10, 64)
	maxAge, _ := strconv.ParseInt(r.URL.Query().Get("max_age"), 10, 64)
	p, v, err := h.service.Position(r.Context(), id, chi.URLParam(r, "symbol"), currency, asOf, maxAge)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(positionResponse{Symbol: chi.URLParam(r, "symbol"), Quantity: p.Quantity.String(), CostBasis: p.CostBasis.String(), RealizedPnL: p.RealizedPnL.String(), MarketValue: v.MarketValue.String(), UnrealizedPnL: v.UnrealizedPnL.String(), Currency: v.Currency, PriceAsOf: v.AsOf})
}
func (h *PortfolioHandlers) cash(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		http.Error(w, "invalid portfolio id", 400)
		return
	}
	currency := r.URL.Query().Get("currency")
	balance, err := h.service.Cash(r.Context(), id, currency)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Currency string `json:"currency"`
		Balance  string `json:"balance"`
	}{currency, balance.String()})
}

type accountingTradeRequest struct {
	Side     string          `json:"side"`
	Quantity decimal.Decimal `json:"quantity"`
	Price    decimal.Decimal `json:"price"`
}

func (h *PortfolioHandlers) trade(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		http.Error(w, "invalid portfolio id", 400)
		return
	}
	var req accountingTradeRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	kind := domain.EventKind(req.Side)
	if kind != "BUY" && kind != "SELL" {
		http.Error(w, "side must be BUY or SELL", 400)
		return
	}
	p, err := h.service.Trade(r.Context(), id, chi.URLParam(r, "symbol"), domain.AccountingEvent{Kind: kind, Quantity: req.Quantity, UnitPrice: req.Price})
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

type cashRequest struct {
	Currency       string          `json:"currency"`
	Amount         decimal.Decimal `json:"amount"`
	IdempotencyKey string          `json:"idempotency_key"`
}

func (h *PortfolioHandlers) moveCash(w http.ResponseWriter, r *http.Request, direction string) {
	id, err := portfolioID(r)
	if err != nil {
		http.Error(w, "invalid portfolio id", 400)
		return
	}
	var req cashRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	balance, err := h.service.MoveCash(r.Context(), id, req.Currency, direction, req.Amount, req.IdempotencyKey)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Currency string `json:"currency"`
		Balance  string `json:"balance"`
	}{req.Currency, balance.String()})
}
func (h *PortfolioHandlers) fund(w http.ResponseWriter, r *http.Request) { h.moveCash(w, r, "FUND") }
func (h *PortfolioHandlers) withdraw(w http.ResponseWriter, r *http.Request) {
	h.moveCash(w, r, "WITHDRAW")
}
