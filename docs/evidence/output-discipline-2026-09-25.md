# Bounding tool output cut cache writes by 57% to 67%, and the figure differs by model

**Measured 2026-09-25.** Build `v0.6.2-7-g6871022`, four models, 18 runs,
$9.4728 total at list. One machine, one operator.

`replay context` reads tool output as the largest single consumer of a session's
context: 53.4% and 533k tokens over 1,278 calls in `eec05948`, 45.7% and 440k
over 760 calls in the session that ran this experiment. That identifies a target.
It does not say what bounding it is worth, and nothing in this repository did.

This is a controlled A/B on that lever, with the work held constant.

## Why this design rather than a before and after

The [dogfood baseline](dogfood-baseline-2026-09-25.md) exists because a forward
comparison of totals measures the calendar: next month is different work. Here the
task, the model, the tool set and the repository state are identical across arms,
and **only the output discipline varies**, so the difference is not corpus drift.

It also succeeds where the [seeded intervention
pilot](seeded-intervention-prereg-2026-09-25.md) failed. There the defect could
not be caused on demand. Output discipline is entirely under the operator's
control, so the treatment is real.

## Task

Identical in every arm and every model:

> How many Go test functions (lines starting with `func Test`) are defined across
> the files matching `cmd/replay/b*_test.go`? Reply with only the number.

Scope: 15 files, 95,753 bytes, roughly 24k tokens read whole.

**Ground truth is 60**, established three independent ways before any run, with a
check confirming no `func Test` occurs off a line start that could justify 61.

| Arm | Instruction appended |
|---|---|
| VERBOSE | read the contents of each matching file in full, no counting or filtering shortcuts |
| BOUNDED | counting and filtering commands only, never print more than 50 lines, never read a whole file when a count answers it |

## Result

Medians. Answers are every replicate, not a summary.

### Which runs these figures come from

**Added 2026-09-26. No published figure changed.** The tables below did not say
which runs produced them, and the corpus holding those runs holds nine
experiment directories. Sweeping all of them gives 54,418 cache-write for haiku
verbose where the first table says 53,700, and 24,475 for opus bounded where it
says 35,289. A reader checking this evidence would have got different numbers
and reasonably concluded it was wrong.

Two run sets back this document and they are **not** interchangeable:

| table | run set | runs | sha256 of the fixture |
|---|---|---:|---|
| the medians below | `bench/` | 18 | `00896480a43e2c79eb9b6583a47e2273d8b1a50f041aecbea16915877ac71a96` |
| the n=10 replication | `n10/` | 40 | `ba635faf1104adc7b51f915355f272b4a7cae7f23f903581d4d7d2bc6041e02e` |

Ten filenames appear in both directories and are different runs, so the
committed fixtures qualify every run name with its set.

`internal/outputdiscipline` re-derives both tables from those fixtures and is
part of `make ci`:

```sh
go test ./internal/outputdiscipline/
```

**What the harness discloses that the tables do not.** Six of the 58 runs do not
hold a bare number in their result: one appends a note from an agmsg watcher
that failed to start, three wrap the answer in markdown bold, and one answers in
prose. **None is excluded from any published figure**, because the published
tables included them. Only the first table publishes answers, and every result
in `bench/` begins with its number, so no heuristic is applied to the other
forms.

**On rounding.** Published means are rounded rather than truncated: the haiku
verbose mean is 54,412.60 and prints as 54,413. One figure, the haiku bounded
mean, lands exactly on 18,115.5, and half-to-even and half-up both give the
published 18,116, so **this document does not settle which rule was used.** The
harness uses half-to-even because Go's `%.0f` does, and records that the choice
is unconstrained by this evidence.

The raw run artifacts stay outside this repository at
`~/Development/replay-bench-2026-09-25/`. The committed fixtures carry only the
fields the tables use.

