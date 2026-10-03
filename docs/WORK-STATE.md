# Work state

The durable cursor. **Where the work is, and what the next legal action is.**

This file is not a journal and not a summary of the conversation. It holds only
what a fresh agent cannot reconstruct from the repository itself. Established
facts live in `docs/evidence/`, decisions live in `docs/adr/`, release status
lives in `docs/ROADMAP.md`, and capability lives in `docs/SURFACES.md`. This file
points at those; it never copies them, because a copy drifts and the original
does not.

## RECONCILE FIRST

**Do not act on anything below until it has been checked against the repository.
Where this file and the repository disagree, the repository wins.**

```sh
git rev-parse --abbrev-ref HEAD      # expect the branch named below
git log -1 --format='%h %s'          # expect the commit named below
git status --porcelain               # expect the working tree named below
make lint && make docs-lint          # expect green
```

If any check disagrees, **repair this file before continuing**, and say in the
report that it had drifted. A fresh agent must never infer that work is complete
because this file says so.

---

## Objective

Carry the state-ledger workstream to a committed, gated state behind the frozen
Release 1.0 certification, without wiring anything the evidence says not to.

## Phase

CLOSE. Release 1.0 is certified and frozen at `9694f1e`. The state-ledger
package, its evidence and this cursor are committed on top of it as a separate
workstream. Nothing is being built; what remains is recorded in Open.

## Anchors

| | |
|---|---|
| branch | `fix/compaction-observed-vs-inferred` |
| frozen release commit | `9694f1e`, `certify release 1.0 production surface gate`; never amend, reset or rewrite it |
| this workstream | the commit after it, `state ledger: commit the prototype, its evidence and the cursor` |
| gates at that commit | `gofmt -l` 0, `go vet ./...` clean, `git diff --check` clean, `go test ./... -count=1` 37 ok 0 FAIL, race 37 ok 0 races, observed 2026-10-03 |

**This file had drifted by eight days when reconciled on 2026-10-03.** It still
named commit `6871022`, phase VALIDATE and a working tree of uncommitted Grok
reader files that had long since been committed. The reconciliation rule above
is what caught it; the repository won and this section was rewritten.

### Working tree expectation

Clean after the state-ledger commit, except for `.claude/settings.local.json`
and `.claude/worktrees/`, which are ignored. A tree that differs is drift, not
progress. Find out which before continuing.

### The state ledger, as it stands

`internal/stateledger` is a prototype representation of beliefs, checks and
open questions. It is **deliberately not in the shipped binary**
(`docs/design/UNWIRED-LOG.md` §15; `internal/regression/unwired_packages_test.go`
lists it with the reason). It has no persistence, no carrier across a process
boundary and no production consumer, and the evidence says not to build those
yet: `docs/evidence/e4-01-state-representability-2026-10-02.md` found the
taxonomy adequate and the carrier gated on an outcome signal no corpus carries,
and `docs/evidence/p1-architecture-decision-2026-10-02.md` chose to extend
`internal/ledger` rather than build a work graph. The one primitive that did
ship is the event kind, `docs/evidence/p3-canonical-event-kind-2026-10-02.md`,
inside the frozen release. On 2026-10-03 the prototype gained one repair: a
check or revision recorded against a claim that does not exist is rendered
under ORPHANED instead of vanishing (`internal/stateledger/orphan_test.go`,
two hand mutations killed).

## Next action

```sh
# 1. confirm the gates still hold
gofmt -l cmd internal && go vet ./... && git diff --check && go test ./... -count=1
```

**Expected:** nothing from gofmt, nothing from vet, whitespace clean, 37 ok.
**Then** pick from Open only with new evidence; everything there is open for a
stated reason, and re-proposing a Do-not-repeat item is a regression.

## Established, with its anchor

Each row names where the claim is checkable. Do not restate a finding here that
the anchor already holds.

**Anchor sufficiency is a property of the claim-anchor PAIR, not of the anchor.**
E008 measured this: an anchor naming a check the agent can perform but which
cannot settle the claim produced apparent closure and HALVED detection of a
false inherited claim, 8/10 with no anchor against 4/10 with an executable but
insufficient one. So each row below says what its anchor can actually do.

- **dispositive** - performing the check settles the claim
- **locates evidence** - shows where related evidence lives; does NOT settle it

