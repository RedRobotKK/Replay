# The MCP server: what it exposes, to whom, and what happens when it is busy

**2026-09-05. Design, not yet built.** Numbers here were measured on the
maintainer's machine on that date and are the reason the design looks as it
does.

## Why this exists

Replay's users are increasingly not people at a prompt. Someone runs Claude
Code, the bill surprises them, and the useful question — *which turn did that,
and why* — is one the agent should be able to ask directly rather than the human
remembering a CLI exists and switching windows to use it.

The CLI stays the product. This is a second front door onto the same engine.

## The measurement that shapes everything

```text
full corpus, 86 sessions, 1.3 GB     17.4 s
one project directory, 11 sessions   13.3 s
one session file, 336 MB              4.3 s
```

**No tool call can do a cold read.** An MCP client will time out, and an agent
that blocks four seconds on the cheapest possible question is one nobody keeps
installed. Every design decision below follows from this, and any future change
that makes a tool read transcripts synchronously undoes the whole thing.

So: **the server answers from an index, and building the index is a job.**

## Transport: stdio, and not HTTP

The client spawns `replay mcp` and speaks over stdin and stdout. It is not a
daemon, it opens no port, and it is not always on.

An HTTP MCP server would be a listening socket on a tool whose entire argument
is that it runs on your machine and talks to nobody. It would also need auth,
because a port on localhost is reachable by every process on the box including a
browser tab. stdio inherits the client's identity and lifetime and needs none of
that.

The cost of stdio is that each client gets its own process, and three clients
would build three indexes. That is why the index lives on disk rather than in
memory: **the shared state is a file, not a server.**

## Services

Five groups. The split is not by convenience — it is by what each group can do
to the machine.

### 1. Measure — read-only, always available

| Tool | Answers |
|---|---|
| `replay_cost` | cost per task, and the share that was billed twice |
| `replay_blame` | what is eating prompt tokens, ranked |
| `replay_diff` | which turn broke the cache, and the cause |
| `replay_context` | what entered a session's context, by tool |
| `replay_route` | what switching model would change, structurally |
| `replay_trim` | what a byte cap on tool output would have saved |

These read the index. None writes, none reaches the network, and none can be
made to. They are the default and the only group enabled without a flag.

### 2. Advise — reads by default, writes only when the human said so

| Tool | Effect |
|---|---|
| `replay_advise` | suggestions with predicted savings. Read-only |
| `replay_advise_apply` | **writes `~/.claude/settings.json`** |
| `replay_learn` | **writes `~/.replay/policy.json`** |

`replay_learn` deserves a sentence of its own. `policy.json` is read by
`replay serve` at each new session's first request, so a tool that looks like
analysis silently changes what the proxy sends to the provider. An agent must
not be able to reach it by default.

Both write tools require `replay mcp --allow-write` and both return the diff
they would apply before applying it.

### 3. Corpus — reads locally, never sends

| Tool | Effect |
|---|---|
| `replay_corpus_report` | the calibration report, as text. Local |

Submission is deliberately absent. Sending a user's data off their machine on an
agent's initiative is not a thing an agent should be able to do, however good the
payload is. If submission is ever built it belongs behind a human confirmation in
a terminal, not behind a tool call an agent can decide to make.

### 4. Live — only when the proxy is already running

| Tool | Answers |
|---|---|
| `replay_proxy_status` | spend, caps, breaks so far this session |
| `replay_proxy_metrics` | the Prometheus series, as structured data |

These read `/replay/status` and `/replay/metrics` from a running `replay serve`.
If nothing is listening they say so; they never start one. **An agent must not be
able to stand up a process that intercepts the user's provider traffic.** That is
a decision a person makes at a terminal, with the flags in front of them.

### 5. Health — how to tell whether to believe any of the above

| Tool | Answers |
|---|---|
| `replay_health` | index age, coverage, what is stale, what is running |

Described in full below, because it is the part that makes the rest honest.

## Access: what is on by default, and what is not

| Group | Default | Requires | Never |
|---|---|---|---|
| Measure | **on** | — | — |
| Corpus report | **on** | — | — |
| Live status | **on**, read-only | a running proxy | starting one |
| Advise (read) | **on** | — | — |
| Advise apply, learn | **off** | `--allow-write` | silent application |
| Corpus submit | — | — | **not exposed** |
| `serve`, `redact`, `rules --update` | — | — | **not exposed** |

