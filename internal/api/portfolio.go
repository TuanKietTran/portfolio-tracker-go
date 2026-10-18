package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	r.Post("/portfolios/{id}/trades", h.trade)
	r.Get("/portfolios/{id}/summary", h.summary)
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

func portfolioID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, errors.New("nonzero portfolio UUID required")
	}
	return id, nil
}
func writePortfolioError(w http.ResponseWriter, err error) {
	status, code := 400, "invalid_request"
	switch {
	case errors.Is(err, command.ErrPaymentCashUnavailable):
		status, code = 503, "payment_unavailable"
	case errors.Is(err, repository.ErrInsufficientCash):
		status, code = 409, "insufficient_cash"
	case errors.Is(err, repository.ErrIdempotencyConflict):
		status, code = 409, "idempotency_conflict"
	}
	writeError(w, status, code, err.Error())
}
func (h *PortfolioHandlers) position(w http.ResponseWriter, r *http.Request) {
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
	p, v, err := h.service.Position(r.Context(), id, chi.URLParam(r, "symbol"), r.URL.Query().Get("currency"), asOf, maxAge)
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
		writePortfolioError(w, err)
		return
	}
	currency := r.URL.Query().Get("currency")
	balance, err := h.service.Cash(r.Context(), id, currency)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeCash(w, currency, balance)
}
func writeCash(w http.ResponseWriter, currency string, balance decimal.Decimal) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Currency string `json:"currency"`
		Balance  string `json:"balance"`
	}{currency, balance.String()})
}
func valuationTimes(r *http.Request) (int64, int64, error) {
	asOf, err := strconv.ParseInt(r.URL.Query().Get("as_of"), 10, 64)
	if err != nil || asOf <= 0 {
		return 0, 0, errors.New("positive integer as_of required")
	}
	maxAge, err := strconv.ParseInt(r.URL.Query().Get("max_age"), 10, 64)
	if err != nil || maxAge < 0 {
		return 0, 0, errors.New("non-negative integer max_age required")
	}
	return asOf, maxAge, nil
}
func (h *PortfolioHandlers) summary(w http.ResponseWriter, r *http.Request) {
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
	summary, err := h.service.Summary(r.Context(), id, r.URL.Query().Get("currency"), asOf, maxAge)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

type accountingTradeRequest struct {
	Currency       string          `json:"currency"`
	Symbol         string          `json:"symbol"`
	Side           string          `json:"side"`
	Quantity       decimal.Decimal `json:"quantity"`
	Price          decimal.Decimal `json:"price"`
	IdempotencyKey string          `json:"idempotency_key"`
}

func requestKey(r *http.Request, body string) (string, error) {
	header := r.Header.Get("Idempotency-Key")
	if header != "" && body != "" && header != body {
		return "", errors.New("idempotency header and body disagree")
	}
	if header == "" {
		header = body
	}
	if strings.TrimSpace(header) == "" || len(header) > 200 {
		return "", errors.New("idempotency key must contain 1 through 200 nonblank bytes")
	}
	return header, nil
}
func (h *PortfolioHandlers) trade(w http.ResponseWriter, r *http.Request) {
	id, err := portfolioID(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	var req accountingTradeRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	key, err := requestKey(r, req.IdempotencyKey)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	kind := domain.EventKind(req.Side)
	if kind != domain.Buy && kind != domain.Sell {
		writePortfolioError(w, errors.New("side must be BUY or SELL"))
		return
	}
	symbol := chi.URLParam(r, "symbol")
	legacyPath := symbol == ""
	if legacyPath {
		symbol = req.Symbol
	}
	if !legacyPath && req.Symbol != "" && req.Symbol != symbol {
		writePortfolioError(w, errors.New("body symbol and path disagree"))
		return
	}
	p, err := h.service.Trade(r.Context(), id, symbol, domain.AccountingEvent{Kind: kind, Currency: req.Currency, Quantity: req.Quantity, UnitPrice: req.Price}, key)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	if !legacyPath {
		_ = json.NewEncoder(w).Encode(p)
	}
}

type cashRequest struct {
	Currency       string          `json:"currency"`
	Amount         decimal.Decimal `json:"amount"`
	IdempotencyKey string          `json:"idempotency_key"`
}

func (h *PortfolioHandlers) moveCash(w http.ResponseWriter, r *http.Request, direction string) {
	id, err := portfolioID(r)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	var req cashRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	key, err := requestKey(r, req.IdempotencyKey)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	balance, err := h.service.MoveCash(r.Context(), id, req.Currency, direction, req.Amount, key)
	if err != nil {
		writePortfolioError(w, err)
		return
	}
	writeCash(w, req.Currency, balance)
}
func (h *PortfolioHandlers) fund(w http.ResponseWriter, r *http.Request) { h.moveCash(w, r, "FUND") }
func (h *PortfolioHandlers) withdraw(w http.ResponseWriter, r *http.Request) {
	h.moveCash(w, r, "WITHDRAW")
}
