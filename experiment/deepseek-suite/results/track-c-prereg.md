# Preregistration: Track C, reasoning and optimisation

Written before execution. Zero DeepSeek API calls have been made for this track.
The instrument is `experiment/harness/taskclasses.py`, its fixture tests are
`experiment/harness/test_taskclasses.py`, and both are committed before any
prediction below can be checked.

Evidence classes are used strictly: OBSERVED, DERIVED, ASSUMED, NOT_OBSERVED.
Nothing in this file is OBSERVED about the five new classes. The two figures
quoted from a prior run are labelled where they appear.

## What this track exists to settle

A prior run measured `reasoning_effort: "none"` on exactly two task classes and
found the lever class-conditional: nearly free on one, ruinous on the other
(OBSERVED, `experiment/deepseek/reasoning-scope-2026-09-29.md`).

| cell | pass | rate | output tokens | evidence |
| --- | ---: | ---: | ---: | --- |
| mechanical, reasoning ON | 18/18 | 100% | 1,075 total | OBSERVED |
| mechanical, reasoning OFF | 17/18 | 94% | 55 total | OBSERVED |
| aggregative, reasoning ON | 24/24 | 100% | 22,337 total | OBSERVED |
| aggregative, reasoning OFF | 7/24 | 29% | 2,328 total | OBSERVED |

Two classes is not a taxonomy. `policy.config_for` currently sends everything
that is not a lookup to reasoning ON, which is safe and may be expensive by a
large factor on classes nobody has measured. Track C measures five.

The lever is UNDOCUMENTED in the disabling direction. The vendor guides show
only how to enable reasoning. Treat every result here as a description of
current behaviour, never as a contract.

## C1. The five classes

Each boundary is a rule about the ANSWER, not about how the question sounds.
This matters because the previous attempt at this experiment voided a cell by
labelling a positional counting question "mechanical": it asked which function
sat on a given line number of a 48,000-character document. That is counting
wearing a lookup's label, and the label was the assumption that failed.

| class | rule about the answer | tasks |
| --- | --- | ---: |
| LOOKUP | written verbatim at ONE place in the corpus, reached by matching a name. No arithmetic, no enumeration. | 5 |
| COUNTING | the cardinality of a set defined by a mechanical predicate. Written nowhere. | 4 |
| AGGREGATION | combines TWO OR MORE separately located facts by a rule stated in the question. Written nowhere. | 3 |
| TRANSFORMATION | ONE located value put through a deterministic rule stated in the question. Distinguished from AGGREGATION by arity. | 3 |
| MULTI_STEP | several DEPENDENT hops, where the target of hop n is named only by the result of hop n-1. | 3 |

MULTI_STEP is the boundary most open to argument, so it is stated here rather
than defended later. Its answers may be verbatim in the corpus. What is not
verbatim is the path to them. A model that can retrieve but cannot dereference
will fail MULTI_STEP while passing LOOKUP, and separating those two abilities is
the point of the class.

One example per class, quoted from the harness:

- **LOOKUP** `lookup.method.receiver`: "Exactly one method in the source above
  is named `noteReads`. Reply with the name of its receiver type, without any
  pointer marker."
- **COUNTING** `counting.func.lines`: "In the source above, how many lines
  begin, at their first character, with the four letters f, u, n, c followed by
  a single space?"
- **AGGREGATION** `aggregation.const.mixed`: "Subtract from the value of
  `minInjectedTokens` the product of the values of `labelMaxLen` and
  `minReads`."
- **TRANSFORMATION** `transform.sort.funcname`: "Reply with that function's name
  written with its characters reordered into ascending Unicode code point
  order."
- **MULTI_STEP** `multistep.receiver.lastfield`: "Find the method named
  `noteReads` and read its receiver type. Find the struct type declaration of
  that receiver type. Reply with the name of the last field declared in it."

### The corpus

Real Replay source: `internal/advisor/advisor.go` then
`internal/transcript/wire.go`, each behind a `===== FILE: ... =====` marker,
48,166 characters. It is the same corpus as the prior run, which makes that
run's counting truths (39 lines beginning `func `, 14 beginning `type `) a
cross-check on this harness and not only on the model. A fixture test asserts
both.

The corpus is the SHARED block passed to `session.Run.fan`, so it is the cached
prefix and every question is the suffix. `policy.assemble` refuses a prefix
whose leading bytes vary, and these do not.

### Ground truth is computed, never authored

