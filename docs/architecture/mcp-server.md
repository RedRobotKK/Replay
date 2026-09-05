# MCP server

A second front door onto the same engine, for agents rather than people.

**Nothing here is built.** There is no `replay mcp` command. This records the
design and, more usefully, the measurements that killed the first version of
it — because the first version was wrong in ways that looked right.

## What the measurements changed

The original draft was written on 2026-09-05 and reviewed the same day. Three
findings reshaped it. Each is a number, not an opinion.

**The lead was wrong.** The draft's headline was memory hygiene: price every
`CLAUDE.md` and `MEMORY.md` entry, and say which never earned its place. In one
real ledger record, on this machine:

```text
tool definitions   226,829 bytes   133 tools
system prompt        9,243 bytes
CLAUDE.md            6,815 bytes
```

Tool definitions are **33× the memory file**, and unlike memory they are
already attributable per name (`Prompt.Tools []ToolDef`), with
`advisor.unusedTools` already pricing tools that were never called. The
expensive thing was measured, itemised, and sitting in the ledger the whole
time.

**The founding constraint was measured on an outlier.** "No tool call can do a
cold read" came from 4.3 seconds to parse one session. That session is 336 MB.
The distribution across 86 top-level sessions:

```text
p50    161 KB    0.00 s
p90    1.4 MB    ~0.05 s
p99   84.7 MB    ~1.0 s
max    352 MB     4.24 s
```

A one-second budget covers 85 of 86 sessions. **Per-session tools need no index
at all.** Only whole-corpus and whole-project questions do — 16.7 s and 12.6 s
respectively, both reproduced. The architecture below is narrower as a result.

**The strongest-sounding idea had no data behind it.** Pricing a memory entry
as "paid once in the cached prefix" versus "re-billed every turn" requires
knowing where the cache breakpoints sit. Nothing records them. The proxy keeps
`Prompt.CacheControlCount` — a *count*, incremented and then discarded in
`summarize.go`.

The marker is not missing from the wire, which makes the fix cheaper than it
first appears: `transcript.RawBlock` decodes `cache_control` at
`wire.go:51`, and the domain `transcript.Block` simply does not carry it
forward. The positions arrive and are dropped. Recording the block index of
each marker is a small change to a struct and a conversion, not a new capture
path — but until it happens, the distinction the whole idea rested on is not
available on either tier, and it surfaces only as a residual with an ±89%
error bar.

## Why this exists

Replay's users are increasingly not people at a prompt. The bill surprises
someone, and the useful question — *which turn did that, and why* — is one the
agent should be able to ask directly.

But a second UI onto post-hoc forensics is not worth the surface area. The CLI
already does that, better, to a human who applies judgement. **The only things
worth exposing here are the ones where the consumer is the agent and the agent
can act on the answer.** That test cuts most of what was proposed, and it is
applied explicitly at the end.

## The lead: tool-definition hygiene

Every MCP server a user connects loads its tool definitions into the system
prompt, on every request, for the life of the session. Fourteen connected
servers is a normal number. One or two get used.

This is the rare case where Replay knows something nobody else does, the data
already exists, and the consumer can act:

| Tool | Answers | Tier |
|---|---|---|
| `replay_tool_cost` | Every tool definition by name, its bytes, its share of the prefix, and whether it was called | measured |

Two properties make it worth building first. It is **per-name attributable from
data already in the ledger**, so it needs no new capture. And it is **the one
question an agent can act on alone**: disconnecting an unused server is
reversible, cheap, and needs no judgement about the work.

It has a second, sharper use. Tool definitions live in the *prefix*, so adding,
removing or renaming a server invalidates the cache for the whole session. That
makes tool churn both a standing cost and a break cause, and it is why
`internal/analysis/order.go` treats `ScopeTools` as prefix-rewriting alongside
the system prompt.

## Prefix-safe ordering

`internal/analysis/order.go`, built 2026-09-05 and the one piece of this design
that exists.

N prefix-rewriting actions interleaved with appends cost N re-prefills; the same
N done together cost one. That is the whole optimisation, it is free, and it is
not obvious.

**It returns arithmetic and a truth tier, never a verdict.** An earlier shape
answered `would_break(action)` with a boolean. A review argued for cutting it
and the reasoning holds: a human reading a report applies judgement, but an
agent calling a boolean mid-flow treats it as a gate, and gates earn trust
monotonically — ten correct answers buy the eleventh unquestioned. The error
modes are asymmetric in the direction Replay cannot see. A wrong "safe" costs
one re-prefill: bounded, and exactly the waste already being paid. A wrong
"breaks" makes the agent skip a legitimate read, which costs a worse
engineering outcome — unbounded, silent, and invisible to a tool that measures
spend rather than success. **The bill would fall while the work got worse, on
instrumentation incapable of noticing.**

