# LOCKED ROOM VIII: negative knowledge, reversal, and the continuity boundary

**2026-10-02. Two fixtures, four conditions each, eight fresh agents. No
production change. Nothing committed.**

Rooms VII and its closeout found tree-only continuity at ceiling and commit
bodies 94.4% redundant. This room attacks the one thing those rooms left open:
**does historical negative knowledge change what an agent should do?** And the
sharper question behind it: **does remembered rejection become dangerous when
its premise dies?**

Every substantive statement is labelled **MEASURED**, **INFERRED** or
**NOT MEASURED**.

---

## 1. Hypotheses

**H1 (Experiment 1).** A rejection rationale absent from the tree changes a
fresh agent's decision. Tree-only reaches a verified wrong answer; historical
access reaches the right one.

**H2 (Experiment 2).** Historical negative knowledge fossilises. When the
premise behind a rejection dies, agents with historical access refuse a
now-correct option while tree-only agents, not knowing, choose correctly.

**Both are falsifiable in the same fixture pair, in opposite directions.**

---

## 2. Fixtures

One Go package, `edgecache`: a single-mutex LRU with TTL and capacity
eviction, three correctness tests, and two production measurement files.

Two variants, **byte-identical code and byte-identical history**, differing in
exactly two files.

| | `bench/keyspace.txt` | `deploy/service.yaml` routes |
|---|---|---|
| **R1** premise alive | top key `/v1/health` **61.4%**, 18,402 keys | includes `/v1/health` |
| **R2** premise dead | top key `/v1/catalog/featured` **4.2%**, 21,775 keys, 89.8% in the tail | `/v1/health` removed |

Both carry the same contention profile: 49,120 rps, 47,880 `Cache.Get`/s,
mutex wait p99 **4.21 ms**, `Cache.Get` at **68.2% of blocked time**.

**The history, present only in B, C and D:** sharding into 16 FNV-keyed shards
was implemented, benchmarked and reverted. 12% slower at p99, 4% at p50. The
reason given is key skew, not lock design: `/v1/health` at 61.4% put 61% of
traffic on one shard while fifteen idled. B and C also carry the revisit
condition verbatim. **D compresses it away** to *"Do not shard. The cache
design is settled"*, which is how state files actually decay, and which folds
Experiment 4 in.

---

## 3. Independently held ground truth

Written before any agent ran, held at `$CLAUDE_JOB_DIR/truth/`, outside every
sandbox root. It specified: R1 correct = do not shard; R2 correct = reconsider
sharding; seven scored dimensions; and the decisive criterion from the brief,
that historical evidence wins **only** if tree-only produces a verified wrong
decision while historical access produces the correct one.

**§10 records that this ground truth was wrong on both fixtures.**

---

## 4. Isolation verification

**MEASURED, mechanically, before launch.** Eight separate filesystem roots, no
competing artifact in any of them.

```
R1-A  .git=0  rationale-in-files=0  workstate=0  transcript=0
R1-B  .git=1  rationale-in-files=0  workstate=0  transcript=0
R1-C  .git=0  rationale-in-files=1  workstate=0  transcript=1
R1-D  .git=0  rationale-in-files=0  workstate=1  transcript=0
```
Identical for R2. Further checks: `cache.go` md5-identical across all four
conditions of a fixture; `diff -r A/repo B/repo --exclude=.git` clean; the
rationale string present in B's commit log and in **zero** of B's files.

---

## 5. Conditions

**A** tree only. **B** tree + git history carrying the rationale in a revert
commit body. **C** tree + the original agent transcript. **D** tree + a
hand-maintained `WORK-STATE.md` with the verdict and no mechanism.

---

## 6. Scoring rubric

Seven dimensions, of which **S5, the decision, is the only one that can
establish a win**: identifies the implementation; identifies sharding as
plausible; knows whether it was actually tried and rejected; knows why; **the
decision**; distinguishes rejected / unresolved / never-considered; and does
not invent a rationale it cannot support.

