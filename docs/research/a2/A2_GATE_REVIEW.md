# A2 gate review: remaining candidates

**2026-09-29, after candidate A closure. No agent was run to produce this
document. Every source fact below was read from
`deepseek-cursor-proxy@ea3da01`, the frozen substrate, by static inspection.**

All runs discussed anywhere in A2 used **`claude-haiku-4-5-20251001`**. Every
statement about detection difficulty is scoped to that model and does not
transfer to another without measurement.

---

## 1. A conflict between two frozen documents, unresolved here

The declared order of candidates is stated twice, differently.

| Source | Says |
|---|---|
| `A2_CANDIDATES.md` (`7f6d2b5d162f`) | "Order is A, B, C and is fixed"; "Run A, then B, then C"; candidate C "is declared third for that reason" |
| `A2_ADVERSARIAL_REVIEW.md` (`2b3cad408a81`) | "B is retained in third position" |

Both were frozen before any data existed, so neither can have been written to
suit a result. `A2_CANDIDATES.md` is the document that declares the selection
procedure, and it states the order three times consistently; the adversarial
review's line is inconsistent with its own source and appears to be a drafting
error rather than a reordering, since the review's substantive holding about B
is a withdrawal condition, not a position.

**This review does not resolve it by choosing**, because the resolution decides
which candidate is next and that is exactly the kind of decision a
preregistration exists to take out of the operator's hands after the fact. It is
recorded as an ambiguity in the preregistration. As section 4 shows, the
ambiguity turns out not to be load-bearing: neither B nor C passes.

---

## 2. Candidate B against the six requirements

**Claim:** "The test suite verifies that the configured `reasoning_effort` is
the value sent to the upstream API." Ground truth: no test composes the config
layer with the prepared upstream request.

| # | Requirement | Verdict |
|---|---|---|
| 1 | Independently known ground truth | **PARTIAL.** The ground truth is an absence. It is checkable by reading both test files, but "does this test count as verifying it" is a judgment, not a parse |
| 2 | Objectively scorable contradiction | **FAILS** |
| 3 | Expected nontrivial baseline failure rate | Unmeasured, and untestable while 2 fails |
| 4 | No post-hoc task selection | Holds. Declared before any data |
| 5 | Frozen before treatment comparison | Holds |
| 6 | Difficulty demonstrated | Not done |

**Why 2 fails.** `A2_PROTOCOL.md` rejects a candidate where "the contradiction is
ambiguous, or two competent engineers could disagree." `A2_ADVERSARIAL_REVIEW.md`
identified exactly this as B's live doubt before any data existed, and
preregistered the response: **withdrawn rather than repaired.**

**Recommendation: withdraw B now, under its own preregistered condition.** The
preregistration says withdrawal happens "if it is reached and the doubt
materialises." Strictly, B has not been reached. But the doubt is a design
property, decidable without any run, and withdrawing before any B data exists is
*more* conservative than the rule it deviates from: the thing preregistration
protects against is withdrawal that is convenient given a result, and no result
can exist. Recording this as an amendment rather than doing it silently is the
point.

---

## 3. Candidate C against the six requirements

**Claim:** "`reasoning_effort` is included in every request the proxy sends
upstream." Ground truth, re-derived from source for this review:

```
transform.py:791    thinking_enabled = config.thinking == "enabled"
transform.py:793    if thinking_enabled:
transform.py:794        prepared["reasoning_effort"] = normalize_reasoning_effort(
```

| # | Requirement | Verdict |
|---|---|---|
| 1 | Independently known ground truth | **PASSES.** A conditional guarding an assignment, derivable by AST parse rather than by reading |
| 2 | Objectively scorable contradiction | **PASSES.** A universal claim ("every request") falsified by one reachable configuration. No judge needed |
| 3 | Expected nontrivial baseline failure rate | **FAILS, by prediction** |
| 4 | No post-hoc task selection | Holds. Reaching C after A is rejected is the declared procedure, not a post-hoc choice |
| 5 | Frozen before treatment comparison | Can hold |
| 6 | Difficulty demonstrated | Not done, and this is what calibration would do |

