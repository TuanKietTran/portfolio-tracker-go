package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func pdAPI(value string) decimal.Decimal { return decimal.RequireFromString(value) }

func TestCorporateHTTPConcurrentReplayCashAndWorkedValuation(t *testing.T) {
	prices := repository.NewStaticPriceFeed()
	svc, _ := command.NewPortfolioService(repository.NewInMemoryState(), prices, nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	base := "/v1/portfolios/" + uuid.NewString()
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		path, body, key string
		status          int
	}{
		{base + "/cash/fund", `{"currency":"USD","amount":"100"}`, "fund", 200},
		{base + "/trades", `{"symbol":"ACME","currency":"USD","side":"BUY","quantity":"2","price":"10"}`, "buy", 201},
		{"/v1/prices", `{"symbol":"ACME","currency":"USD","amount":"5","as_of":200}`, "mark", 201},
	} {
		if w := request("POST", tc.path, tc.body, tc.key); w.Code != tc.status {
			t.Fatalf("setup=%d %s", w.Code, w.Body)
		}
	}
	path := base + "/corporate-actions"
	split := `{"symbol":"ACME","kind":"SPLIT","currency":"USD","numerator":2,"denominator":1}`
	var wg sync.WaitGroup
	responses := make(chan string, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request("POST", path, split, "split")
			if w.Code != 201 {
				t.Errorf("split=%d %s", w.Code, w.Body)
			}
			responses <- w.Body.String()
		}()
	}
	wg.Wait()
	close(responses)
	var original string
	for body := range responses {
		if original == "" {
			original = body
		}
		if body != original {
			t.Fatal("concurrent duplicate responses differ")
		}
	}
	for _, tc := range []struct{ body, key string }{
		{`{"symbol":"ACME","kind":"DIVIDEND","currency":"USD","amount":"4"}`, "dividend"},
		{`{"symbol":"ACME","kind":"FEE","currency":"USD","amount":"1","idempotency_key":"fee"}`, ""},
	} {
		if w := request("POST", path, tc.body, tc.key); w.Code != 201 {
			t.Fatalf("action=%d %s", w.Code, w.Body)
		}
	}
	if w := request("POST", path, split, "split"); w.Code != 201 || w.Body.String() != original {
		t.Fatalf("replay=%d %s", w.Code, w.Body)
	}
	cash := request("GET", base+"/cash?currency=USD", "", "")
	if cash.Code != 200 || !strings.Contains(cash.Body.String(), `"balance":"83"`) {
		t.Fatalf("cash=%d %s", cash.Code, cash.Body)
	}
	w := request("GET", base+"/summary?currency=USD&as_of=200&max_age=0", "", "")
	var summary command.PortfolioSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil || w.Code != 200 || len(summary.Positions) != 1 || !summary.TotalValue.Equal(pdAPI("20")) || !summary.Positions[0].Quantity.Equal(pdAPI("4")) || !summary.Positions[0].RealizedPnL.Equal(pdAPI("3")) || !summary.UnrealizedPnL.IsZero() {
		t.Fatalf("summary=%d %s err=%v", w.Code, w.Body, err)
	}
	for _, tc := range []struct {
		body, key string
		status    int
	}{
		{`{"symbol":"ACME","kind":"SPLIT","currency":"USD","numerator":3,"denominator":1}`, "split", 409},
		{`{"symbol":"ACME","kind":"FEE","currency":"USD","amount":"100"}`, "insufficient", 409},
		{`{"symbol":"ACME","kind":"FEE","currency":"EUR","amount":"1"}`, "eur", 409},
		{`{"symbol":"MISSING","kind":"FEE","currency":"USD","amount":"1"}`, "missing", 404},
		{`{"symbol":"ACME","kind":"SPLIT","currency":"USD","numerator":1,"denominator":3}`, "inexact", 400},
		{`{"symbol":"ACME","kind":"FEE","currency":"USD","amount":"0.001"}`, "subcent", 400},
		{`{"symbol":"ACME","kind":"FEE","currency":"USD","amount":"1","numerator":1}`, "extra-ratio", 400},
		{`{"symbol":"ACME","kind":"BAD","currency":"USD"}`, "unknown", 400},
		{`{"symbol":"ACME","kind":"SPLIT","currency":"USD","numerator":9223372036854775808,"denominator":1}`, "overflow", 400},
		{`{`, "invalid", 400}, {split, "", 400},
	} {
		if w := request("POST", path, tc.body, tc.key); w.Code != tc.status {
			t.Fatalf("%+v got=%d %s", tc, w.Code, w.Body)
		}
	}
	if w := request("POST", "/v1/portfolios/00000000-0000-0000-0000-000000000000/corporate-actions", split, "nil"); w.Code != 400 {
		t.Fatal("nil portfolio accepted")
	}
}