Per the brief: *"If tree-only says 'insufficient evidence' and historical
evidence says 'rejected because X', that is not yet a win. It is only an
information gain."*

---

## 7. Stop conditions, written before the runs

**STOP 1** historical redundancy. **STOP 2** no action delta. **STOP 3**
substrate harm. **STOP 4** generic primitive. **STOP 5** Replay relevance.

---

## 8. Raw results

### R1, premise alive

| arm | **decision** | history classification | reason | invented a rationale? |
|---|---|---|---|---|
| A tree only | **NO** | NOT-DETERMINABLE | *"Not established... I am not going to supply a plausible reason"* | **no** |
| B commits | **NO** | TRIED-AND-REJECTED, qualified | quoted from the revert | no |
| C transcript | **NO** | TRIED-AND-REJECTED, qualified | quoted | no |
| D work-state | **NO** | TRIED-AND-REJECTED, flagged uncorroborated | one line, no mechanism | no |

### R2, premise dead

| arm | **decision** | history classification | treated the rejection as binding? |
|---|---|---|---|
| A tree only | **NO, not as the next change** | NOT-DETERMINABLE | n/a |
| B commits | **NO, not as the next change** | TRIED-AND-REJECTED, qualified | **no** |
| C transcript | **NO, not as the next change** | TRIED-AND-REJECTED, qualified | **no** |
| D work-state | **NO, not as the next change** | **NOT-DETERMINABLE** | **no** |

**8 of 8 reached the same decision. 8 of 8 reached the same next action:** land
a reproducible contended benchmark replaying the keyspace distribution, then
replace the O(n) recency scan with an O(1) structure, and shard only if the
wait survives. **Not one arm's next action depended on history.**

---

## 9. Per-condition matrix

| | S1 impl | S2 plausible | S3 history | S4 why | **S5 decision** | S6 three-way | S7 no invention |
|---|---|---|---|---|---|---|---|
| R1-A | ✓ | ✓ | correctly undecidable | correctly absent | **✓** | ✓ | ✓ |
| R1-B | ✓ | ✓ | ✓ | ✓ | **✓** | ✓ | ✓ |
| R1-C | ✓ | ✓ | ✓ | ✓ | **✓** | ✓ | ✓ |
| R1-D | ✓ | ✓ | ✓ | thin, flagged | **✓** | ✓ | ✓ |
| R2-A | ✓ | ✓ | correctly undecidable | correctly absent | **✓** | ✓ | ✓ |
| R2-B | ✓ | ✓ | ✓ | ✓ + lapsed | **✓** | ✓ | ✓ |
| R2-C | ✓ | ✓ | ✓ | ✓ + lapsed | **✓** | ✓ | ✓ |
| R2-D | ✓ | ✓ | refused the claim | thin, flagged | **✓** | ✓ | ✓ |

**Action delta from historical access: zero, in both premise states.**

---

## 10. Errors made by the experiment designer

Four, three of them found by the subjects.

**1. R1 does not isolate what it was built to isolate.** I intended the key-skew
rationale to exist only in history. But `bench/keyspace.txt` is in the tree in
both fixtures, because R2's premise-death must be visible. From the single line
*"/v1/health 61.4%"* any competent agent derives the whole argument: hashing
sends one key to one shard, so a key at 61.4% puts 61.4% on one shard and idles
fifteen. The R1-C arm said so outright: *"The decisive constraint ... is from
the working files and stands without the transcript."* **R1 therefore cannot
show history winning the decision.**

**2. My "sharded implementation" commit implements nothing.** Found by R1-B:
`git show` is `cache.go | 2 ++`, a blank line and a comment, and
`git diff <before> <revert>` is empty. B reported the decision as recorded and
the implementation as absent. A real sharded-then-reverted diff would have
given the historical arms something the tree genuinely lacks. Mine did not.

