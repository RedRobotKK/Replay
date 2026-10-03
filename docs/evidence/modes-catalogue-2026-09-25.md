# What works, what fails, and what I believed that was wrong

**Every run from 2026-09-25, catalogued by evidence strength.** Build
`v0.6.2-7-g6871022`, one machine, one operator, one repository. Roughly 100 runs.

A failure mode known precisely is worth as much as a success mode, so both are
listed with the same rigour, and a third section lists the claims that died
tonight so they cannot quietly return.

## The task pair everything rests on

**COUNTING.** "How many Go test functions are defined across
`cmd/replay/b*_test.go`?" Ground truth 60. **The search string is handed to you
in the question.**

**COMPREHENSION.** "Explain why the burn command reports two separate counts for
Ollama cache reuse instead of a single average." The answer is a selection
argument at `burnollamashare_test.go:13`. **The answer's vocabulary is exactly
what the question does not contain.**

That difference is the axis the whole catalogue turns on.

## WORKS, repeatable

### W1. Bounding tool output on locating and counting work

| model | verbose cacheWR | bounded cacheWR | saving | verbose cost | bounded cost | turns |
|---|---|---|---|---|---|---|
| haiku | 54,413 | 18,116 | **66.7%** | $0.1344 | $0.0438 | 17 to 2 |
| sonnet | 77,901 | 33,878 | **56.5%** | $0.3699 | $0.1734 | 17 to 4 |
| opus | 83,302 | 35,290 | **57.6%** | $1.0942 | $0.4241 | 19 to 4 |
| fable | 79,140 | 32,048 | **59.5%** | $1.7195 | $0.6748 | 17 to 2 |

Four models spanning 13x in price. n=10 on the first two, n=2 on the last two.
95% CI on haiku [66.1, 67.3], on sonnet [54.9, 58.1].

**Correctness is never worse and is sometimes better.** Bounded scored 10/10 on
both models. Verbose scored 6/10 on haiku.

**Replicated.** An independent sonnet run at n=5 returned cache-write 34,528
against the original 33,878, and 5/5 correct.

### W2. `cache_creation_input_tokens` as the metric

Coefficient of variation 0.8% to 5.5% across every cell. The read column reached
53% in one cell and swung 7.4x between byte-identical runs. **Any metric summing
reads inherits that.** Cost tracks writes because a write bills at 1.25x and a
read at 0.1x.

### W3. Reading whole files when the question is "why"

Haiku comprehension: verbose 3/5 reached the selection argument, bounded 0/5.
This is the only cell where verbose wins, and it is why the rule cannot be
global.

## FAILS, repeatable

### F1. Bounding output on comprehension questions

| model | arm | reached the argument | cost |
|---|---|---|---|
| haiku | bounded | **0 of 5** | $0.0712 |
| haiku | verbose | 3 of 5 | $0.1279 |
| sonnet | bounded | **2 of 5** | $0.2015 |

**The mechanism, traced through the transcripts.** Thirty-eight search patterns
were issued across the five haiku runs: `two.*count`, `separate.*count`,
`hit rate`, `measured|inferred`, `observed|inferred`. The answer is written as
"a population selected for having been cached". **No pattern targeted
"population", "selected", "resident" or "n_past".**

Every search used the question's vocabulary. This is the vocabulary mismatch
problem (Furnas et al. 1987): on average 80% of the time two people name the
same thing differently, and exact matching fails on it.

### F2. Reading the file late does not repair it

Two of five bounded runs did open `burnollamashare_test.go` and still missed.
bounded_3 cites "lines 97-105 and 123"; the argument is at line 15. It arrived
carrying the frame its failed searches had built and read for confirmation.

**Filtering does not merely fail to find the answer. It pre-commits the reader to
the wrong question.**

### F3. The search-rephrase spiral

bounded_3: thirteen tool calls, `inferred` grepped five separate times, the whole
file read anyway, 35.2 seconds, **$0.1314 against a verbose median of $0.1279**,
and wrong.

**The 44% comprehension saving is a median over runs where the first guess landed.**
When it does not land the bounded arm pays full price for a wrong answer.

### F4. Verbose is not a safe default either

Haiku counting, verbose arm: **5 of 10 wrong** (four off-by-one at 61, plus one prose non-answer I originally missed). Reading
fifteen files and counting by hand introduced an error that `grep -c` does not
make. Verbose is not "the careful option"; it is a different failure surface.

## FALSIFIED tonight, and not to be restated

