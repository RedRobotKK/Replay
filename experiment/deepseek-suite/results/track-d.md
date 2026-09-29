# Track D: instrumentation validity

**Date** 2026-09-28. **Spend** $0.00. Zero DeepSeek API calls were made. Every
figure below comes from source already in the tree, artifacts already on disk,
or a fixture reproduction in a scratch directory.

**Evidence classes** OBSERVED, DERIVED, ASSUMED, NOT_OBSERVED, INVALID. Nothing
is promoted between them. Experiments classified INVALID are preserved and
registered in `../invalid/register.md`.

| task | question | result |
|---|---|---|
| D1 | is the stale-bytecode mechanism what was claimed? | **CONFIRMED** |
| D2 | do float hazards remain on the money paths? | **PARTIALLY SUPPORTED**: 3 new defects, 2 decision-changing |
| D3 | is every billable request fully evidenced? | **PARTIALLY SUPPORTED**: 479/479 paired, 2 without a timestamp, 1 open hole now closed |
| D4 | are the prior experiments valid? | **8 VALID, 12 LIMITED, 3 INVALID, 1 unrun** |

Harness suite after this work: **9 test files, 128 tests, 0 failing**
(`make harness-test`). Every guard added here was mutation-checked: 8 mutations
applied, **8 killed, 0 survived**, with a restore-and-run control after each.

---

## D1. Mutation test validity

### Claim under test

A prior mutation sweep rewrote a module, tested, restored the original with
`cp`, and CPython reused the mutant's cached bytecode because `cp` gave the
restored file an mtime inside the same whole second as the `__pycache__` entry.
Three mutations read as SURVIVED against code no longer on disk, and the
restored unmutated file read as FAILING.

### Procedure

Reproduced in `/tmp/d1repro`, outside the harness, on CPython 3.14.7. A
two-line module and a one-line importer. Four constructions:

- **A** original compiled, then a same-length mutant written with `os.utime`
  set into the whole second the `.pyc` recorded.
- **B** mutant compiled, then the original restored into the recorded second.
- **C** the same two sequences with `PYTHONDONTWRITEBYTECODE=1` and
  `rm -rf __pycache__`.
- **D** a mutation that changes the file's byte length.
- **E** twenty trials of the natural race: plain `cp`, no clock manipulation.

### Observations, OBSERVED

The timestamp `.pyc` header (flags=0) records exactly two facts about its
source: `source_mtime`, truncated to a whole second, and `source_size`. CPython
reuses the cached bytecode whenever both still match.

| construction | source on disk says | interpreter ran | reading |
|---|---|---|---|
| A: mutant, same second, same size | `return 2` | `1` | mutation **falsely SURVIVED** |
| B: original restored, same second | `return 1` | `2` | control **falsely FAILING** |
| C: guard in force | `return 2` / `return 1` | `2` / `1` | correct both ways |
| D: mutant one byte longer | `return 22` | `22` | correct, `.pyc` rejected on size |
| E: natural `cp` race, no `utime` | mutant | original | **20 of 20 trials stale** |

### Result: CONFIRMED

The mechanism is exactly as claimed, and it reproduces without any clock
manipulation at all: 20 out of 20 plain `cp` cycles executed bytecode that was
not on disk. Two refinements the original account did not state:

1. **The size field is a partial guard.** A mutation that changes the file's
   byte length is rejected, so an operator swap like `>` to `>=` was tested
   correctly. Only same-length mutations slip through in direction A.
2. **The restore direction has no such protection.** The restored file is by
   definition the same length as the original, so direction B, the falsely
   failing control, can follow *any* mutation whatsoever. This is why the
   symptom presented as "three SURVIVED plus a failing control" rather than as
   three quiet false negatives.

### How it was detected

By the control, not by the sweep. A restore-and-run of the unmutated file read
as FAILING, which is impossible if the interpreter is running the source. A
sweep that reports only per-mutation verdicts and never re-runs the restored
baseline has no detector for this at all: direction A alone is silent.

### Permanent guard

Two lines, already carried by `scripts/harness-test`:

```text
export PYTHONDONTWRITEBYTECODE=1
rm -rf experiment/harness/__pycache__
```

No `.pyc` is written, so none can be reused, so the question cannot arise. Note
that a stale `experiment/harness/__pycache__/pricing.cpython-314.pyc` was
present in this worktree at the start of Track D, written by some run that did
not use the guard. It is gitignored, not tracked, and was removed.

