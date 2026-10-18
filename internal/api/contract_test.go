package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

func TestOpenAPIParsesAndDeclaresImplementedRoutesAndStatuses(t *testing.T) {
	b, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]struct {
			Responses map[string]any `yaml:"responses"`
		} `yaml:"paths"`
	}
	if err = yaml.Unmarshal(b, &spec); err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}
	want := map[string]map[string][]string{
		"/v1/portfolios/{id}/trades":                    {"post": {"201", "400"}},
		"/v1/portfolios/{id}/summary":                   {"get": {"200", "400", "500"}},
		"/v1/portfolios/{id}/positions/{symbol}":        {"get": {"200", "400"}},
		"/v1/portfolios/{id}/positions/{symbol}/trades": {"post": {"201", "400", "409"}},
		"/v1/portfolios/{id}/cash":                      {"get": {"200", "400"}},
		"/v1/portfolios/{id}/cash/fund":                 {"post": {"200", "400", "409"}},
		"/v1/portfolios/{id}/cash/withdraw":             {"post": {"200", "400", "409"}},
	}
	if len(spec.Paths) != len(want) {
		t.Fatalf("OpenAPI paths=%d want=%d", len(spec.Paths), len(want))
	}
	for path, methods := range want {
		for method, statuses := range methods {
			operation, ok := spec.Paths[path][method]
			if !ok {
				t.Errorf("missing %s %s", method, path)
				continue
			}
			for _, status := range statuses {
				if _, ok := operation.Responses[status]; !ok {
					t.Errorf("%s %s missing %s", method, path, status)
				}
			}
		}
	}
}

func TestPortfolioCashAndValuationHTTPStatuses(t *testing.T) {
	state := repository.NewInMemoryState()
	prices := repository.NewStaticPriceFeed()
	prices.Set(domain.Price{Symbol: "ACME", Currency: "USD", Amount: decimal.NewFromInt(12), AsOf: 100})
	service, err := command.NewPortfolioService(state, prices, nil, domain.FIFO)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Mount("/v1", NewPortfolioHandlers(service).Routes())
	id := uuid.NewString()
	request := func(method, path, body string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	if got := request(http.MethodPost, "/v1/portfolios/"+id+"/cash/fund", `{"currency":"USD","amount":"100","idempotency_key":"f1"}`); got != 200 {
		t.Fatalf("fund status=%d", got)
	}
	if got := request(http.MethodPost, "/v1/portfolios/"+id+"/cash/withdraw", `{"currency":"USD","amount":"101","idempotency_key":"w1"}`); got != 409 {
		t.Fatalf("insufficient withdrawal status=%d", got)
	}
	if got := request(http.MethodPost, "/v1/portfolios/"+id+"/cash/withdraw", `{"currency":"USD","amount":"25","idempotency_key":"w2"}`); got != 200 {
		t.Fatalf("withdraw status=%d", got)
	}
	if got := request(http.MethodGet, "/v1/portfolios/"+id+"/cash?currency=USD", ""); got != 200 {
		t.Fatalf("cash status=%d", got)
	}
	if got := request(http.MethodPost, "/v1/portfolios/"+id+"/positions/ACME/trades", `{"side":"BUY","quantity":"2","price":"10","idempotency_key":"trade-1"}`); got != 201 {
		t.Fatalf("trade status=%d", got)
	}
	if got := request(http.MethodGet, "/v1/portfolios/"+id+"/positions/ACME?currency=USD&as_of=100&max_age=5", ""); got != 200 {
		t.Fatalf("position status=%d", got)
	}
	if got := request(http.MethodGet, "/v1/portfolios/nope/cash?currency=USD", ""); got != 400 {
		t.Fatalf("invalid id status=%d", got)
	}
	if got := request(http.MethodPost, "/v1/portfolios/"+id+"/cash/fund", `{`); got != 400 {
		t.Fatalf("invalid json status=%d", got)
	}
}
