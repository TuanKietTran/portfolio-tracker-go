# ADR 0003: Immediate keyed local corporate actions

Status: accepted for the synthetic in-memory runtime.

Corporate actions are explicit SPLIT, DIVIDEND or FEE commands against an existing portfolio position with immutable instrument currency. Cash amounts are **total amounts**, not per-share rates, and must settle in exact supported minor units. Dividends and splits require current shares. Fees may be charged against a previously held closed position. Dividends add income and fees subtract expense in the existing `RealizedPnL` projection; neither changes share quantity or cost basis. These are investment income/performance expenses, not investor contributions or withdrawals.

The repository lock commits holdings, cash, identity and original response together. Corporate keys use a separate global namespace. Same-payload replay returns original holdings and cash, not later balances; different portfolio/symbol/kind/currency/amount/ratio conflicts. Invalid actions, insufficient fee cash and unrepresentable split shares have no effect and do not consume keys. Unused unkeyed split adapter mutation was replaced by this keyed boundary; pure split helpers/tests remain.

Splits preserve authoritative basis under ADR0002. Post-action simulated marks must be published explicitly; corporate actions never mutate price history. Current holdings are not historical ownership snapshots, and actions apply immediately rather than retroactively by an event timestamp. No automatic dividend entitlement calculation, cash-in-lieu, external posting or durable restart recovery is claimed.

Worked test: fund100, buy two at10, dividend4, fee1, split two-for-one => cash83, quantity4, basis20, realized income3. Publishing a mark5 values the securities at20. `TestCorporateHTTPConcurrentReplayCashAndWorkedValuation` proves the route; `TestConcurrentCorporateFeesCannotOverdrawAndFailureDoesNotConsumeKey` proves100 concurrent fees2 against cash80 yield40 successes without overdraft.