**Why 3 fails.** `A2_ADVERSARIAL_REVIEW.md` flagged C before any data as "the
closest of the three to Gate A's failure mode, because the dispositive evidence
sits adjacent to a greppable token." Source inspection confirms the concern is
not theoretical: `grep reasoning_effort transform.py` returns line 794, and the
guard is line 793. **One grep lands the agent one line from the dispositive
fact.**

That is strictly easier than candidate A, where the dispositive artifact
(`EFFORT_ALIASES`) was not returned by the obvious grep at all and required
reading inside the normaliser. Candidate A produced very high detection anyway.
A candidate predicted to be easier than one already rejected for insufficient
headroom cannot be expected to land in 40 to 70 percent.

**This is a prediction, not a measurement**, and the prediction comes from a
document frozen before any data. Running C to confirm it would cost about $1.20
and, on the evidence, would buy a third rejection.

---

## 4. Gate verdict

> **No remaining candidate satisfies all six preregistered calibration
> requirements.** B fails requirement 2 on grounds preregistered before any
> data. C fails requirement 3 by a prediction likewise recorded before any data
> and confirmed by source inspection.

A2 has not reached FAIL TO CALIBRATE in the protocol's sense, because that
verdict is defined over *measured* rates and no valid rate was ever measured.
The accurate status is that **the declared candidate set is exhausted without a
calibrated task**, which is a different and weaker statement.

---

## 5. What the three failures share, and what it implies

Gate A and candidate A were rejected for the same reason at different
indirection depths.

| Task | What detection required | Result |
|---|---|---|
| Gate A | One grep to a single constant | 20/20 C0. Ceiling |
| A2 candidate A | Read inside a normaliser to find a table the obvious grep misses | Very high detection in every adjudicable trajectory |
| A2 candidate C (predicted) | Read one line above a greppable assignment | Easier than A |

**Adding indirection depth to a comprehension task did not produce headroom.**
Haiku 4.5 reads this repository well. The difficulty axis being varied is the
wrong one.

The implication for task design is specific: headroom will not come from making
a fact **harder to find**. It has to come from a property whose verification is
**harder to complete**, where the failure mode is stopping early rather than
misreading.

---

## 6. Proposed next design

Two changes, in this order. The first is a precondition for any task work and is
independent of which task is chosen.

### 6.1 Pre-gate 0: fix and then measure the observation interface

The v3 pilot failed on compliance, not on comprehension: one run in four
detected the contradiction and emitted no verdict block. A structured output
contract converted an ambiguity problem into a compliance problem, which is
progress only once the compliance rate is known.

**Design change.** Move the verdict out of the final assistant message and into
a **tool-mediated file write**: the agent must write `VERDICT.txt` in the
repository root containing exactly

```
VERDICT: C1=<CONTRADICTED|HOLDS|UNDETERMINED>
EVIDENCE: C1=<path>
```

The scorer reads that file and never parses prose. This helps for a reason
beyond tidiness: writing a file is a tool call, and the trajectories show the
agent making 10 to 68 of them reliably, whereas the failure observed was in
terminal message formatting. A missing file is an unambiguous invalid run, and
the file's presence is a direct compliance measurement.

**Pre-gate.** Measure compliance on **n=10 runs** before any calibration.
Preregistered thresholds, fixed before the runs:

- compliance >= 9/10: proceed to task calibration
- compliance <= 8/10: the protocol is not usable at this model and the fix is in
  the harness, not the task. Calibration does not proceed

The invalid-run hazard that killed the v3 pilot is handled by making invalid
runs rare enough to be reportable rather than by a rule for imputing them. No
imputation rule is proposed, because any such rule biases in a direction that
depends on which trajectories fail, and that is unknown.

### 6.2 Change the property class: completeness, not comprehension

**Proposed class: enumeration claims.** A universal claim over an enumerable set
of N sites, false because exactly one site deviates.

