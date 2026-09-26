# Research closure

**2026-09-24. Terminal memo for this research phase. No code changed to produce it.**

## 1. Frozen product capability

Demonstrated by running the shipped binary and protected by tests. **None of this is evidence of
market value.**

| Capability | Invocation | Guard |
| --- | --- | --- |
| Transcript-derived session and task cost | `replay`, `replay cost` | Unpriced models excluded, never billed as zero |
| Cache-break attribution | `replay diff` | Eleven-cause taxonomy, ordered most to least specific |
| TTL-expiry identification | `replay diff` | Reports the gap and the TTL it exceeded |
| Context and compaction analysis | `replay context` | PARTIAL / OVERSTATED / Complete; recorded and inferred separated |
| Learn with held-out validation | `replay learn` | Training mean, interval and held-out mean on one row |
| Advise, with measured coverage | `replay advise` | Prints how much of its own output the verifier cannot reach |
| Before and after comparison | `replay cost --compare <date>` | Refuses below ten tasks per side |
| Window activity | `replay since` | Marker-based digest |
| Prediction evaluation | `cost --compare --predicted` | Judged against the outcome |
| Privacy, disclosure, purge | `replay privacy`, `replay purge` | Thirteen-store registry, four guards |

## 2. Falsified hypotheses

| Hypothesis | Falsified by |
| --- | --- |
| Longitudinal capability is missing | `replay cost --compare 2026-09-01` returned a working comparison on the real corpus |
| A new snapshot data model is required | Every task already carries `At`; `splitByDate` partitions one corpus into two windows. No persistence added |
| Post-hoc attribution is missing | `replay diff` prints `cause:`, `where:`, `evidence:` per break, over 929 real breaks |
| Holdout validation is missing or unsurfaced | `learn` prints columns `saving (interval)` and `held-out` adjacent, with `*` for estimated |
| Existing capabilities are broadly undiscoverable | `replay --help` names `--compare <date> for before/after`, `select one with held-out checks`, and `with its cause`. **Three of five claimed gaps were false** |
| An AI Week screen is needed to expose longitudinal value | The value is exposed by an existing command. The screen was ~20% computable |
| Evidence or provenance discipline is a defensible moat | Anthropic Cache Diagnostics is GA on the same surface with an equivalent taxonomy and equivalent refusals, in its own words |
| Deterministic reconstruction is a differentiator | Three independent agentic runs converged on the same reconstruction including the hardest inference |

## 3. Surviving limitations

Genuine, currently supported by repository evidence. **Not roadmap items.**

1. High-level analysis is Claude Code specific. `forEachSession` is typed to one vendor's parse
   output; four of five parsers reach no high-level command.
2. A cost change across `--compare` is not attributed to a cause. Two medians and a delta.
3. The command line cannot observe whether a finding was opened. `advise` prints all findings at once.
4. No saving can be claimed without a measured counterfactual.
5. `--predicted` is not named in the top-level help. One flag.
6. No vendor beyond Claude Code has established high-level support.

## 4. Epistemic closure

**P0, closed in commit `6871022`.** `ContextGap.Compactions` was one counter that a recorded
compaction and a shrinking prompt both incremented, and `Note()` wrote "The history was compacted N
times" from it. The heuristic also fires on a rewind and a resume, which the comment at the increment
site already said. **61 real sessions were affected.**

Verified after the fix: **10 recorded, 61 inferred**, distinct wordings. Three mutants killed:
dropping the increment, giving the inferred branch the recorded wording, hedging the recorded case.
Canonical gates green.

**Separate observation, not a defect.** The tagline says "This names the turn it broke on, the cause,
and what it cost." On the real corpus **760 of 929 causes (81.8%)** are `client re-rendered history
(no edit visible in transcript)` or `prefix diverged at an unknown block`; **169 (18.2%)** are
specific. The cause strings are honest about their own limits, so the output does not overstate. No
evidence of user misinterpretation exists, so this is recorded as wording rather than a defect.

## 5. Longitudinal closure

`replay since` = window activity. `replay cost --compare <date>` = before and after median cost per
task, with task-volume change stated separately. **Minimum evidence: ten tasks per side.**

Real corpus, `--compare 2026-09-01`:

```text
  before   60 tasks, median $0.84
  after    64 tasks, median $0.51
  change   -39% per task, on +7% task volume
