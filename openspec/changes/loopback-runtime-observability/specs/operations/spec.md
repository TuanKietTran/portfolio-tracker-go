## MODIFIED Requirements

### Requirement: Honest runtime configuration
HTTP_ADDR MUST default to 127.0.0.1:8105, use a valid numeric port, and bind to loopback. CASH_MODE MUST default to local; explicit payments mode MUST require a loopback PAYMENTS_URL and distinct bounded synthetic source/destination account identifiers. PAYMENTS_URL in local mode and DATABASE_URL MUST fail startup rather than silently changing modes. COST_METHOD MUST be FIFO (default) or AVERAGE_COST. Health/readiness MUST report local-memory initialization, not external dependency readiness; unavailable readiness returns503. Metrics expose per-router request/error/panic counters and process heap bytes/objects, goroutine count, active HTTP connections and most recent GC pause. Loopback-only `/debug/pprof/` exposes standard Go runtime profiles; CPU and trace captures MUST be capped at 10 seconds. Metrics counters reset on restart.

#### Scenario: Invalid payment configuration
- **WHEN** payments mode lacks endpoint/accounts or selects a non-loopback endpoint
- **THEN** startup fails rather than silently using local simulation

#### Scenario: Loopback-only process diagnostics
- **GIVEN** default or configured loopback binding
- **WHEN** `/metrics` or `/debug/pprof/` is requested
- **THEN** process gauges and Go profiles are returned
- **AND** non-loopback HTTP_ADDR values are rejected at startup