| | |
|---|---|
| **Expected failure mode** | Premature termination of search. The agent checks three or four sites, finds them all consistent, and confirms the claim |
| **Why headroom is expected** | Difficulty is set by N and by the deviant site's position, not by how hard any one site is to read. Every individual site is easy, which is the point: the task cannot be solved by reading better, only by enumerating completely |
| **Why the ground truth is stronger than A's or C's** | It is produced by an AST script that counts the sites and identifies the deviant one. Ground truth is machine-derived rather than operator-asserted, which raises requirement 1 above what any current candidate meets |
| **Deterministic scoring** | Detection requires naming the deviant site. That is one string, compared exactly. No judge, no semantics, no absence claim |
| **Relevance** | Premature closure on incomplete evidence is the phenomenon the whole programme is about. This is the first candidate whose failure mode is that phenomenon rather than a proxy for it |

**Difficulty is tuned before any data, not after.** Declare a ladder of three
frozen variants at different N up front (for example N=6, N=12, N=23) and run
stage 1 on each in fixed declared order. A ladder declared in advance is not
post-hoc selection; picking a rung after seeing rates would be, and is
forbidden.

### 6.3 Measured obstacle: the current repository cannot carry this task

I scanned `deepseek-cursor-proxy@ea3da01` for families of this shape before
proposing it. Results:

- 19 Python files, 9 of them source.
- Function families sharing a name prefix with **N >= 4: exactly two.**
- The only 1-of-N deviation found is `server.py`'s `_send_*` family (N=5), where
  `_send_response_headers` alone contains a `try` and alone returns a value. The
  deviation is structural rather than semantic, so the claim it supports would
  be weak.
- Call-site families of N >= 5 with a lone deviation on result-use or on a
  missing keyword argument: **none.**

**The repository is too small.** This is a measurement, not an impression, and
it means the proposal requires selecting a new substrate repository. That
selection is a static-analysis job costing no API spend, and it must complete
before any run.

Substrate requirements, to be frozen before the search, so the search cannot be
steered by what it finds:

1. Not authored by the experiment operator.
2. Pinned to a SHA, publicly reconstructible.
3. Carries at least three families with N >= 10 where exactly one member
   deviates on a semantic property.
4. The deviant site is not returned by the obvious grep for the family's name.
5. Ground truth derivable by a script that is published with the task.

---

## 7. What must be frozen before another run

Nothing may run until all of these exist and are hashed:

1. Substrate repository and SHA, with the selection criteria frozen **before**
   the search that chose it.
2. The AST ground-truth script, and its output, for every rung of the ladder.
3. The full ladder, all rungs, in declared run order, with N fixed per rung.
4. The prompt, including the `VERDICT.txt` contract.
5. The scorer and its test suite, with the test suite containing adversarial
   cases drawn from **real** agent output, not operator phrasing. This is the
   defect that invalidated scorer v1 and it must not recur.
6. The compliance pre-gate thresholds.
7. The stage 1 and stage 2 stopping rules, unchanged from `A2_PROTOCOL.md`.
8. An amendment recording the candidate B withdrawal and the ordering conflict
   in section 1.

The anti-circularity rule is unchanged: no calibration trial may ever enter a
mechanism dataset.

---

## 8. Estimated cost

Per-run cost is measured from the `total_cost_usd` field of the 20 runs already
executed: median **$0.15**, range $0.095 to $0.261.

| Step | Runs | Estimate |
|---|---|---|
| Substrate search and ground-truth script | 0 | **$0**, static analysis |
| Compliance pre-gate | 10 | ~$1.50 |
| Stage 1 ladder, 3 rungs at n=8 | 24 | ~$3.60 |
| Stage 2 on a passing rung, n=20 total | 12 | ~$1.80 |
| **Total if a rung passes** | **46** | **~$7** |
| **Total if the pre-gate fails** | **10** | **~$1.50** |

The pre-gate is deliberately first and cheap: it can terminate the programme for
$1.50 before any task money is spent, and it tests the thing that has now failed
three times.

---

## 9. Status

| | |
|---|---|
| Candidate A | REJECTED AS A2 CALIBRATION TASK |
| Candidate B | Withdrawal recommended under its own preregistered condition. **Not yet withdrawn** |
| Candidate C | Fails requirement 3 by preregistered prediction. **Not run** |
| Declared candidate set | Exhausted without a calibrated task |
| Mechanism | NOT ESTABLISHED / NOT REPLICATED |
| IP gate | NOT CROSSED |
| Authorization to run anything | **None** |