### Implemented: `experiment/harness/cleanstate.py` with `test_cleanstate.py`

The guard above prevents the defect. It does not detect a tree that has already
been poisoned, and a sweep that ran without it is not repaired by adding it
afterwards. `cleanstate.py` is the second half:

- `read_header` / `would_be_reused` reimplement CPython's acceptance test
  rather than asking the import system, which is the component under suspicion.
- `bytecode_matches_source` marshals the cached code object, compiles the
  source fresh, and compares a structural digest.
- `inspect` classifies each cache entry: `ACCEPTED_AND_STALE` is the D1 defect
  and nothing else is; `ACCEPTED_AND_FRESH`, `REJECTED`, `NO_SOURCE` and
  `UNREADABLE` are the others.
- `assert_clean(*dirs, strict=True)` raises `CleanStateError` when bytecode
  writing is enabled, when any entry is `ACCEPTED_AND_STALE`, or (strict) when
  any `__pycache__` exists at all, which is the right setting for a sweep.

11 tests. One of them constructs the defect and then checks a real subprocess
actually executes the stale bytecode, so the detector is validated against
CPython's behaviour rather than against its own model of it. Three are negative
controls: a matching entry must read `ACCEPTED_AND_FRESH`, a size-changing edit
must read `REJECTED`, and a tree with no cache must pass in strict mode.
`TestTheHarnessRunsUnderTheGuard` fails if `scripts/harness-test` ever loses its
`PYTHONDONTWRITEBYTECODE` line.

### Remaining gaps

- `assert_clean` is available but is not yet called by any sweep driver, because
  no sweep driver is checked in. A future sweep must call it before the first
  mutation and after the final restore.
- The digest comparison excludes `co_filename` and `co_firstlineno`. A mutation
  that changes only line numbers and nothing else would compare equal. It also
  cannot change behaviour, so this is a stated limit rather than a hole.
- PEP 552 hash-based `.pyc` files are handled but were not exercised against a
  real one; this platform writes timestamp-based caches.

---

## D2. Floating-point accounting

### Procedure

Every monetary expression in `pricing.py`, `meter.py`, `fanout.py`,
`session.py` and `reconstruct.py` was read and classified as either a
**comparison against a threshold**, which can change a decision, or a
**display or aggregate figure**, which cannot. The spending scripts
(`arms.py`, `cacheprobe.py`, `cacheclean.py`, `cachefalsify.py`,
`reasoning_scope.py`, `reconcile.py`, `settle_test.py`) were read for their use
of those paths but not executed.

### The audit

| site | kind | verdict |
|---|---|---|
| `fanout.Budget.reserve` `committed + settled + est > ceiling` | threshold | **safe**, all integer nano-USD |
| `fanout._nano` | conversion | **safe**, exact for every published rate |
| `pricing.is_peak` `1 <= h < 4`, `6 <= h < 10` on `hour + minute/60` | **threshold**, 2x rate | **safe**, proven exhaustively, now pinned |
| `pricing.cost_usd` | display, DERIVED | tolerable, single expression, no accumulation |
| `meter.derived_usd += usd` | aggregate | tolerable, never compared to a ceiling |
| `meter.record` `pct = 100*total/ceiling` | display | guarded against a zero ceiling |
| `meter.summary` `100*derived/ceiling` | display | **DEFECT, fixed** |
| `meter._fmt_usd` `v < 0.01` | display | tolerable |
| `session.ask` settle path | threshold via Budget | safe, floats convert at the boundary |
| `reconstruct` `hit + miss != prompt_total` | threshold | safe, integers |
| `reconstruct` reconciliation interval | **threshold** | **DEFECT, fixed** |
| `arms.py` `round(m.derived_usd - before, 6)` then `base/r` | headline ratio | **hazard, reported not fixed** |

### New defect 1: the reconciliation interval excluded its own lower bound

`reconstruct.py` decided whether a DERIVED total lies inside what the OBSERVED
balance can distinguish, with `lo, hi = obs - 0.005, obs + 0.005` and
`lo <= derived < hi`. At an observed $0.05, `0.05 - 0.005` evaluates to
`0.045000000000000005`, which is **above** the bound it represents. A derived
total of exactly $0.045 was therefore reported as outside an interval it is the
endpoint of: a reconciliation reading as failed when it passed.

