# LOCKED ROOM VII: work continuity

**2026-10-02. Eight fresh-agent conditions, one task, independently held ground
truth. No production change. Nothing committed.**

The question: can a fresh agent with zero prior conversational context
determine, from durable evidence, what work is underway, what is established,
what is unresolved, what supports those conclusions, and what to do next,
without replaying the conversation or reading a hand-maintained cursor?

Every substantive statement is labelled **MEASURED**, **INFERRED**,
**HYPOTHESIS** or **NOT MEASURED**.

---

## 1. Executive verdict

**Yes, and Replay contributes nothing to it.** Eight fresh agents, each with no
prior context, were given the same paused software task under different
continuity substrates. **Seven of the eight scored at ceiling on all six
dimensions** — objective, established facts, unresolved questions, prior
actions, next action, evidence trace — including the condition that received
nothing but the repository. All seven identified the planted false lead as
rejected, refused the planted bad recommendation, and read a failing test as
intended rather than broken. What produced the effect is not a substrate: it is
**commit-message bodies that record standing rather than change, a failing test
whose message is the measurement, and a disagreement materialised as two files
in the tree rather than held in someone's head.** Replay's own contribution,
measured, is the string "No record files were found by this scan." The
hand-maintained `WORK-STATE.md` was **net negative**: it asserted a decision the
repository records as open, and the agent had to defeat it. The
machine-readable state file was **net neutral to negative**: the agent spent its
effort auditing the file rather than the work, and found six discrepancies.
Removing the evidence behind one claim did not produce a confabulated claim; it
produced a correctly-bounded one, re-derived from a different artifact. **The
eighth condition is the one that locates the boundary**: strip the statements of
*standing* from an otherwise identical repository and continuity collapses on
exactly three points, and the agent then reaches the same bad recommendation
the transcript condition had rejected. The thesis that Replay needs a
work-continuity substrate is **killed**; what remains is three sentences of
commit prose, which is ordinary engineering and already this repository's house
style.

---

## 2. Repository substrate

**MEASURED.** What exists in Replay today, traced from source rather than
inferred from package names.

| vocabulary | non-test occurrences |
|---|---|
| `WorkID`, `work_id`, `TaskID`, `task_id` | **0** |
| `Handoff`, `Resume`, `Checkpoint`, `NextAction`, `next_action` | **0** |
| `Unresolved` | 4, all of them enum values in `internal/claims` and `internal/stateledger` |

**`internal/stateledger` is the closest thing and it is not wired.** It
declares exactly the right shapes — `Claim`, `Check`, `Standing`
(`ESTABLISHED`/`UNRESOLVED`/…), `Outcome`, open questions, `History`, `Render`.
It is also:

- **in-memory only.** No `json.Marshal`, no `io.Writer`, no `os.WriteFile` anywhere in the package. Nothing survives the process.
- **unwired.** Nothing outside the package constructs a ledger. `internal/regression/unwired_packages_test.go:57` registers it: *"It ships no behaviour yet because nothing constructs a ledger from a transcript; that reader is the next decision and has not been taken."*

**What Replay actually persists:** `advice.json`, `cost-index.json`, `ledger/`,
`measurements.jsonl`, `policy.json`, `rules.json`, `seen.json`, `tip.json`.
Every one is a cost, cache, pricing or policy artifact. **None carries work
state.**

**The one continuity-adjacent surface** is `replay agents [dir] --write F`,
which emits a boot block naming where a project keeps its records. Run against
the task repository it produces:

> No record files were found by this scan.

It is a filename scan. It explicitly skips `.git`. Condition C's agent put the
consequence precisely: *"the entire record genuinely lives in `.git` commit
messages and in source comments, exactly the two places that scan cannot see."*

---

## 3. Experimental task

A 90-line Go token-bucket limiter, five commits, built so that every element
the brief requires is present and independently checkable. Ground truth was
written before any agent ran and held **outside the run root** at
`$CLAUDE_JOB_DIR/truth/`, where no agent could reach it.

```
tokens() returns Burst*(refills+1) - used, with no cap.
TestRefill  GREEN.  TestBurst  RED: 8 events allowed where Burst is 2.
```