Every expected answer is produced from the corpus at import time by a reader
function in `taskclasses.py`. A hand-typed expected answer is an unversioned
second copy of the corpus that goes stale without failing anything.

This is not a claim, it is a test. `test_taskclasses.py` perturbs the corpus and
requires the answers to move: adding a function increments the `func ` count,
changing `HashedLabelBytes` from 12 to 77 moves the lookup answer, the
three-constant sum and the digit-reversal together, and renaming a struct field
moves the multi-step answer. A hardcoded truth cannot pass those tests, and the
mutation check below confirms they bite.

Each reader also asserts the uniqueness its question claims. A question that
says "exactly one" against a corpus where two match is unanswerable, and a wrong
answer to an unanswerable question is a harness failure recorded as a model
failure. `CorpusAmbiguity` is raised at construction instead.

### No question leaks its answer or its class

Two fixture tests enforce it. The answer, normalised to lowercase
alphanumerics, must not appear in the question. That normalisation is
deliberately aggressive: it collapses `hot-file` onto `KindHotFile`, which is
the leak shape Go's convention of naming a constant after its own value
produces, and it is why no typed string constant is used as a LOOKUP target.
Separately, no question may contain the words lookup, count, aggregat,
transform, multistep or step by step. The class is the variable under study and
a question that names it is measuring something else.

The corpus necessarily contains every LOOKUP answer. That is what makes it a
lookup. What must not contain the answer is the question.

## C2. Scoring, fixed before execution

**Binary. No partial credit anywhere.** A scored answer is right or wrong. An
arm that costs less and answers worse must show up as a pass-rate drop, not as
a softer average.

**Checkers are independent Python predicates.** The model never grades itself
and no checker reads the model's reasoning about whether it was right.

Two rules, applied by class of answer:

| answer shape | rule |
| --- | --- |
| numeric | the LAST number appearing anywhere in the response must equal the truth exactly |
| literal | the last non-empty line, stripped of code fences, quotes, backticks and one trailing period, must equal the truth; or that line's last whitespace-separated token must |

"Last" rather than "only" is deliberate and arm-neutral. A reasoning-ON response
is verbose by construction and will restate intermediate numbers. An "only" rule
would score that arm wrong for being verbose and the result would read as an
accuracy finding. "Last" reads the answer and ignores the work.

The residual risk is a response that enumerates and happens to end on the truth.
That inflates the arm that enumerates, which is the reasoning-ON arm, so the
bias runs AGAINST this campaign's cost-saving hypothesis. That is the safe
direction. It is recorded, not corrected.

Literal checkers are case-sensitive, because Go is: `suggestion` is not
`Suggestion`.

No checker may raise. `session.Run` records `passed=None` when a checker throws,
which converts a paid call into a measurement of nothing. A fixture test feeds
every checker empty strings, `None`, integers, lists, bare code fences, 20,000
characters of padding, `NaN` and `1.2.3`, and requires a bool every time.
Garbage is wrong, not undecided.

### Truncation, timeout and the undecided outcome

A TRUNCATED response is recorded UNDECIDED and excluded from the accuracy
denominator. It is never scored wrong. The adapter raises on
`finish_reason` in `("length", "max_tokens")` and `session.Run` maps that to
`Outcome.TRUNCATED` with `passed=None`; `score_by_class` counts it as undecided.
The same holds for `REFUSED_BUDGET`, `ERROR`, and an ANSWERED row whose checker
raised. Four kinds of undecided, none of them a wrong answer.

`max_tokens = 4096`. This is a MEASURED requirement, not a guess. The
aggregative reasoning-ON cell averaged about 931 output tokens per call, and an
earlier run of that experiment voided itself entirely by capping at 1024 so that
reasoning-ON never reached an answer. A fixture test pins the floor at 4096.

A cell whose undecided fraction exceeds 20% is reported VOID, not rounded into
an accuracy. An empty cell has accuracy `None`, never 0%.

### Token and cost measurement

Per call, from the provider's own usage block: `fresh_in`, `cache_read`,
`cache_write`, `out`, `reasoning`, `finish_reason`. Anything the surface did not
report stays `None`, which is NOT_MEASURED, and is never coerced to zero.

Dollar figures are DERIVED from `experiment/deepseek-suite/pricing.md` by
`pricing.cost_usd`. They are arithmetic over observed usage and a published
rate. The only OBSERVED cost source is `/user/balance`, whose resolution floor
is near one cent and therefore coarser than this whole track.