This is a threshold comparison on money and it decides a published verdict, so
it is in the dangerous class. Fixed by extracting `within_resolution(observed,
derived)`, which builds the interval in integer nano-USD. The old float form is
kept inside the test as the falsifier, so if the fix is ever reverted the test
also proves the revert reintroduces the defect.

Probability of a real corpus landing exactly on the boundary is very low. That
is a reason to expect it not to fire, not a reason for the guard to be wrong.

**Interaction with Track B, recorded so neither fix eats the other.** Track B
found a second and orthogonal defect in the same function: the interval is half
its true width, because the quantity being reconciled is a *difference of two*
cent-resolution balance readings, each carrying its own quantisation, so the
correct bound is plus or minus $0.01 and not $0.005. Track B deferred the fix to
avoid a concurrent-edit clobber and recorded it in
`results/DEFECT-interval-width.md`. The two findings are independent: Track D
fixed *where the boundary lies*, Track B will fix *how wide the interval is*.
`within_resolution` is now the one place that fix has to land, and the Track D
tests read the width from `reconstruct.HALF_CENT_NANO` rather than hardcoding
it, so widening the constant is a one-line change. This was verified: the whole
of `test_money.py` passes unchanged at both $0.005 and $0.01. The DS-F5
conclusion is unaffected either way, because its two hypotheses are 29x apart.

### New defect 2: a zero ceiling crashed the run report

`Meter.record` guards `100 * total / self.ceiling` against a zero ceiling.
`Meter.summary` did not, and raised `ZeroDivisionError`. A run constructed with
a zero ceiling refuses every call, which is precisely the run whose summary
someone needs to read. Fixed, with a positive control asserting a real spend
still prints its real percentage.

### New hazard 3: per-arm cost is a difference of two float running totals

`arms.py` records each arm's cost as `round(m.derived_usd - before, 6)` and then
publishes `base["usd"] / r["usd"]`, which is the source of the DS-OPT "6.5x
cheaper" headline. Two problems, neither fixed here because fixing them means
re-running a spending experiment:

- the subtraction inherits `meter.derived_usd`'s accumulated drift;
- `round(..., 6)` quantises to one micro-dollar, and an arm costing less than
  $0.0000005 rounds to exactly zero, at which point the published ratio becomes
  `float("inf")`.

The drift is OBSERVED in the campaign's own artifacts. `tmp/probe/arms.json`
reports `settled_usd` 0.043330728 against `derived_usd` 0.043330728000000006
for the same 72 calls; `rscope.json` reports 0.0921174 against
0.09211739999999996; `rscope2.json` 0.041821632 against 0.04182163200000001. The
exact figure is the integer nano one. This is direct evidence that the nano
conversion is doing its job and that the meter's float total is not the figure
to quote.

### What is provably safe

`pricing.is_peak` computes `h = hour + minute / 60` and compares that float
against four thresholds that decide a 2x rate. It is safe, and "happens to be
safe" is not a property anyone should have to re-derive, so it is now pinned:
all 1,440 minutes of a weekday are asserted against an exact integer rule, both
edges of all four boundaries are asserted individually, and the weekend is
asserted off-peak at every hour. The reason it is safe is that `minute / 60` is
exactly 0.0 when `minute` is 0 and strictly between 0 and 1 otherwise, so no
value can land on a boundary except exactly, and every boundary falls on a whole
hour.

`is_peak` does have a real limitation, already documented in its own docstring
and not a float issue: Chinese public holidays are excluded from peak by the
provider and are not modelled, so it over-states cost on those days.

### Result: PARTIALLY SUPPORTED

The `fanout.Budget` conversion to integer nano-USD holds and is now pinned at
the exact ceiling where it first bit. The premise that converting it closed the
question does not hold: two further defects were found on threshold
comparisons, both fixed with regression tests, and one hazard was found on a
published headline and is reported rather than fixed.

`test_money.py`, 13 tests. Mutation-checked: reverting the interval to floats,
widening its upper bound to inclusive, and restoring the unguarded division are
each killed.

### Remaining gaps

- The DS-OPT arm ratios are not recomputed. Doing it properly means re-deriving
  each arm's cost from its saved response artifacts rather than from a meter
  delta, which is free and offline but is a re-analysis of another track's
  headline, not an instrumentation fix.
- `pricing.cost_usd` takes no `cache_write` argument, so the Anthropic dialect's
  `cache_creation_input_tokens` is priced at zero. The provider publishes no
  cache-write charge, so zero may be correct; it is ASSUMED, not verified. Only
  2 of 479 saved responses use that dialect and both report 0 creation tokens,
  so nothing in the current corpus depends on it.