| model | arm | n | prompt tokens | cache write | cost | turns | answers |
|---|---|---|---|---|---|---|---|
| haiku | verbose | 3 | 144,256 | 53,700 | $0.1352 | 17 | **61, 61, 61** |
| haiku | bounded | 3 | 72,307 | 18,545 | $0.0467 | 2 | 60, 60, 60 |
| sonnet | verbose | 2 | 218,399 | 81,282 | $0.3896 | 18 | 60, 60 |
| sonnet | bounded | 2 | 143,135 | 32,818 | $0.1570 | 3 | 60, 60 |
| opus | verbose | 2 | 433,625 | 83,301 | $1.0942 | 19 | 60, 60 |
| opus | bounded | 2 | 143,752 | 35,289 | $0.4241 | 4 | 60, 60 |
| fable | verbose | 2 | 185,642 | 79,139 | $1.7195 | 17 | 60, 60 |
| fable | bounded | 2 | 92,685 | 32,048 | $0.6748 | 2 | 60, 60 |

| model | tokens saved | cost saved |
|---|---|---|
| haiku | 49.9% | **65.4%** |
| sonnet | 34.5% | **59.7%** |
| opus | 66.8% | **61.2%** |
| fable | 50.1% | **60.8%** |

## The definitive reading: n=10 per arm, two models

Everything above is a median of two or three runs. This is ten.

| model | arm | n | cache-write mean | sd | cv | cost median | turns |
|---|---|---|---|---|---|---|---|
| haiku | verbose | 10 | 54,413 | 445 | 0.8% | $0.1344 | 17 |
| haiku | bounded | 10 | 18,116 | 505 | 2.8% | $0.0438 | 2 |
| sonnet | verbose | 10 | 77,901 | 1,786 | 2.3% | $0.3699 | 17 |
| sonnet | bounded | 10 | 33,878 | 1,856 | 5.5% | $0.1734 | 4 |

| model | cache-write saving, 95% CI | cost saving |
|---|---|---|
| haiku | **66.7%** [66.1%, 67.3%] | 67.4% |
| sonnet | **56.5%** [54.9%, 58.1%] | 53.1% |

### This corrects an earlier claim in this file

The small-n rounds reported cost saving as a tight band of 59.7% to 66.5% across
four models, and that band was called the strongest result here.

**At n=10 the two intervals do not overlap**, 66.1-67.3 against 54.9-58.1, and
sonnet's cost saving of 53.1% falls below the floor of the band that was
supposed to contain it.

So the tight band was an artifact of two and three run samples. **The saving is
real on both models and it is not one number.** It differs by model by about ten
points, and any single figure quoted across models is wrong.

The cache-write metric held up exactly as chosen: coefficient of variation 0.8%
to 5.5% across all four cells.

### Correctness at n=10

| model | arm | answers |
|---|---|---|
| haiku | verbose | **61 x4, 60 x6** |
| haiku | bounded | 60 x10 |
| sonnet | verbose | 60 x10 |
| sonnet | bounded | 60 x10 |

**This also corrects an earlier reading.** At n=3 and n=5 haiku's verbose arm
answered 61 every time and this file called it a reproducible systematic error.
At n=10 it is 4 wrong and 6 right. The early run of identical wrong answers was
luck being read as a mechanism.

The surviving statement is narrower: on the one model where this task sits near
the edge of capability, reading everything produces a wrong answer about 40% of
the time and filtering does not. On sonnet both arms are perfect, so nothing
here is a general accuracy claim.

## Replication, round two, independent runs

The design was re-run from scratch on haiku and sonnet.

| model | round | tokens saved | cost saved |
|---|---|---|---|
| haiku | R1 | 49.9% | 65.4% |
| haiku | R2 | 50.3% | 66.5% |
| sonnet | R1 | 34.5% | 59.7% |
| sonnet | R2 | 48.4% | 61.8% |

**Cost replicates tightly. Tokens do not.** Cost saved now spans 59.7% to 66.5%
across four models and two rounds, a 6.8 point band. Token saved spans 34.5% to
66.8%, and sonnet alone moved 14 points between rounds on an identical design.

**So the cost figure carries the claim and the token figure must not be quoted as
a headline.** The reason is mechanical: cost tracks cache writes, which are
stable, while total prompt tokens are dominated by cache reads, which are cheap
and vary with whatever the model happens to do.