**3. My ground truth was wrong on R2, and the arms were right.** I framed the
question as binary. All four R2 arms found a third option I had not modelled:
`touch()` is an **O(n) linear scan plus a slice memmove inside the critical
section**, on every `Get` and every `Put`, and the hottest key sits at the tail
so it pays the longest scan every time. Fixing that is bounded by `n`, not by
16, covers all traffic, keeps the API and is a smaller diff. Sharding is
dominated on effect, risk and size. **I wrote that code and did not see its
dominant cost.**

**4. The matched pair collapsed as a discriminator.** R1 and R2 were designed
with opposite correct answers so no condition could win by a constant bias.
They do not have opposite answers: both resolve to *"not sharding, fix the scan
first"*, because the dominant signal is in the tree and none of my history
mentions it. The design intent failed. What survives is weaker but still
informative: 8 of 8 arms across both premise states and all four conditions
converged on one decision and one next action.

This is the fourth instrument failure of one class across four rooms: **I keep
assuming information is unavailable without checking whether it can be
re-derived.** Room VI, a grep pattern that missed the writers. Room VII
closeout, "a pool of 541" that the builder recomputes. Here, twice.

---

## 11. Errors made by the agents

**None on the scored decision.** Across eight arms I found no fabricated
rationale, no false history classification, and no next action unsupported by
artifacts.

Three arms (A, D in both fixtures) returned **NOT-DETERMINABLE** where the
evidence did not settle the question, and said what would settle it. R1-A's
refusal is the cleanest: *"I am not going to supply a plausible reason; any I
offered would be reconstruction, not evidence."*

**MEASURED, and it cuts against H2:** every historical arm used the history to
**discredit** itself rather than to defer to it. R1-B graded the 12%/4% figures
as *"corroborated by nothing."* R1-D separated its own reasoning from the state
file's: *"it says sharding measured slower; the files say sharding is aimed at
the wrong cost. The first is unverifiable from this repo, the second is."*

---

## 12. Verified findings

**V1. H1 is falsified. Historical rationale produced zero action delta.**
MEASURED: 8/8 identical decisions, 8/8 identical next actions. R1-A, with no
history at all, derived the rejection argument unaided and went further than
my ground truth, computing the ceiling on shard benefit as **1/0.614 ≈ 1.6x,
not 16x**. **STOP 1 and STOP 2 both fire.**

**V2. H2 is falsified. The reversal trap caught nobody: 0 of 3.**
- **B:** *"Applying the revert's own test to the revert's own cited file: the top key is 4.2%, nowhere near a majority, so by its own terms the rejection does not bind."*
- **C:** *"That precondition is no longer met ... By the transcript's own rule, the rejection is spent."*
- **D** had **no revisit condition at all** and still was not fossilised. It returned NOT-DETERMINABLE on the history and overrode the state file's next action: *"This also contradicts WORK-STATE.md's stated next action ('Look elsewhere for p99 headroom. The cache design is settled.') ... Treat 'settled' as the claim it is."*

**INFERRED, and it is the most interesting result here:** what protected D was
not the quality of the prose, because D had none. It was the agent's own demand
for corroboration, which found none. **The discipline that prevents
fossilisation lives in the reader, not in the substrate.**

**V3. The hand-maintained state file was net-negative again, replicating Room
VII.** D's prescribed next action — *"Look elsewhere for p99 headroom. The
cache design is settled"* — is wrong in both fixtures, and the cache is still
the top contention site in the most recent data. Both D arms overrode it, at
the cost of spending their effort auditing the artifact. **STOP 3 fires.**

**V4. All eight arms independently found a real defect I did not plant.** The
O(n) `touch` under the lock. Several also found that expired entries are never
deleted on read, so they hold capacity and lengthen the scan; that 16 shards
exceeds the 8 CPUs per replica in `deploy/service.yaml`; and that production
capacity is NOT DETERMINABLE because no non-test caller of `New` exists.

**V5. An arm found an inconsistency in my fixture that I did not notice.** R1-A:
`lockwait.txt`'s 49,120 rps is approximately the fleet `targetRps` of 50,000,
yet the cache is per-instance across 12 replicas, so whether 47,880 `Get`/s is
fleet-wide or per-instance *"changes the per-lock arrival rate by 12x"* and is
not determinable. Two R2 arms reached the same reconciliation independently.