---

## D3. Artifact completeness

### Procedure

Every file in `/Users/daniel/.claude/jobs/581b6292/tmp/probe/` was read offline
and each saved response checked for the six properties a billable request must
carry: a request artifact, a response artifact, a timestamp, a model, a usage
block, and a derivable pricing regime. The ledger files were then compared
against the artifact count to look for billable calls with no artifact at all.
Then the harness source was read for paths that can bill money without writing
one.

### Observations, OBSERVED

| property | count | gap |
|---|---|---|
| request artifacts `req-*.json` | 479 | 0 |
| response artifacts `res-*.json` | 479 | 0 |
| request with no matching response | 0 | 0 |
| response with no matching request | 0 | 0 |
| responses carrying a usage block | 479 / 479 | **0** |
| responses carrying a model | 479 / 479 | **0** |
| responses carrying a `created` timestamp | 477 / 479 | **2** |
| model priceable from the published table | 479 / 479 | 0 |
| conservation `hit + miss == prompt_tokens` | 477 / 477 chat | 0 violations |
| responses carrying a provider error | 0 | 0 |

Models: `deepseek-flash` 467, `deepseek-v4-pro` 12. Dialects: chat 477,
Anthropic 2. Window, from the provider's own `created`: 2026-09-29 03:26:36 to
03:42:52 UTC.

**The 2 gaps.** `res-05abd72c.json` and `res-8fc0063c.json` are the two
`/anthropic/v1/messages` responses. That dialect returns no `created` field, so
their timestamp is absent at the source, and with it the pricing regime.
`reconstruct.py` substitutes the corpus-wide latest timestamp for them, which is
an ASSUMED regime presented alongside 477 derived ones. In this corpus the
substitution happens to be harmless, because all 479 calls fall inside a
sixteen-minute window that is entirely peak, but the code would make the same
substitution across a regime boundary without saying so.

**No ledgered call is missing an artifact.** The result files account for
72 + 29 + 10 + 10 + 73 + 85 = 279 calls in their summary blocks, plus
`reconcile.json`'s 200 rows, which is exactly 479.

**A residual the artifacts cannot close.** `reconcile.json` records its own
balance bracket, $46.00 before and $45.96 after. It finished at 20:35:49 local,
and `rscope` and `rscope2` ran after it, deriving $0.134. The first settle poll
in `balance_poll.jsonl`, at 05:15 UTC, reads $45.78, which is $0.18 below
$45.96. That leaves about $0.046 unexplained, against a balance instrument whose
resolution is $0.01, so the residual survives its own error bar. The candidates
are settlement lag on earlier calls, cent quantisation, and billable calls whose
responses were never saved. **The artifacts cannot distinguish them.** There is
no balance read taken before the first call in this corpus, so a corpus-wide
reconciliation is not possible from these files at all. This is NOT_OBSERVED. It
is not zero and it is not evidence of unsaved calls either.

### The hole in the harness, now closed

`session.py` is the enforced path and `TestNoScriptBypassesTheRunner` enforces
it statically. Artifact persistence was **not** enforced the same way, and the
harness could produce an unsaved billable call in two distinct ways. Both are
now closed, and both fixes are mutation-checked.

**Runtime.** `adapter._post` ran `curl` with `-o <path>` and then did
`try: json.load(open(o)) except Exception: doc = {}`. If curl reported HTTP 200
and wrote no output file, that exception was swallowed. Every usage field then
stayed `None`, `pricing.cost_usd` returned `None` on a `None` input, `meter`
coerced that to `$0.00`, and `Budget.settle(0.0)` banked a real billed call as
free. The row read `ANSWERED`. This was reproduced against the current code
before the fix.

`_post` now raises `adapter.ArtifactMissing` when the response file is absent or
empty. A body that is **on disk but unparseable** deliberately does not raise:
the bytes the provider sent are saved, classifiable and re-readable, so the call
remains auditable and must not be turned into an exception that discards it.

`session.ask` now refuses to bank a `200` whose usage block never arrived. It
returns `Outcome.ERROR` with an `UNACCOUNTED` reason and **holds** the
reservation rather than releasing it, because the provider answered and the
money is gone. Under-running is the safe error for scarce capital, which is the
same rule the module already applies to an unknown exception.