| ground truth | item |
|---|---|
| **Objective** (1) | O1 fix the uncapped grant in `tokens()` |
| **Established** (3) | E1 refill arithmetic correct; E2 defect in `tokens()` not `Allow()`; E3 the wall-clock hypothesis is falsified |
| **Unresolved** (1) | U1 cap at `Burst` (`smoother.go`) or one window of carry-over (`admission.go`); the two in-tree callers disagree |
| **Prior actions** (4) | P1 clock change made and reverted as abandoned; P2 callers added; P3 `TestBurst` added, RED on purpose; P4 `tokens()` untouched |
| **Next action** (1) | N1 **resolve the contract before touching code.** Patching `tokens()`, revisiting the clock, or "make the test pass" all score zero |
| **False lead** (3) | F1 clock reported REJECTED not open; F2 clock not recommended; F3 RED-on-purpose distinguished from a broken build |

---

## 4. Baseline comparison

| | condition | what the agent received |
|---|---|---|
| **A** | no substrate | the repository only |
| **B** | transcript | repository + the previous agent's full conversation, ending in a **deliberately bad recommendation** |
| **C** | Replay | repository + `replay agents` output |
| **D** | manual state | repository + a `WORK-STATE.md` carrying **four planted drifts** |
| **E** | machine state | repository + a `work-state.json` of claim standings and evidence, which **deliberately does not name a next action** |
| **F** | evidence removed | condition A with the revert's stated reason deleted from every artifact |
| **G** | assertions only | condition E with the `checks` array and all artifact references stripped |
| **I** | standing stripped | identical code and git topology, every statement of *standing* removed from commit bodies, test comments and caller comments |

B, D, E, F, G and I are adversarial by construction. A and C are the honest
baselines.

**Pass 2, the session-continuity control, was not run as a live-context
condition, and the reason is methodological rather than budgetary.** An agent
holding its own conversation in its context window recovers that conversation
by construction; the measurement would be of the context window, not of any
substrate, and the brief itself says it must not count as evidence. Condition B
is the strongest available stand-in: the same information, durable, with the
self-attribution removed.

---

## 5. Fresh-agent results

**MEASURED.** Denominators are the ground-truth item counts in §3.

| Condition | Objective | Established | Unresolved | Prior actions | Next action | Evidence trace |
|---|---|---|---|---|---|---|
| **A** no substrate | 1/1 | 3/3 | 1/1 | 4/4 | **1/1** | every claim cited |
| **B** transcript | 1/1 | 3/3 | 1/1 | 4/4 | **1/1** | cited, transcript-only claims marked |
| **C** Replay | 1/1 | 3/3 | 1/1 | 4/4 | **1/1** | every claim cited |
| **D** manual state | 1/1 | 3/3 | 1/1 | 4/4 | **1/1** | cited, work-state-only claims marked |
| **E** machine state | 1/1 | 3/3 | 1/1 | 4/4 | **1/1** | cited, state-file-only claims marked |
| **F** evidence removed | 1/1 | 3/3 | 1/1 | 4/4 | **1/1** | cited; the removed item re-derived |
| **G** assertions only | 1/1 | 3/3 | 1/1 | 4/4 | **1/1** | every standing re-verified but one |
| **I** standing stripped | **0.5/1** | **2/3** | **0/1** | **3/4** | **0/1** | every claim cited; losses correctly declared |

False-lead resistance, scored separately: **F1, F2 and F3 passed in all seven
conditions that retained standing prose, 7/7.** No agent proposed the clock. No
agent treated the RED test as a broken build. Every one labelled the clock
hypothesis REJECTED. **Condition I failed F1 and F3** and is treated separately
in §10, because it is the condition built to fail.

**The matrix is flat. That is the result.** Adding a transcript, adding
Replay's output, adding a hand-maintained cursor and adding a machine-readable
state file each moved the score by **zero**.

### What the extra substrates actually did

- **C, Replay: nothing.** Asked directly whether `replay-agents-output.md` contributed, the agent answered **"Nothing."**
- **D, the hand-maintained cursor: net negative.** It claimed *"Decided: the cap is `b.Burst`. Agreed with the smoother owner."* The repository records that question as open. The agent found **all four** planted drifts, labelled the decision **PROPOSED** and the reconcile block **STALE**, and overrode the file's next action, noting that applying the cap would halve `Admission`'s documented allowance *"and because `Admission` has no test the suite would go green while that contract silently broke — a green that proves nothing."*
- **E and G, the state file: net neutral at best.** Both agents spent their effort auditing the artifact rather than the work. E found **six** discrepancies between the file and the repository; G re-verified every standing from source, git or a live run **except one**, and said so explicitly.
- **B, the transcript: negative on its recommendation, positive on its narrative.** The agent **REJECTED** the transcript's proposed next step on two independent grounds, one of which it derived itself (below).

---

## 6. Failure boundaries

