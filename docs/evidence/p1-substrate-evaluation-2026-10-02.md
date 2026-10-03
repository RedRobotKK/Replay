# P1: is `internal/ledger` the canonical event log?

**2026-10-02. Phase 1 architectural evaluation. Findings verified by me against
source; the agent survey that surfaced them is cited where it went further.**

## It already is an append-only event log

`internal/ledger/store.go:112-121`: `Append` opens with
`O_CREATE|O_APPEND|O_WRONLY`, writes one JSON line, closes. No seek, no rewrite,
no truncate anywhere in the package.

`ledger.Record` carries `Schema` (versioned), `ts`, `session_id`, **`agent_id`**,
`request_id`, `correlation`, `epoch`, plus policy, status, refusal and body
hashes. Two production writers: `passthrough.go:288` and `refusal.go:95`.

`store.go:272-300` already REPLAYS it: `ReadFile` → `ReadRecords` →
`sessionFromRecords` folds records into a `transcript.Session`. Projection is a
pattern this repo has shipped, not one to invent.

**Correction to my own Phase 0 baseline.** I reported "no Agent entity". Wrong:
`record.go:64-65` carries `AgentID`, "the client-supplied sub-agent header,
empty for the main loop", read from `x-claude-code-agent-id`
(`proxy/server.go:80`) and mapped onto a lane at `store.go:377`. I had searched
for `MessageSent`, `Delegation`, `WorkID` and `TaskID` but never for `AgentID`.
Agent identity is OBSERVED and provider-supplied, and is currently collapsed
into "lane".

## Three verified facts that decide the substrate question

**1. A non-request record is reported as LOST DATA.**
`store.go:374` is an unconditional `b.session.Skipped++` fallthrough. There is
no `Kind` discriminator on `Record`. So any new event type lands in `Skipped`,
which the transcript surface reports as lost conversation lines, in **every**
existing analysis path, until every consumer learns to filter by kind.

**2. The ledger is explicitly less durable than the probe log, in the same repo.**
`store.go:121` ends `return f.Close()`. **No `Sync()`.**
`probe/reading.go:124` ends `return f.Sync()`.
`store.go:176-181` further admits a reader "can land inside" a write, so a
record is observable half-written by design admission. The in-process mutex
gives no cross-process guarantee.

**3. The repository already voted against log-replay for continuity.**
The one capability that genuinely needed to survive restart did NOT replay the
ledger. It wrote a separate snapshot, `proxy/guards.go:421`:

> "spendStateFile holds the day's running total beside the ledger. Without it a
> daily cap resets whenever the proxy restarts ... A cap that resets is worse
> than no cap, because the operator believes it."

Live proxy state (`proxy/state.go:17-50`) is in-memory and rebuilt from nothing
on restart; it is never recovered from the log.

## What this establishes

The ledger is a sound **audit** substrate and an unsound **coordination** one.

| capability | ledger serves it |
|---|---|
| reconstruction | **well**, already does it |
| verification | **well**, `evidence.go` derives checkable claims; body hashes make "bytes unchanged" falsifiable |
| quota measurement | **well**, `Quota` headers captured verbatim |
| durable scratch | **poorly**, the record's charter forbids content (`record.go:1-3`); scratch is content |
| inter-agent communication | **poorly**, `AgentID` is a label not an address; per-session file sharding means no shared ordering, no delivery, no ack |
| realtime | **poorly**, no fsync, no watcher, readers are whole-file rescans |
| work continuity | **poorly**, nothing replays the log to restore live state, and the one case that needed it built a snapshot instead |

## Not yet concluded

Three further evaluations were running when this was written: the full identity
and provenance inventory, what the repo's persistence already proves about
atomicity and concurrency, and an adversarial seat briefed to falsify the Work
Graph hypothesis outright. The architectural decision is NOT made here.

## The persistence finding, and it is the architectural fork

Every persistence idiom in this repository assumes its data is **recomputable**.
Durable scratch would be the first thing in Replay that is not.

Verified by me:

| | measured |
|---|---|
| cross-process locking: `flock`, `LOCK_EX`, `F_SETLK`, `lockfile`, `pidfile`, `.lock` | **0 files**, across 773 `.go` files |
| `os.Rename` sites | **10** |
| `.Sync()` sites | **3**, none after a rename |

And the versioning policy is explicit, `internal/ledger/record.go:22-39`:

> "BUMPING IT DISCARDS EVERY EXISTING LEDGER ... a bump to 3 does not migrate a
> schema-2 file, it makes every record in it unreadable on a machine that
> already holds months of them ... the real rule is: ADD optional fields, never
> rename or repurpose one. If a change genuinely cannot be expressed that way,
> changing this constant is a migration with a cost, not a version bump, and it
> needs a reader that accepts the older number."