**Static.** `transport.py` is a pooled `requests.Session` that posts to the
provider and writes no artifact at all. It is documented as the replacement for
the curl subprocess, it has 7 tests, and nothing stopped a future script
importing it. `t.py` is a byte-identical copy of it. Neither is imported by
anything today. `TestNoModuleReachesTheProviderWithoutPersisting` now enforces
the architecture against the source, in the same shape as the adapter guard:
every module containing a POST marker must be either declared as persisting
(`adapter.py`, checked to actually write both artifacts and to carry the raise)
or quarantined (`transport.py`, `t.py`, checked to actually persist nothing), and
no non-test module may import a quarantined one. A scan that inspects fewer than
six files fails, so the guard cannot pass by finding nothing.

### Result: PARTIALLY SUPPORTED

The corpus is complete on five of the six required properties for all 479
requests and on the sixth for 477. No ledgered billable call is missing an
artifact. The harness could nevertheless produce an unsaved billable call, and
that is now prevented at runtime and guarded statically. `test_persistence.py`,
12 tests, 2 mutations applied and both killed.

### Remaining gaps

- The two Anthropic-dialect responses have no timestamp at the source. The fix
  is to record the client-side send time in the request artifact, which
  `adapter._post` already writes and could timestamp for free. Not implemented:
  it changes the artifact schema, and the corpus is frozen.
- `reconstruct.py` still imputes a regime for a timestamp-less response without
  marking it ASSUMED in its output.
- The $0.046 residual stays NOT_OBSERVED. Closing it needs a balance read taken
  before the first call of a run, which is a protocol change, not a code change.
- `t.py` and `f.py` are near-duplicates of `transport.py` and `fanout.py`. A
  mutation sweep against `fanout.py` leaves `f.py` untouched, so a script
  importing `f` would run unmutated code. They are quarantined, not deleted.

---

## D4. Experimental question validity

### Procedure

Every markdown file under `experiment/deepseek/`, plus `EXPERIMENT-LEDGER.md`,
`EXPERIMENT-STATUS.md` and the suite README, was read against six threat
classes: hidden reasoning requirement, ambiguous wording, incorrect ground
truth, model-dependent scoring, uncontrolled variables, post-hoc hypothesis. The
non-executing source of the task and checker definitions was read alongside,
because a task's real content is in its checker, not in its label. Nothing was
run and nothing was discarded.

24 distinct experiments were found across 15 artifacts.

### The known case: CONFIRMED

The task class labelled "mechanical lookup" asked which function was declared on
a given line number of a 48,167-character document. That is positional counting,
not lookup. The literal prompt string was never committed, because the first
commit of `reasoning_scope.py` already contains the repaired version, but three
descriptions survive:

- `experiment/deepseek/reasoning-scope-prereg-2026-09-29.md:20-21` still
  specifies it as the registered design, and carries no amendment.
- `experiment/deepseek/reasoning-scope-2026-09-29.md:81-85` is the project's own
  confession: reasoning-on truncated 12 of 12 at `max_tokens` 1024 and
  reasoning-off scored 0 of 12. Neither cell was a model result.
- `experiment/harness/reasoning_scope.py:38-41` records the same in a comment.

The finding the project did not record: the void run was correctly discarded,
but the M class was then silently **redesigned** from line-number to
string-constant lookup, and the prereg was never amended. The published DS-RS
therefore did not execute its registered protocol, and the redesign happened
after seeing the void data. That is a prereg deviation, which is why DS-RS is
LIMITED and not VALID despite a very large and almost certainly real effect.

### Tally

