# DeepSeek full experimental suite

One bounded, preregistered campaign. Evidence classes are kept apart everywhere:
**OBSERVED** (in a provider response, a balance response, or a local artifact),
**DERIVED** (arithmetic over observed values), **ASSUMED** (a premise this
campaign has not established), **NOT_OBSERVED** (the evidence was not captured),
**INVALID** (the instrumentation was compromised).

Nothing is promoted between classes silently. NOT_OBSERVED is never FALSE.

## Layout

| path | holds |
| --- | --- |
| `FINAL-REPORT.md` | **the consolidated final report. Start here.** |
| `ledger.json` | machine-readable claim ledger, every claim with its evidence class |
| `PLAN.md` | preregistration, written before execution |
| `LEDGER.md` / `ledger.json` | every experiment, human and machine readable |
| `claims.md` | the claims matrix; evidence-bound statements only |
| `mechanisms.md` | observation -> candidate mechanism -> discriminating experiment |
| `pricing.md` | the exact rate table and its provenance |
| `raw/` | primary evidence, unmodified |
| `derived/` | reconstructions computed from `raw/` |
| `scripts/` | deterministic, re-runnable analysis |
| `results/` | per-experiment records |
| `invalid/` | experiments whose instrumentation failed, PRESERVED not deleted |

## Frozen state at campaign start

| | |
| --- | --- |
| timestamp | `2026-09-29T05:21:53Z` |
| git commit | `30dc687221ae876631595de80165131d121e1c46` |
| balance (OBSERVED) | `$45.78` |
| saved raw responses | 479 |
| pricing table | published 2026-09-28, see `pricing.md` |
| models in corpus | `deepseek-flash`, `deepseek-v4-pro` |
| endpoints in corpus | `/v1/chat/completions`, `/anthropic/v1/messages` |

## Budget

Ceiling $3.00 for the campaign, set by the operator and enforced by
`fanout.Budget` in integer nano-USD. Spent before this suite: **$0.23 OBSERVED**.

## Where the primary evidence lives

`raw/usage-extract.json` is the primary evidence for every derived cost figure.
It holds the provider-reported `usage` block, model and `created` timestamp of
all 575 saved responses, copied field for field.

The full 43 MB corpus of request/response pairs sits outside this repository in
an ephemeral job directory and will not survive it. The extract carries a sha256
over the concatenated source responses, and reconstructing from the extract
reproduces the full-corpus figure to nine decimal places.

Run it: `python3 experiment/harness/reconstruct.py --dir <corpus>`. It makes no
API call.
