# Tasks

- [x] Prove pre-change failures for repeating average full liquidation, FIFO split liquidation and reverse split quantity.
- [x] Track authoritative lot total basis and allocate partial removals at fixed 18-place half-away precision.
- [x] Consume entire remaining basis on full sale and carry partial residuals forward.
- [x] Multiply share quantities before division; reject inexact aggregate or per-lot share quantities without mutation.
- [x] Add conservation checks after every partial sale for FIFO/average and split factors 2 through 9.
- [x] Run full gates plus cross-package aggregate coverage; archive and record exact evidence.
