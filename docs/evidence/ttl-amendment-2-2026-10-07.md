# Amendment 2 to the TTL cache-behavior study: provenance machinery, marker-placement disclosure, the Block 2-6 schedule, and a permanent anchor rule

**2026-10-07. Filed after Block 1 closed (`BLOCK_ENDED`, commit `55be725`,
span not intact, no outcome data used, no sessions relabeled) and before
Block 2 opens.**

Amends: `docs/evidence/ttl-register-2026-10-05.md` (frozen at commit
`2552b53`, sha256 `6fb642fb94e211bde2f9aa353a40d4a49fdd25a7feec216539b8eb7f699a8363`,
recorded in `ttl-register-2026-10-05.sha256`) and
`docs/evidence/ttl-schedule-2026-10-05.json` (derived from that hash in the
same commit). **Neither file is edited, and neither will be edited by any
future amendment to this study.** Where this amendment and those files
differ, this amendment governs on the items named in section 1 below; on
everything else, they govern.

## 0. Why this is a separate file, and why that is now a permanent rule

Three adversarial governance panels examined a draft that proposed
incorporating this amendment's text directly into `ttl-register-2026-10-05.md`,
the way the first three amendments were. The third panel found a conflict
that discipline alone does not resolve: `scripts/ttl-block`'s schedule is
derived entirely from the register file's own SHA-256 (the digest's first
byte parity picks block 1's arm, and the arms alternate from there), and the
tool refuses to run any future block against a register whose hash no
longer matches what the schedule was derived from. Editing the register
to incorporate this amendment changes its hash. Regenerating the schedule
from the new hash would re-run that parity derivation with no connection
to the old one, and nothing in `scripts/ttl-block` checks a regenerated
schedule's block-1 assignment against `ttl-blocks-2026-10-05.jsonl`'s actual
record that block 1 ran as `arm: control`. A panel built and ran this
exact scenario on a reconstructed copy of the register, outside this
repository: appending one trailing space to an otherwise-identical amended
register flipped the computed parity from control to treatment. Nothing in
the code would have caught that contradiction before a future block ran
under it.

**The rule that follows, stated once, for this amendment and every one
after it:** `ttl-register-2026-10-05.md` and `ttl-schedule-2026-10-05.json`
are never edited again, for any reason, by any future amendment to this
study. Every future amendment to this study, this one included, is its own
separate, dated, committed file, following this one's shape. Its freeze
record is the commit that adds it, exactly as `ttl-register-2026-10-05.sha256`
is the freeze record for the register. `scripts/ttl-block`'s existing check,
the live register's hash must equal the schedule's recorded hash, keeps
working exactly as built, because the register it checks is never touched.
This amendment's own restatement of the blocks 2-6 schedule (section 7
below) is checked, not assumed: it is byte-for-byte identical to
`ttl-schedule-2026-10-05.json`'s existing `Blocks` array
(`control, treatment, control, treatment, control, treatment`). No
re-derivation happens because none is needed.

A later amendment that genuinely needs to change the hypothesis, the
metric, the arms, eligibility, or the 30-per-arm target cannot use this
mechanism: the register's own termination conditions (section 9 below)
already route that kind of change to a new, separately frozen register for
the blocks not yet run, with Block 1 (and any block already run by then)
staying under the register it actually ran under. That path exists and is
unused by this amendment, which touches none of those things.

## 1. What this amendment adds, and what it leaves alone

Added: client-version provenance, repository provenance, and
measurement-tool integrity recording, all implemented and verified at
commit `0866f2d`; a disclosed, unresolved marker-placement confound; a
prospective, reporting-only secondary measurement; a restatement (not a
redefinition) of the blocks 2-6 schedule and hard stop; and the permanent
anchor rule in section 0.

Unchanged: the hypothesis, the metric, the arms, the eligibility rules
(settings-span and cache-write exclusions), and the 30-per-arm target. The
register and schedule files themselves, unedited.

## 2. Marker-placement disclosure (unresolved confound, excludes no session)