| claim | anchor | anchor can |
|---|---|---|
| `replay grok` reads 82,320,019 tokens across 104 sessions | run `replay grok` | **dispositive** |
| Grok's `costUsdTicks` scale is unrecoverable, so no dollars are printed | `cmd/replay/grok.go` header comment; `GK2` | **split**: `GK2` is dispositive for "no dollars are printed"; "scale is unrecoverable" only **locates evidence** (the 842-record fit is recorded in prose, not re-run) |
| ~~Grok records are cumulative, so summing them inflates every figure~~ **CONTRADICTED 2026-09-27, see below** | `GK1`, mutation-proven | **NOT dispositive** |
| Bounding tool output on lookup work: haiku 5/5 correct at $0.0377 vs 1/5 at $0.1369 | `docs/evidence/modes-catalogue-2026-09-25.md` | **locates evidence** |
| Every model saves ~70% of cache writes when bounded, clean | same file, CLEAN RE-MEASUREMENT section | **locates evidence** |
| 73-90% of an optimised run is fixed prefix; headroom is 10-27% | same file, Amdahl section | **locates evidence** |
| agmsg costs sonnet 10,450 tokens (28%) per run | same file, CORRECTION section | **locates evidence** |

### The cumulative-records claim is contradicted, and its anchor is why it survived

**The claim is retained above, struck rather than deleted, because this is the
failure mode E008 predicted, occurring in this file.**

`GK1` runs on a three-line synthetic fixture built in a `t.TempDir()` whose
records are cumulative **by construction**. It proves the reader does not sum
records it is given. It cannot establish that real Grok records are cumulative,
because it never reads any.

Measured against the real corpus on 2026-09-27: within a session and model,
**359 of 1,040 consecutive records DECREASE** from the previous record (34.5%).
A monotonic running total cannot decrease. A second session independently
reported the same direction at a different denominator (245 of 653).

**A dispositive anchor for this claim exists and is now named:** count
within-session decreases in `~/.grok`; any decrease falsifies cumulativity.

**Consequence, not yet actioned:** `replay grok`'s 82,320,019-token figure
rests on a max-per-session reading of records that are not monotonic, so it may
be wrong. That is a separate question from this audit and is recorded in Open.

## Open

| question | why it is not closed |
|---|---|
| cold-prefix cost | four attempts, all killed. Script at `~/.claude/jobs/581b6292/tmp/cold/coldtest.sh`, needs 60 min with no `claude -p` running |
| opus measurement | 6 of 6 runs contaminated even with `--disable-slash-commands`; no known isolation |
| cross-model section of the catalogue | superseded table still sits above the clean one; needs rewriting, not annotating |
| `AGENTS_STATE.md` | 105KB, tracked, last touched 2026-09-13 with the `othersurfaces.go` work it describes. Finished, and nothing says so |
| is `replay grok`'s token total correct | the cumulative-records assumption it rests on is contradicted (359/1,040 decreases). Not re-derived. Do not cite the 82,320,019 figure until this is settled |

## Do not repeat

Rejected with evidence. Re-proposing any of these without new evidence is a
regression.

- **PreToolUse output-trimming hook.** +34.5% and +32.8% cache writes on the two
  cleanly measurable cells, plus a correctness loss. A hook caps the output of a
  plan already chosen; an instruction changes the plan.
- **"Search churn predicts failure."** Killed by a scoring error of mine, then
  killed again properly: BOUNDED+ drove churn to 1-2 and correctness did not move.
- **Vocabulary mismatch as the determining mechanism.** Failed its own registered
  falsifier. The real discriminator is a competing causal explanation in context.
- **Cross-model saving differences as a model property.** Mostly the agmsg skill.
- **Installing the released v0.6.2.** It is older than this branch and would drop
  the advisor fix, the stale-writer guard and the compaction fix.
- **`--bare` for clean measurement.** It disables Anthropic auth and returns zero
  tokens.

## Method constraints learned the hard way

- **n=10 before the word "systematic."** Six claims were made and killed on
  2026-09-25; four were small samples read as laws.
- **Never keyword-score prose.** One scorer matched "populations" in an unrelated
  sense and killed a rule that was correct.
- **Check contamination per run, not per cell**, and say how many were excluded.
- Use `cache_creation_input_tokens`. Cache reads swing 7.4x between identical runs.