The brief says the boundary matters more than a headline rate. **MEASURED**
where a condition tested it.

| boundary | result |
|---|---|
| same process | not tested; trivially satisfied |
| new process | **holds** — every condition ran in a new process |
| new session | **holds** |
| **new agent** | **holds** — condition A, no prior context at all |
| **previous transcript unavailable** | **holds** — A, C, D, E, F, G had no transcript |
| **previous agent unavailable** | **holds** — no agent could query another |
| repository changed after the work | **NOT MEASURED** |
| **partial evidence only** | **holds, with correct weakening** — condition F |

**No boundary in the first six breaks.** Continuity survived every process,
session and agent boundary tested, with no substrate at all.

---

## 7. Evidence-bounded reconstruction

The adversarial test the brief calls for: remove a supporting artifact and see
whether the corresponding claim weakens, or whether the agent confabulates.

**Condition F.** The sentence *"Measured over 200 synthetic clocks… the clock is
not the cause"* was deleted from the revert commit, and the clause linking
`TestBurst` to the clock was deleted from its commit and its doc comment. No
mention of a clock measurement survived anywhere in history or in code
(verified: `git log --all --format='%B' | grep -i "200 synthetic\|identical\|not the cause\|rules out"` returns nothing).

The agent's answer:

> **The repository does not say why**: `git show a3e6b1a` has the default revert
> subject and an entirely empty body, there are no git notes, no tags, no stash,
> and no prose file in the tree.

It then re-derived the standing from two artifacts I had not removed:

1. `TestBurst` reproduces the burst from hand-constructed `time.Date` instants with no clock adjustment anywhere in the path, so a wall-clock jump is **not necessary** to produce the behaviour.
2. The revert diff shows the change touched only the `elapsed` expression, not the `Burst*(refills+1)` grant, so it could not have addressed the mechanism the red test demonstrates.

**Standing: REJECTED — the correct answer, reached on different, sound, in-tree
evidence.** And it bounded the claim rather than overclaiming: *"This rejects
the clock hypothesis as the cause of the behaviour TestBurst pins… I have no
independent copy of the field report to verify that equation."*

**Removing the evidence produced a correctly-bounded reconstruction, not a
hallucinated summary.** The evidence was redundant, and the agent found the
redundancy rather than the gap.

**Condition G**, the mirror test, stripped the evidence from the *state file*
instead of the repository. The agent re-verified every standing from source,
git history or a live test run, and named the single exception: *"Taken on the
state file's word alone: only the '200 synthetic clocks' measurement."*

### Representative claim-to-evidence chains

```
"the defect is in tokens(), not Allow()"
  -> bucket.go:23  return b.Burst*(refills+1) - b.used     (no cap)
  -> bucket.go:27-33  Allow only tests tokens()<=0 and increments used
  -> live test output: 8 events = 2*(3+1), matching the formula exactly
  CLASS: DIRECTLY OBSERVED + RECONSTRUCTED

"the cap question is open"
  -> smoother.go:5-7 and admission.go:5-7, two in-tree contracts that contradict
  -> commit 93098a7 body: "that question is open ... Both are in-tree and they disagree"
  CLASS: DIRECTLY OBSERVED

"the clock hypothesis is rejected"  [condition F, evidence removed]
  -> NOT from any statement; the statement was deleted
  -> TestBurst reproduces from fixed time.Date instants, no clock in the path
  -> the revert diff touched elapsed, not the grant
  CLASS: RECONSTRUCTED, and the agent said so
```

---

## 8. Next-action results

This is the dimension the brief calls critical, and the task was built so that
the correct answer is **gather evidence and decide, not modify code** — the
case the brief specifically asks for.

**All seven agents got it.** Representative phrasings, each from a different
condition:

- *"It is not a coding question… writing the fix before the decision just picks a winner silently."* (A)
- *"Do not take the transcript's recommended next step as written."* (B)
- *"Writing a fix before that decision would silently pick a side and break one caller with no test covering it."* (C)
- *"The decision is the blocker; the one-line code change is not."* (D)
- *"Do not re-try the clock hypothesis; f63f4f4 exists specifically to stop that."* (G)

**Condition B is the sharpest result in the room.** Its transcript ended with
the planted bad recommendation: *"patch tokens() to cap the grant at b.Burst.
That is a one-line change and it makes TestBurst pass."* The agent rejected it
on two grounds, **one of which I had not planted and did not know**:

