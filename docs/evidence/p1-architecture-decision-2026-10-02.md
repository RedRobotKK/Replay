# P1 architecture decision: extend the ledger, do not build a Work Graph

**2026-10-02. Decision taken from repository evidence. Every claim cited below
was re-verified against source before it was acted on.**

## 1. Problem statement

Replay 1.0 is mandated to gain durable scratch, work continuity, inter-agent
communication, reconstruction, verification, quota titration and realtime, and
these are required to compose through one canonical multidimensional model of
work rather than becoming seven subsystems.

The question this document answers is not "what would a good architecture look
like". It is: **what does the repository's own evidence license building?**

## 2. Evidence inventory

Four parallel evaluations, one briefed adversarially. Classified by kind.

### OBSERVED (read from source, re-verified by me)

| | |
|---|---|
| `internal/ledger` is append-only | `store.go:112-121`, `O_CREATE\|O_APPEND\|O_WRONLY`, no seek, truncate or delete in the package |
| It is already replayed into a projection | `store.go:272-300`, `ReadFile` → `ReadRecords` → `sessionFromRecords` |
| `Record` carries schema, ts, session, agent, request, correlation, epoch | `record.go:58-178` |
| **It has no event-kind discriminator** | `record.go`, no `Kind`/`Type` field |
| **An unrecognised record is reported as lost conversation data** | `store.go:374`, unconditional `b.session.Skipped++` |
| `Append` does not `Sync()`; the probe log does | `store.go:121` vs `probe/reading.go:124` |
| No cross-process locking exists | 0 hits for `flock`, `LOCK_EX`, `F_SETLK`, `lockfile`, `pidfile`, `.lock` across 773 `.go` files |
| 10 `os.Rename` sites, 3 `.Sync()` sites, none post-rename | measured |
| No migration exists anywhere | `record.go:22-39`, "BUMPING IT DISCARDS EVERY EXISTING LEDGER" |
| Lane identity is derived three incompatible ways | `claudecode.go:365-369` chain-root UUID, `ledger/store.go:377` AgentID header, `cost.go:183` **filename** |
| Agent identity exists and is OBSERVED | `record.go:64-65`, `x-claude-code-agent-id` via `proxy/server.go:80` |
| Every identity field is paired with a provenance bit | `IDMeasured`, `IDFromMessage`, `Correlation`, `TagBasis`, `RequestIDMeasured` |

### DERIVED CLAIMS, registered and verified

| | |
|---|---|
| `RPL-C021` **Established** | "Replay offers no cross-surface grand total" … "Enforced by the shape of the type rather than by a convention, **and a mutation that adds a total is caught**" |
| `RPL-C019` **NoEndpoint** | "**Not a testing gap: there is no endpoint to test.**" No account, tenant, organisation, project or workspace identity exists in the correlation path |
| `RPL-C016` **NotMeasured** | "Replay improves agent task outcomes", DoesNotEstablish "anything in either direction" |

### MEASURED RESULTS

- The preregistered durable-work-state trial: **INVALID EXPERIMENT**, control 5/5, treatment never run.
- Its replication: **NOT REPLICATED**, p=1.0000, risk difference 0%, CI [0%, 0%].
- The only quantitative reading on inter-agent messaging in this repository: **+28% tokens, +$0.063 per run, zero measured benefit.**
- `docs/research/README.md`: "The campaign is **frozen**" · "Production implementation | **NONE AUTHORIZED**".

### HYPOTHESES, explicitly not established

That any of the seven capabilities improves an outcome. That a single substrate
beats separate mechanisms. That anyone needs work continuity. All **NOT MEASURED**.

## 3. Alternatives considered

| candidate | verdict | deciding reason |
|---|---|---|
| **graph database** | **rejected** | No queryability requirement is demonstrated. Adds a dependency, a migration and an operational surface to serve zero measured consumers |
| **event-sourced graph** | **rejected** | Requires the cross-entity identity `RPL-C019` records as having no endpoint |
| **normalized event stream + indexed relationships** | **rejected for now** | The relationships it would index (`depends_on`, `delegated_to`, `communicated_to`) have 0 lines of producing code and no measured consumer |
| **separate durable document store** | **rejected** | Would be a second source of truth beside a log that already replays into a projection |
| **hybrid log + graph projection** | **partially adopted**, see Decision | The projection half is already how this repo works. The graph half is unjustified |
| **current ledger extension** | **ADOPTED** | Smallest change, backward compatible, and it repairs a defect that exists today |

## 4. Adversarial falsification results

The adversarial seat was briefed to falsify, and it succeeded in part.

