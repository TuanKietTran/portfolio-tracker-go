package api

import (
	"context"
	"github.com/shopspring/decimal"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestAccountingTradeHTTPReplayConflictAndRequiredKey(t *testing.T) {
	state := repository.NewInMemoryState()
	id := uuid.New()
	if _, err := state.MoveCash(context.Background(), id, "USD", "FUND", decimal.NewFromInt(100), "fund"); err != nil {
		t.Fatal(err)
	}
	svc, err := command.NewPortfolioService(state, repository.NewStaticPriceFeed(), nil, domain.FIFO)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Mount("/v1", NewPortfolioHandlers(svc).Routes())
	path := "/v1/portfolios/" + id.String() + "/positions/ACME/trades"
	send := func(body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	body := `{"currency":"USD","side":"BUY","quantity":"2","price":"10"}`
	if w := send(body, ""); w.Code != 400 {
		t.Fatalf("missing key=%d", w.Code)
	}
	first := send(body, "trade-key")
	if first.Code != 201 {
		t.Fatalf("first=%d %s", first.Code, first.Body)
	}
	if w := send(`{"currency":"USD","side":"SELL","quantity":"1","price":"15","idempotency_key":"sell"}`, ""); w.Code != 201 {
		t.Fatalf("sell=%d %s", w.Code, w.Body)
	}
	replay := send(body, "trade-key")
	if replay.Code != 201 || replay.Body.String() != first.Body.String() {
		t.Fatalf("replay=%d %s want %s", replay.Code, replay.Body, first.Body)
	}
	if w := send(`{"currency":"USD","side":"BUY","quantity":"3","price":"10"}`, "trade-key"); w.Code != 409 {
		t.Fatalf("conflict=%d", w.Code)
	}
	if w := send(`{"currency":"USD","side":"BUY","quantity":"2","price":"10","idempotency_key":"different"}`, "trade-key"); w.Code != 400 {
		t.Fatalf("disagreement=%d", w.Code)
	}
	if w := send(`{"currency":"USD","side":"BUY","quantity":"2","price":"10","idempotency_key":"trade-key"}`, "trade-key"); w.Code != 201 {
		t.Fatalf("matching keys=%d", w.Code)
	}
}
