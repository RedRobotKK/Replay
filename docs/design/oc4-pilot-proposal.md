# OC-4 headroom pilot: a proposal, not authorized

Written 2026-10-23. Documentation only. Nothing here has been run, no money has been spent, and running any part of it needs the owner's explicit authorization of the spend.

## Decision requested

Whether to authorize a capped control-only pilot that establishes, on fresh runs, whether the pinned task (`count-b-tests-v1`) sits below ceiling under the scorer `replay outcome` ships, and how much of any shortfall is answer format rather than a wrong count. The proposed ceiling is **$10.00 at list**, against a projection of **$5.04**. No treatment is run in this pilot.

**Recommendation: proceed to the owner's review; do not spend until a treatment is defined that is not a formatting instruction.** The reason is in [what the baseline shows](#what-the-corrected-baseline-shows).

## Corrected baseline

Source: `internal/outputdiscipline/testdata/n10.json`, 40 runs, pinned by SHA-256 `ba635faf…6041e02e` (`TestFixtureIdentityIsPinned`). It is the run set behind the n=10 table of [the 2026-09-25 evidence](../evidence/output-discipline-2026-09-25.md). One machine, one operator, build `v0.6.2-7-g6871022`.

Each reply is read by `Run.Answer`, the function `outcome.Score` reads a reply with: the reply's leading integer, after any whitespace. These historical runs left no `run.json` or `tree/`, so this is the answer half of `Score` only; nothing else `Score` checks could be checked.

| cell | n | correct under the rule | states 60 (numeric) | rule can read an integer (format) |
|---|---|---|---|---|
| haiku, bounded | 10 | 10 | 10 | 10 |
| haiku, verbose | 10 | **5** | 6 | 9 |
| sonnet, bounded | 10 | 10 | 10 | 10 |
| sonnet, verbose | 10 | **6** | 10 | 6 |

The ten replies the rule does not score as 60, each read in full:

| run | reply | class |
|---|---|---|
| `n10/verbose_1`, `_6`, `_7`, `_10` (haiku verbose) | `61` | wrong answer: an off-by-one count |
| `n10/verbose_5` (haiku verbose) | a per-file breakdown ending `**60**` | right count, unreadable format |
| `n10/sonnet_verbose_1`, `_8` | `**60**` | right count, unreadable format |
| `n10/sonnet_verbose_3`, `_9` | `**60**` followed by a note that a Monitor tool call needed approval | right count, unreadable format, and the run's environment leaked into the reply |

`TestN10UnderTheShippedAnswerRule` recomputes every figure in both tables from the fixture and pins each of the ten by run name; it fails if any count or classification moves.

**What was published before.** Requirement OC-4 and UNWIRED-LOG #19 named "haiku, verbose, 6 of 10" as the one cell below ceiling. That is the numeric column, given under the rule's name. Under the rule, haiku verbose is 5 of 10 and sonnet verbose, numerically perfect, is 6 of 10. Both documents are corrected as of 2026-10-23, with the old wording quoted.

### What the corrected baseline shows

- Two cells are below ceiling under the shipped rule, and they are below it for different reasons. Haiku verbose has 4 wrong counts and 1 format miss. Sonnet verbose has 0 wrong counts and 4 format misses.
- A treatment that only told the agent to reply with a bare number could take sonnet verbose from 6 to 10 without changing a single count. Under the rule that reads as an improvement. It is not evidence of better reasoning, and this proposal treats it as a formatting result.
- Two of the four sonnet misses carry text about a tool approval in the operator's environment. That is contamination, and it should be excluded from the session, not from the count.

## The question the pilot answers

On fresh runs, under the shipped rule, is each control cell below ceiling, and how many misses are wrong counts versus unreadable formats?

That is what OC-4 requires before any comparison is reported. It is not a test of any treatment.

## Proposed design

