package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
)

func TestPortfolioLifecycleHTTPConcurrentReplayAndSelectedMethod(t *testing.T) {
	id := uuid.MustParse("9113183e-08e9-455b-bfc8-a522c352bd20")
	now := time.Unix(100, 750).UTC()
	s, _ := repository.NewInMemoryStateWithSources(repository.ClockFunc(func() time.Time { return now }), func() (uuid.UUID, error) { return id, nil })
	svc, _ := command.NewPortfolioService(s, repository.NewStaticPriceFeed(), nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	request := func(method, path, body, key string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Errorf("%s %s got%d %s want%d", method, path, w.Code, w.Body, status)
		}
		return w
	}
	body := `{"currency":"USD","cost_method":"AVERAGE_COST"}`
	first := request("POST", "/v1/portfolios", body, "create", 201)
	var p repository.Portfolio
	if e := json.Unmarshal(first.Body.Bytes(), &p); e != nil || p.ID != id || p.CreatedAt != now || p.CostMethod != domain.AverageCost {
		t.Fatal(p, e)
	}
	base := "/v1/portfolios/" + id.String()
	if first.Header().Get("Location") != base {
		t.Fatal(first.Header())
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request("POST", "/v1/portfolios", body, "create", 201)
			if w.Body.String() != first.Body.String() {
				t.Error("different replay")
			}
		}()
	}
	wg.Wait()
	if w := request("GET", base, "", "", 200); w.Body.String() != first.Body.String() {
		t.Fatal("lookup differs")
	}
	request("POST", "/v1/portfolios", `{"currency":"EUR","cost_method":"AVERAGE_COST"}`, "create", 409)
	request("POST", "/v1/portfolios", `{"currency":"USD","cost_method":"FIFO"}`, "create", 409)
	request("POST", base+"/cash/fund", `{"currency":"EUR","amount":"100"}`, "fund", 409)
	request("GET", base+"/cash?currency=EUR", "", "", 409)
	request("POST", base+"/cash/fund", `{"currency":"USD","amount":"100"}`, "fund", 200)
	for i, price := range []string{"10", "20"} {
		request("POST", base+"/positions/ACME/trades", fmt.Sprintf(`{"currency":"USD","side":"BUY","quantity":"1","price":"%s"}`, price), fmt.Sprintf("buy%d", i), 201)
	}
	sold := request("POST", base+"/positions/ACME/trades", `{"currency":"USD","side":"SELL","quantity":"1","price":"30"}`, "sell", 201)
	var position domain.AccountPosition
	if e := json.Unmarshal(sold.Body.Bytes(), &position); e != nil || position.Method != domain.AverageCost || position.RealizedPnL.String() != "15" || position.CostBasis.String() != "15" {
		t.Fatal(position, e)
	}
	request("POST", base+"/trades", `{"symbol":"OTHER","currency":"EUR","side":"BUY","quantity":"1","price":"1"}`, "fx", 409)
	request("POST", base+"/corporate-actions", `{"symbol":"ACME","currency":"EUR","kind":"FEE","amount":"1"}`, "fee", 409)
	if w := request("POST", "/v1/portfolios", body, "create", 201); w.Body.String() != first.Body.String() {
		t.Fatal("metadata changed after trades")
	}
	// Legacy state remains operable but is not silently registered or adopted.
	legacy := "/v1/portfolios/" + uuid.NewString()
	request("POST", legacy+"/cash/fund", `{"currency":"EUR","amount":"5"}`, "legacy", 200)
	request("GET", legacy, "", "", 404)
	request("GET", legacy+"/cash?currency=EUR", "", "", 200)
	request("GET", "/v1/portfolios/nope", "", "", 400)
	request("GET", "/v1/portfolios/"+uuid.Nil.String(), "", "", 400)
	for _, invalid := range []string{`{`, `{"currency":"USD"}`, `{"currency":"usd","cost_method":"FIFO"}`, `{"currency":"USD","cost_method":"BAD"}`, `{"currency":"USD","cost_method":"FIFO","extra":true}`, body + ` {}`} {
		request("POST", "/v1/portfolios", invalid, "invalid", 400)
	}
	request("POST", "/v1/portfolios", body, "", 400)
	request("POST", "/v1/portfolios", `{"currency":"USD","cost_method":"FIFO","idempotency_key":"body"}`, "header", 400)
	request("POST", "/v1/portfolios", `{"currency":"USD","cost_method":"FIFO","idempotency_key":"`+strings.Repeat("x", 17000)+`"}`, "", 413)
}

func TestPortfolioLifecycleHTTPUnavailableSourcesAndRecovery(t *testing.T) {
	now := time.Time{}
	id := uuid.Nil
	s, _ := repository.NewInMemoryStateWithSources(repository.ClockFunc(func() time.Time { return now }), func() (uuid.UUID, error) { return id, nil })
	svc, _ := command.NewPortfolioService(s, repository.NewStaticPriceFeed(), nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	post := func(status int, code string) {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/v1/portfolios", strings.NewReader(`{"currency":"USD","cost_method":"FIFO","idempotency_key":"body-key"}`))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status || !strings.Contains(w.Body.String(), code) {
			t.Fatal(w.Code, w.Body)
		}
	}
	post(503, "portfolio_identity_unavailable")
	id = uuid.New()
	post(503, "history_clock_unavailable")
	now = time.Unix(100, 0)
	post(201, id.String())
	now = time.Time{}
	id = uuid.Nil
	post(201, "USD")
}