A confound predicted before R2 did not appear. Warmer caches might have made the
second round cheaper in absolute terms; they did not. Haiku verbose went $0.1352
to $0.1358 and sonnet verbose $0.3896 to $0.3764, so R2 is a clean replication
rather than a warmed-up one.

**Haiku's off-by-one replicated.** R2 verbose answered 61 again. It is a
reproducible systematic error on the smallest model, not noise. Sonnet answered
60 in both arms in both rounds, so the general accuracy claim stays withdrawn.

## Metric stability, and which number to quote

Drift between the two independent rounds, haiku:

| metric | verbose | bounded |
|---|---|---|
| prompt tokens, including reads | 1.3% | 0.4% |
| **cache WRITE tokens** | 2.0% | **0.2%** |
| cost USD | 0.4% | 2.6% |
| turns | 0.0% | 0.0% |

`cache_creation_input_tokens` is the cleanest quantity, because it excludes cache
reads entirely and reads are where the variance lives: one sonnet run returned
533,992 reads against roughly 71,000 on a byte-identical one, a 7.4x swing.

Turns show 0.0% drift but cannot resolve a change smaller than one whole turn, so
they confirm rather than carry the result.

## What holds

**Cost fell by 59.7% to 61.2% on three models and 65.4% on the fourth.** That is a
5.7 point spread across four models of very different size and price, which is a
tighter result than anything else here.

**The turn count collapsed everywhere**: 17 to 19 turns verbose, 2 to 4 bounded.
Reading files whole costs roughly six times the round trips regardless of model.

**The saving lives in cache writes.** Verbose wrote 53,700 to 83,301 tokens per
run; bounded wrote 18,545 to 35,289. Cost tracks the write column, not the total
token column, which is why the cost band is tight while the token band is not.

## What does not hold, and was claimed before this run

An earlier reading of the haiku data alone concluded that **the cheap arm was also
the correct one**. That is withdrawn.

Haiku answered **61 on all three verbose runs** against a ground truth of 60, a
consistent off-by-one from counting by hand across fifteen files. **Sonnet, opus
and fable all answered 60 in both arms.**

The correctness gap is a haiku artifact, not a property of output discipline. The
prediction that a larger model would count by hand more reliably was written into
this file's limitations before the larger models were run, and it was correct.

**No claim survives that bounding output improves accuracy.**

## Token saving is not monotonic in model size

49.9%, 34.5%, 66.8%, 50.1% for haiku, sonnet, opus, fable. Sonnet saved least and
opus most, with fable between them. Nothing here explains that ordering, and n=2
per cell on three of the four models is too thin to try. **It is reported as
observed and not modelled.**

## What this establishes

On this task shape, across four models: bounding tool output cut cost by roughly
60%, cut turns by about 6x, and cut prompt tokens by between a third and two
thirds.

## The counterweight: a comprehension task, where the bounded arm loses

Every task above is counting, which rewards `grep -c` so heavily that the
bounded arm could barely lose. This file has said repeatedly that **no measured
case exists where the verbose arm wins**, and that a lever with no observed
downside usually means the downside was not looked for. This is the search.

The question asks *why* rather than *where*, so knowing what to filter for is
itself the hard part:

> Reading only the files matching `cmd/replay/b*_test.go`, explain in one
> sentence why the burn command reports two separate counts for Ollama cache
> reuse instead of a single average hit rate.

The true answer is a selection argument, stated in `burnollamashare_test.go:15`
and again in `burn.go:470`: `n_past` appears **only** on requests that were
already fully cached, so a rate computed over that group measures a population
selected for having been cached, and is circular.

| arm | n | cache-write mean | cost median | turns | names the selection argument |
|---|---|---|---|---|---|
| VERBOSE | 5 | 48,615 | $0.1279 | 17 | **3 of 5** |
| BOUNDED | 5 | 21,893 | $0.0712 | 6 | **0 of 5** |

The bounded arm still saved 55% of cache writes and 44% of cost. **It also
missed the point every time.**

### What the answers actually differ on, read in full

