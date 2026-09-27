# Does acting on a cache-break finding reduce re-billed tokens, and by how much?

**Pre-registered 2026-09-25, before any task was chosen, any break was seeded,
and any run was made. Results are appended below the protocol, never edited into
it.**

This is task #33. It exists because the instrument already in the product
cannot answer the question, and the reason is structural rather than a defect.

> ## Provenance boundary, established 2026-09-27
>
> **Historical preregistration status: NOT VERIFIABLE from surviving evidence.**
> **Current document identity: pinned from 2026-09-27 onward.**
>
> The claim above about when this document was written is preserved exactly as
> it was made. It is **not** corroborated, and this note records why, so that a
> reader meets the gap deliberately rather than by accident.
>
> Three sources were checked and none establishes that the protocol existed in
> frozen form before results:
>
> - **Filesystem times say nothing.** This file's mtime is 2026-09-25 22:05,
>   and fourteen other files share that same minute, including files authored
>   by a different session. That timestamp records when this worktree was
>   materialised, not when text was written.
> - **Git says nothing.** The file has never been tracked, on any branch, so
>   there is no commit history to order.
> - **Session transcripts show repeated modification.** Five operations modified this path between 2026-09-25 12:50 and 2026-09-27 04:33, including a whole-file rewrite. The pilot results are structurally separated below under `## Result: pilot, 2026-09-25`, which establishes how the document is **arranged**; it does not establish when the protocol above it was fixed. Operation counts are
>   a heuristic over transcript payloads and are not exact authorship history;
>   what they establish is that the document changed repeatedly over days, not
>   which bytes changed.
>
> **What this pin does and does not do.** It fixes the identity of the text as
> it stands today, and nothing before today. It is forward-only. It does **not**
> retroactively establish preregistration status, and the document should not be
> cited as a preregistration on the strength of it.
>
> The protocol below is unmodified. Nothing was edited to make this note tidier.

## Why the shipped verifier cannot answer this

`internal/advisor/advisor.go:538` opens with:

```go
if kind == KindHotFile || kind == KindCacheBreaks {
    return AdviceOnly, 0
}
```

Thirty lines earlier, those same two kinds carry the largest predicted values in
the advisor. `KindCacheBreaks` is credited with the whole `WriteReadUSD`, because
a break that does not happen re-bills nothing; every other kind is multiplied by
`trimShare`, a fraction.

**The two findings worth the most are the two the verifier refuses to score.**

The refusal is correct. `track` measures a target's share of tokens falling
across sessions. A break has no share to fall: it either happened or it did not.
Asking a session-over-session share tracker to score it would produce exactly the
circularity that was removed from this file, where a fall was both the evidence
that a change had been made and the measure of whether it worked. That defect
returned VERIFIED with an 8-point realized saving to a reader who had changed
nothing, and produced the 20-against-1 figure that this repository later
established was corpus drift.

So the exclusion stays, and a different measurement is needed. This is that
measurement.

## Hypothesis

Applying a `KindCacheBreaks` finding reduces the re-billed tokens a task incurs,
by an amount large enough to state.

## Why this is affordable when the quota titration was not

[Experiment 1](gtm-preregistration-2026-09-24.md) is blocked because an
account-wide rate-limit counter moves in 1% steps and needs over 30M tokens per
arm to move enough of them.

**Re-billed tokens are per-request and provider-reported.** `cache_creation` and
`cache_read` arrive on every response. There is no quantisation floor to
overcome and no account-wide confound to remove, because the measurement is taken
from the requests themselves rather than from a counter the whole account shares.

This experiment is therefore cheap, and its cost is the token spend of the tasks
it runs, nothing more.

## Design

Paired arms over seeded tasks, modelled on [seeded blame,
2026-09-11](seeded-blame-2026-09-11.md), which scored 10 of 10 by seeding the
event rather than the symptom.

**Ten tasks.** Each is a bounded piece of agent work that can be run to
completion and scored for success independently of its cost.

**The break is seeded and sealed before any run.** For each task, a known
prefix-breaking pattern is introduced deliberately, written down, and sealed. The
tasks are fixed before `advise` is pointed at anything, so no case is selected
after seeing which ones would look good.

