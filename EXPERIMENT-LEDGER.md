# Experiment ledger

One row per experiment. Dated artifacts are authoritative; this is the index.

| id | date | question | n | result | class | artifact |
|---|---|---|---:|---|---|---|
| DS-BASE | 2026-09-28 | Does DeepSeek run through an existing Replay path? | 2 | Yes, no adapter needed; inclusive counting normalised correctly | OBSERVED | `deepseek/baseline/2026-09-28-cold-warm.md` |
| DS-SURF | 2026-09-28 | What surfaces exist? | ~20 | 6 endpoints, 2 models, 3 usage dialects, no rate-limit headers | OBSERVED | `deepseek/surface-map-2026-09-28.md` |
| DS-GAP | 2026-09-28 | What pricing and telemetry is missing? | n/a | No DeepSeek price row; time-of-day pricing inexpressible; 5 telemetry gaps | DERIVED | `deepseek/gap-analysis-2026-09-28.md` |
| DS-CONC | 2026-09-28 | Where is the concurrency ceiling? | 112 | None found at 64; latency fell as concurrency rose | OBSERVED | `deepseek/campaign-prereg-2026-09-28.md` |
| DS-F1 | 2026-09-28 | Does disabling reasoning preserve outcome? | 120 | Yes on 2 of 3 tasks: cost and latency down, 20/20 held | MEASURED IMPROVEMENT | `deepseek/findings-2026-09-28.md` |
| DS-F2 | 2026-09-28 | Does `reasoning.effort` work? | 3 | Accepted and silently ignored | OBSERVED | same |
| DS-F3 | 2026-09-28 | Does prefix ordering generalise? | 80 | Only under prefix recurrence: 90.4% cheaper shared, 0% unique | MEASURED IMPROVEMENT, CONDITIONAL | same |
| DS-F4 | 2026-09-28 | Does derived cost match observed? | 440 | No: $0.1458 derived against $0.05 observed | **OPEN DISCREPANCY** | same |

## Killed or corrected

- **98.3% prefix-ordering claim, generalised**: corrected. The effect is
  conditional on prefix recurrence. DS-F3.
- **Constant filler seed** in the first task design made every trial share a
  prefix and hid the ordering effect. Fixed.
- **`max_tokens=24`** was consumed entirely by reasoning, scoring every arm
  0/20. An instrument failure, not a model result.
