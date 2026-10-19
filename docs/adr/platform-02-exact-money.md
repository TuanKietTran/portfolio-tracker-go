# Platform 02: Exact money

Status: Accepted.

Portfolio quantities, prices, cash, and accounting values use `shopspring/decimal`; currencies are explicit on current accounting routes. Local settlement rejects fractional minor units rather than silently rounding. Lot allocation and valuation precision limits are documented in the linked service ADRs. No floating-point money arithmetic is used.
