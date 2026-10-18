package api

import (
	"context"
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

func TestHistoryClockHTTPUnavailableHasNoEffectAndSameKeyCanRetry(t *testing.T) {
	now := time.Unix(100, 0)
	state, _ := repository.NewInMemoryStateWithClock(repository.ClockFunc(func() time.Time { return now }))
	svc, _ := command.NewPortfolioService(state, repository.NewStaticPriceFeed(), nil, domain.FIFO)
	router := NewRouter(svc, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	id := uuid.New()
	base := "/v1/portfolios/" + id.String()
	request := func(path, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", base+path, strings.NewReader(body))
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	if w := request("/cash/fund", `{"currency":"USD","amount":"100"}`, "fund"); w.Code != 200 {
		t.Fatalf("fund=%d %s", w.Code, w.Body)
	}
	if w := request("/trades", `{"currency":"USD","symbol":"ACME","side":"BUY","quantity":"2","price":"10"}`, "buy"); w.Code != 201 {
		t.Fatalf("buy=%d %s", w.Code, w.Body)
	}
	actions := []struct {
		path, body, key string
		status          int
	}{
		{"/cash/fund", `{"currency":"USD","amount":"10"}`, "more", 200},
		{"/trades", `{"currency":"USD","symbol":"ACME","side":"BUY","quantity":"1","price":"5"}`, "more-buy", 201},
		{"/corporate-actions", `{"currency":"USD","symbol":"ACME","kind":"FEE","amount":"1"}`, "fee", 201},
	}
	now = time.Unix(90, 0)
	for _, action := range actions {
		w := request(action.path, action.body, action.key)
		var result errorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 503 || result.Error.Code != "history_clock_unavailable" || result.RequestID == "" {
			t.Fatalf("unavailable=%d %s err=%v", w.Code, w.Body, err)
		}
	}
	rows, _ := state.History(context.Background(), id, 0, 1000)
	cash, _ := state.CashBalance(context.Background(), id, "USD")
	if len(rows) != 2 || !cash.Equal(pdAPI("80")) {
		t.Fatal("clock errors partially changed state/history")
	}
	now = time.Unix(110, 0)
	for _, action := range actions {
		if w := request(action.path, action.body, action.key); w.Code != action.status {
			t.Fatalf("retry=%d %s", w.Code, w.Body)
		}
	}
	rows, _ = state.History(context.Background(), id, 0, 1000)
	if len(rows) != 5 || !rows[4].Snapshot.Cash["USD"].Equal(pdAPI("84")) || !rows[4].Snapshot.Positions["ACME"].Quantity.Equal(pdAPI("3")) {
		t.Fatal("same-key recovery/history wrong")
	}
}