Falsification work on the TTL resolver itself found it byte-identical
across client versions 2.1.278, 2.1.291 and 2.1.292: the function that
decides 5m versus 1h did not change. A different function, the one that
places `cache_control` markers in the request, did change between 278 and
the later two builds: `cache_control` occurrence counts of 73, 87 and 93
were measured across the three builds respectively, confirmed
independently more than once.

This is disclosed as an unresolved confound, not an established effect and
not an established non-effect. It is not known whether marker-placement
changes reach the sessions in this study's corpus, and it is not known
whether, if they do, they move the metric in either direction. No session
is excluded on this basis. Every report of this study's results states the
confound in these terms: the TTL resolver is unchanged; marker placement
changed; whether that matters here is open.

## 3. Secondary measurement: `cache_control` occurrence count (prospective, reporting only)

Starting with block 2, each session's `cache_control` occurrence count is
recorded alongside the primary and secondary metrics, per arm, per client
version. This is reporting only: it decides no arm membership, voids no
block, and excludes no session. It exists so that if the marker-placement
confound above turns out to matter, the data needed to say so by how much
was collected prospectively rather than reconstructed after the fact.

## 4. Complete client-version provenance

Previously, `Session.ClientVersion` kept only the first client version a
transcript's lines showed (`internal/transcript/claudecode.go`, the capture
at the version field). A session built partly under 2.1.291 and partly
under 2.1.292 was silently recorded as 2.1.291 only, with no way to tell a
straddling session from a session run entirely on one version.

`Session.ClientVersions` now records every client version a session's
transcript shows, first-seen order, deduplicated.
`Session.ClientVersionProvenance()` classifies the session as one of three
states: unmeasured (no version recorded at all), single (every line that
carried a version carried the same one), or `CLIENT_VERSION_STRADDLING`
(more than one version shown). `ClientVersion` is kept, unchanged in
meaning, as the first-seen version, for existing consumers outside this
study. Nothing in this study's current eligibility logic reads
`ClientVersion`, `ClientVersions`, or the provenance classification: arm
membership continues to run entirely on the span and cache-write rules
already in the frozen register, unchanged by this amendment.

## 5. Straddling-session disclosure (not an eligibility rule)

A session whose `ClientVersionProvenance()` is `CLIENT_VERSION_STRADDLING`
is recorded and reported. It is NOT excluded from either arm, NOT treated
as ineligible, and does not silently reduce the analyzable denominator for
either arm. This is disclosure only, on the same footing as the
marker-placement confound above: the field is reported beside the primary
and secondary metrics, per arm, so a reader can see how many sessions in
each arm straddled a client-version change, without that count changing
which sessions were counted.

An earlier draft of this section treated `CLIENT_VERSION_STRADDLING` as a
third exclusion, alongside the settings-span exclusion the register already
defines. On review, that was found to be a real eligibility change dressed
as a provenance disclosure: the register's only existing exclusion for a
client-version effect is behavioral (mixed cache writes in a session's own
usage breakdown, with "a client update" named as one possible cause of that
mixture), not provenance-based (which binary wrote which transcript line).
The resolver-byte-identical finding above means a straddling session will
typically show clean, single-TTL writes and would pass the register's
existing behavioral rule unchanged. Excluding it anyway, on provenance
grounds, would be a new eligibility rule, not a restatement of the old one,
and this amendment does not make that change.

If the project later wants `CLIENT_VERSION_STRADDLING` to function as an
eligibility exclusion, that is a future path, not this amendment: it
requires a separately frozen register, preregistered before the rule is
used to exclude or reweight any session, following the same discipline this
study's own register was built under. Nothing about that future register is
sketched or implied here beyond the fact that the path exists and is
separate from this one.

## 6. Block 1's validity: the literal register rule, retained, never reconciled retroactively, never extended forward

The frozen register states both that "a changed file does not void the
record" (block end) and that a valid exposed block requires
`span_intact: true`, among three necessary conditions: "A block that fails
any of the three is reported as such and contributes no sessions." Block 1
closed with `span_intact: false` (`ttl-blocks-2026-10-05.jsonl`,
`BLOCK_ENDED`, 2026-10-07).

