# Loopback runtime observability

## Why

The load harness needs process-level heap, goroutine, connection, GC-pause and profile evidence while ensuring diagnostic endpoints cannot be exposed on a public interface.

## Scope

Bind the default server to 127.0.0.1 and reject non-loopback addresses. Track active HTTP connections; expose heap/goroutine/latest-GC-pause gauges and standard pprof on the loopback API. Cap CPU/trace profile captures at ten seconds and align OpenAPI/docs/tests.

## Out of scope

Hosted profiling, external telemetry backends, and metrics persistence.
