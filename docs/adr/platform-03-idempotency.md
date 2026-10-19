# Platform 03: Idempotency

Status: Accepted for in-memory mutations.

Write routes require an idempotency key (header or body where supported). A repeated key with the same payload returns the original result; a changed payload conflicts. The key and result are stored atomically with the local mutation. They are memory-only and are lost on restart, so this is not a durable exactly-once guarantee.