| Arm | Construction |
|---|---|
| CONTROL | the task run with the seeded break in place |
| TREATED | the same task, same prompt, same model, with the specific advised change applied and nothing else changed |
| SHAM | the same task with an unrelated change of comparable size applied instead |

The SHAM arm is the negative control. **Without it, any reduction could be the
second run of anything being cheaper than the first**, and the trial could not
fail.

## Observable

Provider-reported usage fields from the transcripts of the runs themselves:
`cache_creation_input_tokens`, `cache_read_input_tokens`, `input_tokens`,
`output_tokens`.

**Re-billed tokens are computed from those fields and not from the advisor's own
prediction.** `PredictedUSD` is Replay's arithmetic about what a break costs.
Scoring Replay's advice with Replay's own prediction of that advice's value is
the circularity this file exists to avoid. The prediction is recorded alongside
the outcome so the two can be compared, but **the prediction is never the
outcome.**

## Second observable, and it is not optional

**Task success, scored against a rubric written before the runs.**

A change that reduces tokens by breaking the work is not a saving. If the TREATED
arm completes fewer tasks than CONTROL, that is reported with the same prominence
as the token figure, and a token reduction accompanied by a success reduction is
**not** reported as an improvement.

## Estimand

Median reduction in re-billed tokens, TREATED against CONTROL, per task, with the
SHAM arm's reduction as the floor that must be cleared.

Reported as: absolute tokens, share of that task's total tokens, and dollars at
list with the build and command named, per the repository's standing rule that
every figure names both.

## Thresholds, fixed now

**POSITIVE.** The TREATED reduction is bounded away from zero, it exceeds the
SHAM arm's reduction, and task success does not fall. The result licenses one
sentence only:

> On ten seeded tasks, applying the advised change reduced re-billed tokens by a
> median of N, against a sham change that reduced them by M.

It does **not** license "Replay reduces your bill by N%". Ten seeded tasks on one
machine are not a population, and the ceiling below bounds any extrapolation.

**NULL.** The TREATED reduction is not distinguishable from the SHAM reduction,
or it is indistinguishable from zero. Then the advisor's cache-break findings are
recorded as **detected but not demonstrated to be actionable**, the
`AdviceOnly` exclusion becomes permanent rather than provisional, and no
optimization claim may be made for this finding class.

**INCONCLUSIVE.** The seeded break did not reproduce; or the arms are not
comparable because the agent took materially different paths; or task success
moved in a way that makes the token comparison meaningless.

**A reduction that comes with a success drop is not POSITIVE.** It is recorded as
a cost shift and named as one.

## The ceiling on any positive result

Stated in advance so that a good number cannot be extrapolated past it later.

- Re-billed is **4.18%** of the measured corpus, 123 sessions, 105.2M tokens,
  $620.47 at list, read 2026-09-17. A perfect fix of every break caps there.
- The same corpus reads **2.75%** on the shipping build and **4.99%** on v0.5.4,
  so the instrument moves the figure by more than most interventions will.
- The one lever previously studied, keepalive, found **$77.24 recoverable against
  $508.83 that is not**, and concluded the lever was wrong because 87% of
  cache-creation spend falls on gaps under five minutes where the cache had not
  expired and something rewrote the prefix while it was warm.

**Any headline from this experiment is a low single-digit percentage of spend, or
it is wrong.**

## Limitations, stated before the test

- Ten tasks, one operator, one machine, one client version. This is a case study
  with controls, not a population estimate.
- The breaks are seeded, so their distribution is chosen rather than observed.
  The experiment measures whether the advice works on breaks of the kind seeded,
  not how often such breaks occur in anyone's real work.
- An agent is not deterministic. Paired arms reduce that, they do not remove it.
- The operator applying the advice wrote the advice. That is the weakest
  available position for judging whether it helps, and it is why task success is
  scored against a rubric fixed in advance rather than judged run by run.

## What a POSITIVE result would unlock, and what it would not

