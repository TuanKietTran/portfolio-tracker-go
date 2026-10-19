# Playback

## Purpose

Bounded loopback HTTP replay validates the production router, transient failure recovery, idempotent duplicate effects, and terminal outcomes inside a fixed request window. The historical 1,000-call legacy adapter replay remains a separate regression only.

## Requirements

### Requirement: Finish 10,000 production-router requests inside one minute
The playback client SHALL complete 10,000 recorded calls in less than 60 seconds against the production API router over a real loopback HTTP listener. Playback SHALL use bounded concurrency and a separately bounded overall context.

#### Scenario: Faulted duplicate trade burst
- **GIVEN** 9,800 unique keyed trade commands and 200 additional submissions repeating an existing key and identical body
- **AND** at least 5% of requests receive injected 503, some receive 429, and 1% experience per-attempt timeouts
- **WHEN** the production router is replayed using at most 6 attempts and capped exponential full jitter
- **THEN** every request reaches a terminal 201 response in under 60 seconds
- **AND** each unique trade is applied exactly once
- **AND** cash and position state reflect the 9,800 unique effects

### Requirement: Retry only transient failures
The client SHALL retry transport errors and HTTP 408, 429, or 5xx with at most 6 attempts and capped exponential full jitter, and SHALL NOT retry any other status. Per-attempt timeout SHALL be retryable while the overall replay context remains live; overall cancellation SHALL be final.

#### Scenario: HTTP 400 is final
- **WHEN** the server returns HTTP 400
- **THEN** the client stops after 1 attempt

#### Scenario: Repeated 503 stops at the attempt limit
- **WHEN** the server returns HTTP 503 on every attempt and the limit is 2
- **THEN** the client stops after 2 attempts
- **AND** the outcome status is 503

### Requirement: Retain legacy adapter playback independently
The historical 1,000-call legacy adapter regression SHALL remain separately tested and SHALL NOT count as production-router replay evidence.
