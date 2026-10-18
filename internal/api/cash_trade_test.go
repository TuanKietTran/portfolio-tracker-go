package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestCashLinkedTradeHTTPAndHeaderReplay(t *testing.T) {
	svc, _ := command.NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	router := chi.NewRouter()
	router.Mount("/v1", NewPortfolioHandlers(svc).Routes())
	base := "/v1/portfolios/" + uuid.NewString()
	send := func(method, path, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, base+path, strings.NewReader(body))
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	buy := `{"symbol":"ACME","side":"BUY","currency":"USD","quantity":"2","price":"10"}`
	if w := send("POST", "/trades", buy, "buy"); w.Code != 409 {
		t.Fatalf("unfunded buy=%d %s", w.Code, w.Body)
	}
	fund := `{"currency":"USD","amount":"20"}`
	funded := send("POST", "/cash/fund", fund, "fund")
	if funded.Code != 200 {
		t.Fatalf("fund=%d %s", funded.Code, funded.Body)
	}
	if w := send("POST", "/trades", buy, "buy"); w.Code != 201 {
		t.Fatalf("buy=%d %s", w.Code, w.Body)
	}
	if w := send("GET", "/cash?currency=USD", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"balance":"0"`) {
		t.Fatalf("cash=%d %s", w.Code, w.Body)
	}
	replay := send("POST", "/cash/fund", fund, "fund")
	if replay.Code != 200 || replay.Body.String() != funded.Body.String() {
		t.Fatalf("fund replay=%d %s", replay.Code, replay.Body)
	}
	if w := send("GET", "/cash?currency=USD", "", ""); !strings.Contains(w.Body.String(), `"balance":"0"`) {
		t.Fatal("fund replay changed balance")
	}
	for _, tc := range []struct {
		path, body, key string
		status          int
	}{
		{"/trades", `{"symbol":"ACME","side":"BUY","quantity":"1","price":"1"}`, "missing-currency", 400},
		{"/cash/fund", `{"currency":"usd","amount":"1"}`, "lower", 400},
		{"/cash/fund", `{"currency":"USD","amount":"0.001"}`, "subcent", 400},
		{"/cash/fund", `{"currency":"USD","amount":"1","idempotency_key":"different"}`, "header", 400},
		{"/cash/fund", `{"currency":"USD","amount":"21"}`, "fund", 409},
		{"/cash/fund", fund, "", 400},
	} {
		if w := send("POST", tc.path, tc.body, tc.key); w.Code != tc.status {
			t.Fatalf("%+v got=%d %s", tc, w.Code, w.Body)
		}
	}
}