**That reader does not exist.** Every version gate in the repository, the
ledger's, the cost index's, the surface counters', is discard-and-rebuild.

That is **correct** for derived data: a discarded cost index costs one cold walk.
It is **data loss** for work state a person cannot reconstruct.

So the proven patterns transfer only partly. Reusable as-is: derived schema keys
over hand-maintained constants, which two shipped wrong-number incidents paid
for; keying on build identity as well as shape; per-entry validity by
`(size, mtime-nanos)`; the four-way read classification that keeps a torn tail,
a superseded schema and lost bytes apart; append-only with last-line-wins;
bookkeeping failures never failing the request.

Not reusable, and each is load-bearing for scratch:

1. **No cross-process coordination exists at all.** The nearest precedent is the
   UDS dial-probe at `proxy/uds.go:141-168`, which is exclusion-by-ownership
   rather than locking.
2. **No durability past rename.** A crash window at all ten sites. Seven of nine
   atomic-write sites also use a fixed `path+".tmp"`, which two processes
   clobber; only `quotastore.go:53` uses `os.CreateTemp`.
3. **No migration, anywhere.** This is the fork: a durable scratch that discards
   on upgrade is not durable.

## Position after three of four evaluations

The ledger is a sound audit substrate. The persistence discipline is excellent
for recomputable data. Neither property survives contact with work state that a
person would lose.

**The adversarial evaluation has not reported. No architectural decision is
made here.**

## PHASE 1 DECISION: the scope claim is falsified; the modest core is supported

Four parallel evaluations, one briefed adversarially. Every claim below was
re-verified by me against source before being acted on.

### Falsified, by evidence already in this repository

**1. "All seven capabilities compose through one canonical substrate."**
The repository has not merely failed to build composition; it has built and
mutation-tested a guard AGAINST it.

`RPL-C021`, `Result: Established` — "Replay offers no cross-surface grand total",
establishing "figures with incommensurable units are not added", and
"Enforced by the shape of the type rather than by a convention, **and a mutation
that adds a total is caught**."

`RPL-C019`, `Result: NoEndpoint` — the identity any cross-agent join requires:
"**Not a testing gap: there is no endpoint to test.** Two records from different
accounts sharing a provider request id are indistinguishable from the same
request seen twice, because nothing in the evidence model names the account."

**2. "Spanning time" as an evidence frontier.** Already classified NOT
APPLICABLE to the wired product: the analysis path is postmortem by construction.

**3. Any implicit premise that durable work state is known to help.** The one
preregistered outcome trial returned **INVALID EXPERIMENT, control 5/5,
treatment never run**. The replication attempt returned **NOT REPLICATED,
p=1.0000, risk difference 0%, CI [0%, 0%]**.

### Not licensed, though not false

Communication, dependencies and outcomes have **0 lines each**. Agent identity is
a client-supplied header with one consumer, and `docs/DIRECTION-2026-09-24.md:26`
states: "**Not present, and must not be assumed:** any agent identity model
across sessions, any stored longitudinal history, any team or multi-user
concept."

The only quantitative reading on inter-agent messaging in this repository is a
**+28% token tax with zero measured benefit**.

`docs/research/README.md`: "The campaign is **frozen**. Nothing in this directory
authorizes an experiment." · "Production implementation | **NONE AUTHORIZED**."

### Conceded as justified, and buildable now

- **Evidence-bounded** is correct and is this repository's strongest asset.
- **One vocabulary, not one graph**, is the repo's own documented position:
  "One evidence model, several lenses", and "BUILD NOW: one internal vocabulary
  so these stop being per-surface conventions".
- **The state taxonomy is already adequate.** E4-01 rejected its own null: all
  eight constructed states produced distinct fingerprints and `Settle` already
  enforces action-is-not-outcome.
- **The serializable carrier gap is real**, verified twice: `stateledger` has
  0 JSON tags, no `Marshal`, no production caller, so a claim crossing a process
  boundary arrives as ASSERTED with no way to tell "nobody has looked" from
  "an action is outstanding" from "this was dropped".

### The architectural reason, not merely the evidentiary one

Every persistence idiom here assumes its data is **recomputable**. A discarded
cost index costs one cold walk. Durable scratch would be the first
non-recomputable thing in Replay, and the repository has **no migration
anywhere**, **no cross-process locking in 773 files**, and **no fsync after any
of ten renames**.

Building a ten-dimension substrate on that foundation inverts the dependency: it
puts the largest new construction on the least-proven ground.

### What is supported

One vocabulary, plus a serializable carrier for the state taxonomy that already
exists and is already adequate. That is a bounded piece of work with verified
need. It is not seven capabilities and it should not be described as them.
