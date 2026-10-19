# Platform 01: CQRS boundaries

Status: Accepted for the local simulation.

Commands own writes and accounting invariants; query handlers expose read models. This keeps mutation validation separate from read projections. The current `internal/query` summary is a legacy view and returns positions with zero-valued totals; it is not the marked valuation API. The current runtime uses `internal/command` and dedicated valuation queries.
