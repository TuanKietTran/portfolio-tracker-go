package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/api"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/query"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func failFirst(next http.Handler) http.Handler {
	var mu sync.Mutex
	seen := map[string]int{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		mu.Lock()
		seen[string(body)]++
		n := seen[string(body)]
		mu.Unlock()
		if n == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TestThousandPerMinuteAndPlayback(t *testing.T) {
	repo := repository.NewInMemoryPortfolio()
	h := api.NewHandlers(command.NewRecordTradeHandler(repo), query.NewGetSummaryHandler(repo))
	router := chi.NewRouter()
	router.Use(failFirst)
	router.Mount("/v1", h.Routes())
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	id := uuid.New()
	path := "/v1/portfolios/" + id.String() + "/trades"
	calls := make([]Call, RequestsPerMinute)
	for i := range calls {
		calls[i] = Call{
			Method: http.MethodPost,
			Path:   path,
			Body: []byte(fmt.Sprintf(
				`{"idempotency_key":"t-%d","symbol":"ACME","side":"BUY","quantity":"1","price":"10"}`, i)),
		}
	}
	start := time.Now()
	first, err := Play(context.Background(), srv.Client(), srv.URL, calls, DefaultPolicy())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed >= Window {
		t.Fatalf("handled %d requests in %s, want under %s", len(calls), elapsed, Window)
	}
	t.Logf("first play %d requests in %s", len(calls), elapsed)
	for i, o := range first {
		if o.Err != nil || o.Status != http.StatusCreated || o.Attempts != 2 {
			t.Fatalf("call %d status %d attempts %d err %v body %s", i, o.Status, o.Attempts, o.Err, o.Body)
		}
	}
	assertQty(t, srv, id, decimal.NewFromInt(RequestsPerMinute))

	second, err := Play(context.Background(), srv.Client(), srv.URL, calls, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for i, o := range second {
		if o.Attempts != 1 || !IsIdempotentReplay(o.Status, o.Body) {
			t.Fatalf("playback %d status %d attempts %d body %s", i, o.Status, o.Attempts, o.Body)
		}
	}
	assertQty(t, srv, id, decimal.NewFromInt(RequestsPerMinute))
}

func TestTradeWithoutKeyAppliesEveryTime(t *testing.T) {
	repo := repository.NewInMemoryPortfolio()
	h := api.NewHandlers(command.NewRecordTradeHandler(repo), query.NewGetSummaryHandler(repo))
	router := chi.NewRouter()
	router.Mount("/v1", h.Routes())
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	id := uuid.New()
	call := Call{
		Method: http.MethodPost,
		Path:   "/v1/portfolios/" + id.String() + "/trades",
		Body:   []byte(`{"symbol":"ACME","side":"BUY","quantity":"1","price":"10"}`),
	}
	for n := 0; n < 2; n++ {
		out, err := Play(context.Background(), srv.Client(), srv.URL, []Call{call}, DefaultPolicy())
		if err != nil || out[0].Status != http.StatusCreated {
			t.Fatalf("post %d %+v %v", n, out, err)
		}
	}
	assertQty(t, srv, id, decimal.NewFromInt(2))
}

func assertQty(t *testing.T, srv *httptest.Server, id uuid.UUID, want decimal.Decimal) {
	t.Helper()
	out, err := Play(context.Background(), srv.Client(), srv.URL, []Call{{
		Method: http.MethodGet,
		Path:   "/v1/portfolios/" + id.String() + "/summary",
	}}, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != http.StatusOK {
		t.Fatalf("summary %d %s", out[0].Status, out[0].Body)
	}
	var got struct {
		Positions []struct {
			Symbol   string
			Quantity decimal.Decimal
		}
	}
	if err := json.Unmarshal(out[0].Body, &got); err != nil {
		t.Fatal(err)
	}
	var qty decimal.Decimal
	for _, p := range got.Positions {
		if p.Symbol == "ACME" {
			qty = p.Quantity
		}
	}
	if !qty.Equal(want) {
		t.Fatalf("quantity %s, want %s", qty, want)
	}
}
