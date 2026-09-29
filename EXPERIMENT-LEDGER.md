# Experiment ledger

One row per experiment. Dated artifacts are authoritative; this is the index.

| id | date | question | n | result | class | artifact |
|---|---|---|---:|---|---|---|
| DS-BASE | 2026-09-28 | Does DeepSeek run through an existing Replay path? | 2 | Yes, no adapter needed; inclusive counting normalised correctly | OBSERVED | `deepseek/baseline/2026-09-28-cold-warm.md` |
| DS-SURF | 2026-09-28 | What surfaces exist? | ~20 | 6 endpoints, 2 models, 3 usage dialects, no rate-limit headers | OBSERVED | `deepseek/surface-map-2026-09-28.md` |
| DS-GAP | 2026-09-28 | What pricing and telemetry is missing? | n/a | No DeepSeek price row; time-of-day pricing inexpressible; 5 telemetry gaps | DERIVED | `deepseek/gap-analysis-2026-09-28.md` |
| DS-CONC | 2026-09-28 | Where is the concurrency ceiling? | 112 | **FRAMING CORRECTED 2026-09-29**: docs publish 2,500 concurrent for flash / 500 for v4-pro with HTTP 429 beyond. We tested 64, ~39x below the limit, and reported no wall we were never near. Latency falling as concurrency rose stands | OBSERVED, bounded by published limit | `deepseek/docs-reconciliation-2026-09-29.md` |
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
| DS-RS | 2026-09-29 | Where does `reasoning_effort:"none"` apply? | 84 | **CLASS-CONDITIONAL**: lookup 100%->94% at 19.5x less output; aggregation 100%->**29%**, confidently wrong | OBSERVED, n=18/24 per cell | `deepseek/reasoning-scope-2026-09-29.md` |
| DS-DOC | 2026-09-29 | What did the documentation already say? | 0 | Pricing confirmed exact; 128-token block is genuinely undocumented; `reasoning_effort:"none"` is **undocumented** and fragile; concurrency limit is published | n/a, zero cost | `deepseek/docs-reconciliation-2026-09-29.md` |
| DS-WF | 2026-09-29 | Put the levers in the development workflow | 0 calls | `policy.py` + 18 tests, 8 mutations all killed; 43 harness tests wired into `make ci` and CI; `make ci` green end to end | n/a, zero cost | `docs/DEEPSEEK-OPTIMIZATION.md` |
| DS-RW | 2026-09-29 | Route every spending experiment through one enforced path | 0 calls | `session.Run`; 19 tests, 10 mutations all killed; architecture guard verified to catch a bypassing script | n/a, zero cost | `docs/DEEPSEEK-OPTIMIZATION.md` |
| DS-B1 | 2026-09-29 | Reconstruct the corpus from saved responses alone | 0 calls | 479/479 responses, 4 conservation identities, 0 violations; $0.229946832 by two independent paths agreeing to 9 decimals | **CONFIRMED** | `deepseek-suite/results/track-b.md` |
| DS-B2 | 2026-09-29 | Account for calls whose response was not saved | 0 calls | 5 raw-curl probe calls NOT_OBSERVED; lower bound $0.23005 | PARTIALLY SUPPORTED | same |
| DS-B6 | 2026-09-29 | Does per-request cent truncation explain the gap? | 0 calls | **REFUTED.** Max per-request cost $0.0056997, 0 of 479 reach $0.01; truncation predicts a $0.00 session total against an observed $0.23 fall | **REFUTED, no spend required** | same |
| DS-A1 | 2026-09-29 | Is cache identity byte-exact? | 7 | Leading space, capitalisation, punctuation at start all 0. Trailing space added still 2048. Mid-prefix change 896 = 7x128 | **SUPPORTED**: prefix-wise, block-granular | `deepseek-suite/claims.md` C3, C4 |
| DS-A2 | 2026-09-29 | H-A, H-B, H-D or none? | 44 | **H-B 18/18 discriminating rungs, H-A 0/18, H-D1 0/18, H-D2 4/22.** Every count a multiple of 128 | **H-B SUPPORTED, rivals REFUTED** | same C1, C2 |
| DS-A4 | 2026-09-29 | Is population completion-dependent? | 4 | Sequential B 3200, concurrent B 0, with `B dispatched before A returned` from wall-clock | **SUPPORTED** | same C5 |
| DS-A6 | 2026-09-29 | Are sampling parameters part of cache identity? | 8 | temperature, max_tokens, top_p, frequency_penalty, presence_penalty, stop: all 3200 against a 3200 control | **SUPPORTED**, but `reasoning_effort` held constant so NOT covered | same C6 |
| DS-A7 | 2026-09-29 | Does the cache cross endpoints? | 6 | 0 in both directions with working controls. **Contradicted a prior n=1 observation** | superseded by DS-A9 | same C10 |
| DS-A8 | 2026-09-29 | Is the cache account-global or model-scoped? | 6 | flash->v4-pro 0, v4-pro->flash 0, both controls working | **model-scoped** | same C9 |
| DS-A9 | 2026-09-29 | Why did A7 contradict the original? | 8 | **The request body's key set.** Cross-dialect reuse 3200 without `reasoning_effort`, 0 with it. Both earlier readings were correct | **RESOLVED** | same C10 |
| DS-A10 | 2026-09-29 | Is the cache key the raw request body? | 9 | **No.** An ignored junk key still hit 3200; toggling `reasoning_effort` either way hit 0 | `reasoning_effort` participates in cache identity; mechanism NOT_OBSERVED | same C7, C8 |
| DS-B4 | 2026-09-29 | Does settlement converge? | 0 | 13 polls over 24.1 min, all $45.78, zero changes, zero inference in window | **CONFIRMED** | same C14 |

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