Five claims made and killed on the same day, in order.

| claim | how it died |
|---|---|
| "The cheaper arm is also the more accurate one" | haiku-only artifact; sonnet, opus and fable answer correctly in both arms |
| "Haiku's 61 is a reproducible systematic error" | 5 of 5 at small n became **4 of 10**; a run of luck read as a mechanism |
| "Cost saving is a tight 59.7-66.5% band across models" | at n=10 the intervals do not overlap and sonnet's 53.1% falls **below the floor** of the band meant to contain it |
| "Two or more distinct search patterns predicts failure" | **This entry was itself wrong.** compbound_4 was scored REACHED by a keyword scorer that matched the word "populations" used in a different sense; its answer is the sparse-logging framing, not the selection argument. Churn was never falsified by data. It is still not a rule, for a different reason: BOUNDED+ drives churn to 1-2 and reads the file in 5/5 runs, and comprehension does not move, so churn is a symptom of a failing search rather than a cause of a wrong answer |
| "The bounded arm never saw the source file" | 2 of 5 read it in full and missed anyway |

**Four of the five were small samples read as laws.** That is the dominant error
mode of this session, and it is more repeatable than anything in the WORKS
section.

## The ceiling: Amdahl's law applies, and we are already near it

Cache writes decompose into a fixed prefix, written whatever the task is, plus a
per-call task cost. Fitting both arms against their tool-call counts:

| model | verbose | bounded | per call | **FIXED floor** | fixed share of bounded | max further saving |
|---|---|---|---|---|---|---|
| haiku | 54,413 | 18,116 | 2,469 | **14,906** | 82% | **18%** |
| sonnet | 77,901 | 33,878 | 3,079 | **26,181** | 77% | **23%** |
| opus | 83,302 | 35,290 | 3,201 | **25,687** | 73% | **27%** |
| fable | 79,140 | 32,048 | 3,139 | **28,909** | 90% | **10%** |

**15,000 to 29,000 cache-write tokens are spent before the task begins**: system
prompt, tool definitions, hooks.

Output discipline took the variable part from roughly 40,000 tokens to about
3,000. It worked, and **it worked so well that 73% to 90% of what remains is the
fixed floor.** The ceiling on any further tool-output optimisation is 10% to 27%.

That is Amdahl's law in its ordinary form: the unoptimisable fraction caps the
total. **Another hundred experiments on how to phrase a search cannot return more
than a fifth of a bounded run.**

A usable constant falls out of the same fit: **a tool call costs 2,469 to 3,201
cache-write tokens**, consistent across four models differing 13x in price.

### Where the larger lever is

Not tool output. The fixed prefix, which is 73% to 90% of what an optimised run
still pays.

Sizing it on haiku: a 30% cut to a 14,906-token floor saves about 4,472 tokens,
**25% of a whole bounded run, which is larger than the entire remaining headroom
in tool output.**

## CLEAN RE-MEASUREMENT: the model spread was mostly the skill

Re-run with `--disable-slash-commands`, every run checked individually for
off-task calls, contaminated runs excluded rather than averaged in.

| model | arm | n (excluded) | cache writes | cost median | turns | correct |
|---|---|---|---|---|---|---|
| haiku | verbose | 5 (0) | 56,063 | $0.1369 | 17 | **1/5** |
| haiku | bounded | 5 (0) | **15,087** | **$0.0377** | 2 | **5/5** |
| sonnet | verbose | 5 (0) | 71,987 | $0.3245 | 17 | 5/5 |
| sonnet | bounded | 4 (1) | 19,692 | $0.0981 | 2 | 4/4 |
| fable | verbose | 1 (2) | 65,744 | $1.4304 | 17 | 1/1 |
| fable | bounded | 1 (2) | 20,831 | $0.4461 | 2 | 1/1 |
| opus | either | **0 (6)** | not measurable | | | |

| model | clean saving | contaminated | change |
|---|---|---|---|
| haiku | **73.1%** | 66.7% | +6.4 pts |
| sonnet | **72.6%** | 56.5% | +16.1 pts |
| fable | **68.3%** | 59.5% | +8.8 pts |

### The spread collapsed, and it was never a model property

Contaminated, the four models spanned **56.5% to 66.7%**, and this file treated
that 10.2 point spread as a model difference worth explaining. Clean, they sit at
**68.3% to 73.1%**, a spread of 4.8 points.

