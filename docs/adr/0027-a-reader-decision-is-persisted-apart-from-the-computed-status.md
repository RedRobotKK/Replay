# 27. A reader decision is persisted apart from the computed status

**Status:** Proposed
**Date:** 2026-09-23

## Context

[8f32a31](https://github.com/RedRobotKK/Replay/commit/8f32a31) (2026-09-09)
removed the circularity inside `advisor.track`: a fall in a target's share used
to *be* the application, and the size of that same fall then decided whether the
prediction held. After that commit `track` refuses to judge anything until a
reader has said they applied it, and `Suggest`'s doc comment records the new
contract: `applied` "carries the ids the reader marked applied in `replay tui`,
and is the only thing that lets a suggestion be judged."

The commit closed the loop inside `track` and left it open outside. The caller
that supplies `applied` is `cmd/replay/triage.go:appliedIDs`, and it reads:

```go
if s.Status == advisor.Applied || s.Status == advisor.Verified ||
    s.Status == advisor.NotVerified {
    out[s.ID] = true
}
```

`Verified` and `NotVerified` are produced by `track` and by nothing else. No
reader can set them. Feeding them back as `applied=true` means every status the
pre-8f32a31 verifier had already written to disk is read, forever, as a human
decision. The state the old logic persisted was never cleared, so the fix landed
in one function and the old inference kept running through the file.

Measured on the machine this was written for, not argued. `~/.replay/advice.json`
held 147 records: 2 pending, 20 not verified, 124 advice only, 1 verified, and
**zero** reader dispositions ever recorded (0 applied, 0 dismissed). Regenerating
the same corpus with the current binary produced 222 records: 19 pending, 202
advice only, 1 verified. Of the 21 previously non-Pending records, 15 became
pending, 4 disappeared, the 1 verified became pending, and **1 moved from
"not verified" to "verified"** with nobody having marked anything. That single
promotion is the defect firing on live state.

The narrow repair, restricting `appliedIDs` to `Status == advisor.Applied`, is
wrong in a way that is worth writing down, because it is the change anybody
reading the function will reach for first. `Applied` is the only status a reader
can set today, so the filter looks exact. But `advise` recomputes every status
on every run and writes the result back, so the moment `track` turns a reader's
`Applied` into `Verified`, the next run reads `Verified`, finds it is not
`Applied`, passes `applied=false`, and `track` returns `Pending`. The reader's
keystroke is destroyed by the verifier acting on it. The lifecycle
pending to applied to verified cannot survive one round trip.

The underlying error is that one field is carrying two facts: what a person
decided, and what the verifier computed. They have different authors, different
lifetimes and different trust. They cannot share a slot.

## Decision

The reader's decisions are persisted separately from the computed status, in a
new file-level `decisions` object on `advice.json` keyed by suggestion id, whose
only permitted values are `applied` and `dismissed` (`advisor.Decision`). The
per-suggestion `status` field stays, is computed fresh on every run, and is
never read back as an input to anything.

`AdviceFileSchema` goes from 1 to 2. **A file at schema 1 contributes no
decisions.** Its statuses have unknown provenance: on the machine above, 21 of
them were written by a verifier that has since been proved wrong, and nothing in
the file distinguishes those from a status a reader caused. Reading them would
be the same laundering under a new name. They are discarded, every suggestion
returns to `pending`, and the reader marks again what they actually did. This
follows the precedent set for the ledger at schema 2, recorded in the changelog:
"Files written by schema 1 are skipped by the reader rather than misread."

Three rules follow from the split:

1. `appliedIDs` derives from the decisions object alone, and only from
   `applied`. **`dismissed` is never folded in.** A dismissal is a reader
   decision and it is not an application: nothing about the corpus moved, so
   there is nothing to verify, and passing it as `applied` would recreate this
   exact defect with a different constant. `advisor.Dismissed`'s own doc comment
   already says so.
2. `advisor.ApplyDecisions` overlays the reader's decision on the computed
   status **only where `track` returned `Pending`**, which is exactly the case
   where the verifier has nothing to say. A measurement is never overwritten:
   `Verified`, `NotVerified` and `AdviceOnly` stand. This is what lets the
   lifecycle round trip, because the decision, not the status, is what
   persists, and it keeps the `AdviceOnly` coverage figure in `advise`'s footer
   honest, since that count is taken from the status strings.
3. `markAdvice` refuses a file whose schema this build does not understand, the
   way `adviceFromCache` already refuses one, and refuses any status that is not
   a reader decision. A computed status can no longer enter the record at all,
   by construction rather than by care.

## Consequences

Anybody on a schema-1 `advice.json` loses their apparent statuses on the next
`replay advise`. On the measured machine that is a loss of 21 records, 20 of
which were false and the 21st unverifiable, so the loss is a correction. Nobody
loses a real decision, because the census found no real decision had ever been
recorded: 0 applied and 0 dismissed across 147 records.

Between upgrading and the next `replay advise`, `replay tui`'s `a` and `x` keys
refuse, because the file on disk is still schema 1. The refusal names the
command that fixes it. Silently upgrading the file instead would mean carrying
the old statuses forward under a schema number that asserts they are trustworthy,
which is the thing this record exists to stop.

A decision survives a suggestion disappearing and coming back, because it is
keyed by id at file level and carried forward wholesale, rather than living on a
suggestion record that a run may not regenerate. Four suggestions vanished
between the two runs measured above, so this is not hypothetical. The cost is
that `decisions` grows without bound and is never pruned; at one short id and one
word per entry that is a few kilobytes at the observed scale, and pruning it
would need a rule for how long a decision about an absent suggestion stays true,
which nothing here can answer yet.

`advise --out <path>` writes the reader's decisions into whatever file it is
given, as it already wrote their statuses. That is unchanged, and it is the
existing surface recorded in `docs/SURFACES.md`.

## Alternatives considered

**A persisted `applied_by_reader bool` on the suggestion, with the same schema
bump.** The obvious shape, and it fails on lifetime. The flag lives on a
suggestion record, and `advise` rebuilds that array from the corpus on every
run, so a suggestion that falls below `MinShare` for one session takes the
reader's decision with it and comes back pending. Four of the 21 tracked
suggestions disappeared between two runs over the same corpus, so the loss is
measured rather than imagined. Carrying the flag forward across runs requires a
map from id to flag held outside the array, which is the decisions object with
extra steps. It also gives dismissal nowhere to live: a second boolean, or a
boolean and an enum disagreeing with each other.

**Never persisting computed statuses at all, recomputing them each run.** The
right instinct, and it breaks the one thing the file exists for besides
tracking. `adviceFromCache` renders `Status` straight from disk so that
`replay tui` opens instantly instead of spending the 7.7 seconds a full corpus
analysis costs; drop the field and the screen either shows no status or pays
that cost on every open. The decision here gets the same guarantee more cheaply:
the status is persisted for display and is not an input to anything, so it can
be stale or wrong without a suggestion promoting itself.

**Clearing the old statuses in place and keeping schema 1.** Cheaper, and it
leaves no way to tell a cleaned file from one an older binary has since written
to. Two builds share `~/.replay`, and the schema number is the only signal that
survives that.

---

[ADR index](README.md) . [Documentation index](../README.md)