| | Proposed |
|---|---|
| Task | `count-b-tests-v1`, the embedded manifest, unchanged |
| Substrate | a clean checkout of commit `6871022`, verified by tree id `188abd59` before each run |
| Arm | control only: the verbose configuration of 2026-09-25, the arm that showed headroom. The bounded arm is not run |
| Models | haiku and sonnet, the two with n=10 evidence |
| Runs | 10 per model, which is above the predeclared minimum of 5 eligible runs an arm |
| Recording | each run recorded with `replay outcome-record`, exit code from the shell, usage from the run's own report |
| Scoring | `replay outcome` per run, then the three measures above |
| Contamination | each session run with slash commands disabled and no Monitor tool, and any reply carrying environment text recorded as contaminated, not dropped |

No treatment arm is defined here. A treatment is proposed only after this pilot, and only one whose mechanism is not answer formatting, or one whose analysis reports numeric correctness separately and requires it not to fall.

## Predeclared criteria

- **Headroom shown** for a cell: at most 8 of 10 correct under the rule, with at least 1 wrong count. A cell short of ceiling by format misses alone shows formatting headroom, and is reported as that.
- **At ceiling**: 10 of 10 under the rule. That cell is not used as a control for any later comparison.
- **Stop**:
  - stop at the cap, even if runs remain;
  - stop at the first run whose substrate tree id is not `188abd59`;
  - stop if more than 2 runs in a cell are contaminated.
- **Reported regardless of result**: every run, its verdict and reasons, the three measures, the cost each run reported, and every exclusion with its reason.

## Uncertainty

Ten runs a cell cannot estimate a rate tightly. If 5 of 10 are observed, the true rate is consistent with anything from about 20% to 80% (the exact binomial 95% interval is 19% to 81%). The pilot can show that a cell is below ceiling. It cannot measure by how much, and it cannot carry to another model, task or machine. Model versions have also moved since 2026-09-25, so the old cells are a reason to look, not a prediction.

## Costs

**Observed historical cost.** Each fixture run carries a `cost` field: the cost the client reported, at list price. It was not reconciled against a bill.

| cell | runs | median cost per run | source |
|---|---|---|---|
| haiku, verbose | 10 | $0.1344 | `n10.json`, published in the n=10 table |
| sonnet, verbose | 10 | $0.3699 | the same |
| haiku, bounded | 10 | $0.0438 | the same |
| sonnet, bounded | 10 | $0.1734 | the same |

The two run sets total $9.4728 (18 runs, the figure the evidence document states) and $7.1981 (40 runs).

**Calculated from recorded usage: not possible.** The fixtures record input, cache-read and cache-write tokens. They do not record output tokens or the cache-write TTL, so no run's cost can be recomputed from its usage. The figures above are the client's own and stand on that alone.

**Projected pilot cost (an assumption, not authorized):**

| item | arithmetic | projection |
|---|---|---|
| haiku control runs | 10 × $0.1344 | $1.344 |
| sonnet control runs | 10 × $0.3699 | $3.699 |
| **total** | | **$5.043** |

It assumes list prices and per-run usage like 2026-09-25, and neither is verified for today. The proposed hard cap is $10.00, about twice the projection, to absorb drift. Spending stops at the cap.

## Risks and other explanations

- Answer format dominates sonnet's shortfall. A pilot read only under the rule would mistake formatting for capability.
- Environment text in replies, as in two sonnet runs, is operator configuration, not the model.
- Haiku's off-by-one count may depend on the model version, which has changed since September.
- One machine and one operator, as before.
- The arm label in `run.json` is operator-attested. The pilot's arm assignment rests on the operator.

## Reproduce the corrected baseline

```sh
go test ./internal/outputdiscipline -run 'TestN10UnderTheShippedAnswerRule|TestTheAnswerRuleAtItsEdges|TestThePublishedHeadroomFigureNamesItsMeasure' -count=1 -v
```

It reads only the pinned fixture and two documents, and reaches no network.