**FALSIFIED: "all seven capabilities compose through one canonical substrate."**
The repository has not merely failed to build composition. `RPL-C021` is an
`Established` claim, enforced by type shape, with a mutation that adds a
cross-surface total being caught. `RPL-C019` is `NoEndpoint` on the identity any
cross-agent join requires. Composition is guarded against, deliberately.

**FALSIFIED: "spanning time" as an evidence frontier.** Classified NOT
APPLICABLE: the analysis path is postmortem by construction.

**FALSIFIED: any premise that durable work state is known to help.** Two
measured nulls, above.

**NOT LICENSED, though not false:** communication, dependencies and outcomes as
dimensions, 0 lines each with no measured consumer. `DIRECTION-2026-09-24.md:26`:
"**Not present, and must not be assumed:** any agent identity model across
sessions, any stored longitudinal history, any team or multi-user concept."

**CONCEDED AS JUSTIFIED:** evidence-bounded semantics; one *vocabulary* rather
than one graph, which is the repo's own documented position; the state taxonomy
is already adequate (E4-01 rejected its own null); and the serializable-carrier
gap is real.

## 5. Decision

**Extend `ledger.Record` with an optional event-kind discriminator and preserve
unknown events. Build nothing else.**

Concretely, and only this:

1. An optional `Kind` field on `Record`. Absent means the existing meaning, a
   provider request, so every historical record keeps its interpretation.
2. A record whose kind is unrecognised is **preserved as an unknown event**
   rather than counted as lost conversation data.
3. Provenance for any Replay-synthesized identifier, following the house rule
   that every identity field is paired with a bit saying where it came from.

This is adopted because it is the only change in the candidate set that is
justified by a defect existing **today**, independent of any future capability:
`store.go:374` reports an unrecognised record as `Session.Skipped++`, which the
transcript surface renders as lost conversation lines. That is a measured
overclaim of exactly the kind this repository exists to police.

The layering the mandate describes, evidence → canonical event model →
reconstructed state → capability projections, is **preserved in shape**: the
ledger is the evidence, `sessionFromRecords` is the projection. What is not built
is the Work Graph between them, because its entities have no producers and its
composition is type-guarded against.

## 6. Explicitly rejected

- A graph store, a graph model, and an event-sourced graph.
- First-class `Message`, `Delegation`, `Dependency`, `Work` and `Task` entities.
- Cross-agent relationship indexing.
- Any cross-session or cross-account identity.
- A migration that reinterprets historical records.

Each is rejected for absence of a demonstrated producer or consumer, not for
implementation cost.

## 7. Why this is minimally sufficient

It is the smallest change that makes the substrate **extensible without being
speculative**. A kind discriminator costs one optional field and unlocks every
future event type without committing to any of them. Preserving unknown events
is required whether or not a Work Graph is ever built, because the alternative
is the reader silently reporting data loss.

It satisfies the mandate's own layering while declining the part of the mandate
the evidence falsifies.

## 8. What remains experimentally unproven

**Everything the seven capabilities would be for.** Work continuity, durable
scratch benefit, inter-agent communication value, quota titration benefit: all
**NOT MEASURED**, and four separate research efforts died on the same missing
primitive, a machine-checkable task-outcome signal joinable to a resource trace.

This decision does not make them measurable. It makes the substrate able to
carry them **if** evidence ever arrives.

## 9. Migration strategy

**None required, and that is the point.** `record.go:36-39` permits exactly one
form of evolution: "ADD optional fields, never rename or repurpose one."
`SchemaVersion` stays at 2. An old reader ignores an unknown field; a new reader
sees it as absent and applies the existing meaning. Historical records are not
reinterpreted.

If a kind is ever added that an older binary cannot interpret, that binary must
preserve it as unknown rather than discard it, which is precisely the behaviour
this decision implements.

## 10. Replacement Test

`AgentID` is `x-claude-code-agent-id`, a Claude Code header. It must not become
Replay's ontology.

Under this decision it does not: the canonical addition is a *kind*, which is
Replay's own vocabulary, and provider identity continues to enter as an observed
field with a provenance bit. Replacing Claude Code with another runtime changes
which adapter populates `AgentID` and changes nothing about the kind
discriminator or the unknown-event rule.

The deeper coupling identified, three incompatible lane-id derivations, is
recorded as a known defect and is **not** repaired here, because the one that
caused a release blocker (`cost.go:183`, the filename) was already fixed.

## 11. Failure, restart and concurrency implications

This decision deliberately does **not** change durability, concurrency or
restart behaviour, because none of those are required by it.

Recorded for whoever does change them: `Append` does not `Sync()`; the mutex is
process-local; there is no cross-process locking anywhere in 773 files; there is
no fsync after any of ten renames; and every version gate discards rather than
migrates. Those are acceptable for a recomputable audit log and would not be
acceptable for non-recomputable work state. **That gap is the real cost of the
capabilities this decision declines to build.**
