// Package payments adapts the payment orchestration HTTP contract for portfolio cash.
package payments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TuanKietTran/portfolio-tracker-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const maxAttempts = 6

type paymentStatusError struct{ status int }

func (e paymentStatusError) Error() string {
	return fmt.Sprintf("payments API returned HTTP %d", e.status)
}
func (e paymentStatusError) PaymentIntentConflict() bool { return e.status == http.StatusConflict }

type Client struct {
	baseURL            string
	sourceAccount      string
	destinationAccount string
	httpClient         *http.Client
	attemptTimeout     time.Duration
	baseDelay          time.Duration
	maxDelay           time.Duration
}

func NewClient(baseURL, sourceAccount, destinationAccount string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("payments URL must be an absolute HTTP URL without user information")
	}
	host := parsed.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, errors.New("payments URL must use loopback")
	}
	if len(sourceAccount) > 200 || len(destinationAccount) > 200 || strings.TrimSpace(sourceAccount) == "" || strings.TrimSpace(destinationAccount) == "" || sourceAccount != strings.TrimSpace(sourceAccount) || destinationAccount != strings.TrimSpace(destinationAccount) || sourceAccount == destinationAccount {
		return nil, errors.New("distinct payment source and destination accounts are required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), sourceAccount: sourceAccount, destinationAccount: destinationAccount, httpClient: httpClient, attemptTimeout: 3 * time.Second, baseDelay: 50 * time.Millisecond, maxDelay: 2 * time.Second}, nil
}

type createPaymentRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	SourceAccount  string `json:"source_account"`
	DestAccount    string `json:"dest_account"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
}
type createPaymentResponse struct {
	PaymentID string `json:"payment_id"`
}
type paymentRecord struct {
	ID         string `json:"ID"`
	Status     string `json:"Status"`
	LedgerTxID string `json:"LedgerTxID"`
}
type reconciliation struct {
	Checked       int `json:"checked"`
	Matched       int `json:"matched"`
	Discrepancies int `json:"discrepancies"`
}

// Move transfers cash only after payment status, ledger identity and global
// reconciliation are confirmed. Its deterministic payment key lets a caller
// replay an ambiguous request without creating another payment.
func (c *Client) Move(ctx context.Context, portfolio uuid.UUID, currency, direction string, amount decimal.Decimal, key string) error {
	if portfolio == uuid.Nil || key == "" || amount.Sign() <= 0 {
		return errors.New("portfolio, positive amount and idempotency key are required")
	}
	if currency != "USD" && currency != "EUR" && currency != "GBP" {
		return errors.New("payments contract supports USD, EUR and GBP only")
	}
	if err := domain.ValidateCashAmount(amount, currency); err != nil {
		return err
	}
	from, to := c.sourceAccount, c.destinationAccount
	if direction == "WITHDRAW" {
		from, to = to, from
	} else if direction != "FUND" {
		return errors.New("unsupported cash direction")
	}
	intent := stableIntent(portfolio, key)
	request := createPaymentRequest{IdempotencyKey: intent, SourceAccount: from, DestAccount: to, Amount: amount.String(), Currency: currency}
	var created createPaymentResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/v1/payments", request, &created); err != nil {
		return fmt.Errorf("create payment intent: %w", err)
	}
	paymentID, err := uuid.Parse(created.PaymentID)
	if err != nil || paymentID == uuid.Nil {
		return errors.New("payments create returned an invalid payment_id")
	}
	path := "/v1/payments/" + paymentID.String()
	var record paymentRecord
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := c.getJSON(ctx, path, &record); err != nil {
			if !c.pause(ctx, attempt) {
				return err
			}
			continue
		}
		if record.Status == "SETTLED" {
			break
		}
		if record.Status != "PENDING" && record.Status != "SCREENED" {
			return fmt.Errorf("payment is not recoverable from status %q", record.Status)
		}
		processErr := c.requestJSONAttempts(ctx, http.MethodPost, path+"/process", nil, &record, 1)
		var statusErr paymentStatusError
		if errors.As(processErr, &statusErr) && statusErr.status != http.StatusRequestTimeout && statusErr.status != http.StatusTooManyRequests && statusErr.status < 500 {
			return processErr
		}
		if processErr != nil && !c.pause(ctx, attempt) {
			return fmt.Errorf("process payment: %w", processErr)
		}
		// Always re-read after process, including ambiguous transport/5xx results.
	}
	if record.Status != "SETTLED" || record.LedgerTxID == "" {
		return errors.New("payment did not reach SETTLED with a ledger transaction identity")
	}
	var result reconciliation
	if err := c.getJSON(ctx, "/v1/reconciliation?limit=200", &result); err != nil {
		return fmt.Errorf("payment reconciliation unavailable: %w", err)
	}
	if result.Checked == 0 || result.Discrepancies != 0 || result.Matched != result.Checked {
		return errors.New("payment reconciliation reports unmatched or discrepant payments")
	}
	return nil
}

func stableIntent(portfolio uuid.UUID, key string) string {
	digest := sha256.Sum256([]byte(portfolio.String() + ":" + key))
	return "portfolio-cash-" + hex.EncodeToString(digest[:])
}

func (c *Client) requestJSON(ctx context.Context, method, path string, request, response any) error {
	return c.requestJSONAttempts(ctx, method, path, request, response, maxAttempts)
}

func (c *Client) requestJSONAttempts(ctx context.Context, method, path string, request, response any, attempts int) error {
	var body io.Reader
	if request != nil {
		data, err := json.Marshal(request)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	for attempt := 0; attempt < attempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, c.attemptTimeout)
		req, err := http.NewRequestWithContext(attemptCtx, method, c.baseURL+path, bodyForRetry(body))
		if err != nil {
			cancel()
			return err
		}
		if request != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			cancel()
			if ctx.Err() != nil || attempt == attempts-1 || !c.pause(ctx, attempt) {
				return doErr
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			if attempt == attempts-1 || !c.pause(ctx, attempt) {
				return readErr
			}
			continue
		}
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if attempt == attempts-1 || !c.pause(ctx, attempt) {
				return paymentStatusError{status: resp.StatusCode}
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return paymentStatusError{status: resp.StatusCode}
		}
		if response != nil && len(data) != 0 {
			if err := json.Unmarshal(data, response); err != nil {
				return fmt.Errorf("decode payments response: %w", err)
			}
		}
		return nil
	}
	return errors.New("payments retry limit exhausted")
}

func bodyForRetry(body io.Reader) io.Reader {
	if body == nil {
		return nil
	}
	if bytesReader, ok := body.(*bytes.Reader); ok {
		clone := *bytesReader
		return &clone
	}
	return body
}

func (c *Client) getJSON(ctx context.Context, path string, response any) error {
	return c.requestJSON(ctx, http.MethodGet, path, nil, response)
}

func (c *Client) pause(ctx context.Context, attempt int) bool {
	capDelay := c.baseDelay << attempt
	if capDelay > c.maxDelay || capDelay < 0 {
		capDelay = c.maxDelay
	}
	if capDelay <= 0 {
		return ctx.Err() == nil
	}
	delay := time.Duration(rand.Int63n(int64(capDelay) + 1))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
