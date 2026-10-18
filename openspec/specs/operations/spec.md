# Local runtime operations

### Requirement: Honest runtime configuration
HTTP_ADDR MUST default to :8105 with a valid numeric port. CASH_MODE MUST be local. COST_METHOD MUST be FIFO (default) or AVERAGE_COST. Unsupported PAYMENTS_URL/DATABASE_URL MUST fail startup. Health/readiness MUST report local-memory semantics, not external dependency readiness; unavailable readiness returns 503. Metrics expose per-router request/error/panic counters reset on restart.

#### Scenario: Unsupported adapter
- **WHEN** external payment/database configuration is supplied without an implemented adapter
- **THEN** startup fails rather than silently using local simulation

### Requirement: Bounded uniform HTTP
Production responses MUST carry a UUID X-Request-ID, preserving only valid nonzero incoming UUIDs. Errors MUST contain JSON error code/message and request_id. Mutation requests MUST contain one JSON value with known fields within16384 bytes. Malformed/unknown/trailing JSON returns400; oversized bodies return413. Keys MUST be nonblank and no longer than200 bytes. Nil portfolio UUIDs MUST be rejected.

#### Scenario: Readiness panic before output
- **WHEN** a readiness check panics
- **THEN** a generic500 is returned and counted/logged without exposing the panic payload

### Requirement: Bounded graceful HTTP lifecycle
Server read-header/read/write/idle timeouts MUST be bounded (default5/10/15/60 seconds) with bounded header size. Context cancellation MUST stop accepting requests and drain in-flight work within the default five-second grace, then force-close on expiry and join the serving goroutine.

#### Scenario: Drain timeout
- **GIVEN** a handler blocked past the shutdown deadline
- **WHEN** cancellation initiates shutdown
- **THEN** the connection is forced closed and the caller observes timeout
