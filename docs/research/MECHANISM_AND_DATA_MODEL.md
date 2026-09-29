# Candidate mechanism and proposed data model

**HYPOTHESIS, NOT A DESIGN DECISION. Nothing here is implemented, and nothing
here should be implemented until R3 replicates on a second model and a second
repository. Written 2026-09-29.**

## Observed phenomenon, and its exact ceiling

**This section states only what has been measured. Nothing below it may be read
back into this section.**

| Demonstrated | Evidence |
|---|---|
| A structured claim representation changes agent RETRIEVAL and SEARCH behaviour | Origin campaign: 0/8 with no cursor, 0/4 with the same claim in prose, 9 to 10 of 10 with a three-row table. Separately 51/60 against 0/12, p=1.9e-08. Effort flat at 21 to 25 tool calls per arm |
| Agents that reach the evidence almost always name it | `saw but missed` was 1/80 in the 2026-09-29 replication and near zero at origin |
| Anchor capability changes EFFORT | 2026-09-29 replication: mean tool calls 8.4 (no anchor) against 5.2 (dispositive), identical detection |

**Three upgrades that are NOT licensed by the above and must never be written:**

- structured state improves work
- Replay prevents premature closure
- Replay improves task completion

Each needs an objective task-outcome measure. None exists on any surface
examined (see `PROVIDER_EVIDENCE_MATRIX.md`), so none of the three is currently
measurable, let alone measured.

### Replication result, and the rule applied to it

| | |
|---|---|
| C0, no anchor | **20/20, 100%** |
| T2, executable but insufficient | **20/20, 100%** |
| Fisher exact, two-tailed | **p = 1.0000** |
| Risk difference | +0%, bootstrap 95% CI [+0%, +0%] |
| T1 dispositive | 20/20 |
| T3 stale | 17/20, p=0.2308 against C0, not significant |

**Verdict: NOT REPLICATED.**

The frozen interpretation rule, quoted from `PREREG.md` rather than paraphrased:

> T2 **not** below C0: **the mechanism is NOT established.** Report and stop.

That is the strongest conclusion the preregistration licenses, and it is weaker
than refutation. **Mechanism status: UNCONFIRMED / NOT REPLICATED.**

The replication did not reproduce the effect under this task. Because both
conditions reached 100% detection, the experiment had no observed headroom on
the primary endpoint. The result therefore does not discriminate whether the
mechanism is absent or whether the task failed to provide sufficient difficulty.

**None of these may be written:** the mechanism is false; the mechanism is
disproven; weak anchors have no effect; the hypothesis failed universally; the
phenomenon does not exist. The preregistration does not support any of them.

### What the failed replication actually taught us

**This is the most useful thing the 80 runs produced, and it is a finding about
method rather than about agents.**

A task on which both control and treatment reach 100% detection cannot test a
*reduction* in detection probability. The measurement had no room to move. The
control's own rate is the ceiling on any observable decrease, and at 100% that
ceiling is zero.

The origin's control sat at 8/10 on a larger, self-authored repository with a
harder contradiction. The replication used a single constant in a 39-file
repository, reachable by one obvious grep. **The preregistration contained no
pilot establishing that the control would land below ceiling**, and that omission
is why 80 runs produced no information about the hypothesis.

**The next experiment must not be the same task with larger n.** More runs on a
ceiling task produce a more precise measurement of nothing. What is required is
a contradiction-detection task whose baseline difficulty is **independently
established before any treatment arm runs**.


## The candidate mechanism, a HYPOTHESIS

```
CLAIM
  -> EVIDENCE                 what was recorded
  -> EVIDENCE CAPABILITY      what that evidence can DO
  -> VERIFICATION             the check actually performed
  -> CLAIM RESULT             what the check returned
  -> REMAINING EVIDENCE GAP   what is still not established
  -> REQUIRED NEXT CHECK      what would close the gap
```

Three capabilities, in increasing strength:

| Capability | Can do | Cannot do |
|---|---|---|
| `LOCATES` | names where related evidence lives | settle anything |
| `EXECUTABLE` | be run to completion | settle anything |
| `DISPOSITIVE` | settle the claim | n/a |

### The invariant under test

> **`EXECUTABLE != DISPOSITIVE`.** A command that succeeds is not thereby
> evidence that the claim is true.

This is the whole hypothesis. It is trivially true as a statement about logic
and completely untested as a statement about agent behaviour. The running
80-trial experiment is the first attempt to measure whether the distinction has
any behavioural consequence at all.

If R3 fails, this document is wrong and should be deleted rather than softened.

### Also modelled, each currently a hypothesis

- **Contradiction without replacement truth.** Finding a claim false teaches one
  thing, not two. A contradiction with no replacement leaves an open question.
- **Unresolved questions as first-class state.** Zero of 28 products surveyed
  represent these. Verified by schema reading, not by failed search.
- **Required verification.** The check that would close a gap, named before it
  is run.
- **Evidence availability time vs verification time.** Two different clocks. A
  claim verified before the evidence existed is not verified.