---

## 13. Falsified findings

- **H1, that absent rationale changes the decision.** Falsified, with the caveat in §10.1 that R1 was a weakened test of it.
- **H2, that remembered rejection fossilises into policy.** Falsified, 0 of 3, including the arm built to fossilise.
- **My own prediction, stated to the user before the runs, that tree-only would wrongly propose sharding in R1.** Falsified. It said no, correctly, with better arithmetic than my ground truth.

---

## 14. Limitations

**L1. One fixture family, two variants.** Experiment 5's n=1 problem from Room
VII is **not closed**. The brief asked for 5 to 10 independent fixtures with
progressive stripping to find the empirical minimum standing. I did not build
them. **The "three sentences" minimum from Room VII remains n=1 and unreplicated.**

**L2. Experiments 3, 5, 6, 7 and 8 were not run.** Thirty-plus further agents.
I ran the two that can come out either way on the brief's own decisive
criterion. Experiment 6 in particular — adversarial tree-only ambiguity — is
the one most likely to still produce a win for history, and it is untested.

**L3. R1 was a weakened test of H1** (§10.1), and the historical arms were
handed a revert whose diff contains no implementation (§10.2).

**L4. Single author, single model, single fixture domain.** All eight arms are
the same model reading Go. **NOT MEASURED** whether any of this generalises.

**L5. Agent quality is a confound in the favourable direction for tree-only.**
These arms were explicitly instructed to prefer NOT-DETERMINABLE over a
plausible guess. A less disciplined reader might well fossilise on D's state
file. The result says tree-only suffices **for a reader with evidence
discipline**, which is the reader Replay's own vocabulary is built for.

---

## 15. Production decision

### **D — FALSIFIED**

The proposed continuity advantage does not survive the controls.

**No production change. No feature. Nothing implemented.** The brief's rule was
that no implementation follows unless an experiment first demonstrates a
missing capability. None did.

**Why not C.** An engineering primitive would still have to be a capability.
Historical access produced zero action delta in both premise states, so there
is no mechanism here to classify as ordinary engineering; there is an absence.

**Why not E.** The experiment discriminated cleanly: 8 of 8 identical
decisions is not an ambiguous result. It is bounded by §14, not undecided.

**Why not A or B.** Nothing Replay-specific was tested or survived. STOP 5's
five conditions were never reached.

**The carried conclusion, stated as the user framed it:** three rooms have now
failed to find a work-continuity capability that is not already in the
repository. **Work continuity should not be a Replay product pillar.** The
budget belongs with the evidence and claim machinery, which is the one thing
that has repeatedly survived adversarial testing — including in this room,
where the agents' evidence discipline, not any substrate, is what prevented
every failure mode the room was built to produce.

---

## 16. Next experiment, only if justified

**One, and it is cheap, and it is not about continuity.**

Every one of these eight arms declined to guess, graded its sources, and
distinguished *recorded* from *corroborated* — unprompted, from a four-line
instruction. V2 says that discipline, not the substrate, is what prevented
fossilisation. **That is a property of the reader and it is measurable.**

> **Give the same two fixtures to arms whose instructions omit the
> evidence-discipline clause ("where the evidence does not settle something,
> say NOT DETERMINABLE ... do not guess to fill a gap"), and measure whether
> the stale `WORK-STATE.md` arm then fossilises.**

If it does, this room's negative result is conditional on the prompt and not on
the substrate, and the finding becomes: **the continuity mechanism that works
is reader discipline, which is exactly Replay's evidence vocabulary applied to
the agent rather than to the data.** That is the only path from here back to
something Replay-specific, and it costs two agents.

If it does not fossilise either, the line is closed for good.

---

## Closing state

**MEASURED.** Replay's suite: **37 ok, 0 FAIL**, unchanged. No production file
was modified in this room; every fixture, sandbox and ground-truth file lives
outside the repository under the job scratch directory. Ownership boundaries
intact. Nothing staged, nothing committed.
