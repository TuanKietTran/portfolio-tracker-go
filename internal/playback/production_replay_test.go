//go:build !race

package playback

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/api"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type disabledLogHandler struct{}

func (disabledLogHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (disabledLogHandler) Handle(context.Context, slog.Record) error { return nil }
func (h disabledLogHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h disabledLogHandler) WithGroup(string) slog.Handler           { return h }

func TestProductionRouterTenThousandRequestsWithFaultsAndDuplicates(t *testing.T) {
	state := repository.NewInMemoryState()
	service, err := command.NewPortfolioService(state, repository.NewStaticPriceFeed(), nil, domain.FIFO)
	if err != nil {
		t.Fatal(err)
	}
	portfolioID := uuid.New()
	if _, err := state.MoveCash(context.Background(), portfolioID, "USD", "FUND", decimal.NewFromInt(10000), "initial-fund"); err != nil {
		t.Fatal(err)
	}
	router := api.NewRouter(service, slog.New(disabledLogHandler{}), nil)
	var mu sync.Mutex
	attempts := map[string]int{}
	var injected503, injected429, injectedTimeout atomic.Int64
	faults := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		mu.Lock()
		attempts[key]++
		n := attempts[key]
		mu.Unlock()
		if n == 1 {
			index, _ := strconv.Atoi(strings.TrimPrefix(key, "trade-"))
			switch {
			case index < 500:
				injected503.Add(1)
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			case index < 600:
				injected429.Add(1)
				http.Error(w, "temporary", http.StatusTooManyRequests)
				return
			case index < 700:
				injectedTimeout.Add(1)
				// Deliberately exceed the client's per-attempt deadline without
				// invoking the business handler; the still-live window retries.
				timer := time.NewTimer(1200 * time.Millisecond)
				defer timer.Stop()
				select {
				case <-r.Context().Done():
					return
				case <-timer.C:
				}
				return
			}
		}
		router.ServeHTTP(w, r)
	})
	// httptest.NewServer binds a real loopback listener, not an in-process function call.
	srv := httptest.NewServer(faults)
	t.Cleanup(srv.Close)

	const unique = 9800
	calls := make([]Call, 0, RequestsPerMinute)
	path := "/v1/portfolios/" + portfolioID.String() + "/positions/ACME/trades"
	for i := 0; i < unique; i++ {
		key := fmt.Sprintf("trade-%05d", i)
		calls = append(calls, Call{Method: http.MethodPost, Path: path, Header: map[string]string{"Idempotency-Key": key, "X-Request-ID": "b48ea791-2c18-4f21-8bdc-e354f7b8c571"}, Body: []byte(`{"currency":"USD","side":"BUY","quantity":"1","price":"1"}`)})
	}
	for i := 0; i < 200; i++ {
		calls = append(calls, calls[i])
	}
	windowCtx, cancel := context.WithTimeout(context.Background(), Window)
	defer cancel()
	client := srv.Client()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 128
	transport.MaxIdleConnsPerHost = 64
	client.Transport = transport
	t.Cleanup(transport.CloseIdleConnections)
	start := time.Now()
	policy := DefaultPolicy()
	policy.AttemptTimeout = time.Second
	policy.Concurrency = 64
	outcomes, err := Play(windowCtx, client, srv.URL, calls, policy)
	elapsed := time.Since(start)
	if err != nil {
		completed := 0
		for _, outcome := range outcomes {
			if outcome.Status != 0 {
				completed++
			}
		}
		t.Fatalf("play failed after %s (%d/%d outcomes have a response): %v", elapsed, completed, len(outcomes), err)
	}
	if elapsed >= Window {
		t.Fatalf("10,000 requests took %s, window %s", elapsed, Window)
	}
	for i, outcome := range outcomes {
		if outcome.Status != http.StatusCreated || outcome.Err != nil || outcome.Attempts < 1 || outcome.Attempts > 6 {
			t.Fatalf("request %d terminal outcome: status=%d attempts=%d err=%v", i, outcome.Status, outcome.Attempts, outcome.Err)
		}
	}
	if injected503.Load() < 500 || injected429.Load() < 100 || injectedTimeout.Load() < 100 {
		t.Fatalf("fault injection counts 503=%d 429=%d timeout=%d", injected503.Load(), injected429.Load(), injectedTimeout.Load())
	}
	position, err := state.GetAccountPosition(context.Background(), portfolioID, "ACME")
	if err != nil {
		t.Fatal(err)
	}
	cash, err := state.CashBalance(context.Background(), portfolioID, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if !position.Quantity.Equal(decimal.NewFromInt(unique)) || !position.CostBasis.Equal(decimal.NewFromInt(unique)) {
		t.Fatalf("trade effects quantity=%s basis=%s, want %d each", position.Quantity, position.CostBasis, unique)
	}
	if !cash.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("cash=%s, want 200", cash)
	}
	t.Logf("completed %d requests (%d unique, 200 duplicate submissions) in %s; injected 503=%d 429=%d timeouts=%d; state quantity=%s cash=%s", len(outcomes), unique, elapsed, injected503.Load(), injected429.Load(), injectedTimeout.Load(), position.Quantity, cash)
}