The keyword scoring above is too crude on its own, and reading the responses
shows a narrower and more interesting split than a clean sweep.

Verbose, when it succeeds, names the selection: "`n_past` appears only on
requests whose prompt was already resident, creating two distinct populations."

Bounded consistently reframes the problem as **sparseness** rather than
selection: "combining them into a single average would misrepresent sparse cache
data", and "reporting only an average would hide whether the logging is
complete." Those are not nonsense. They identify that something is missing. They
do not identify that what remains is defined by the property being measured,
which is the whole reason the average is refused.

So the honest statement is not that bounded produced garbage. It is that
**bounded described the symptom and verbose reached the cause**, and only on a
question where the cause had to be understood rather than located.

### Why the bounded arm missed, traced through the transcripts

The first explanation written here was that the bounded arm never reached the
source. **That is wrong, and the transcripts say so.**

**The greps searched the question's vocabulary, not the answer's.** Thirty-eight
patterns across five runs: `two.*count`, `separate.*count`, `hit rate`,
`measured|inferred`, `observed|inferred`, and one run guessing outright with
`cannot tell the two counters|two counters cannot be the same variable`. The
answer is written as "a population selected for having been cached" and "`n_past`
appears only when the whole prompt was already resident". **No pattern targeted
"population", "selected", "resident" or "n_past".**

That is the mechanism. Grep needs you to already know roughly how the answer is
phrased. On a "where is X" question the string is in hand; on a "why is X"
question the answer's wording is precisely what is missing.

**Two of five runs did read the file in full and missed anyway.** bounded_3 and
bounded_5 both opened `burnollamashare_test.go`. bounded_3's answer cites "the
test comments at lines 97-105 and 123"; the argument is at line 15. It arrived
carrying the frame its failed greps had built, and read for confirmation of that
frame rather than meeting the argument.

So the filtering did not merely fail to locate the answer. **It pre-committed
the model to the wrong question.**

### The cost advantage disappears when the search fails

| run | cache-write | cost | turns | sec |
|---|---|---|---|---|
| bounded_3 | 26,681 | **$0.1314** | 16 | 35.2 |
| verbose median | 53,883 | $0.1279 | 17 | 24.2 |
| verbose_3 | 23,079 | **$0.0624** | 5 | 20.2 |

bounded_3 grepped `inferred` five separate times, ran thirteen tool calls, read
the whole file anyway, and **cost the same as the verbose median while still
getting it wrong.**

The cheapest run in the entire set was verbose_3, which was also correct.

**So the 44% saving is an average over runs where the guess landed.** When it
does not land, the bounded arm pays full price for a wrong answer, and the
median hides that.

### What this does to the headline

The saving is real and it is not free. On counting work the bounded arm was
equal or better on correctness. On a comprehension question it was cheaper and
wrong.

**Any recommendation to bound tool output has to carry that distinction**, and
until now this file could not state it because the losing case had not been
found.

n=5 per arm, one model, one question. This is the smallest possible evidence for
a real effect, and it is reported as a floor rather than a measurement.

## Search churn: a signal, pre-registered before its replication

Tracing the transcripts produced a candidate discriminator. **Distinct search
patterns issued with no intervening file read:**

| arm | outcome | distinct patterns per run | median |
|---|---|---|---|
| counting, bounded | won 10/10 | 1,1,1,1,1,0,1,1,1,1 | 1 |
| comprehension, bounded | lost 0/5 | 5,5,12,2,5 | 5 |
| comprehension, verbose | won 3/5 | 1,1,1,1,1 plus 15 reads | 1 |

Perfect separation at n=15. One pattern means the string was known; five means
the wording was being guessed at.

### The rule, stated before the test that could break it

> **Two or more distinct search patterns with no intervening file read means the
> filter is guessing. Unbind and read.**

The signal is available at the SECOND failed search, before any answer exists.
That is the attach point.

**Timing is the whole rule.** bounded_3 did read the file, after twelve distinct
patterns, and still answered wrong: it cited lines 97-105 when the argument is at
line 15, because the failed searches had already built the frame it read
through. Reading late cost full price and missed anyway.

