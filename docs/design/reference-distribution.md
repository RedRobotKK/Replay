# A reference distribution, so one machine can be read against many

**What this is:** a design for carrying published population figures the way
`rules.json` carries published provider figures, so that Replay can say where a
reader sits without a corpus, without contributions, and without waiting for a
pool that has one member.

**Status:** design, not built.

## The problem this solves, stated exactly

Every number Replay prints is about one machine. `internal/analysis/outlier.go`
opens by saying why:

> A bare figure is not actionable. "This session cost $3.40" leaves the reader
> asking whether that is high, and neither they nor this tool can answer from a
> population — the pooled corpus has one member, and publishing a population
> figure derived from one machine is the shape of claim this project has
> already retracted twice.

That is correct and it is the whole constraint. `--contribute` exists to lift
it, ADR-0009 promises the payoff — "your error share is 22%, the median is 4%"
— and the pool still has one member, so the promise is unredeemed. The corpus
grows at the speed of pull requests.

There is a second source of population figures, and it does not require anyone
to contribute anything: **published measurements of the same workload, by other
people, at scales this project will never reach.** Two landed in 2026.

| paper | population | what it measures |
|---|---|---|
| [TraceLab, arXiv:2606.30560](https://arxiv.org/abs/2606.30560) | 4,265 sessions, 43 developers, Claude Code and Codex | prefix cache hit rate, prefill amplification, prefix share of cost |
| [Agentic Coding in the Wild, arXiv:2608.00101](https://arxiv.org/abs/2608.00101) | 13.5M sessions, 760.5M LLM calls, 95T tokens | cache hit rate within and across turns, idle-gap decay, prompt composition |

Measured against this repository's own development corpus, they already say
something no Replay surface can say today:

| metric | this machine | published |
|---|---|---|
| cached share of prompt | 99% | 95.7% (TraceLab) · 98% within-turn (Copilot) |
| system prompt share of input | 28.8% | 14% (Copilot) |
| conversation history share | 6.0% | 48% (Copilot) |
| tool and function traffic | ~32% | 28% (Copilot) |

The system-prompt row is a factor of two and the history row is a factor of
eight. Neither is a defect. They say this operator runs a far more tool-heavy
and far less conversational workload than the population, which is a *profile*
— and a profile is the thing a reader cannot obtain from their own machine by
any amount of measurement.

## The shape, and why it is the shape it is

A reference document is `rules.json` with the provider swapped for a
publication. That is not an analogy reached for after the fact; the two
documents have the same job. Both carry numbers somebody else published, both
are dated, both are worthless without provenance, both must be refusable, and
both change faster than release cycles.

So it reuses, rather than parallels:

- **The document envelope.** `Schema`, `Version`, `Source`, `FetchedAt`,
  `CheckedAt` (`internal/cachemodel/rules.go:26-46`). A reference set is
  installed by `replay reference --update <file|https URL>`, validated through
  the same loader every run uses, written to `~/.replay/reference.json` at
  0600, loaded once at startup. There is no background refresh and no
  check-for-updates, for the reason `rules.go:143-146` already gives.
- **The compiled-in fallback.** A machine that has never installed one behaves
  exactly as before, and the compiled set is a floor rather than a stale
  document. `RulesBuiltIn` versus `RulesFetched` versus `RulesUndated`
  (`internal/tui/measured.go`) is the three-value distinction this needs too.
- **The staleness notice**, with a different threshold and a different reason —
  see "How a population goes stale" below.
- **The store registry.** A new file under `~/.replay` must be registered in
  `cmd/replay/stores.go` or `replay privacy` will not disclose it. This is not
  optional and it is the kind of thing that gets forgotten.

## The comparison is derived, never declared

This is the load-bearing rule and it is taken directly from
`internal/cachemodel/claim.go`, which already solved the harder version of this
problem for provider figures:

> The verdict is computed from the two. It is never written into the file: a
> hand-written "consistent" is another claim wearing a verdict's clothes, and
> the whole point is to have one field in this system that nobody can simply
> assert.

`Claim` carries `DeclaredStatus` **solely so that `validate()` can refuse a
document that sets it**. A reference set does the same. The file carries the
published figure and the population it was measured over. It carries no verdict,
no band, no "typical range", no "good" and no "high". Those are computed here,
from the local measurement, or they are not stated.

```go
// Reference is one published figure, and the population it describes.
type Reference struct {
    Metric     string  `json:"metric"`     // "cachedShare", "systemPromptShare"
    Value      float64 `json:"value"`
    Unit       string  `json:"unit"`       // "share", "ratio", "tokens"
    // Population is what was measured, in the publication's own terms. It is
    // mandatory. A figure without one is a number with no referent, which is
    // the shape of claim this project has retracted twice.
    Population string  `json:"population"` // "13.5M sessions, 760.5M LLM calls"
    Citation   string  `json:"citation"`   // "arXiv:2608.00101"
    MeasuredAt string  `json:"measuredAt"` // YYYY-MM, when the DATA was collected
    // Spread, when the publication reports one. Absent is absent: a figure
    // with no reported dispersion must not be rendered as a point estimate
    // with an implied tight band.
    P10 *float64 `json:"p10,omitempty"`
    P90 *float64 `json:"p90,omitempty"`
    // DeclaredVerdict exists only to be refused. See claim.go.
    DeclaredVerdict string `json:"verdict,omitempty"`
}
```

### The asymmetry, restated for populations

`Claim.Status()` encodes that falsification is asymmetric: one machine seeing a
prompt cached below the published minimum refutes the figure, and no sample size
is needed; one machine agreeing proves very little.

A population comparison is asymmetric in a **different** direction, and getting
this backwards is the failure mode:

- A provider's published minimum is a claim about the world, and one
  counterexample refutes it.
- A population's published median is a claim about *a population*, and one
  machine differing from it **refutes nothing at all.** It is one draw.

So the derived verdict has a deliberately narrow vocabulary. Not "high", not
"above average", not "worse":

| verdict | when |
|---|---|
| `unmeasured` | the local figure is not computed on this machine |
| `no-reference` | nothing published covers this metric |
| `within` | the local figure falls inside the published spread, where a spread was published |
| `outside` | it falls outside a published spread |
| `differs` | a spread was not published, so only the direction and magnitude can be stated |

`outside` and `differs` are **descriptions of a difference, not judgements of
it.** The rendering rule follows: every comparison prints the population
definition beside the number, always, in the same line. "28.8% here, 14% across
13.5M Copilot sessions" is a sentence a reader can weigh. "28.8%, roughly double
the average" is not, and is the sentence this design exists to make unwriteable.

## How a population goes stale, which is not how a price goes stale

A price is wrong the moment the provider changes it, which is why
`rulesStaleAfter` is thirty days.

A published population is never *wrong*. arXiv:2608.00101 measured 13.5M Copilot
sessions in June 2026 and will describe them forever. What decays is its
**applicability**: agent harnesses change, model context windows change, the
median session in 2027 is not the median session in 2026.

Two consequences.

`MeasuredAt` is the date of the **data collection**, not of the fetch or the
publication, and it is what ages. A paper published in August 2026 about traces
from June 2026 ages from June.

The notice says what is uncertain, not that the number is wrong:

```text
reference    copilot-2026-06, measured 14 months ago
             agent harnesses change; a comparison against this population is
             weaker than it was, and nothing here says by how much
```

The threshold should be longer than thirty days and should be argued, not
inherited. Twelve months is a starting proposal and is a judgement, which
ADR-0009 says every threshold in this tool currently is.

## Population definition is part of the datum

The single largest risk in this design is that a reader takes "14%" as *the*
system-prompt share and concludes their 28.8% is wrong.

Copilot's 13.5M sessions are **Copilot users on Copilot's harness**, with
Copilot's system prompt and Copilot's tool set. TraceLab is 43 developers.
Neither is a sample of "people who run coding agents"; each is a census of one
product's traffic. A difference between this machine and either is, in the first
instance, a difference in *what the two are doing*.

The document therefore makes `Population` mandatory and `validate()` refuses a
reference without one, exactly as `rules.go` refuses a document without a
version: "a report has to be able to name the rules that produced it".

## Validation

`validate()` refuses, with the reason in the error:

- a missing or empty `Population`, `Citation` or `MeasuredAt`
- any `DeclaredVerdict`, borrowing claim.go's refusal verbatim in spirit
- a `Value` outside its unit's domain: a share outside 0..1, a negative ratio
- `P10 > P90`
- a `MeasuredAt` in the future, which is the same defect the build-staleness
  work found: a date ahead of this clock must not read as freshest
- two references for the same `Metric` from the same `Citation`, which would
  make the comparison depend on file order — the rule `rules.go` already
  applies to overlapping dated model rows

## Where it surfaces

Nowhere by default, on first run. This is a comparison, and
`internal/analysis/outlier.go` is explicit that a comparison printed on every
run is one the reader learns to skip.

- **`replay context`** is the strongest fit and should be first. It already
  ranks what entered the context; a `published` column beside `share` is one
  column and it is the metric with the largest measured gap on the corpus
  tested here.
- **`replay reference`** prints the whole set with its citations, the way
  `replay rules` prints the table. This is where a reader goes to see what they
  are being compared against, and it must exist before any comparison appears
  anywhere else.
- **`replay doctor`** gets one row, dating the set, alongside the rules
  document and the price table. It does not compare anything.
- **The TUI context screen**, after the CLI. Note the row budget: that screen's
  body is already tight and `pad()` truncates the last line silently.
- **`replay advise`** must NOT use a reference figure as a threshold for
  whether to raise a finding. That converts a description into a trigger, and
  ADR-0009's rule about undertested thresholds applies with more force here,
  because the number came from somebody else's workload.

## What ships first

One metric, end to end: `cachedShare`. It is computed on this machine today
(`replay burn` prints it, `internal/analysis` has it), both publications report
it, and the local figure and the published one are close enough that the first
thing a reader sees is not alarming. Getting the plumbing right on a boring
metric is the point.

Then `systemPromptShare` and `historyShare`, which are where the interesting
differences are, and which need `replay context`'s categories mapped onto the
publication's four buckets — a mapping that is itself a judgement and should be
recorded in the document rather than in code.

## What this does not do

It does not replace `--contribute`. A published population answers "how does
this compare to a large sample of somebody else's traffic". Only a pool of
Replay users answers "how does this compare to other people running Replay",
and only that pool can carry the metrics no publication reports — cache-break
causes, avoidable share, the waste taxonomy ADR-0009 is actually about. The
reference set is the thing that works while the pool has one member; it is not
the pool.

It does not make a difference actionable on its own. Knowing that this machine's
system prompt is twice the published share does not say whether that is
avoidable, and `replay advise` remains the surface that has to answer that from
local evidence.

And it introduces a new way to be wrong: a citation that does not say what the
document claims it says. Nothing in the loader can check that. The mitigation is
that every reference carries its citation on screen wherever it is used, so the
claim is falsifiable by a reader who follows it — which is the same standard
docs/evidence/ holds itself to.