- **Reconstruction provenance.** Whether a fact was observed or derived.
- **Independent vs agent-generated state.** R11. Untested, and if it does not
  matter it leaves the thesis.


## Alternative explanations, and what would discriminate each

The candidate mechanism is one of at least eleven readings of the origin
result. Listing them is the point: the origin measured that a table changes
retrieval, and every row below is also consistent with that.

| Alternative | Discriminating experiment |
|---|---|
| Simple retrieval cueing: the table just mentions more nouns | Hold noun count constant between table and prose arms |
| Path localization: the anchor names a file and that is all | Anchor naming the right file but no command, against a command naming no file |
| Actionability bias: executable text attracts effort | Compare an executable anchor against a non-executable one of equal specificity |
| Completion bias: finishing anything ends the search | **Irrelevant successful check** (Gate F class 5). If an irrelevant success harms as much as a relevant-insufficient one, targeting is not the variable |
| Confirmation bias: the claim is believed because it is asserted | Vary claim truth while holding the anchor fixed |
| Tool-result salience: the anchor's output crowds the context | Compare anchors producing long against short output |
| Prompt framing | Vary the instruction while holding the table fixed |
| Claim-table effects independent of anchors | Already partly done: table against prose, 0/4 |
| Anchoring on the first-named location | Randomize the order of table rows |
| Model-specific behaviour | Gate C, a materially different model |
| Repository-specific behaviour | Gate D, an independent repository |

**The candidate mechanism has no privileged status among these.** It was
promoted because it was counterintuitive, which is a reason to test it, not a
reason to believe it.

## Proposed schemas

Field notation: `O` observed, `D` derived, `M` mutable, `I` immutable.
"Can settle" is the load-bearing column.

### Claim
| Field | Meaning | Source | O/D | M/I | Can settle |
|---|---|---|---|---|---|
| `id` | stable handle | minted | D | I | no |
| `text` | the proposition | author | O | I | no |
| `origin` | where it came from | author or transcript | O | I | no |
| `standing` | ASSERTED / SUPPORTED / CONTRADICTED / SUPERSEDED / UNRESOLVED | derived from checks | D | M | no |
| `replacement` | nullable; **null means genuinely unknown** | only a dispositive check | D | M | no |
| `history` | every standing held, each naming its cause | append-only | O | I | no |

### Evidence
| Field | Meaning | Source | O/D | M/I | Can settle |
|---|---|---|---|---|---|
| `locator` | where it lives | author | O | I | **no, locating only** |
| `artifact_class` | transcript / ledger / repository / provider record | observer | O | I | no |
| `available_from` | when the evidence began to exist | observed | O | I | no |

### EvidenceCapability
| Field | Meaning | Source | O/D | M/I | Can settle |
|---|---|---|---|---|---|
| `kind` | LOCATES / EXECUTABLE / DISPOSITIVE | author, **typed not inferred** | O | I | only DISPOSITIVE |

Capability is declared by the author and is itself a claim. The system does not
infer it, and cannot: inferring whether a check can settle a proposition is the
original problem.

### Verification
| Field | Meaning | Source | O/D | M/I | Can settle |
|---|---|---|---|---|---|
| `action` | what was run | observed | O | I | no |
| `result` | what came back | observed | O | I | no |
| `outcome` | supports / contradicts / inconclusive / not-run | derived | D | I | no |
| `at` | verification time | observed | O | I | no |

### Contradiction, UnresolvedQuestion, RequiredCheck
Each records a gap rather than a conclusion. `UnresolvedQuestion` has no
resolution field by design: adding one invites filling it.

### Reconstruction
| Field | Meaning | O/D | Can settle |
|---|---|---|---|
| `method` | how the value was derived | O | no |
| `tier` | **maps to the EXISTING estimated/measured tier**, not a new vocabulary | D | no |
| `coverage` | what fraction of the subject it covers | D | no |

### EvidenceFrontier
The boundary between what is established and what is open, at a point in time.
Derived, never stored as a field, for the same reason `Pool.Totals()` is a
method: a stored frontier drifts from its parts.

## Existing invariants this must not weaken

Each is load-bearing and each has been violated once already in this repository.

1. **Missing evidence is not negative evidence.** ADR-0018. A nil is a third state.
2. **Action is not outcome.** Running a check is not answering the question. This is the mechanism itself.
3. **No scalar confidence.** A number would collapse capability, coverage and standing into one figure nobody can decompose.
4. **`NOT_MEASURED` is a claim result, never a value.** It may not be rendered as zero.
5. **`RECONSTRUCTED` maps to the existing estimated tier.** No parallel vocabulary.
6. **Evidence availability time matters.** Two clocks, kept apart.
7. **Claim gates are explicit.** `Settle()` refuses non-dispositive checks and says why.

## What is deliberately absent

No confidence score. No embedding. No retrieval ranking. No general-purpose
memory store. No field added because it might be useful later. If R3 fails,
none of this is built.