**Block 1's official treatment.** The register's "Valid exposed block"
clause is applied to Block 1 literally, as originally worded, with no
amendment-driven reinterpretation: `span_intact: true` is one of the three
necessary conditions; Block 1 recorded `span_intact: false`; a block that
fails any of the three conditions "contributes no sessions." Block 1, taken
as a whole block, therefore does not meet the register's own "valid exposed
block" bar, and Block 1 contributes zero analyzable sessions to the study's
arm-level evidence. Block 1 was control-only (`ttl-blocks-2026-10-05.jsonl`,
`BLOCK_STARTED`, `arm: control`; the frozen schedule fixes block 1 as
control), so this was never a choice between contributing to one arm
instead of the other: there was only ever one arm Block 1 could have
contributed to, and it contributes nothing to it. This is derived directly
from the frozen register's own text as it stood before this amendment, not
invented here. This is Block 1's OFFICIAL treatment, fixed as of this
amendment, not subject to being revisited in Block 1's favor later, by this
or any future amendment, without a new register frozen before the
revisiting is proposed.

An earlier draft of this section resolved the apparent tension in the
register's own text by reading "block validity" and "session eligibility"
as two separate questions, in a way that let Block 1 contribute its
individually span-intact sessions despite `span_intact: false`. That
reading was written 44 minutes after Block 1 closed, by the operator whose
own block it determines the fate of, and it resolved the only real case it
governed in that case's favor. This amendment does not adopt it as Block
1's determination, for that reason, independent of whether the reading is
sound in the abstract.

**Sensitivity analysis / governance observation, not a validity
determination.** Separately from the official treatment above, it may be
informative to also report what Block 1 would have contributed under a
session-level-span reading: block validity as a property of
`BLOCK_STARTED`/`BLOCK_ENDED`/at-least-one-eligible-session, and per-session
eligibility as a separate question turning on each session's own span
against the settings-file hash at both ends, independent of the block's own
`span_intact` flag. Under that reading, Block 1 would contribute whichever
of its sessions individually never crossed the settings write or restore.
This reading MAY be shown, labelled exactly as "sensitivity analysis" or
"governance observation," alongside the official figure above. It is NOT a
replacement validity determination, it is NOT the official treatment of
Block 1, and no report of this study's results may present it as Block 1's
contribution without the official (zero-contribution) figure stated first,
with equal or greater prominence, beside it.

**No retroactive effect, and no future reopening.** This amendment does not
retroactively alter Block 1's determination. The official treatment above
is fixed; it does not become eligible for reconsideration by a later
amendment, a different operator, or a different mood about Block 1's cost
to the schedule.

**No new eligibility rule for blocks 2 through 6 either.** This amendment
does not adopt the session-level-span reading as the validity rule for any
block, including blocks 2 through 6. The register's literal "valid exposed
block" definition, applied to Block 1 above, is the sole operative rule for
every remaining block: `span_intact: true` is a necessary condition for a
block to contribute any sessions, and a block that fails it contributes
none, exactly as for Block 1. This amendment introduces no new eligibility
rule here, prospective or otherwise, and leaves nothing discretionary about
which validity rule governs a future block's span break. If a future design
wants a block to be able to contribute its individually-intact sessions
despite `span_intact: false`, that is the same kind of change the
straddling-session section above already routes correctly: a named future
path, not adopted by this amendment, requiring a separately frozen
register, preregistered before any block runs under it.

## 7. Measurement-tool integrity check

`scripts/ttl-block` now hashes its own measurement tool: a sha256 over the
sorted, concatenated contents of every `.go` file under `scripts/ttl-block/`
and `internal/transcript/`, read directly (no subprocess), captured as
`tool_tree_sha256` at `BLOCK_STARTED` and `BLOCK_ENDED`. A mismatch is
recorded as `tool_tree_intact: false` with the changed files named in
`tool_tree_changed_files`. Exactly as with the settings span, a mismatch is
disclosed, not an automatic exclusion and not a reason to void the block.
Analysis of a block's sessions is pinned to the `tool_commit` recorded at
that block's `BLOCK_STARTED`, which this check does not alter.

A second integrity check is added by this amendment and implemented
alongside it: `scripts/ttl-block` now refuses to start or end any block if
the schedule's recorded arm for an already-started block disagrees with
that block's own `BLOCK_STARTED` record in the log. This closes the gap
section 0 describes mechanically rather than by discipline alone: a future
session that forgot, or never knew, that the register and schedule are
never edited again would be refused by the tool itself, not merely by this
document, the moment its intended schedule disagreed with history.

