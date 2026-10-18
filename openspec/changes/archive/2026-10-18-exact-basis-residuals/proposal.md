# Preserve exact remaining cost basis across repeating allocations

Unit costs can repeat in decimal form. Multiplying a rounded average or split unit cost by sold quantity can create or destroy tiny amounts of cost basis, especially when closing a position. Preserve authoritative total lot basis separately from display unit cost, allocate partial-sale basis to a fixed 18 decimal places (half away from zero), carry any remainder in the remaining lot/position, and consume the entire exact remaining basis on full sale.

Split share quantities multiply before division and must be exactly representable at 18 fractional decimal places. Reject unsupported repeating share quantities without mutation; cash-in-lieu is not implemented. Per-unit basis may repeat, but its display approximation must never be the source of realized P&L.

Regression tests must first demonstrate the existing failures, then prove FIFO/average conservation through partial and full liquidation and forward/reverse splits. No legacy playback assertions are removed.