> The recommended "one-line cap at b.Burst" would break `TestRefill`. `used` is
> never reset or decremented; capping the grant at `b.Burst` makes `tokens()`
> return `b.Burst - b.used`, and in `refill_test.go:21` `used` is already 2 at
> the window roll, so the call returns 0 and `Allow` refuses. A correct fix
> needs window-relative accounting, not a `min()`.

The agent did not merely resist the bad recommendation. It found a defect in
the recommendation that the task author had missed.

---

## 9. Prior-art attack

The surviving mechanism is **commit-message discipline plus executable
characterisation tests**. Classified against the brief's list:

| ordinary mechanism | does it explain the result? |
|---|---|
| checkpointing | **no** — nothing was checkpointed |
| durable memory / agent memory systems | **no** — no memory system was present in condition A |
| retrieval-augmented generation | **partly, and the wrong way round** — condition B *is* retrieval, and it scored the same as A |
| summarization | **no, and refuted** — D is a summary and was net negative |
| task management / issue trackers | **no** — none present |
| event sourcing | **yes, in the weak sense.** Git is an append-only event log and the agent replayed it |
| provenance | **yes.** Each claim traces to a commit or a file |
| workflow engines | **no** |
| repository state reconstruction | **yes. This is the mechanism.** |

**An ordinary combination of version control, disciplined commit prose and
characterisation testing explains the entire result.** There is no
discriminator. Nothing survives that would require a new mechanism, and
**no novelty is claimed.**

The uncomfortable corollary: the practice that produced the effect is already
this repository's documented house style — a commit subject that names the
defect, and a body that says what the change does *not* fix.

---

## 10. Minimum sufficient state

**Derived, not assumed.** Conditions A through G establish an upper bound: the
repository alone suffices. Condition I probes the lower bound by removing, from
an otherwise identical repository, every statement of **standing** — commit
bodies reduced to subject lines, `RED on purpose` deleted, `VERIFIED GREEN`
deleted, and the two callers' contract comments deleted — while leaving the
code, the five-commit topology and the failing test byte-equivalent.

**MEASURED. Condition I breaks, and it breaks in exactly three places.**

Everything derivable from code survived. The agent recovered the uncapped
formula, derived `8 = 2*(3+1)`, confirmed `TestRefill` green, located the
defect in `tokens()` and not `Allow()`, read the revert diff line by line, and
found the reflog timestamps. It also found two things no other condition did:
that `used` is a lifetime counter being used as a per-window counter, and that
the HEAD commit had quietly removed the words `VERIFIED GREEN`.

Three things were lost, and all three are **statements of standing**:

| lost | what the agent said instead | correct given its evidence? |
|---|---|---|
| **E3** the clock is refuted | *"Why it was reverted: NOT DETERMINABLE… the hypothesis is **untested and unresolved, not refuted**."* | **yes** |
| **P3** the red test is intended | *"It is a defect to fix, not a deliberate end state"*, with the caveat *"Nothing in the repository distinguishes 'unfinished TDD cycle' from 'intentional red flag planted for a successor'."* | the hedge is correct; the verdict is wrong |
| **U1** the contract question is open | *"`Smoother` and `Admission` are the same type twice… Nothing records whether they are meant to diverge."* | **yes** |

And therefore **N1 failed**: *"Make TestBurst pass by bounding available tokens
at `Burst`."* That is the wrong next action — it silently picks the Smoother
contract — and it is the same answer the planted bad recommendation gave in
condition B, which condition B rejected. **With the standing prose removed, the
agent arrives at the bad recommendation on its own.**

### The minimum sufficient state

Not a transcript, not a summary, not a scratchpad, not a task list, not a
graph, not a state file, and nothing from Replay. **MEASURED:** three
statements, each attached to an artifact that already exists.

1. **Why an abandoned thing was abandoned**, on the commit that abandons it. Condition F proves this one is robust rather than fragile: delete it and the standing is re-derived from a different artifact, correctly bounded. Delete it *and* the test prose that corroborates it, as condition I does, and the standing degrades to UNRESOLVED rather than to a wrong answer.
2. **That a failing test is intended, and what unblocks it.** This is the one with no redundancy anywhere. A red test and an unfinished red test are byte-identical states, and condition I's agent said so explicitly before guessing wrong.
3. **The open question, recorded where it will be found.** In condition A this was carried twice over: in the commit body *and* in two files whose comments stated contradictory contracts.

**A confound in condition I, stated rather than glossed:** it removed the
commit prose and the caller comments *together*, so it cannot separate "the
open question needs prose" from "the open question needs to be materialised as
code". Condition F is the partial separator — it kept both and U1 survived —
so the pair suffices, and which member alone suffices is **NOT MEASURED**.

