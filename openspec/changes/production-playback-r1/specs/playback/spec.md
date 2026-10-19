## MODIFIED Requirements

### Requirement: Finish 10,000 production-router requests inside one minute
The playback client SHALL complete 10,000 recorded calls in less than 60 seconds against the production API router over a real loopback HTTP listener. Playback SHALL use bounded concurrency and a separately bounded overall context.

#### Scenario: Faulted duplicate trade burst
- **GIVEN** 9,800 unique keyed trade commands and 200 additional submissions repeating an existing key and identical body
- **AND** at least 5% of requests receive injected 503, some receive 429, and 1% experience per-attempt timeouts
- **WHEN** the production router is replayed using at most 6 attempts and capped exponential full jitter
- **THEN** every request reaches a terminal 201 response in under 60 seconds
- **AND** each unique trade is applied exactly once
- **AND** cash and position state reflect the 9,800 unique effects

### Requirement: Retain the legacy adapter regression
The historical 1,000-call legacy adapter fixture SHALL remain separately tested and SHALL NOT count as production-router replay evidence.