### How the arms are realised, and the one harness change this needs

`session.Run.fan` requires a `task_class` and accepts no kwargs override, so the
reasoning setting is decided entirely by the class handed to it. Track C needs
both settings on all five classes, so `policy_class_for(cls, arm)` hands `fan`
the class whose policy config carries the wanted setting: `TaskClass.LOOKUP` for
the OFF arm, `TaskClass.AGGREGATE` for the ON arm, and the honest mapping for
the third, production-shaped arm.

That is an abuse of the enum and it is written down so nobody reads a run log
and concludes Track C believed COUNTING was a lookup. The mapping onto
`policy.TaskClass` is lossy in two named places:

- `MULTI_STEP -> AGGREGATE` is lossy. Dereferencing is not aggregation. It lands
  there because `policy` has nowhere else that keeps reasoning on.
- `TRANSFORMATION -> UNKNOWN` is lossy and deliberate. `config_for` sends UNKNOWN
  to reasoning ON, which is the fail-safe. An unmeasured class must not be cheap
  by default.

Forcing has a cost the first draft of the harness got wrong and a test caught: a
forced cell is INVISIBLE to `policy.review`, because review is handed the same
lie the transport is. `review_plan` therefore reviews the TRUE class against the
kwargs actually sent, which is the only arrangement where the forcing appears in
an audit. A fixture test asserts the finding is emitted.

**The honest fix is one line**: an `extra_kwargs` passthrough in
`session.Run.fan` to `ask`, which already accepts it. It is not made here
because Track C does not own `session.py` and a shared harness should not grow a
parameter on a subagent's say-so. It is the one harness change this track needs
and it is recorded as a request, not applied.

### Mutation check on the instrument

A check that cannot fail is not evidence. Run with `PYTHONDONTWRITEBYTECODE=1`
and `__pycache__` removed before each run, because a restored file whose mtime
lands in the same second as the mutant's bytecode is reused silently.

| mutation | tests failing |
| --- | ---: |
| `expect_int` always returns True | 9 |
| `expect_literal` always returns True | 15 |
| `counting.func.lines` truth hardcoded to 39 | 1 |
| uniqueness guard `_only` disabled | 1 |
| control, file restored byte-identical | 0 |

## Predictions, per class and reasoning setting

Design: 18 tasks, 3 repetitions, 2 arms, 108 calls. Per-class n is tasks times
3. Predictions are registered now so the result is a test rather than a
description. The two prior cells are replication predictions and a disagreement
with them is itself a finding.

| class | n per arm | ON, predicted | OFF, predicted | basis |
| --- | ---: | ---: | ---: | --- |
| LOOKUP | 15 | 100% | 93%, range 87 to 100 | replication of OBSERVED 17/18 |
| COUNTING | 12 | 100% | 29%, range 17 to 42 | replication of OBSERVED 7/24 |
| AGGREGATION | 9 | 100%, floor 89 | 44%, range 22 to 67 | NOT_OBSERVED. Each fact is a lookup and survives; three-term arithmetic without intermediate tokens is where it should break |
| TRANSFORMATION | 9 | 100%, floor 89 | 33%, range 11 to 56 | NOT_OBSERVED. Sharpened below |
| MULTI_STEP | 9 | 100%, floor 89 | 11%, range 0 to 33 | NOT_OBSERVED. A dereference chain has nowhere to put hop 1's result when there are no intermediate tokens |

The TRANSFORMATION prediction is deliberately sharper than a range, because a
per-task prediction is easier to falsify than a rate. With reasoning OFF I
predict `transform.reverse.digits` (a two-digit reversal) passes 3/3 and both
character-level transforms on 13 and 15 character identifiers fail 0/3, giving
exactly 3/9. If the character-level ones pass, the "no scratchpad, no
character surgery" account is wrong.

Secondary predictions, each independently falsifiable:

1. Output token ratio ON to OFF: LOOKUP about 19.5x and COUNTING about 9.6x,
   each within 30% (replication). For the three new classes, between 8x and 20x,
   with no point prediction. NOT_OBSERVED.
2. Zero truncations at `max_tokens = 4096` in every cell.
3. `cache_read >= 15,000` tokens on every fanned call after the warm call.
   Below that the fan-out ran cold and every cost figure in the run is void.
4. `reasoning` tokens exactly 0 on every OFF call. `policy.check_reasoning_honoured`
   raises otherwise and the run stops.
