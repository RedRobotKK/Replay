# ledger-bench

Before and after, for each product that meters agent spend.

```sh
scripts/ledger-bench/bench.py                      # the committed fixture
scripts/ledger-bench/bench.py --transcripts ~/.claude/projects
scripts/ledger-bench/bench.py --json
scripts/ledger-bench/run.sh                        # the same, with expectations
```

## What it does

Every tool in this class counts tokens. This runs the same turns through each
product's own accounting rule and then through a per-model, cache-tiered price
table, and prints both columns side by side.

| Product | Before, their own rule | After |
|---|---|---|
| **QM** | `estimateCostUsd(input + cacheRead + cacheWrite)` at a flat $5/MTok, output not counted | per model, cache tiers separated |
| **Memorable** | four token classes, captured correctly, no rate card | the same classes, priced |
| **River AI** | `usage` returned on an OpenAI-compatible endpoint | **NOT PRICED** — no pricing units published |
| **GBrain** | — | **NOT MEASURED** — needs a bearer token |

QM's rule is not a paraphrase. It is `claude-harness.ts:570`, `wiring.ts:1562`,
`budget.ts:15` and `pi-models.ts:541` reproduced exactly, including the decision
not to count output at all.

## On the fixture

```text
6,638,500 prompt tokens, 3 models, 94.8% from cache

  QM         $33.19  ->  $5.72      5.8x over
  Memorable  6,638,500 tokens, no price  ->  $5.72
  River AI   NOT PRICED
  GBrain     NOT MEASURED
```

The fixture is committed and hand-written, so those figures are the same on every
machine. `run.sh` pins them and fails if the scoring moves.

**The ratio tracks cache share, and that is the point.** The fixture caches 94.8%
and scores 5.8×. This machine's real corpus caches 98.7% and scores
[8.1×](../../docs/evidence/qm-budget-2026-09-08.md). A workload that never caches
would score about 1× and the defect would be invisible. The number is a property
of the workload; the mechanism is a property of the code.

## NOT MEASURED is not zero

Two of these four cannot be scored from outside, and neither is reported as
costing nothing. `run.sh` asserts that directly, because the failure that matters
here is not a wrong number — it is an unknown quietly rendered as a zero, which
is the single defect class this repository has found most often.

That assertion has been shown to fail. Turning GBrain's `NOT MEASURED` into
`$0.00` is caught, as is moving the flat rate, as is setting the cache-read
multiplier to 1.0.

## Limits

Anthropic models only, because the cache multipliers here (read 0.10×, write
1.25×) are Anthropic's. Codex nests its cached tokens inside the input figure and
Ollama excludes the cached prefix entirely, so neither can be scored with this
arithmetic — see [SURFACES.md](../../docs/SURFACES.md).

Prices come from `docs/TOKEN-PRICES.md`, dated 2026-09-07 and cross-checked
against OpenRouter and LiteLLM. They go stale. The date is printed so a reader can
tell how stale.
