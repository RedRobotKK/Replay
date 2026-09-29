# Experiment ledger

One row per experiment. Dated artifacts are authoritative; this is the index.

| id | date | question | n | result | class | artifact |
|---|---|---|---:|---|---|---|
| DS-BASE | 2026-09-28 | Does DeepSeek run through an existing Replay path? | 2 | Yes, no adapter needed; inclusive counting normalised correctly | OBSERVED | `deepseek/baseline/2026-09-28-cold-warm.md` |
| DS-SURF | 2026-09-28 | What surfaces exist? | ~20 | 6 endpoints, 2 models, 3 usage dialects, no rate-limit headers | OBSERVED | `deepseek/surface-map-2026-09-28.md` |
| DS-GAP | 2026-09-28 | What pricing and telemetry is missing? | n/a | No DeepSeek price row; time-of-day pricing inexpressible; 5 telemetry gaps | DERIVED | `deepseek/gap-analysis-2026-09-28.md` |
| DS-CONC | 2026-09-28 | Where is the concurrency ceiling? | 112 | None found at 64; latency fell as concurrency rose | OBSERVED | `deepseek/campaign-prereg-2026-09-28.md` |
| DS-F1 | 2026-09-28 | Does disabling reasoning preserve outcome? | 120 | Yes on 2 of 3 tasks: cost and latency down, 20/20 held | MEASURED IMPROVEMENT | `deepseek/findings-2026-09-28.md` |
| DS-F2 | 2026-09-28 | Does `reasoning.effort` work? | 3 | **RECLASSIFIED NOT_MEASURED**: the tested shape is not the documented control (`reasoning_effort` is flat) | **NOT_MEASURED** | same |
| DS-F3 | 2026-09-28 | Does prefix ordering generalise? | 80 | Only under prefix recurrence: 90.4% cheaper shared, 0% unique | MEASURED IMPROVEMENT, CONDITIONAL | same |
| DS-F4 | 2026-09-28 | Does derived cost match observed? | 440 | Comparison invalid: lagged balance against a partial subtotal | **WITHDRAWN, see DS-F4b** | `deepseek/f4-reconciliation-2026-09-28.md` |
| DS-F4a | 2026-09-28 | Controlled batch, cent-quantization 3.8% | 300 | observed $0.13 vs derived $0.1378, ratio 0.943 | INCONCLUSIVE, residual outside quantization | same |
| DS-F4b | 2026-09-28 | Controlled batch, quantization 1.19% | 200 | observed $0.43 vs derived $0.4219, ratio 1.0193, diff 1.93% | **RESOLVED A — reconciled within declared 3%** | same |

| WP-01 | 2026-09-28 | Can DeepSeek find evidence/claim-boundary risks in the repo? | 30 calls | 5 findings: 4 verified, 1 partial, 0 false positives | USEFUL | `deepseek/wp01/` |
| WP-02 | 2026-09-28 | Does the repo contract establish F1 as a defect? | 7 calls | **CONTRACT GAP**; test-only, 0 production files | USEFUL | commit `1d14036` |

| HH-01 | 2026-09-28 | Harden the harness at zero cost | 0 calls | O1 and O2 implemented and mutation-checked; O3 not implemented | n/a | `experiment/harness/test_adapter.py` |
| HH-02 | 2026-09-28 | Rebuild the fan-out to production shape | 0 calls | Four defects fixed, a fifth found while testing (float money); 10 mutations all killed | Saving is DERIVED, not OBSERVED | `experiment/deepseek/fanout-audit-2026-09-28.md` |
| DS-C1 | 2026-09-29 | How is the prefix cache shaped? | 49 | 128-token blocks, final block never cached; 17/17 rungs exact | OBSERVED | `deepseek/cache-characterisation-2026-09-29.md` |
| DS-C2 | 2026-09-29 | What busts the cache? | in C1 | A single leading space does; temperature and max_tokens do not; hits serve at +0.0s; cache is shared across dialects | OBSERVED | same |
| DS-F2b | 2026-09-29 | Does a reasoning control exist? | 5 | **YES**, `reasoning_effort:"none"` and `thinking:{type:disabled}`. Control produced 40 reasoning tokens and ZERO content | OBSERVED | `deepseek/optimization-arms-2026-09-29.md` |
| DS-OPT | 2026-09-29 | Baseline plus 5 one-variable arms | 72 | Best arm 6.5x cheaper and 6.9x faster at 12/12; parallel-alone is 0.99x cost at 0% hit | OBSERVED | same |
| DS-F5 | 2026-09-29 | Do cache reads bill at the hit rate? | 200 | **YES**. Observed $0.04 vs $0.03909 derived-at-hit (1.023) vs $1.13994 derived-at-miss (0.035) | **RESOLVED**, blocker 3 closed | same |

## Killed or corrected

- **F2 "reasoning control is silently ignored"**: withdrawn. The tested body used
  `reasoning: {effort: ...}`; the documented control is the flat
  `reasoning_effort`. An API ignoring an unrecognised key is not a defect. The
  original observation is preserved; the interpretation is not.

- **"Derived cost over-states observed spend by 2.9x"**: withdrawn. The original
  comparison read the balance before billing settled and compared it against a
  derived figure covering 440 of roughly 574 calls. Both errors were in the
  analysis. Preserved as DS-F4 rather than deleted.

- **98.3% prefix-ordering claim, generalised**: corrected. The effect is
  conditional on prefix recurrence. DS-F3.
- **Constant filler seed** in the first task design made every trial share a
  prefix and hid the ordering effect. Fixed.
- **`max_tokens=24`** was consumed entirely by reasoning, scoring every arm
  0/20. An instrument failure, not a model result.

- **First fan-out mutation sweep**: VOID. `cp` restored files with an mtime in
  the same whole second as the mutant's `__pycache__` entry, so CPython reused
  mutant bytecode and three mutations read as SURVIVED against code that was no
  longer there. Redone with `PYTHONDONTWRITEBYTECODE=1`. Same class as counting
  cached `go test` invocations as one clean run. HH-02.

- **19-28x warm-then-fan projection**: corrected downward at small shapes. The
  arms run measured 4.4x at a 3,400-token prefix over 12 questions, where output
  cost dilutes the ratio. The projected figure holds at the shape it was
  projected for: 29.2x OBSERVED at 19,000 tokens over 200 questions. DS-OPT.
- **Session-level cost reconciliation**: OPEN, not resolved. The isolated 200-call
  batch reconciles at 1.023; the session total is $0.05 observed against $0.0959
  derived, a factor of 1.9. Settling lag and per-request cent truncation both
  explain it and make opposite predictions. Not inferred either way, because the
  first F4 comparison died of exactly that. DS-F5.