**VALID, 8.** DS-BASE, DS-SURF, DS-F3, DS-F4b, DS-F5, DS-DOC, DS-WF, DS-RW.
(DS-WF, DS-RW and HH-01 are engineering work with no empirical claim; HH-01 is
counted under LIMITED's sibling HH-02 in the register only where a claim exists.)

**LIMITED, 12.** DS-GAP, DS-CONC, DS-F1, DS-F4a, DS-C1, DS-C2, DS-F2b, DS-OPT,
DS-RS, WP-01, WP-02, HH-02.

**INVALID, 3.** DS-SURF-PO, DS-F2, DS-F4. All three were already withdrawn or
corrected by the project itself. All three are preserved in
`../invalid/register.md`.

**Not run, 1.** `settle_test` BIG/SMALL. Designed, refuses without `--confirm`,
never executed. No result exists, so it is neither valid nor invalid.

Full one-line reasons are in the register for the INVALID set. The LIMITED set
is limited for these recurring reasons, in order of how many experiments they
touch:

1. **n = 1 run per configuration.** DS-OPT, DS-C1, DS-C2, DS-F2b and DS-F5 each
   ran their configuration once. Repetition counts are within-run, so no
   run-to-run variance exists behind any cost or latency headline.
2. **Permissive checkers.** `arms.py` scores a bare substring match for a
   six-digit value against a document containing twelve six-digit values, and
   every arm scored 12 of 12. `reasoning_scope.py` scores
   `str(n) in re.findall(r"\d+", text)`, so any answer containing the digit
   anywhere passes; at a ground truth of 2 that is very likely from any prose.
   Mitigating for DS-RS specifically: the reasoning-on cell emitted about 2.4
   content tokens per call, so those answers were bare numbers, and the
   permissiveness if anything inflates the reasoning-off cell, making the
   reported 100% to 29% effect conservative.
3. **DERIVED presented as a headline.** Every cost ratio except DS-F4b and
   DS-F5 is arithmetic over a published table. DS-C1, DS-RS and every DS-OPT arm
   cost less than the balance endpoint's $0.01 resolution and were never
   reconciled against money at all.
4. **Regime attribution is unrecorded.** `meter.record` prices each call at
   `now()` and `meter.summary` labels the whole run at summary time, so a run
   crossing 04:00 or 10:00 UTC would mix regimes inside one arm and then print
   one label. No artifact carries a per-call UTC timestamp, so the campaign
   prereg's own peak/off-peak control cannot be enforced retrospectively.
5. **Post-hoc rescue, twice.** DS-C1 refit its block model after a failed
   prediction (the preregistered falsification round was 4 of 5, not 5 of 5,
   and the ledger's "17/17 rungs exact" is the post-hoc refit number). DS-RS
   redesigned its task class after a void run. Both are disclosed in the
   artifacts; neither was re-registered; both headlines quote the post-rescue
   figure.

### A live sibling of the known defect

`tasks.py:32-33` labels a counting task "A: mechanical", and DS-F1 reports that
cell as mechanical and finds reasoning-**off** better, 18 of 20 against 9 of 20.
DS-RS classifies counting as **aggregative** and finds reasoning-off
catastrophic at 29%, and `policy.py` ships the DS-RS rule. Nothing in the ledger
reconciles the two. This is the same defect class as the line-number case, in a
task class that is still live, and it is the single highest-value thing to
re-run.

### Ground truth defect

`reasoning_scope.py:69-74` asks for every line beginning with the exact five
characters `type` plus a space, and scores against `re.findall(r"^type \w+", corpus, re.M)`.
The regex requires a word character after the space, so a Go grouped
declaration `type (`, which the question counts, is excluded from the truth. It
also counts occurrences rather than lines. The published truth of 14 is the
regex's answer, not the question's, and a model answering the question as
literally worded could be scored wrong.

### Numbers in the ledger their own sources do not support

- "17/17 rungs exact" (`EXPERIMENT-LEDGER.md:23`, `policy.py:19`,
  `docs/DEEPSEEK-OPTIMIZATION.md:56`) against 4 of 5 preregistered and 5 of 5
  fully observed in `cache-characterisation-2026-09-29.md:27-29`.
- Three incompatible campaign spend totals: $0.18 observed
  (`docs-reconciliation-2026-09-29.md:85`), about $0.86
  (`EXPERIMENT-STATUS.md:25`), $0.23 OBSERVED
  (`deepseek-suite/README.md:41`). Only the $0.23 figure is defensible and it is
  worded as if cumulative.
- `EXPERIMENT-STATUS.md:3` is dated 2026-09-28 on a body reporting the
  2026-09-29 sweep.

### Result: classification delivered, known case CONFIRMED

8 VALID, 12 LIMITED, 3 INVALID, 1 designed but never run. Nothing discarded.

### Remaining gaps

- The three INVALID experiments were already withdrawn by the project, so this
  track found no *undisclosed* invalidity. That is a real and unusual credit to
  the campaign's self-correction discipline, and it also means the classification
  rests largely on the project's own confessions.
- The DS-F1 counting cell is classified LIMITED rather than INVALID because its
  extraction and ordering cells are sound and separable. If the DS-RS
  classification is correct, the counting cell alone is INVALID and should move.
  Deciding that needs a re-run, not a re-read.
- No experiment was re-scored. The permissive-checker finding bounds the
  direction of the error for DS-RS and does not bound it for DS-OPT.