**Every model saves roughly 70%.** What differed was how much overhead each was
carrying, which is a property of the operator's skill configuration rather than
of the model. The earlier cross-model section is superseded, not annotated.

### Opus defeated the isolation, 6 of 6

`--disable-slash-commands` took sonnet from 12/12 contaminated to 1/5 and fable to
2/3, but **opus reached for the agmsg Monitor on every single run**, in both arms.

That is a finding in its own right: opus pursues a directive it has seen more
persistently than the other three, and it cannot be measured clean with the tools
used here.

### The counting result got stronger

Clean haiku: **verbose 1/5 correct at $0.1369, bounded 5/5 at $0.0377.** That is
3.6x cheaper and five times the accuracy.

The contaminated measurement put verbose at 5/10. **The noise was concealing how
badly reading everything performs on this model**, not exaggerating it.

### Limits

Fable rests on **n=1 per arm** after exclusions and is indicative only. Sonnet
bounded is n=4. Opus is absent. The three surviving models agree closely enough
that the ~70% figure is the safer claim than any per-model number.

## CORRECTION: the agmsg cost is 3x what this file first said, and it is not a hook

The figure below (+3,278 tokens, +$0.031) was derived by splitting sonnet's own
runs into those where the directive fired and those where it did not. **That
comparison understates it by a factor of three**, because the skill sat in the
system prompt of BOTH groups; the "clean" group simply declined to act on it.

Isolating it properly with `--disable-slash-commands`, same task, same model:

| sonnet, identical task | cache writes | cost | tools used |
|---|---|---|---|
| skills disabled | **26,580** | **$0.1337** | Bash, Bash |
| skills loaded | 37,030 | $0.1965 | ToolSearch, Monitor, Bash, Bash |
| **agmsg costs** | **+10,450 (+28%)** | **+$0.063** | +2 calls |

**It is also not a hook.** agmsg delivery mode is `off` for both project paths
with zero SessionStart entries registered. The subprocesses were reading the
skill's "ensure monitor is running first" instruction out of the available-skills
listing and acting on it. That is why turning agmsg off did not stop it, and why
the contamination tracks MODEL rather than configuration: sonnet and opus comply,
haiku and fable ignore it.

### What this invalidates

**Every sonnet figure measured before this point carries 10,450 contaminating
tokens**, including the 56.5% cache-write saving and its [54.9, 58.1] interval.
Checked across the 12 hook A/B runs: **sonnet was contaminated in 12 of 12.**
Haiku was mostly clean.

The cross-model comparison is therefore **withdrawn pending re-measurement**, not
annotated. A spread that was reported as a model difference is partly a
difference in which models obey a skill directive.

### It also reversed the PreToolUse hook result

Recomputed on runs where agmsg never fired:

| | nohook | hook | |
|---|---|---|---|
| haiku task A | 18,648 | 25,075 | **+34.5%** |
| haiku task B | 19,090 | 25,347 | **+32.8%** |
| sonnet | 0 clean runs of 12 | | not measurable |

The single positive cell in the hook experiment was an artifact: the unhooked
arm's average was inflated by agmsg, and the clean unhooked run was 18,648 rather
than 30,477. **The trimming hook increases cache writes by about a third in every
cell that can be measured cleanly**, and on task B it cost correctness too.

The mechanism is wrong rather than the parameters: a hook caps the output of a
plan already chosen, while an instruction changes the plan. **PreToolUse output
trimming is recorded here as tried and rejected.**

## CONFOUND: the agmsg hook, found only because the task was optimised

Off-task tool calls in the counting task, bounded arm:

| model | task calls | off-task | share |
|---|---|---|---|
| haiku | 1.3 | 0.0 | 0% |
| sonnet | 1.1 | **1.4** | **56%** |
| opus | 1.0 | **2.0** | **67%** |
| fable | 1.0 | 0.0 | 0% |

The off-task calls are `ToolSearch` and `Monitor` firing the agmsg SessionStart
directive. **Sonnet and opus obey it; haiku and fable ignore it entirely.**

Within sonnet's ten runs, three escaped the hook and seven did not:

| | cache writes | cost |
|---|---|---|
| clean | 31,583 | $0.1442 |
| hooked | 34,861 | $0.1751 |
| **overhead** | **+3,278 (10%)** | **+$0.031** |

**This taints the cross-model comparison.** Sonnet's 56.5% saving against haiku's
66.7% was reported as a model difference; part of it is an operator hook that
fires on one model and not the other. Every cross-model figure in W1 carries this
asterisk.

