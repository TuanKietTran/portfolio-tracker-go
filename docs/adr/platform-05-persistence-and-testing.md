# Platform 05: Persistence and testing

Status: Accepted for the current simulation; durable adapter deferred.

The runtime uses in-memory repositories to keep local runs deterministic and dependency-light. Tests cover repository concurrency and HTTP behavior without a database service. State, histories, and idempotency records are lost at process restart; there is no Postgres parity or durability claim.
