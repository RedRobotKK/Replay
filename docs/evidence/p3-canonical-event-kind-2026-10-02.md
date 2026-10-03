# P3: the canonical substrate's first primitive, an event kind

**2026-10-02. Implemented, RED-first, mutation-proven. This is the whole of what
the P1 decision licensed.**

## P2: the minimum canonical model

The P1 decision rejected a Work Graph. What survived is one addition, and the
model is deliberately this small:

| primitive | why it is justified | why nothing else is |
|---|---|---|
| **`Record.Kind`**, optional string | A reader must be able to tell a record it cannot INTERPRET from one it does not IMPLEMENT. Without it the two are the same counter | Every other candidate entity (Work, Task, Message, Delegation, Dependency) has **0 producing lines** and no measured consumer |
| **`Session.UnknownKinds`**, counter | A record diverted but not counted is indistinguishable from one never written | — |

**Identity:** nothing added. `Kind` is Replay's own vocabulary, not a provider's.
`AgentID` remains an observed, provider-supplied field entering through the
adapter boundary, which is where the Replacement Test requires it to stay.

**Temporal:** nothing added. The analysis path is postmortem by construction, so
event time, availability time and analysis time do not yet diverge on any wired
surface.

**Provenance:** `Kind` is observed, read from the record as written. No
synthesized identifier was introduced, so no new provenance bit was needed. The
house rule, that every identity field is paired with a bit saying where it came
from, is not weakened.

**Migration:** none. `SchemaVersion` stays at **2**. `record.go:36-39` permits
exactly one evolution, "ADD optional fields, never rename or repurpose one", and
this is that. An absent `Kind` keeps its original meaning, so no historical
record is reinterpreted.

## The defect this repairs, which existed before any of this

`store.go` triaged by request shape and ended at an unconditional
`b.session.Skipped++`. `transcript.Session` documents `Skipped` as "lines the
parser could not interpret ... reported so a format change does not pass
silently", and `analysis/report.go:186` surfaces it.

So a record that parsed perfectly and merely carried a kind this binary does not
implement was reported to the user as **lost conversation data**. That is the
unknown-versus-unparseable conflation, on a surface that tells a user their
transcript is damaged.

## RED, before the repair

Both carriers were added first with no logic, so the failure is observable
rather than inferred:

```
Skipped:1  UnknownKinds:0
```

`TestUK1` failed with the record counted as a parse failure. `TestUK2` failed
with it counted nowhere at all.

**A positive and a negative control guarded the fixture, and one of them caught
my own error.** The first version of the positive control wrote a file
containing only an unparseable line; `ReadFile` refuses a file with no records
at all, so the control proved nothing. It is now paired with a valid record.

## The repair

A switch at the top of `SessionBuilder.Add`, before any request-shape triage,
because a record naming a kind is not a provider request and classifying it by
request criteria decides its fate on grounds that do not apply. A switch rather
than an `if`, so the place to add an implemented kind is a case above the
default.

## Mutations

| | verdict |
|---|---|
| C0 comment-only no-op (control) | **SURVIVED**, as required |
| M1 remove the kind gate, the pre-repair state | **KILLED** |
| M2 count an unknown kind as Skipped as well | **KILLED** |
| M3 treat a kindless record as unknown, reinterpreting every historical ledger | **KILLED** |
| M4 drop the unknown record silently | **KILLED** |

M3 is the one worth keeping: it is the migration hazard, and `TestUK3` exists
specifically to catch it.

## Verification

Full suite **37/37 ok, 0 FAIL**. Race clean, `-count=1`, on `internal/ledger`,
`internal/transcript` and `cmd/replay`. `gofmt` clean. `SchemaVersion` still 2.
The cost index key did not move, because `costUnit` was not touched.

## What this is NOT

It is not durable scratch, work continuity, inter-agent communication, quota
titration or realtime. It is the one primitive that lets any of those be carried
later without committing to them now, plus the repair of a defect that existed
today.

Those five capabilities remain **NOT MEASURED**, and the research directory
recording them is still frozen with production implementation **NONE
AUTHORIZED**.
