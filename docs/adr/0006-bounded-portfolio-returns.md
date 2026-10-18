# ADR 0006: Bounded historical portfolio returns

Status: accepted for explicitly supported local query windows.

`/v1/portfolios/{id}/returns` requires currency, from/to, max_age and method TWR or MWR. Cutoffs are exact starts of Unix seconds. A repository read captures initial/final holdings and external flows under one lock; an independent copied price history is then used consistently for every valuation in the calculation. These are consistent per-store views, not a distributed transaction or publication-time market audit. Later historical mark insertion can revise future query results.

Start NAV substitutes starting capital once. Only FUND/WITHDRAW in `(from,to]` are investor cash flows, negative for contributions and positive for withdrawals. All commands at from are included in the starting projection; no duplicate initial funding flow is appended. Each external flow uses its own sequence snapshot, including same-time flows. Trading, dividend income, fee expense and splits are internal investment effects. Every start/end/flow valuation requires eligible fresh marks under the same single-currency/no-FX policy.

## TWR

Link pre-flow NAV / previous post-flow NAV, followed by final NAV / last post-flow NAV, then subtract one. Each ratio and the final result round half away from zero to18 fractional places; geometric multiplication itself uses decimal. It is a period return, not annualized. Positive start/end and pre/post-flow NAV are required. Zero-capital or total-loss windows, including an18-place ratio that rounds to zero, fail explicitly. The existing pure total-loss rejection test remains unchanged; support for those cases is not claimed.

Worked contribution: NAV100 grows to120, a120 contribution makes240, ending NAV300 =>1.2*1.25-1=0.5. Same-time sequence regression: start100, fund100, fee20, fund100, end280 =>-0.1. Looking up the final same-time holdings for every flow would be wrong.

## MWR

Annualized ACT/365 uses fixed elapsed seconds /31536000. Initial negative NAV, period investor flows and final positive NAV form dated cash flows. Solver cash projections round to18 places, must fit canonical30-coefficient-digit/exponent-18..18 bounds, and remain decimal; year fractions and math.Pow discount factors use float64. Equal computed-time flows are netted. Require one chronological negative-to-positive sign change; reject potentially multiple-IRR patterns rather than arbitrarily choose a root.

The guarded solver uses fixed bracket[-0.999999999999,10],1..256 iterations(default128), rate tolerance1e-12..1e-3(default1e-10) and currency-unit NPV tolerance1e-12..0.01(default1e-8). BOTH computed absolute residual and bracket width must pass, or a computed exact root is found. Response diagnostics include iterations, tolerances, residual and rate width. These bound the **computed discount model**, not a formal real-arithmetic/Pow ULP error guarantee. The preserved legacy pure MWR primitive is not the HTTP solver.

One-year100->110 analytic and bounded-rate fuzz regressions assert rate error<=1e-8 and both computed diagnostics. Worked contribution/same-time-fee cases compare with independent quadratic rate formulas. Unbracketed, nonconventional, undefined-capital and nonconvergent cases return400 without partial rates.

## Limits and remaining scope

A query supports at most3650 elapsed days,4096 portfolio commands and256 external flows. A non-snapshot-capable feed returns503; missing starting history404; invalid/FX/stale data/limits400. Full snapshots/price copies still have memory costs and no durable recovery guarantee. Portfolio lifecycle, payment-backed cash/outbox, Postgres parity and integrated S3 evidence remain unimplemented. No universal multiple-IRR, zero-capital or formally certified floating-discount error support is claimed.
