package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestNewClientRequiresLoopbackAndDistinctAccounts(t *testing.T) {
	for _, tc := range []struct{ url, source, destination string }{
		{"https://example.test", "a", "b"},
		{"http://127.0.0.1", "same", "same"},
		{"http://user:pass@127.0.0.1", "a", "b"},
	} {
		if _, err := NewClient(tc.url, tc.source, tc.destination, nil); err == nil {
			t.Fatalf("accepted URL/accounts %q %q %q", tc.url, tc.source, tc.destination)
		}
	}
}

func TestMoveReplaysStableIntentAfterAmbiguousCreateAndProcess(t *testing.T) {
	paymentID := uuid.New()
	var mu sync.Mutex
	var createBodies []createPaymentRequest
	createCalls, processCalls, reconcileCalls := 0, 0, 0
	settled := false
	ledgerID := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payments":
			createCalls++
			var request createPaymentRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			createBodies = append(createBodies, request)
			if createCalls == 1 {
				// The service committed the intent but its response was lost.
				http.Error(w, "ambiguous", http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"payment_id":%q}`, paymentID.String())
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payments/"+paymentID.String()+"/process":
			processCalls++
			settled = true
			http.Error(w, "response lost after settlement", http.StatusBadGateway)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/payments/"+paymentID.String():
			w.Header().Set("Content-Type", "application/json")
			status, ledger := "PENDING", ""
			if settled {
				status, ledger = "SETTLED", ledgerID
			}
			_, _ = fmt.Fprintf(w, `{"ID":%q,"Status":%q,"LedgerTxID":%q}`, paymentID.String(), status, ledger)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/reconciliation":
			reconcileCalls++
			_, _ = fmt.Fprint(w, `{"checked":1,"matched":1,"discrepancies":0,"items":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "synthetic-source", "synthetic-destination", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.baseDelay = time.Millisecond
	client.maxDelay = time.Millisecond
	portfolio := uuid.New()
	for i := 0; i < 2; i++ {
		if err := client.Move(context.Background(), portfolio, "USD", "FUND", decimal.RequireFromString("12.34"), "funding-key"); err != nil {
			t.Fatalf("move replay %d: %v", i, err)
		}
	}
	if createCalls != 3 || processCalls != 1 || reconcileCalls != 2 {
		t.Fatalf("create=%d process=%d reconcile=%d", createCalls, processCalls, reconcileCalls)
	}
	if len(createBodies) != 3 || createBodies[0].IdempotencyKey == "" || createBodies[0].IdempotencyKey != createBodies[1].IdempotencyKey || createBodies[1].IdempotencyKey != createBodies[2].IdempotencyKey {
		t.Fatalf("unstable create intent keys: %+v", createBodies)
	}
	for _, request := range createBodies {
		if request.Amount != "12.34" || request.Currency != "USD" || request.SourceAccount != "synthetic-source" || request.DestAccount != "synthetic-destination" {
			t.Fatalf("unexpected payment request: %+v", request)
		}
	}
}

func TestMoveRejectsUnreconciledOrUnsettledPayment(t *testing.T) {
	paymentID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payments":
			_, _ = fmt.Fprintf(w, `{"payment_id":%q}`, paymentID.String())
		case r.Method == http.MethodGet && r.URL.Path == "/v1/payments/"+paymentID.String():
			_, _ = fmt.Fprintf(w, `{"ID":%q,"Status":"SETTLED","LedgerTxID":%q}`, paymentID.String(), uuid.NewString())
		case r.Method == http.MethodGet && r.URL.Path == "/v1/reconciliation":
			_, _ = fmt.Fprint(w, `{"checked":1,"matched":0,"discrepancies":1,"items":[{"payment_id":"bad","status":"MISSING_LEDGER"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "source", "destination", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.baseDelay = time.Millisecond
	if err := client.Move(context.Background(), uuid.New(), "USD", "FUND", decimal.NewFromInt(1), "key"); err == nil {
		t.Fatal("accepted reconciliation discrepancy")
	}
}
