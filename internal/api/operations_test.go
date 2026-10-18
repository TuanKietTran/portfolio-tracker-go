package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
)

func TestOperationalRequestsRecoveryReadinessAndMetrics(t *testing.T) {
	svc, _ := command.NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	var logs bytes.Buffer
	router := NewRouter(svc, slog.New(slog.NewJSONHandler(&logs, nil)), func(context.Context) error { panic("private-panic-payload") })
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-Request-ID", "7cf8e2ed-418b-4270-9f42-226604a9bf1a")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	if w := request("GET", "/healthz"); w.Code != 200 || !strings.Contains(w.Body.String(), `"cash_mode":"local"`) {
		t.Fatalf("health=%d %s", w.Code, w.Body)
	}
	w := request("GET", "/readyz")
	var body errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 500 || body.Error.Code != "internal_error" || body.RequestID != w.Header().Get("X-Request-ID") {
		t.Fatalf("recovery=%d %s err=%v", w.Code, w.Body, err)
	}
	if strings.Contains(w.Body.String(), "private-panic") || strings.Contains(logs.String(), "private-panic") {
		t.Fatal("panic payload leaked")
	}
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/absent", 404}, {"POST", "/healthz", 405}} {
		if w := request(tc.method, tc.path); w.Code != tc.status {
			t.Fatalf("%+v status=%d", tc, w.Code)
		}
	}
	w = request("GET", "/metrics")
	var metrics map[string]uint64
	if err := json.Unmarshal(w.Body.Bytes(), &metrics); err != nil || metrics["requests"] != 5 || metrics["errors"] != 3 || metrics["panics"] != 1 {
		t.Fatalf("metrics=%s err=%v", w.Body, err)
	}
	unavailable := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { return errors.New("private-health-details") })
	w = httptest.NewRecorder()
	unavailable.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "private-health") {
		t.Fatalf("readiness=%d %s", w.Code, w.Body)
	}
	uninitialized := NewRouter(nil, nil, nil)
	w = httptest.NewRecorder()
	uninitialized.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 {
		t.Fatal("nil service ready")
	}
	ready := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	w = httptest.NewRecorder()
	ready.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 200 {
		t.Fatal("initialized local service not ready")
	}
	if _, err := uuid.Parse(w.Header().Get("X-Request-ID")); err != nil {
		t.Fatal("request ID missing")
	}
}

func TestStrictBoundedBodiesAndUniformErrors(t *testing.T) {
	svc, _ := command.NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	path := "/v1/portfolios/" + uuid.NewString() + "/cash/fund"
	valid := `{"currency":"USD","amount":"10","idempotency_key":"fund"}`
	for _, tc := range []struct {
		body   string
		status int
	}{{valid + ` {}`, 400}, {`{"unknown":1}`, 400}, {`{`, 400}, {strings.Repeat(" ", int(MaxRequestBytes)) + valid, 413}, {`{"currency":"USD","amount":"1","idempotency_key":"   "}`, 400}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, strings.NewReader(tc.body))
		req.Header.Set("X-Request-ID", "invalid-client-value")
		router.ServeHTTP(w, req)
		var result errorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != tc.status || result.Error.Code == "" || result.RequestID == "" || result.RequestID == "invalid-client-value" {
			t.Fatalf("response=%d %s err=%v", w.Code, w.Body, err)
		}
	}
	// Rejected JSON did not consume key or change cash.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(valid)))
	if w.Code != 200 {
		t.Fatalf("valid=%d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, strings.NewReader(valid))
	req.Header.Set("Idempotency-Key", strings.Repeat("x", 201))
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatal("oversized key accepted")
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/portfolios/00000000-0000-0000-0000-000000000000/cash?currency=USD", nil))
	if w.Code != 400 {
		t.Fatal("nil portfolio ID accepted")
	}
}