5. Failures with reasoning OFF are confidently wrong values of the right type,
   not refusals or empty answers. Under 5% of OFF failures will be refusals.

## Falsifiers, declared now

| id | if this happens | then |
| --- | --- | --- |
| F1 | LOOKUP OFF below 80% | the "lookups are safe with reasoning off" rule is wrong and `policy.config_for` must change. This is the rule the whole cost case rests on |
| F2 | COUNTING OFF at or above 80% | the OBSERVED 7/24 was an artifact of the earlier question wording, not a property of the class, and the prior finding needs retracting |
| F3 | TRANSFORMATION OFF at or above 80% | `TRANSFORMATION -> UNKNOWN` is over-conservative and the class earns its own policy entry with reasoning off |
| F4 | MULTI_STEP OFF at or above 80% | the `MULTI_STEP -> AGGREGATE` mapping is wrong and dereferencing is cheap |
| F5 | AGGREGATION OFF and COUNTING OFF differ by more than 30 points | collapsing both onto `TaskClass.AGGREGATE` is lossy in a way that costs money, and `policy` needs a third class |
| F6 | any cell's undecided fraction exceeds 20% | that cell is reported VOID. No accuracy is quoted from it |
| F7 | `cache_read` below 15,000 on fanned calls | every dollar figure in the run is void and the cache geometry claim needs re-measuring |
| F8 | any OFF call reports non-zero reasoning tokens | the undocumented parameter has changed. The whole run is void and every lookup cost figure downstream of the prior run is suspect |
| F9 | an LLM-based checker for LOOKUP comes in under $5.49e-5 per task | the C5 conclusion that the checker route needs a non-LLM checker is falsified |

A prediction that survives is not a law. n is 9 to 15 per cell, one model, one
corpus. Six claims died in one day on this campaign for being read as laws at
n below 10.

## C5. Independent-checker economics, analytic

No spend. Every figure DERIVED from the published table in
`experiment/deepseek-suite/pricing.md` and the OBSERVED token counts above.

### Inputs

| quantity | value | evidence |
| --- | ---: | --- |
| corpus characters | 48,166 | OBSERVED |
| corpus tokens | 16,055 | ASSUMED, 3 characters per token, `policy.CHARS_PER_TOKEN_DENSE` |
| cached tokens | 15,872 | DERIVED via `128 * (floor(n/128) - 1)`, MEASURED geometry, 17/17 rungs |
| uncached input per call | 228 | DERIVED, the unserved final block plus the question |
| output tokens per call, LOOKUP | 59.7 ON, 3.1 OFF | DERIVED from OBSERVED totals |
| output tokens per call, COUNTING | 930.7 ON, 97.0 OFF | DERIVED from OBSERVED totals |
| rates, deepseek-flash peak | 0.006 hit, 0.30 miss, 1.20 output, per 1M | published |

### Formulas

Per-call cost, warm cache:

```text
c_in  = cached * r_hit + uncached * r_miss
c_on  = c_in + out_on  * r_out
c_off = c_in + out_off * r_out
```

Baseline, reasoning enabled, cost per correct answer:

```text
B = c_on / p_on
```

Strategy ESCALATE: run the cheap arm, pay an independent checker of cost `k`,
and on rejection re-run with reasoning enabled.

```text
E[cost]    = c_off + k + (1 - p_off) * c_on
E[correct] = p_off + (1 - p_off) * p_on
k*         = B * E[correct] - c_off - (1 - p_off) * c_on
```

Strategy RETRY: run the cheap arm, pay the checker, retry the cheap arm until it
passes. Attempts are ASSUMED independent, which is the weakest assumption in
this section and is given its own falsifier below.

```text
k* = B * p_off - c_off
```

With `p_on = 1.0`, which is what was OBSERVED in both measured cells, the two
break-evens coincide at `k* = p_off * c_on - c_off`. They diverge as soon as
`p_on` is below 1.

### Results

Peak rates, warm 16k corpus:

| class | c_in | c_on | c_off | cost ratio | B per correct | k* break-even |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| LOOKUP | $1.637e-4 | $2.354e-4 | $1.674e-4 | 1.41x | $2.354e-4 | **$5.49e-5** |
| COUNTING | $1.637e-4 | $1.281e-3 | $2.801e-4 | 4.57x | $1.281e-3 | **$9.34e-5** |

