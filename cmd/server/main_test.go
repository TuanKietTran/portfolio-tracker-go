package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Exercise the actual server router: mounting two independent chi routers at /v1
// used to panic before ListenAndServe, which handler-only tests did not detect.
func TestRuntimeRouterSharesTradePositionAndMarkedSummary(t *testing.T) {
	state := repository.NewInMemoryState()
	feed := repository.NewStaticPriceFeed()
	feed.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: decimal.NewFromInt(15), AsOf: 100})
	svc, err := command.NewPortfolioService(state, feed, nil, domain.FIFO)
	if err != nil {
		t.Fatal(err)
	}
	router := newRouter(svc)
	base := "/v1/portfolios/" + uuid.NewString()
	send := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, base+path, strings.NewReader(body)))
		return w
	}
	if w := send("POST", "/cash/fund", `{"currency":"USD","amount":"100","idempotency_key":"seed"}`); w.Code != 200 {
		t.Fatalf("fund=%d %s", w.Code, w.Body)
	}
	buy := `{"currency":"USD","symbol":"ACME","side":"BUY","quantity":"2","price":"10","idempotency_key":"buy"}`
	for i := 0; i < 2; i++ {
		w := send("POST", "/trades", buy)
		if w.Code != 201 || w.Body.Len() != 0 {
			t.Fatalf("buy=%d %s", w.Code, w.Body)
		}
	}
	w := send("GET", "/positions/ACME?currency=USD&as_of=100&max_age=0", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"quantity":"2"`) {
		t.Fatalf("position=%d %s", w.Code, w.Body)
	}
	w = send("GET", "/summary?currency=USD&as_of=100&max_age=0", "")
	var summary command.PortfolioSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil || w.Code != 200 || !summary.TotalValue.Equal(decimal.NewFromInt(30)) || !summary.UnrealizedPnL.Equal(decimal.NewFromInt(10)) || len(summary.Positions) != 1 {
		t.Fatalf("summary=%d %s err=%v", w.Code, w.Body, err)
	}
	sell := `{"currency":"USD","side":"SELL","quantity":"1","price":"16","idempotency_key":"sell"}`
	if w = send("POST", "/positions/ACME/trades", sell); w.Code != 201 {
		t.Fatalf("sell=%d %s", w.Code, w.Body)
	}
	w = send("GET", "/summary?currency=USD&as_of=100&max_age=0", "")
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil || !summary.TotalValue.Equal(decimal.NewFromInt(15)) || !summary.Positions[0].RealizedPnL.Equal(decimal.NewFromInt(6)) {
		t.Fatalf("summary=%s err=%v", w.Body, err)
	}
	for _, path := range []string{"/summary", "/summary?currency=USD&as_of=abc&max_age=0", "/summary?currency=USD&as_of=100&max_age=-1", "/summary?currency=USD&as_of=101&max_age=0", "/positions/ACME?currency=USD&as_of=100&max_age=oops"} {
		if w = send("GET", path, ""); w.Code != 400 {
			t.Fatalf("path=%s status=%d", path, w.Code)
		}
	}
	for _, body := range []string{`{"currency":"USD","symbol":"ACME","side":"BUY","quantity":"1","price":"10"}`, `{"currency":"USD","symbol":"ACME","side":"OTHER","quantity":"1","price":"10","idempotency_key":"bad"}`, `{"currency":"USD","symbol":"ACME","side":"SELL","quantity":"2","price":"10","idempotency_key":"oversell"}`} {
		if w = send("POST", "/trades", body); w.Code != 400 {
			t.Fatalf("invalid=%d %s", w.Code, w.Body)
		}
	}
}