The last row is the one to defend. `serve` starts a long-running interceptor.
`redact` writes an arbitrary output path. `rules --update` installs a document
that changes every figure the tool prints. Each is legitimate at a terminal and
none is something an agent should choose to do on a user's behalf.

**The principle:** an agent may read anything Replay can read, and may change
nothing without a flag the human typed when they configured the server.

## Always on? No. Busy? Say so, and answer anyway

The server is spawned per client and exits with it. What persists is
`~/.replay/index`, and that is where "busy" lives.

**Three states, and every tool reports which one it answered from:**

- `fresh` — the index covers every transcript, and no file has changed since it
  was built. Answers are current.
- `stale` — the index is usable but N sessions have changed or appeared since.
  **Answer from it anyway, and say what is missing.** A number from ten minutes
  ago with its age attached is worth far more than a seventeen-second wait, and
  an agent can decide whether to refresh.
- `building` — a refresh is running. Return the previous index with its age, plus
  progress. Never block, never return nothing.

**Refresh is a job, not a call.** `replay_refresh_index` starts it and returns a
handle immediately. `replay_health` reports progress. Two clients asking at once
share one build: the second gets the same handle rather than a second scan of
1.3 GB.

**Incremental by mtime.** A new session costs its own parse, not the corpus's.
This is what makes `stale` a short state instead of the normal one.

**Scope narrows the work.** `replay_diff` on one session is 4.3 seconds cold and
should still be a job, but a warm index answers it immediately. Tools take an
optional session or project scope precisely so an agent can ask a narrow question
and get a fast answer.

## Observability

The failure mode to design against is not the server falling over. It is the
server answering confidently from an index built before the thing the user is
asking about.

**Every response carries its provenance.** Not in a log — in the response:

```json
{
  "result": { "medianTaskUsd": 0.65, "avoidableShare": 0.05 },
  "provenance": {
    "indexState": "stale",
    "indexBuiltAt": "2026-09-05T18:22:04Z",
    "indexAgeSeconds": 4127,
    "sessionsIndexed": 1395,
    "sessionsChangedSince": 12,
    "truthTier": "estimated",
    "priceTable": "2026-06-24",
    "rules": "anthropic-2026-09-01"
  }
}
```

This is the truth-tier discipline the CLI already has, applied to a surface where
the reader is a model. A model will quote a number without its caveat unless the
caveat is attached to the number, so it is attached to the number.

**`replay_health` answers the questions a human debugging this would ask:**

| Field | Why |
|---|---|
| `indexState`, `builtAt`, `ageSeconds` | is the answer current |
| `sessionsIndexed` / `sessionsOnDisk` | is the index complete |
| `sessionsChangedSince` | what would a refresh add |
| `refreshInProgress`, `refreshProgress` | is it working on it |
| `lastRefreshDurationMs`, `lastRefreshError` | did the last one fail, and how |
| `proxyRunning`, `proxyAddr` | are live tools available |
| `writeEnabled` | can anything here change my machine |
| `priceTableAgeDays` | are the dollar figures still trustworthy |
| `toolCalls` by name, and p50/p99 | which tools are used and which are slow |

**Logs go to stderr, never stdout.** stdout is the protocol; a stray print
corrupts the stream. Structured lines, no message text ever, same promise the
ledger keeps.

**Counters are local and readable.** `~/.replay/mcp-stats.json`, readable by
`replay doctor`. Nothing is transmitted; the point is that a user can see what
their agent has been asking, which is a property worth having on a tool that
sits this close to their work.

## What this does not solve

**An index is a cache, and caches go wrong.** The mtime check catches edits and
misses a file rewritten with a preserved timestamp. That is rare and the
consequence is a stale figure with `stale` next to it, which is the failure this
design is willing to take.

**A well-known MCP server card still does not apply.** That advertises a hosted
server at a URL. This one is spawned on a laptop and reachable by nobody. If the
agent-readiness scan is the reason to build something, this is not it.

**The 17-second cold build has to happen once.** First run is slow, and the
honest thing is to say so during install rather than let the first tool call be
the surprise.

---

[Architecture](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