It is also the principle in miniature. The hook costs a fixed ~3,278 tokens
regardless of task. On a verbose run that is 5% of calls and invisible. On a
bounded run it is 56% of everything the agent does. **Optimising the task is what
made the overhead visible**, and the same is true of the prefix floor above.

## CORRECTION: the mechanism is a competing explanation, not vocabulary mismatch

An independent reverse-engineering pass (100 further runs, ~$16.4) overturned the
working theory and two of the scores this file rests on.

### The vocabulary-mismatch theory failed its own registered falsifier

Registered before the test: *"if BOUNDED+ reaches the answer no more often than
BOUNDED, vocabulary mismatch is not the mechanism."*

BOUNDED+ on haiku drove retrieval from 3/5 to **5/5 perfect** and REACHED stayed
at **0/5**. The falsifier fired. Vocabulary mismatch is real and explains the
churn, but it does not determine the answer.

### What does determine it

`burnollamashare_test.go:95-105` carries a SECOND, different causal story about
the same behaviour: a bad build would "turn a statement about sparse logging into
a statement about a well-cached machine". **Every MISS in the original data is a
paraphrase of that passage.**

Static crossover, no tools, views differing only by those 11 lines, n=5 per cell:

| model | without the distractor | with it |
|---|---|---|
| haiku | **3/5** | 0/5 |
| sonnet | 4/5 | 2/5 |
| opus | **5/5** | 2/5 |
| fable | 5/5 | **5/5** |

Haiku pooled across five view variants: **6/10 against 0/15, Fisher p = 0.0012.**

**Grep-view versus Read-view made no difference at all** (3/5 vs 3/5). Format,
fragmentation and line gaps do nothing. Deleting eleven lines rescues the grep
view completely, and prepending rather than appending changes nothing, so it is
presence rather than position.

Fable is immune in both conditions, so **the rule is model-conditional**, which is
a weakness and not a footnote.

### Scores in this file that do not reproduce

- "haiku verbose comprehension 3/5": no rubric produces 3. Strict gives 1/5,
  mechanism-clause-only gives 5/5.
- "sonnet bounded 2/5": strict gives 1/5.
- "compbound_4 REACHED": a keyword false positive, corrected above.

**The comprehension outcome is hand-scored prose and two defensible rubrics
disagree on 3 of 15 runs, including the decisive one.** Every comprehension figure
in this file inherits that.

### Further confounds found in the design

- Child sessions carry a **~207,000-token MCP system prompt**, pruned by
  `--allowedTools`. That flag string silently set the measurement baseline, so the
  Amdahl floor figures are internally consistent but not portable.
- `cache_creation` absorbs assistant output as well as tool results: verbose
  emitted 3,478 output tokens against bounded's 438. The arm difference is real;
  attributing all of it to reading discipline is not.
- `--allowedTools ""` does not disable tools.
- The corpus restriction was requested, never enforced.

### The rule that replaces it

> **When the question is WHY, the assembled context must not contain a second,
> different causal explanation of the same surface. If it does, the model reports
> the more concrete one. Retrieval breadth does not fix this; only excision does.**

n=1 corpus, n=1 distractor pair, and falsified already for fable.

## The rule that survives

> **Bound tool output when the work is locating, counting or checking.** The
> search string is known, the saving is 56% to 67% of cache writes, turns drop
> about sixfold, and correctness is never worse.
>
> **Do not bound it when the work is understanding why.** The answer's wording is
> the thing being searched for, filtering commits you to the wrong frame, and it
> does so cheaply and confidently.

The asymmetry in evidence is stated rather than hidden: **the works side is n=10
on two models plus n=2 on two more, replicated. The fails side is n=5 on one task
and one model pair.** The benefit is far better evidenced than the penalty.

## UNKNOWN

- **BOUNDED+**, the literature's fix: filter first, and on a failed search open
  the likeliest file rather than rephrasing. Running. It predicts verbose-rate
  correctness at near-bounded cost, and if it does not deliver that, the
  vocabulary-mismatch theory is wrong.
- Sonnet comprehension verbose: not yet run, so the comprehension penalty is
  currently one-sided on that model.
- Opus and fable comprehension: never run.
- Cold-prefix cost: three attempts killed, a fourth scheduled.
- **Whether any of this transfers beyond this repository and this task pair.**
  Counting across files rewards filtering unusually well, and one comprehension
  question is not a category.