It would let the product state a measured reduction for a seeded case, and it
would justify revisiting the `AdviceOnly` exclusion with a design that fits the
claim.

It would **not** produce a realized-savings figure inside `replay advise`. That
surface measures a share falling across sessions, which a break still does not
do. A positive result here changes what may be claimed in documentation, not what
the verifier prints.

## Gates

No production code changes to make this run. No test may be weakened. `FS2`,
`track`'s exclusion, and every refusal boundary stay exactly as they are until
the result is in, because changing the instrument to suit the experiment is the
defect this repository has found most often.

## Result: pilot, 2026-09-25

The ten-task trial did not start. A pilot ran first to answer the gating
question, **can a cache break be seeded and detected on demand**, and the answer
is no for both mechanisms tried.

Seven runs, `claude-haiku-4-5-20251001`, one machine, **$0.639490 total**. A
~36,800 token inert prefix, read once to establish a cache, then resumed.

### Attempt 1: change the appended system prompt

Two arms resumed the same base session, with system prompts of identical length
differing only in content.

| Arm | cache_creation | cache_read | cost |
|---|---|---|---|
| PRESERVED | 7,510 | 49,554 | $0.098015 |
| BROKEN | 2,077 | 57,064 | $0.108540 |

**No break.** `cache_read` went up in the arm meant to break, not down.

It also surfaced a confound that would have contaminated the full trial. The two
arms resumed the **same** base session back to back, so the second arm read a
cache the first arm had just written. That is worth **7,510 tokens** running in
favour of whichever arm goes second. Had the ten-task trial run this way and
reported a reduction, the reduction would have been arm order.

### Attempt 2: change the tool set, from independent base sessions

Rebuilt with a separate base session per arm to remove the order confound.

| Arm | cache_creation | cache_read | cost |
|---|---|---|---|
| CONTROL, same tools | 269 | 56,647 | $0.091167 |
| TREATED, tools added | 1,162 | 56,570 | $0.094240 |

**No break either.** `cache_creation` rose by 893 tokens, which is about the size
of the added tool definitions themselves, while `cache_read` stayed flat at
roughly 56,600 and moved by 77 tokens.

**A real break has a signature this does not have.** It would collapse
`cache_read` toward zero and push `cache_creation` up to the full prefix. What
was observed is new content being written on top of an intact cache, which is
ordinary growth and not invalidation.

### A third finding, unlooked for

The two base runs were byte-identical in prompt, model, tools and file. One
answered **1401** and the other **1400**, on a task that is counting lines.

**The agent is not deterministic on a trivial task.** Any per-task token
comparison carries that noise, and it is another reason the success rubric is
not optional.

### Classification

**INCONCLUSIVE**, on the pre-registered condition: "the seeded break did not
reproduce."

No claim is made in either direction about whether acting on a cache-break
finding reduces re-billed tokens. The experiment did not reach the question.

### What the pilot actually established

Three things, all about the instrument rather than the hypothesis:

1. **A prompt cache is harder to break deliberately than assumed.** Neither the
   appended system prompt nor the tool set invalidated it. Both sit outside the
   part of the prefix that was cached, or after the breakpoint.
2. **Arm order is a large confound** and independent base sessions are mandatory,
   not a refinement.
3. **Run-to-run nondeterminism is present at the smallest scale**, so n must be
   sized against it.

The first of these points somewhere useful. The [keepalive
study](keepalive-vs-prefix-stability-2026-09-10.md) found 87% of cache-creation
spend on gaps under five minutes, where the cache had not expired and **something
rewrote the prefix while it was warm**. This pilot failed to be that something
with either of the two levers available from outside the client. That narrows the
candidates toward mutation of the conversation history itself, which is
compaction, rewind and resume, and which this repository has already observed
firing at exhaustion near one million tokens.

**A seeded trial cannot proceed until a mechanism is found that reproducibly
breaks a prefix.** Finding one is now the blocking question, and it is a smaller
and cheaper question than the trial it gates.

## Conclusion

Not reached. The hypothesis is untested, the design is sound, and the harness
works. What is missing is a reliable way to cause the defect the experiment
exists to fix.
