package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/command"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/payments"
	"github.com/TuanKietTran/portfolio-tracker-go/internal/repository"
	"github.com/google/uuid"
)

func TestPaymentsCashHTTPReplayRecoversAmbiguousSettlement(t *testing.T) {
	var mu sync.Mutex
	intents := map[string]uuid.UUID{}
	settled := map[uuid.UUID]bool{}
	ledgerIDs := map[uuid.UUID]string{}
	createCalls, processCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payments":
			var body struct {
				IdempotencyKey string `json:"idempotency_key"`
				SourceAccount  string `json:"source_account"`
				DestAccount    string `json:"dest_account"`
				Amount         string `json:"amount"`
				Currency       string `json:"currency"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			createCalls++
			id, exists := intents[body.IdempotencyKey]
			if !exists {
				id = uuid.New()
				intents[body.IdempotencyKey] = id
			}
			if body.SourceAccount != "synthetic-source" || body.DestAccount != "synthetic-destination" || body.Amount != "15" || body.Currency != "USD" {
				t.Errorf("unexpected payment create body: %+v", body)
			}
			if createCalls == 1 {
				http.Error(w, "ambiguous create response", http.StatusBadGateway)
				return
			}
			_, _ = fmt.Fprintf(w, `{"payment_id":%q}`, id.String())
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/process"):
			id, err := uuid.Parse(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/payments/"), "/process"))
			if err != nil {
				http.Error(w, "bad id", http.StatusBadRequest)
				return
			}
			processCalls++
			settled[id] = true
			ledgerIDs[id] = uuid.NewString()
			http.Error(w, "committed but response lost", http.StatusBadGateway)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/payments/"):
			id, err := uuid.Parse(strings.TrimPrefix(r.URL.Path, "/v1/payments/"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			status, ledger := "PENDING", ""
			if settled[id] {
				status, ledger = "SETTLED", ledgerIDs[id]
			}
			_, _ = fmt.Fprintf(w, `{"ID":%q,"Status":%q,"LedgerTxID":%q}`, id.String(), status, ledger)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/reconciliation":
			_, _ = fmt.Fprint(w, `{"checked":1,"matched":1,"discrepancies":0,"items":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	paymentClient, err := payments.NewClient(server.URL, "synthetic-source", "synthetic-destination", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	svc, err := command.NewPortfolioService(repository.NewInMemoryState(), repository.NewStaticPriceFeed(), paymentClient, domain.FIFO)
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(svc, nil, nil)
	portfolio := uuid.NewString()
	fund := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/portfolios/"+portfolio+"/cash/fund", strings.NewReader(`{"currency":"USD","amount":"15"}`))
		req.Header.Set("Idempotency-Key", "fund-once")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	first := fund()
	if first.Code != http.StatusOK {
		t.Fatalf("fund status=%d body=%s", first.Code, first.Body)
	}
	replay := fund()
	if replay.Code != http.StatusOK || replay.Body.String() != first.Body.String() {
		t.Fatalf("fund replay status=%d body=%s want %s", replay.Code, replay.Body, first.Body)
	}
	if createCalls != 3 || processCalls != 1 {
		t.Fatalf("payment create calls=%d process calls=%d", createCalls, processCalls)
	}
	cashReq := httptest.NewRequest(http.MethodGet, "/v1/portfolios/"+portfolio+"/cash?currency=USD", nil)
	cashResp := httptest.NewRecorder()
	router.ServeHTTP(cashResp, cashReq)
	if cashResp.Code != http.StatusOK || !strings.Contains(cashResp.Body.String(), `"balance":"15"`) {
		t.Fatalf("local cash=%d %s", cashResp.Code, cashResp.Body)
	}
}
