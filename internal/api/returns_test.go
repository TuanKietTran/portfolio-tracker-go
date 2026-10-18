package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
)

func TestReturnHTTPAnalyticRatesDiagnosticsValidationAndNoPartialFailure(t *testing.T) {
	start := int64(100)
	end := start + command.SecondsPerYear
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return time.Unix(start, 0) }))
	prices := repository.NewStaticPriceFeed()
	svc, _ := command.NewPortfolioService(state, prices, nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	id := uuid.NewString()
	base := "/v1/portfolios/" + id
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s got%d %s want%d", method, path, w.Code, w.Body, status)
		}
		return w
	}
	request("POST", base+"/cash/fund", `{"currency":"USD","amount":"100","idempotency_key":"fund"}`, 200)
	request("POST", base+"/trades", `{"currency":"USD","symbol":"ACME","side":"BUY","quantity":"10","price":"10","idempotency_key":"buy"}`, 201)
	request("POST", "/v1/prices", `{"symbol":"ACME","currency":"USD","amount":"10","as_of":100,"idempotency_key":"start"}`, 201)
	request("POST", "/v1/prices", `{"symbol":"ACME","currency":"USD","amount":"11","as_of":`+strconv.FormatInt(end, 10)+`,"idempotency_key":"end"}`, 201)
	query := "currency=USD&from=100&to=" + strconv.FormatInt(end, 10) + "&max_age=0"
	for _, method := range []string{"TWR", "MWR"} {
		w := request("GET", base+"/returns?"+query+"&method="+method, "", 200)
		var result command.PortfolioReturn
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Rate.Sub(pdAPI("0.1")).Abs().GreaterThan(pdAPI("0.00000001")) || result.ExternalFlows != 0 || !result.StartNAV.Equal(pdAPI("100")) || !result.EndNAV.Equal(pdAPI("110")) || result.Annualized != (method == "MWR") {
			t.Fatalf("result=%s err=%v", w.Body, err)
		}
		if method == "MWR" && (result.Solver == nil || result.Solver.Iterations <= 0 || result.Solver.NPVResidual.GreaterThan(result.Solver.NPVTolerance) || result.Solver.RateBracketWidth.GreaterThan(result.Solver.RateTolerance)) {
			t.Fatalf("missing numerical proof=%s", w.Body)
		}
	}
	request("GET", base+"/returns?"+query+"&method=MWR&rate_tolerance=0.000001&npv_tolerance=0.000001&max_iterations=128", "", 200)
	w := request("GET", base+"/returns?"+query+"&method=MWR&max_iterations=1", "", 400)
	if strings.Contains(w.Body.String(), `"rate":`) {
		t.Fatal("nonconvergence returned partial rate")
	}
	for _, suffix := range []string{
		"&method=OTHER", "&method=TWR&max_iterations=10", "&method=MWR&rate_tolerance=bad",
		"&method=MWR&npv_tolerance=bad", "&method=MWR&rate_tolerance=" + strings.Repeat("1", 65),
		"&method=MWR&rate_tolerance=1e999999", "&method=MWR&npv_tolerance=0",
		"&method=MWR&max_iterations=bad", "&method=MWR&max_iterations=0", "&method=MWR&max_iterations=257",
	} {
		request("GET", base+"/returns?"+query+suffix, "", 400)
	}
	for _, q := range []string{
		"currency=USD&to=" + strconv.FormatInt(end, 10) + "&max_age=0&method=TWR",
		"currency=USD&from=100&max_age=0&method=TWR",
		"currency=USD&from=100&to=" + strconv.FormatInt(end, 10) + "&method=TWR",
		"currency=USD&from=100&to=100&max_age=0&method=TWR",
		"currency=USD&from=100&to=" + strconv.FormatInt(start+11*command.SecondsPerYear, 10) + "&max_age=0&method=TWR",
		"currency=EUR&from=100&to=" + strconv.FormatInt(end, 10) + "&max_age=0&method=TWR",
		"currency=USD&from=101&to=" + strconv.FormatInt(end, 10) + "&max_age=0&method=TWR",
	} {
		request("GET", base+"/returns?"+q, "", 400)
	}
	request("GET", base+"/returns?currency=USD&from=99&to="+strconv.FormatInt(end, 10)+"&max_age=0&method=TWR", "", 404)
	request("GET", "/v1/portfolios/00000000-0000-0000-0000-000000000000/returns?"+query+"&method=TWR", "", 400)
	readonly, _ := command.NewPortfolioService(state, command.PriceFunc(func(_ context.Context, _, _ string) (domain.Price, error) { return domain.Price{}, nil }), nil, domain.FIFO)
	unavailable := NewRouter(readonly, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	w = httptest.NewRecorder()
	unavailable.ServeHTTP(w, httptest.NewRequest("GET", base+"/returns?"+query+"&method=TWR", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "returns_unavailable") {
		t.Fatalf("readonly=%d %s", w.Code, w.Body)
	}
}
