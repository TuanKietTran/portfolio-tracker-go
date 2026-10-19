# Playback design

- Package `internal/playback`. `Play` uses a bounded worker pool; it returns one outcome per submitted call and propagates overall-context cancellation.
- Each attempt may use its own deadline. An attempt deadline is retryable while the overall window context remains active; overall cancellation is terminal.
- `DefaultPolicy` uses at most 6 attempts, 50 ms exponential base delay, 2 s cap, full jitter, 5 s per-attempt timeout, and 32 concurrent workers.
- Retryable statuses are 408, 429, and any 5xx; business 4xx are final. Transport and response-read errors are retried unless the overall request was canceled.
- The production replay test sends 10,000 requests through `api.NewRouter` on a real `httptest` loopback listener, including 9,800 unique keyed trades, 200 duplicate submissions, 500 injected 503s, 100 injected 429s, and 100 per-attempt timeouts. It asserts all 10,000 terminal responses and direct position/cash state. Race-instrumented tests cover the playback package separately; the timing acceptance is run without race instrumentation.
- `TestLegacyThousandPerMinuteAndPlayback` remains an independent regression against the old legacy adapter and is not R1 evidence.
