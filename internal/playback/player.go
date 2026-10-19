package playback

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	RequestsPerMinute = 10000
	Window            = time.Minute
)

type Call struct {
	Method string
	Path   string
	Body   []byte
	Header map[string]string
}

type Outcome struct {
	Status   int
	Body     []byte
	Attempts int
	Err      error
}

type Policy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	AttemptTimeout time.Duration
	Concurrency    int
}

func DefaultPolicy() Policy {
	return Policy{MaxAttempts: 6, BaseDelay: 50 * time.Millisecond, MaxDelay: 2 * time.Second, AttemptTimeout: 5 * time.Second, Concurrency: 32}
}

func Retryable(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func IsIdempotentReplay(status int, body []byte) bool {
	return status == http.StatusBadRequest && bytes.Contains(body, []byte("idempotency key already used"))
}

// Play sends calls with bounded concurrency. Per-attempt deadlines are distinct
// from the overall context, so a timed-out request can be retried while its
// replay window remains open.
func Play(ctx context.Context, client *http.Client, baseURL string, calls []Call, policy Policy) ([]Outcome, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}
	if policy.Concurrency < 1 {
		policy.Concurrency = 1
	}
	if policy.MaxDelay <= 0 {
		policy.MaxDelay = 2 * time.Second
	}
	out := make([]Outcome, len(calls))
	callErrors := make([]error, len(calls))
	jobs := make(chan int)
	workers := policy.Concurrency
	if workers > len(calls) {
		workers = len(calls)
	}
	var wg sync.WaitGroup
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				out[i], callErrors[i] = playOne(ctx, client, baseURL, calls[i], policy)
			}
		}()
	}
	for i := range calls {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return out, ctx.Err()
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	for _, err := range callErrors {
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func playOne(ctx context.Context, client *http.Client, baseURL string, call Call, policy Policy) (Outcome, error) {
	var last Outcome
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		if attempt > 1 && policy.BaseDelay > 0 {
			capDelay := policy.BaseDelay << (attempt - 2)
			if capDelay > policy.MaxDelay || capDelay < 0 {
				capDelay = policy.MaxDelay
			}
			// Full jitter samples uniformly from [0, the exponential cap].
			delay := time.Duration(rand.Int63n(int64(capDelay) + 1))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return last, ctx.Err()
			case <-timer.C:
			}
		}
		attemptCtx := ctx
		cancel := func() {}
		if policy.AttemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, policy.AttemptTimeout)
		}
		req, err := http.NewRequestWithContext(attemptCtx, call.Method, baseURL+call.Path, bytes.NewReader(call.Body))
		if err != nil {
			cancel()
			return last, err
		}
		if len(call.Body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		for key, value := range call.Header {
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		last.Attempts = attempt
		if err != nil {
			last.Err = err
			last.Status = 0
			cancel()
			if ctx.Err() != nil {
				return last, ctx.Err()
			}
			if attempt == policy.MaxAttempts || !retryableErr(err) {
				return last, nil
			}
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			last.Err = readErr
			last.Status = resp.StatusCode
			if ctx.Err() != nil {
				return last, ctx.Err()
			}
			if attempt == policy.MaxAttempts || !retryableErr(readErr) {
				return last, nil
			}
			continue
		}
		last.Status = resp.StatusCode
		last.Body = body
		last.Err = nil
		if !Retryable(resp.StatusCode) || attempt == policy.MaxAttempts {
			return last, nil
		}
	}
	return last, nil
}

func retryableErr(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled)
}