Off-peak halves every rate uniformly, so every dollar figure halves and every
ratio, including `k*` as a fraction of `c_in`, is unchanged. LOOKUP `k*` becomes
$2.75e-5 and COUNTING `k*` becomes $4.67e-5.

### The finding that matters most

**The cached prefix eats the lever.** Reasoning-off cuts LOOKUP output tokens
19.5x, which is real, and cuts total LOOKUP cost 1.41x, which is what a budget
sees. With a 16,000-token shared corpus in every call, input is 70% of the
reasoning-ON cost and 98% of the reasoning-OFF cost. Isolated from the corpus
the same lever is worth 19.5x on LOOKUP and 9.6x on COUNTING:

| condition | LOOKUP c_on / c_off | COUNTING c_on / c_off |
| --- | ---: | ---: |
| warm 16k corpus | 1.41x | 4.57x |
| no shared corpus | 19.5x | 9.6x |

Any claim that reasoning-off saves an order of magnitude is a claim about
prompts with no document in them.

### The minimum accuracy for any checker to help

`k* > 0` requires `p_off > c_off / c_on`. Below that floor the cheap arm plus a
FREE checker still loses to reasoning-on, because the escalations cost more than
they save.

| class | floor `c_off / c_on` | OBSERVED `p_off` | margin |
| --- | ---: | ---: | --- |
| LOOKUP | 0.711 | 0.944 | comfortable |
| COUNTING | 0.219 | 0.292 | 7 points, thin |

COUNTING clears its floor by seven points on n=24. That margin is not a
foundation. One more failure in that cell would have put it under.

### Is any real checker cheap enough

An independent checker that is itself an LLM call over the same corpus cannot
cost less than `c_in`, which is $1.637e-4 even with a perfect cache hit and zero
output tokens. Against the break-evens:

| class | k* budget | cheapest possible LLM checker | verdict |
| --- | ---: | ---: | --- |
| LOOKUP | $5.49e-5 | $1.637e-4 | 3.0x over budget. CLOSED |
| COUNTING | $9.34e-5 | $1.637e-4 | 1.8x over budget. CLOSED |

So the checker must be a program, not a model. A regex, a grep or an AST walk
costs no API dollars and comfortably clears both budgets.

And that is where COUNTING closes anyway, for a reason that is not price. A
program that can verify a count is a program that computes the count. Its cost
is not the issue, its existence is: once you have it, it is the answer, and the
model call is redundant. The same holds for TRANSFORMATION, whose stated rule is
by construction a Python function.

The general rule this track will state or retract:

> A checker is economically useful only where verifying is strictly cheaper than
> solving. Where the checker must redo the work, it replaces the model rather
> than rescuing it.

By that rule, per class, before any measurement:

| class | verify cheaper than solve? | route |
| --- | --- | --- |
| LOOKUP | yes. Confirming a located string is a substring test; finding it is a search | OPEN, with a non-LLM checker |
| COUNTING | no. Verification is the computation | CLOSED by redundancy, not by price |
| AGGREGATION | partly. The rule is cheap to apply, but the operands still have to be located | UNDECIDED, and a Track C question |
| TRANSFORMATION | no. The stated rule is a Python function | CLOSED by redundancy |
| MULTI_STEP | yes in principle. Confirming a chain is cheaper than searching for it | OPEN, and the most interesting cell |

Marked ASSUMED, because the checker recall assumed throughout this section is
1.0. A checker that misses wrong answers converts the whole calculation into an
accuracy claim it cannot support, and no such checker has been measured.

### Falsifiers for C5

| id | if this happens | then |
| --- | --- | --- |
| F10 | measured `c_on / c_off` on LOOKUP differs from 1.41x by more than 25% | the token proxy of 3 characters per token is wrong and every `c_in` here is wrong with it |
| F11 | retry attempts are correlated, that is, a task that fails once fails its retries above chance | the RETRY break-even is void. Failures were OBSERVED to be confident wrong values, which is exactly the shape that repeats |
| F12 | AGGREGATION OFF clears its own `c_off / c_on` floor with room | the UNDECIDED verdict resolves to OPEN and a cheap-arm plus program-checker route exists for it |

## Execution preconditions

None of the following may be skipped, and none of them has been done.

1. This file is committed before any call is made.
2. `make harness-test` passes.
3. The run uses `session.Run` end to end. No hand-rolled call wrapper.
4. A ceiling is set on the `Run` and the campaign ledger is updated from
   `/user/balance` before and after.
5. No credential is printed or committed at any point.
