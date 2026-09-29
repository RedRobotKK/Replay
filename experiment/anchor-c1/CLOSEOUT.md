# Evidence-anchor research: CLOSED

**STATUS: CLOSED. Do not reopen without new evidence.**

Opened 2026-09-29, closed the same day. Four experiments, 150 agent trials,
two compaction refutations, one prior-art investigation. **No implementation
followed, deliberately.**

This record exists so a future agent cannot mistake a behavioural finding for a
shipped Replay capability, or reopen a refuted hypothesis.

## Final evidence ledger

### OBSERVED, in the tested setup only

- An addressable evidence path materially increased retrieval.
- Correct evidence destinations produced substantially more evidence-supported
  resolution than plausible incorrect ones.
- A plausible incorrect destination could produce confident wrong answers.
- The effect occurred **after** retrieval had already begun.
- The behaviour is therefore consistent with a distinction between locating
  evidence and successfully resolving a claim from it.

### REFUTED

| hypothesis | how it died |
| --- | --- |
| Compaction causes sustained work loss (C4) | no effect at any window from 25 tool-turns outward; sign reversed at 200 |
| Post-compaction rediscovery (C3) | +9.1% became **-13.7%** once the summary turn's own filename mentions were excluded. 0 of 5 sessions positive, all leave-one-out CIs negative. It was the summary quoting filenames |
| Addressability is itself sufficient (L5) | both arms retrieved the supplied path 25/25 while resolution split 25/25 against 12/25 |
| A `[dispositive]` label causes verification | agents invoked the label **0/25** when the destination contradicted it |
| The experiment established truthful anchoring as the mechanism | it established destination sufficiency, which is not the same claim |

### NOT_OBSERVED

Generalisation across models, repositories or agent architectures. An
independent relevance effect. Runtime evidence-sufficiency verification by
Replay. Any quota, cost or performance optimization derived from this work.

### NOT_VERIFIED

Patent novelty. Any patentability conclusion.

## Experimental results

| experiment | design | primary endpoint | result |
| --- | --- | --- | ---: |
| C1 | 3 arms x 25, anchor vs none vs unrelated-wrong | retrieval | A 6/25, B 25/25, C 19/25. B vs A p = 1.2e-08 |
| C2 | 3 arms x 25, anchor vs none vs **plausible**-wrong | evidence-supported resolution | A 5/25, B 25/25, C 12/25. B vs C +52%, CI [+32%, +72%], p = 2.9e-05 |

C2 stage 1, opening the supplied path: **B 25/25 and C 25/25, p = 1.0.** The
entire difference arises after retrieval.

C2 arm C trajectory classification, prespecified: 9 answered from the aggregate
after visiting the wrong artifact, 7 recovered, 4 stopped there, 5 reached the
correct artifact. **Zero** ignored the supplied path. **12 of 13 failures
asserted a wrong answer rather than declining.**

Commits: prereg `588f600` (C1), prereg `45cdad4` (C2), ledger `0568d19`, all
committed **before** their results were seen.

## Historical correction, preserved rather than rewritten

The experiments labelled the supplied file anchor `[dispositive]`.

Repository inspection afterwards showed Replay's own taxonomy, in
`Replay-p5/docs/WORK-STATE.md`, distinguishes:

| anchor type | sufficiency |
| --- | --- |
| executable or recomputable, `run replay grok`, test `GK2` | **potentially dispositive** |
| file citation, `docs/evidence/*.md` | **locates evidence** |

**Every dispositive anchor in that table is executable. Every file citation is
marked as locating only.** The tested anchor was therefore a **locating**
anchor that happened to contain the answer, not a verified dispositive one.

The measured numbers stand. The label on them was wrong. This correction is
recorded rather than applied retroactively, because the experiment that ran is
the experiment that ran.

## Prior-art conclusion

Evidence-carrying termination and sufficiency gating already exist in the
literature. The closest single item is "When May an Agent Stop? Evidence-Carrying
Termination for Tool-Using LLMs" (arXiv:2608.23623, 2026-08-22), which binds each
claim to trace evidence with a deterministic replay check and uses sufficiency as
the termination gate.

Sufficiency as distinct from relevance is stated verbatim in at least three
independent 2026 sources, and ships in at least one MCP server. The ancestry runs
back through FEVER's NEI label, PCAOB AS 1105, and Toulmin (1958).

```text
PATENT PRIOR ART SEARCH = NOT_VERIFIED
NO NOVELTY CLAIM
```

Full record: `prior-art.md`.

## Replay implementation audit

Replay already contains `claim -> cited evidence -> predicate` in
`internal/regression/frozen_claims_test.go`, across 8 frozen guards. FD6 requires
a rate-limit claim to cite an evidence file and stats that the file exists.

**It is test infrastructure, not a runtime subsystem.** Its predicates are
hand-written per-claim Go test functions and nothing outside tests imports them.

`replay advise` has deterministic outcome predicates: `realized >= verifyShare *
predicted`, and its output is byte-identical across runs on the same corpus.
**An outcome predicate is not an evidence-sufficiency predicate.**

> Replay has ingredients of evidence verification. It does not currently possess
> a general runtime mechanism for proving that an advice claim is settled by a
> particular artifact.

This is an observation. **It is not an implementation requirement.**

## Implementation decision

```text
DO NOT IMPLEMENT
```

1. The proposed runtime mechanism is not sufficiently specified.
2. File addressability is not evidence sufficiency.
3. The existing frozen-claim predicates are hand-written per-claim test logic.
4. Generalising them to runtime would be a new subsystem.
5. The conceptual mechanism has substantial prior art.
6. The experiment does not establish that such a subsystem is necessary.

## The one product lesson, which is a design principle and not a claim

> If Replay ever exposes claims to agents, evidence locations should be
> resolvable and must never be represented as verified evidence unless Replay can
> actually establish the verification semantics.

**Not implemented. Not scheduled. Not a current product claim.**

A counter-signal worth carrying: PaperTrail (CHI 2026) found that exposing
claim-evidence mapping **lowered** researcher trust against baseline.

## Do not reopen

The compaction seam is closed by refutation. The anchor seam is closed by a
completed result plus a prior-art finding that the representation is
conventional. Reopening either requires **new evidence**, not a new idea.
