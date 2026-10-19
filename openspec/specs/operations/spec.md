# Local runtime operations

### Requirement: Honest runtime configuration
HTTP_ADDR MUST default to 127.0.0.1:8105, use a valid numeric port, and bind to loopback. CASH_MODE MUST default to local; explicit payments mode MUST require a loopback PAYMENTS_URL and distinct bounded synthetic source/destination account identifiers. PAYMENTS_URL in local mode and DATABASE_URL MUST fail startup rather than silently changing modes. COST_METHOD MUST be FIFO (default) or AVERAGE_COST. Health/readiness MUST report local-memory initialization, not external dependency readiness; unavailable readiness returns503. Metrics expose per-router request/error/panic counters and process heap bytes/objects, goroutine count, active HTTP connections and most recent GC pause. Loopback-only `/debug/pprof/` exposes standard Go runtime profiles; CPU and trace captures MUST be capped at 10 seconds. Metrics counters reset on restart.

#### Scenario: Invalid payment configuration
- **WHEN** payments mode lacks endpoint/accounts or selects a non-loopback endpoint
- **THEN** startup fails rather than silently using local simulation

#### Scenario: Local mode remains explicit default
- **WHEN** no payment configuration is provided
- **THEN** startup selects local cash simulation

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