## 8. Repository provenance

`Session.RepositoryID` now records which project directory a session's
transcript was recorded under, taken from the transcript file's own
location on disk, never from anything parsed out of the transcript's
content. A session with no file location carries the explicit value
`UNKNOWN`, never a blank string. Case and symlinks are not folded: two path
strings that differ only by case or only by symlink indirection are
treated as different repositories, by design, because under-merging is the
safer failure for the `>=2 repositories` requirement than over-merging.

The register already requires sessions to span at least 2 repositories;
this field closes the recording gap, but the requirement itself is not yet
mechanically enforced: no production code outside `internal/transcript`
collects `RepositoryID` across a study's eligible sessions or compares the
set's size against 2. Meeting the `>=2 repositories` requirement remains a
governance requirement checked by a person reading the recorded values,
until that gate is built and verified with the same discipline as the rest
of this amendment's machinery.

## 9. Schedule: blocks 2 through 6, restated, never re-derived

Block 2 treatment, block 3 control, block 4 treatment, block 5 control,
block 6 treatment, exactly as `ttl-schedule-2026-10-05.json` already
records. No block 7. No reopening block 1. Each block runs at most 10
calendar days. The study's absolute end is 2026-11-26 UTC, whichever block
is open when that instant arrives. Target at least 30 eligible sessions per
arm, spanning at least 2 repositories as defined above, both preserved
unchanged from the first amendment.

## 10. Shortfall treatment

If the hard stop (2026-11-26 UTC) arrives before 30 eligible sessions per
arm are reached, or before the `>=2 repositories` requirement is met, the
shortfall is reported as a flagged, preregistered deviation from the
design: the realized n per arm, the realized repository count, and which
target was not met, stated plainly beside the result. The target is never
silently reduced to match what was collected, and the study is never
extended past the hard stop to reach it.

## 11. Objective termination conditions

The study ends, rather than receiving a third amendment, on any of the
following:

- Block 6 closes, by reaching its own end or by the 2026-11-26 UTC hard
  stop, whichever comes first.
- Two consecutive blocks fail to verify (a `BLOCK_STARTED` that does not
  reach `state_change: VERIFIED`).
- A future Claude Code client update is found, by the same falsification
  method used for 2.1.278/291/292, to actually change the TTL resolver
  rather than only the marker-placement function. That finding ends the
  study rather than being absorbed as a confound, because it would mean
  the treatment itself is no longer the single thing varying between arms.
- The operator decides to stop.
- A proposed change would touch the hypothesis, the metric, the arms, the
  eligibility rules, or the 30-per-arm target itself, rather than reporting
  or provenance machinery. That is not an amendment to this study; it is a
  new register, written and frozen as one, before anything runs under it,
  governing the blocks not yet run while Block 1 (and any block already run
  by then) stays under the register it actually ran under.

## 12. Gates added by this amendment

| Gate | PASS | FAIL |
|---|---|---|
| G1 | `ttl-register-2026-10-05.md` and `ttl-schedule-2026-10-05.json` are byte-identical to their state at Block 1's close, for the life of this study | Either file differs: no block may start, per `scripts/ttl-block`'s existing hash check |
| G2 | A block already in `ttl-blocks-2026-10-05.jsonl` has the arm the schedule names for it | Disagreement: `scripts/ttl-block` refuses, per section 7's new check |
| G3 | This amendment is committed before block 2 starts | Block 2 starts without it: the study's provenance machinery (sections 4, 7, 8) is not in force for that block |
| G4 | The operator separately authorizes each block start, as the register's "What starts it" clause already requires | No automated or amendment-triggered start exists or is created by this document |

## Status

FROZEN by the commit that adds this file to the repository. This file is
never edited after that commit; a defect found later is corrected by a
further amendment, not by rewriting this one. The register and schedule
remain exactly as they were at Block 1's close. Block 2 does not start
until the operator runs `go run ./scripts/ttl-block start -block 2 -arm
treatment` themselves, separately from this freeze.