- **"Disable reasoning" as a general recommendation**: killed before it shipped.
  DS-OPT measured 12/12 at `reasoning_effort:"none"` on a lookup task, which is
  the class where reasoning is unnecessary by construction. On aggregation the
  same setting scores 29% and answers 39 as 67, 14 as 1. The lever is real and
  it is conditional. DS-RS.
- **First DS-RS run**: VOID. The "mechanical" class asked which function sat on a
  given line number of a 48,000-char document, which is positional counting, not
  lookup, and `max_tokens=1024` truncated every reasoning-on call. The cell
  measured the question design. Rebuilt and re-run.

- **Reading the vendor documentation came after the campaign, not before.** It
  would not have saved most of the spend, since the block size, the disable
  shapes and the billing behaviour are all things the docs do not settle. It
  would have caught the concurrency framing and flagged the reasoning parameters
  as undocumented before they were written into a recommendation. DS-DOC.

- **The harness test suite ran nowhere.** 25 fixture tests existed for a day in
  neither `make ci` nor any workflow. A suite nobody runs is documentation with a
  confusing file extension. Now `make harness-test`, inside `make ci` and its own
  CI job, with the runner verified to go red on a failing test and on a glob that
  matches nothing. DS-WF.
- **`make ci` was already red before this session**, on markdownlint, across five
  files. Four were prose and are fixed. The fifth is DeepSeek's verbatim WP-01
  report, now excluded with a stated reason: reformatting a model's output to
  satisfy a style rule destroys the one property that makes it evidence. DS-WF.

- **policy.py was imported by nothing that spends.** It encoded the levers and
  six probe scripts each hand-rolled their own budget and call wrapper around
  it, so the levers were enforced by discipline. `session.Run` now enforces them
  by construction, and `TestNoScriptBypassesTheRunner` enforces that against the
  source rather than against a comment. DS-RW.
- **`runner.py` was dead and dangerous.** Imported by nobody, no spending
  ceiling, `ThreadPoolExecutor.map`. It now raises with a pointer instead of
  being deleted, because the failure mode is a future session finding a
  plausible-looking runner and using it. DS-RW.

- **Session-level cost reconciliation: CLOSED, was OPEN at a factor of 1.9.**
  The mechanism is settlement lag, not cent truncation. The 1.9 was reproduced
  exactly (cumulative derived $0.096008 against $0.05 observed at the $45.96
  read) and collapses to 1.0002 once settlement completes. Roughly $0.047 was
  outstanding at the $46.00 read, which is lag directly observed rather than
  inferred. DS-B1, DS-B6.
- **DS-F5 is WEAKENED, not withdrawn.** Given a settlement lag of that size, the
  $0.04 fall across the DS-F5 window cannot be cleanly attributed to that batch,
  so its 1.023 ratio reads tighter than the evidence supports. The session-level
  reconstruction supersedes it and is stronger: 479 responses against a single
  balance delta, rather than one batch against a contaminated window.
- **My reconciliation interval was half its true width.** A difference of two
  cent-resolution readings carries two independent errors, so the band is
  $0.02 and not $0.01. Recorded in
  `deepseek-suite/results/DEFECT-interval-width.md`; the fix is deferred only to
  avoid clobbering a concurrently running agent.

- **The reasoning-off lever is ~1.5x, not ~20x, once the prefix is cached.**
  Track C modelled it and the measured arms confirm it: A3 to A4 cuts output
  tokens 39x and total cost only 1.48x, because a warm 16k prefix makes input
  the dominant term. The "6.5x cheaper" headline is against the naive baseline
  and bundles three levers; ordering plus warming is 4.4x of it. Recorded
  because the small size of the lever, set against a 29% to 11% accuracy cliff
  on non-lookup classes, is what decides whether to use it. DS-C5.
- **`session.Run.fan` does not pass `extra_kwargs` through**, although `ask`
  does. Track C could only force its reasoning arms by handing `fan` a
  `task_class` it knew to be wrong, which couples the semantic classification to
  the mechanical setting and makes the arm unfaithful. Fix deferred while Track
  D is still writing to that module. OPEN.
- **`git add -A` swept up another agent's in-flight files.** Commit `bcf2e2c`
  captured Track C's work mid-edit, and two later edits of its own remained
  uncommitted. Four agents sharing one worktree needs per-path staging, not a
  blanket add. Relates to the standing "stop two sessions editing the same
  worktree" item.

- **The cross-dialect cache claim was wrong, then right, then explained.** The
  original n=1 observation said the cache crossed dialects. A7, better
  controlled, said it did not. Rather than prefer the newer run, the original
  sequence was replayed byte-for-byte and the raw request artifacts compared:
  the original bodies carried no `reasoning_effort` and the replay did. A9
  tested that directly and both observations are correct. The lesson is that
  preferring the better-controlled experiment would have produced a true
  conclusion by luck and lost the actual mechanism. DS-A9.
- **A6's parameter-invariance result does not cover `reasoning_effort`.** All six
  A6 arms were the same task class, so the parameter was constant across them.
  A10 tested it and it is the one request field found to participate in cache
  identity. An ignored junk key does not, which rules out the cache key being
  the raw request body. DS-A10.
- **I corrupted another agent's report with a careless regex.** A substitution
  intended to fix one Markdown code span matched across adjacent spans and
  damaged 70 sites in an uncommitted file. It was exactly invertible and was
  reversed, then the single genuine case was fixed surgically. Sweeping regexes
  over prose are not safe edits; the file had no commit to fall back to.