### Pre-registered predictions for the replication

Fifteen runs on a SECOND model, `claude-sonnet-5`, five per cell, written down
before any of them ran:

1. **Counting, bounded:** median distinct patterns is 1, and correctness stays at
   or near 5 of 5.
2. **Comprehension, bounded:** median distinct patterns is 2 or more, and the
   selection argument is reached in 1 of 5 or fewer.
3. **Comprehension, verbose:** median distinct patterns is 1, reads are high, and
   the selection argument is reached more often than the bounded arm.

**What would falsify the signal:** any overlap between the counting-bounded and
comprehension-bounded pattern counts, or a comprehension-bounded run that reaches
the selection argument on one pattern. Either means churn is not the
discriminator and the threshold of 2 was fitted to fifteen observations.

The threshold of 2 rests on a single boundary observation, bounded_4, which
issued exactly two and lost. That is the weakest number in this file.

**Result: pending.**

## What it does not establish

- **One task shape.** Counting across files rewards filtering unusually well. A
  task needing genuine comprehension of file contents would not behave this way,
  and there is currently no case measured where the verbose arm wins.
- **n = 3 on haiku, n = 2 elsewhere.** The batch was killed for host memory
  partway through, sonnet lost its third replicate, and one truncated file was
  discarded rather than parsed. Opus and fable were re-run at n=2 for consistency.
- **One machine, one operator, one repository.**
- **The instruction differs between arms**, which is the treatment but also a small
  prefix difference. It is far too small to explain the gaps above.
- **This is not a cache-break measurement.** It measures context volume, a
  different lever. The 3.768% re-billed figure in the baseline is untouched by it,
  and nothing here transfers to the advisor's cache-break findings.
- **It says nothing about compaction.** `replay trim` states that trimming does not
  delay auto-compaction, because `count_tokens` is untrimmed, and bounding output
  at the source does not change that.
- Runs after the first six carry a small amount of extra output text from an
  unrelated agmsg monitor warning in the spawned subprocesses. It lands in the
  output-token column, not the prompt-token column where the effect lives.

## The honest headline

> On one counting task across four models, bounding tool output cut cost by 59.7%
> to 65.4% and turns by about 6x.

Not: Replay reduces your bill by 60%. And not, as an earlier version of this file
said, that the cheaper arm is the more accurate one.

## Cold prefix: the design failed a second time

**Measured 2026-09-26 02:43 local, after five attempts.** Three earlier attempts
were killed by host memory pressure before producing a run; a fourth produced
runs whose cold arm was 51-57% cache reads and was recorded as a failure.

The guard this time passed on its own terms: no `claude -p` process anywhere,
and nothing under the scratch tree touched in 60 minutes.

| arm | cache writes | cache reads | reads as share of cached |
|---|---|---|---|
| cold | 22,561 | 49,875 | **68.9%** |
| warm | 17,949 | 55,032 | 75.4% |

**A genuinely cold run is overwhelmingly writes.** This one is 68.9% reads,
worse than the 51-57% that failed the first attempt. The rule was fixed before
the run: if the cold arm is majority reads, the design failed and no cold/warm
ratio may be reported as a finding. **No ratio is reported.**

For reference only, and not as a comparison the above earns: the warm baseline
from haiku bounded n=10 is writes 18,116 (sd 505), reads 65,899. The "warm" arm
here sits near it. The "cold" arm does not sit anywhere near a cold prefix.

### What five attempts establish

**The cold-prefix cost cannot be measured with this instrument**, and the
obstacle is not scheduling. Waiting out the confirmed one-hour TTL with the
machine verifiably quiet still produced a prefix that was two thirds resident.

Something outside the scratch tree keeps it warm, and the guard cannot see it.
That is the limit of what this harness can establish, and it is recorded as a
measurement failure rather than left open as a pending task.

**This does not change any figure elsewhere in this file.** Every saving here is
measured warm-to-warm within an arm pair, so a cold baseline was never load
bearing for them. It would have priced what a break costs from cold, and that
question remains **NOT MEASURED**.
