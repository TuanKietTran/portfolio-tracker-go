package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
)

func TestMarketHTTPManagementAndHistoricalPortfolioValuation(t *testing.T) {
	svc, _ := command.NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	request := func(method, path, body, key string, status int) *httptest.ResponseRecorder {
		t.Helper()
		if !strings.HasPrefix(path, "/v1/") {
			path = "/v1" + path
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s want %d", method, path, w.Code, w.Body, status)
		}
		return w
	}
	request("POST", "/prices", `{"symbol":"ACME","currency":"USD","amount":"10","as_of":100}`, "p1", 404)
	instrument := request("POST", "/instruments", `{"symbol":"ACME","currency":"USD"}`, "i1", 201)
	if w := request("GET", "/instruments/ACME", "", "", 200); w.Body.String() != instrument.Body.String() {
		t.Fatal("instrument lookup differs")
	}
	first := request("POST", "/prices", `{"symbol":"ACME","currency":"USD","amount":"10","as_of":100}`, "p1", 201)
	request("POST", "/prices", `{"symbol":"ACME","currency":"USD","amount":"20","as_of":200}`, "p2", 201)
	if w := request("POST", "/prices", `{"symbol":"ACME","currency":"USD","amount":"10.000","as_of":100}`, "p1", 201); w.Body.String() != first.Body.String() {
		t.Fatal("replay not original")
	}
	request("POST", "/prices", `{"symbol":"ACME","currency":"USD","amount":"11","as_of":100}`, "p1", 409)
	request("POST", "/prices", `{"symbol":"ACME","currency":"USD","amount":"11","as_of":100}`, "other", 409)
	request("POST", "/instruments", `{"symbol":"ACME","currency":"EUR"}`, "other", 409)
	request("POST", "/instruments", `{"symbol":"OTHER","currency":"USD"}`, "i1", 409)
	request("POST", "/prices", `{"symbol":"ACME","currency":"EUR","amount":"1","as_of":100}`, "eur", 409)
	for _, tc := range []struct {
		path   string
		status int
	}{{"/instruments/MISSING", 404}, {"/prices/ACME?currency=USD&as_of=99&max_age=100", 404}, {"/prices/ACME?currency=USD&as_of=151&max_age=50", 400}, {"/prices/ACME?currency=EUR&as_of=150&max_age=50", 400}, {"/prices/ACME", 400}} {
		request("GET", tc.path, "", "", tc.status)
	}
	w := request("GET", "/prices/ACME?currency=USD&as_of=150&max_age=50", "", "", 200)
	var mark domain.Price
	if err := json.Unmarshal(w.Body.Bytes(), &mark); err != nil || mark.AsOf != 100 {
		t.Fatalf("mark=%+v err=%v", mark, err)
	}
	base := "/v1/portfolios/9113183e-08e9-455b-bfc8-a522c352bd20"
	request("POST", base+"/cash/fund", `{"currency":"USD","amount":"50"}`, "fund", 200)
	request("POST", base+"/trades", `{"symbol":"ACME","currency":"USD","side":"BUY","quantity":"2","price":"5"}`, "buy", 201)
	for _, tc := range []struct{ query, want string }{{"as_of=150&max_age=50", `"TotalValue":"20"`}, {"as_of=250&max_age=50", `"TotalValue":"40"`}} {
		w := request("GET", base+"/summary?currency=USD&"+tc.query, "", "", 200)
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("summary=%s", w.Body)
		}
	}
	// Preserve first-successful-trade registration for existing clients.
	request("POST", base+"/trades", `{"symbol":"AUTO","currency":"USD","side":"BUY","quantity":"1","price":"1"}`, "auto", 201)
	request("GET", "/instruments/AUTO", "", "", 200)
	request("POST", "/instruments", `{"symbol":"AUTO","currency":"EUR"}`, "auto-eur", 409)
	request("POST", "/instruments", `{"symbol":"bad symbol","currency":"USD"}`, "bad", 400)
	request("POST", "/instruments", `{"symbol":"ACME","currency":"USD"}`, "", 400)
	request("POST", "/prices", `{`, "bad", 400)
}