```

Natural refusals on the same corpus:

```text
--compare 2026-09-08   Too few tasks to compare: 117 before, 7 after, and each side needs at least 10.
--compare 2026-09-15   Too few tasks to compare: 122 before, 2 after, and each side needs at least 10.
                       No figure is printed rather than a median of noise.
```

**The observed change is not attributed to any cause.** The command computes two medians and a delta.

## 6. Adoption evidence

**The measured distribution channel currently records 9 downloads. That is insufficient evidence to
characterise adoption, demand, retention or willingness to pay.**

It does not mean nine users, nine executions, or nine anything else. Clone counts track CI at 17.78
per Actions run and are not adoption. `go install` is unmeasured. The local surface counter never
leaves the machine.

**A correction belongs here.** An earlier statement in this session read "essentially nobody has run
it." That is an inference from a download count to a usage count, which is precisely the move this
product's own discipline forbids, and it is withdrawn.

## 7. Product statement

> Replay reads Claude Code transcripts that were never instrumented in advance and reports what those
> sessions cost, which cache breaks occurred and what caused each one including expiry past the TTL,
> what entered and left the context window, and which of its own findings its verifier can and cannot
> reach. It compares cost per task across a date boundary and refuses below ten tasks a side. It
> validates optimization candidates against a held-out set. It discloses and removes everything it
> writes to the machine. Where the evidence does not support a conclusion it declines to state one.

## 8. Research status

| Proposition | Status |
| --- | --- |
| Replay reconstructs Claude Code sessions from uninstrumented transcripts | **DEMONSTRATED** |
| It attributes cache breaks to causes | **DEMONSTRATED**, 929 breaks |
| It names TTL expiry where the provider's diagnostics does not | **DEMONSTRATED** |
| It compares cost across a boundary and refuses on thin evidence | **DEMONSTRATED** |
| It validates optimization against a holdout | **DEMONSTRATED** |
| Its epistemic boundaries hold | **PARTIALLY DEMONSTRATED.** One P0 found and fixed this session |
| Its capabilities are discoverable | **PARTIALLY DEMONSTRATED.** Named in help; one flag is not |
| It covers vendors beyond Claude Code | **FALSIFIED** for high-level analysis |
| Deterministic reconstruction differentiates it | **FALSIFIED** |
| Evidence discipline is a moat | **FALSIFIED** |
| Anyone needs this | **UNKNOWN** |
| Anyone will pay for it | **UNKNOWN** |

## 9. The next research question

**Not answerable by implementation, and deliberately left open:**

> What evidence would distinguish a technically real capability from a capability people actually
> need?

Three questions that must not be conflated: whether Replay can do something valuable, which the
repository evidence increasingly supports; whether people understand that value, not established; and
whether people will repeatedly use or pay for it, not established at all.

Nine downloads answers none of the second or third.

---

## 10. The integrity machinery caught the operator twice

Recorded because it is evidence about the repository rather than about the work, and because it is
the kind of thing that is embarrassing in the moment and useful in the record.

**The store registry caught a telemetry naming mismatch.** The surface counter's constant was first
named `surfaceCountFile`, whose suffix the registry scan does not match, so a new file under the
reader's home would have shipped **undisclosed by `replay privacy` and unreachable by `replay
purge`**. Renaming it to the convention made the guard fire, and satisfying it took four separate
disclosure sites: the registry, the guide's store count, the reviewer-facing list, and the ordered
disclosed slice.

**The orphan-document guard caught this memo.** It was written unlinked, and
`TestNoOrphanedDocuments` refused it before any success could be reported: *"documents nothing links
to, so nobody will find them."*

Neither was found by review, by intention, or by the operator noticing. **Both were found by a guard
that was already there and that fired without being asked.** That is the same property the product
claims for its own output, holding inside the repository that produces it, and it is worth more as a
recorded fact than as a principle anybody states.

---

[Documentation index](README.md) · [Repository README](../README.md)