The caller supplies the prefix size and the token counts. Replay does not
forecast them: the byte-to-token fit carries error bars up to ±159% across
sessions, and a recommendation driven by that has no business existing. The
result is labelled `structural` and says so in its own text.

## Transport: stdio, and not HTTP

The client spawns `replay mcp` and speaks over stdin and stdout. Not a daemon,
no port, not always on.

An HTTP server would be a listening socket on a tool whose entire argument is
that it runs on your machine. It would need auth, because a port on localhost is
reachable by every process on the box including a browser tab. stdio inherits
the client's identity and lifetime and needs none of that.

## Latency: an index only where one is needed

Revised from the original, which required an index for everything.

| Scope | Cost | Needs an index |
|---|---|---|
| One session (p50, 161 KB) | 0.00 s | **no** |
| One session (p99, 85 MB) | ~1.0 s | no |
| One session (max, 352 MB) | 4.24 s | it would help |
| One project (1.0 GB) | 12.6 s | **yes** |
| Whole corpus (1.3 GB) | 16.7 s | **yes** |

So: **session-scoped tools read the session.** No staleness, no index, no
apology. Corpus-scoped tools answer from `~/.replay/index`, built by a job.

That split also fixes an incoherence in the original. The session an agent is
*in* is the one file guaranteed to be growing, so an index-backed answer to
"what is in my context now" would be a snapshot from before everything that
prompted the question — confidently wrong in the direction that causes action.
A session-scoped live read has no such problem and costs nothing at the median.

**The index is shared mutable state, so it is treated as such.** Mode 0600,
owner verified, and an index that is group- or world-writable is refused rather
than read: a predictable path under `$HOME` is reachable by every process on
the box, and unlike a port it survives reboot.

## Services

Five groups, split by what each can do to the machine, and mapped onto MCP tool
annotations so the client can enforce the boundary rather than trusting prose.

| Group | Tools | Annotation |
|---|---|---|
| 1. Measure | `replay_tool_cost`, `replay_blame`, `replay_diff`, `replay_context` | `readOnlyHint: true` |
| 2. Cost | `replay_session_cost`, `replay_what_changed` | `readOnlyHint: true` |
| 3. Plan | `replay_order_plan` | `readOnlyHint: true` |
| 4. Advise | `replay_advise` (read), `replay_learn` | read-only; apply requires `--allow-write` |
| 5. Health | `replay_health` | `readOnlyHint: true` |

Never exposed, and this is the row to defend: `serve` starts a long-running
interceptor, `redact` writes an arbitrary path, `rules --update` installs a
document that changes every figure the tool prints, and corpus submission sends
a user's data off their machine. Each is legitimate at a terminal. None is
something an agent should choose on a user's behalf.

**The principle:** an agent may read anything Replay can read, and may change
nothing without a flag the human typed when they configured the server.

## What leaves the machine

The original design had no section on this, and it is the most important one.

Replay's README says *"Everything runs on your machine. Nothing leaves."* **An
MCP tool result is uploaded by definition** — every byte returned over stdio
enters the next request to a hosted model. That is not a bug in the design, it
is the design, and it has to be stated rather than assumed.

Three consequences:

**Paths are content.** `ToolLabel` keeps 400 runes of tool arguments, which for
`Read`, `Edit` and `Bash` is the filesystem. Its own comment scopes it to the
transcript tier, *"where the user's own content may be shown to them"* —
showing the same string to a hosted model is a different act by the same code.
`replay context` already collapses to bare tool names via `toolNameOf`. MCP
results keep that coarsening.

**Transcripts contain secrets.** Tool output is recorded verbatim: the `cat
.env` from three weeks ago is in there. The ledger has a masking promise and
`redact` exists; the transcript tier has neither. Tools reading transcripts
return structure and counts, never content.

**Ranking by size is attacker-steerable.** An adversary who gets text into a
transcript — a fetched issue, a dependency README — controls the one variable a
cost ranking sorts on. A 40 KB hostile payload is rank one. So ranked results
carry names and byte counts, not excerpts, and the provenance block carries a
content-trust field rather than only a staleness one.

**No tool returns a proposed edit.** The original draft's memory feature
returned a diff for `CLAUDE.md`. Replay does not need write access for that to
be dangerous: the agent already holds Edit, so a diff arrives wrapped in a
measurement tool's authority and lands in the file that shapes every future
session. That is a confused deputy, and the trust boundary the design defends —
what Replay's process can write — is the wrong one. Reports are tables. If a
person wants a diff, they run a CLI command and read it.

