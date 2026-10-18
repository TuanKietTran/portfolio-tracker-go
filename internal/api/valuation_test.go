package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
)

func TestHistoricalNAVHTTPExactCutoffClosedHoldingsAndNoFutureLeakage(t *testing.T) {
	now := time.Unix(100, 750)
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
	svc, _ := command.NewPortfolioService(state, repository.NewStaticPriceFeed(), nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	id := uuid.NewString()
	base := "/v1/portfolios/" + id
	request := func(method, path, body, key string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("%s %s got=%d %s want%d", method, path, w.Code, w.Body, status)
		}
		return w
	}
	request("POST", base+"/cash/fund", `{"currency":"USD","amount":"100"}`, "fund", 200)
	now = time.Unix(110, 750)
	request("POST", base+"/trades", `{"currency":"USD","symbol":"ACME","side":"BUY","quantity":"2","price":"10"}`, "buy", 201)
	request("POST", "/v1/prices", `{"symbol":"ACME","currency":"USD","amount":"15","as_of":110}`, "mark", 201)
	request("POST", "/v1/prices", `{"symbol":"ACME","currency":"USD","amount":"25","as_of":200}`, "future-mark", 201)
	now = time.Unix(160, 750)
	request("POST", base+"/trades", `{"currency":"USD","symbol":"ACME","side":"SELL","quantity":"2","price":"20"}`, "sell", 201)
	for _, tc := range []struct {
		at, age, cash, securities, nav string
		positions                      int
	}{{"101", "0", "100", "0", "100", 0}, {"110", "0", "100", "0", "100", 0}, {"111", "1", "80", "30", "110", 1}, {"150", "40", "80", "30", "110", 1}, {"161", "0", "120", "0", "120", 1}} {
		w := request("GET", base+"/valuation?currency=USD&as_of="+tc.at+"&max_age="+tc.age, "", "", 200)
		var value command.HistoricalValuation
		if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil || !value.Cash.Equal(pdAPI(tc.cash)) || !value.SecuritiesValue.Equal(pdAPI(tc.securities)) || !value.NetAssetValue.Equal(pdAPI(tc.nav)) || len(value.Positions) != tc.positions {
			t.Fatalf("value=%s err=%v", w.Body, err)
		}
		if tc.at == "161" && (!value.RealizedPnL.Equal(pdAPI("20")) || value.Positions[0].PriceAsOf != nil) {
			t.Fatalf("closed holdings=%s", w.Body)
		}
		if tc.at == "150" && (value.AsOf.Format(time.RFC3339Nano) != "1970-01-01T00:02:30Z" || value.Positions[0].PriceAsOf == nil || *value.Positions[0].PriceAsOf != 110) {
			t.Fatalf("cutoff/mark=%s", w.Body)
		}
	}
	for _, tc := range []struct {
		path   string
		status int
	}{
		{base + "/valuation", 400}, {base + "/valuation?as_of=150&max_age=40", 400},
		{base + "/valuation?currency=USD&as_of=150&max_age=39", 400},
		{base + "/valuation?currency=USD&as_of=100&max_age=0", 404},
		{base + "/valuation?currency=USD&as_of=9223372036854775807&max_age=0", 400},
		{base + "/valuation?currency=USD&as_of=150&max_age=bad", 400},
		{"/v1/portfolios/" + uuid.NewString() + "/valuation?currency=USD&as_of=150&max_age=0", 404},
		{"/v1/portfolios/00000000-0000-0000-0000-000000000000/valuation?currency=USD&as_of=150&max_age=0", 400},
	} {
		request("GET", tc.path, "", "", tc.status)
	}
	// Legacy summary remains current-holdings/securities-only, not this historical NAV contract.
	w := request("GET", base+"/summary?currency=USD&as_of=150&max_age=40", "", "", 200)
	if !strings.Contains(w.Body.String(), `"TotalValue":"0"`) {
		t.Fatal("legacy summary semantics changed")
	}
	now = time.Unix(210, 750)
	request("POST", base+"/trades", `{"currency":"USD","symbol":"NO_PRICE","side":"BUY","quantity":"1","price":"0"}`, "new", 201)
	request("GET", base+"/valuation?currency=USD&as_of=211&max_age=1000", "", "", 400)
	request("POST", "/v1/prices", `{"symbol":"NO_PRICE","currency":"USD","amount":"5","as_of":220}`, "later", 201)
	request("GET", base+"/valuation?currency=USD&as_of=211&max_age=1000", "", "", 400)
	w = request("GET", base+"/valuation?currency=USD&as_of=221&max_age=1", "", "", 200)
	if !strings.Contains(w.Body.String(), `"net_asset_value":"125"`) {
		t.Fatalf("closed price should not be required: %s", w.Body)
	}
	now = time.Unix(230, 750)
	request("POST", base+"/cash/fund", `{"currency":"EUR","amount":"1"}`, "eur", 200)
	request("GET", base+"/valuation?currency=USD&as_of=231&max_age=1000", "", "", 400)
	// Later cash and instruments do not contaminate an old query.
	w = request("GET", base+"/valuation?currency=USD&as_of=150&max_age=40", "", "", 200)
	if !strings.Contains(w.Body.String(), `"net_asset_value":"110"`) {
		t.Fatalf("future state leaked: %s", w.Body)
	}
}
