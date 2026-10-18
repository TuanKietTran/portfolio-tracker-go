## ADDED Requirements

### Requirement: Honest local runtime configuration
HTTP_ADDR MUST default to :8105 and require a valid numeric port. CASH_MODE MUST be local; COST_METHOD MUST be FIFO (default) or AVERAGE_COST. Unsupported PAYMENTS_URL/DATABASE_URL MUST fail startup. Health MUST report local/memory semantics; readiness MUST indicate local initialization only and return 503 if unavailable. Metrics MUST expose per-router request/error/panic counts, resetting on process restart.

#### Scenario: Unsupported external mode
- **WHEN** payment configuration is supplied without an implemented recovery-safe adapter
- **THEN** startup fails before accepting requests instead of silently selecting simulated funding

### Requirement: Bounded uniform HTTP
Production responses MUST carry a UUID X-Request-ID (preserve valid nonzero incoming IDs, generate otherwise). Errors MUST use JSON error code/message and request_id. Mutations MUST accept exactly one JSON value with known fields within 16384 bytes, rejecting malformed/unknown/trailing JSON with 400 and oversized bodies with 413. Keys MUST be nonblank and no longer than 200 bytes. Nil portfolio UUIDs MUST be rejected.

#### Scenario: Recovered readiness panic
- **WHEN** readiness panics before output
- **THEN** the server returns a generic 500 envelope, logs/counts recovery and does not expose panic payloads

### Requirement: Bounded graceful lifecycle
HTTP servers MUST have read-header/read/write/idle/header-size bounds. Context cancellation MUST stop accepting requests, drain in-flight work within five seconds in the default runtime, force-close on expiry and join the serving goroutine. Lifecycle tests MUST use channels rather than synchronization sleeps.

#### Scenario: Grace expired
- **GIVEN** a request blocked beyond a supplied shutdown deadline
- **WHEN** cancellation initiates shutdown
- **THEN** active connections are closed and the caller receives the shutdown timeout error