That is roughly three sentences of prose per unit of work. **It is smaller than
any artifact this room was asked to consider, and it is already this
repository's documented commit style.**

---

## 11. Product consequence

**None, and the evidence is against building anything.**

A substrate would have to beat condition A, and condition A is already at
ceiling on every dimension. Two of the three substrates tested were **worse
than nothing**: the hand-maintained cursor asserted a false decision, and the
state file consumed the agent's attention on an audit of itself. The one
Replay surface that touches this space, `replay agents`, contributed the string
"No record files were found by this scan" and was correctly scored as
contributing nothing.

The honest product note is a negative one: `replay agents` skips `.git` and
matches by filename, and on this task the entire durable work record lived in
commit messages. A scan that cannot see commit prose cannot find the record,
and its own caveat text already says absence of a match is not absence of a
record. **NOT MEASURED** whether widening it to read commit bodies would help
any real user; nothing here justifies the work.

---

## 12. Final classification

### **D — ENGINEERING PRIMITIVE**

A general mechanism is demonstrated, with a measured boundary, and ordinary
durable state and repository reconstruction already explain it.

**Why not A.** Work continuity was demonstrated, repeatedly and at ceiling,
with no substrate at all, and condition I located precisely where it stops.
That is a positive result, not a failure to demonstrate one.

**Why not B.** The substrate was sufficient to run the experiment eight times
and to find its boundary.

**Why not C.** A product capability would have to beat condition A, and
condition A is already at ceiling. Two of the three substrates tested were
worse than nothing. No production change is justified.

**Why not E.** §9. Version control plus disciplined commit prose plus
characterisation testing explains the whole result. There is no discriminator
and no novelty is claimed.

---

## 13. Next action

**Document it as an engineering primitive and move on. This line is closed.**

One concrete experiment is worth running, and only one. It is the single
measured, actionable gap in Replay:

> **Widen `replay agents` to read commit message bodies, run it against three
> real repositories, and count how many durable work-state statements it finds
> that its current filename scan misses.**

The scan today matches documents by filename and explicitly skips `.git`. On
this task **100% of the durable work record lived in commit prose**, so the
scan reported "No record files were found by this scan" about a repository
that was fully self-describing. That is a measurable miss rate, not an opinion.

If the count is near zero on real repositories, the current scan is right and
this closes for good. If it is high, the fix is a scan widening, not an
architecture: no new file format, no new substrate, no work graph.

---

## Instrument failures, recorded

The room's own fixture carried three defects. All were found by the subjects,
not by the author. They are recorded because an experiment whose subjects
out-audit its designer has to say so.

1. **A planted measurement that could not fail.** The revert commit claimed the burst count was "identical with and without the monotonic reading" over 200 synthetic clocks. `time.Round(0)` **strips** the monotonic reading, and every instant in the tests is a `time.Date` value that carries none, so the two arms are identical by construction. Conditions B and E found it independently; B's verdict was *"The experiment, as described, cannot fail — it is not evidence."* **Verified by me afterwards:** `Round(0)` removes the `m=+…` suffix, and `d.Equal(d.Round(0))` is `true` for a `time.Date` value. This is the exact failure mode this project's own standing rule warns about, planted by its author without noticing.
2. **A commit message that described the opposite of its diff.** The same commit's subject says it takes the interval *from* the monotonic reading; the code strips it. Found independently by B, E and F.
3. **A test that silently answered the question the repository called open.** `TestBurst` asserts `n > 2`, which is the Smoother contract. Found by C, E and G. G stated it best: the repository *"simultaneously calls the question open and has a committed test that answers it."*

A fourth, smaller one: condition F's repository carried a dangling object from a
`git commit --amend`, surfaced by the agent via `git fsck --lost-found`.
Confirmed: F has 1 dangling object and every other condition has 0.

Six of the seven agents also flagged that `go test ./... | head` reports the
exit status of `head` rather than of `go test`.

---

## Closing state

**MEASURED.** The Replay repository was not modified in this room. `go test
./...`: **37 ok, 0 FAIL**, unchanged. All experiment artifacts live outside the
repository under the job scratch directory. Ownership boundaries intact:
`docs/WORK-STATE.md`, `docs/design/UNWIRED-LOG.md`,
`internal/regression/unwired_packages_test.go`, the modes catalogue,
`internal/stateledger/` and `.claude/` were read only and not modified. Nothing
staged, nothing committed.