## Updates: a decision the user makes out loud

The antivirus model is the right frame and its limits are the point. AV vendors
push definitions silently because a stale signature misses a real attack.
Replay's threat model is the opposite: **a wrong price table silently changes
every dollar figure the tool prints, which is worse than a stale one that
announces its age** — and it already announces it, locally, with no network.

So updates are pull-on-request, never push, and reported rather than applied.

Checking is a network call, and Replay's promise is that it makes none except
to the provider you configured. That promise is not broken by default and not
broken by an agent's decision. It is a choice the user records in
`$XDG_CONFIG_HOME/replay/update-consent.toml`, read by `internal/consent`.

**Three states, not a boolean:**

| State | Meaning |
|---|---|
| `Unset` | Nobody has decided. Ask; do not check. |
| `Granted` | Checking is permitted. |
| `Declined` | Checking is forbidden, and asking again is rude. |

`Unset` and `Declined` both mean "do not check now", so a boolean merges them —
and a tool that merges them either nags someone who already said no, or never
asks someone who was never asked. Only `Granted` opens the gate, and a test
fails if a second state ever does.

The consent file is refused if it is a symlink, or writable by group or other:
a grant any process on the box could have written is not this user's decision.
The parser recognises exactly two sentences and refuses everything else,
including an affirmative with an unrecognised line beside it — which is the
realistic shape of an attack on a file like this.

MCP is the right place to surface the question, because `elicitation/create`
lets the server ask the human directly and the agent can act on the answer.
What it reports is a version and a digest. What it never does is download.

## This server must not break the prompt cache

Tool definitions live in the system prompt. Every tool this server adds,
renames or reorders changes that prefix and invalidates the cache for the whole
session.

A cache-forensics tool that costs a prefix break every time it reconnects is
self-refuting. So: the tool list is stable and deterministically ordered, tool
names and descriptions are treated as prefix content and change only in a
release, and `replay_tool_cost` measures this server alongside every other.

## Observability

The failure mode to design against is not the server falling over. It is the
server answering confidently from an index built before the thing being asked
about.

**Every response carries its provenance, in the response and not a log** —
index state and age, session coverage, the truth tier per row rather than one
flattened tier for the result, and whether any content in it is derived from
untrusted transcript text.

**Staleness has a refusal threshold, not just a label.** The original design
attached an age and answered anyway, justified by the observation that a model
will quote a number without its caveat. That justification does not survive its
own premise: if caveats get dropped, attaching one is not a mitigation. Past a
threshold the tool returns an error telling the agent to refresh.

Every other surface in Replay refuses rather than guessing — the engine below
95% per-turn match, `learn` on bad calibration, `advise --guards` below ten
sessions. This is the one that was going to be the exception.

## What was cut, and why

| Idea | Verdict |
|---|---|
| Memory audit, `memory_unused` | **Cut.** No outcome variable exists. A `CLAUDE.md` line that works produces *no event* — its signature is an absence — so a naive detector flags the highest-value entries first. `predictive-design.md` already says it: Replay can measure waste, it cannot measure success. |
| Memory pricing, cached vs re-billed | **Deferred, and cheaper than it looks.** `RawBlock` already decodes `cache_control`; the domain `Block` drops it. Carrying the marker's block index through is a small change, and it is the single capture that would unlock the whole idea. |
| `would_break(action)` boolean | **Cut.** Replaced by `replay_order_plan`, which returns a number and a tier. |
| Agent-initiated corpus submission | **Cut.** An agent cannot give informed consent on a human's behalf, and an agent that has read a hostile page can be made to call it. Stays behind a human confirmation in a terminal. |
| Autonomous self-tuning loop | **Cut.** Its only observable is spend, so its gradient points at "do less work". `serve --trial-share` already does the controlled version, with randomisation, a guardrail and automatic revert. |
| `what_changed(since)` | **Kept, but note** it is `replay cost --compare` with an agent as the reader. |

The pattern worth naming: **everything safe was already a CLI command, and the
two genuinely new surfaces were the two most dangerous.** That is the strongest
argument against MCP as a frame for most of this value, and the reason this
document is shorter than the one it replaces.

## What this does not solve

Aggregation across a team. Everything here is one machine, one user, one file
on disk. "What is my org spending, by team, by repo" is a different product
with identity and storage problems that stdio and a local index deliberately do
not address.

Coverage is also narrower than it looks: `replay doctor` counts 86 top-level
sessions while the corpus holds 1,422 `.jsonl` files — 1,336 are subagent
transcripts it does not walk, and `advise` does. Any health figure here would
inherit that inconsistency until it is fixed.

---

[Architecture](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
