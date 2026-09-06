# 15. Single-tenant state is a boundary, not an implementation detail

**Status:** Accepted
**Date:** 2026-09-06

## Context

A centralised, team-facing Replay is a recurring proposal: point every
developer's `ANTHROPIC_BASE_URL` at one shared proxy, scrape each machine's
`/replay/metrics` into a time-series database, push spend caps down from a
central policy URL, and render the result as a dashboard. The usual framing is
that the engine is finished and what remains is commercial — authentication,
billing, charts.

That framing is wrong in a specific and checkable way, and it is worth writing
down before anyone builds against it. The engine is genuinely portable: the
pricing tables, the cache simulator and the analysis core are pure functions
over plain data with I/O at the edges, and they would lift into a server
unchanged. What does not port is the **state**, because every piece of shared
mutable state in the proxy is scoped to one human, and none of it carries a
tenant dimension.

Measured against the tree on 2026-09-06:

- **The spend guard has two scopes, session and day.** `SpendLimits` is
  `{SessionTokens, DayTokens, SessionUSD, DayUSD}` and `(*stats).costs()`
  returns one process-wide day figure. Behind a shared proxy, `--max-day-usd`
  is therefore an organisation-wide cap: one developer's runaway loop consumes
  the shared budget, and at the ceiling the proxy refuses requests for
  everyone, with nothing in the refusal identifying who spent it. The guard
  that makes the product worth buying becomes an org-wide outage.
- **The session table evicts.** `maxSessions = 256`, least-recently-seen. This
  is the same map whose eviction previously made four counters under-report and
  run backwards — 2,688,000 prompt tokens reported against 8,064,000 observed —
  a defect found and fixed by moving the counters off the walk. Fifty
  developers times parallel sub-agent lanes re-creates the conditions
  continuously rather than occasionally.
- **The metrics listener refuses to be networked.** `listenMetrics` rejects any
  non-loopback bind with "these counters name repositories and token spend, and
  Replay will not publish them to a network", and `readOnlyMux` carries no
  authentication because it never needed any. A central scraper polling each
  engineer's endpoint is the deployment this code declines to permit. The
  refusal is correct: the counters carry repository names.
- **Policies are pinned per session and persisted** (PX-8). A central policy
  file pulled on an interval cannot change a running session, by design — the
  pin exists precisely so a rewritten file never does. A remote configuration
  loop is not a small addition to this; it is the inverse of a deliberate
  guarantee, and it would also make one URL able to alter the spend caps and
  model selection of every developer machine in the organisation.
- **Credentials are forwarded and never persisted, and the ledger proves it.**
  A refusal record is asserted to contain no `sk-ant`, no `Bearer` prefix,
  no `x-api-key` and no home-directory path. On a local proxy this is a strong
  position and costs nothing, because the process already runs on the machine
  that holds the key and grants no access that did not already exist.
  Centralising inverts it: every key in the organisation would transit one
  shared process, which becomes the highest-value target in the building. The
  masking layer does not close this, and does not yet cover the OpenAI path at
  all (`replay_unmasked_requests_total` counts the gap).

There is also an evidence gap that no amount of engineering closes. Across
roughly 100,000 requests in the measured corpus, **every observed model id is
first-party**: 87,264 `claude-opus-5`, 8,895 `claude-opus-4-8`, 1,497
`claude-haiku-4-5`, and no Bedrock inference profile or Vertex publisher path
anywhere. Enterprises overwhelmingly reach Claude through Bedrock or Vertex,
whose caches are a different API with different economics. An enterprise proxy
built today would be pricing traffic this project has never once observed.

## Decision

Single-tenant state is treated as an architectural boundary. Any multi-tenant
Replay must introduce a tenant dimension into the spend guard, the session
table, the metrics surface and the credential path **before** any commercial
layer is built on top of them, and must not present a shared deployment as a
configuration of the local one.

Until that dimension exists, the project does not ship, document or market a
centralised mode, and proposals that assume one are answered with this record.

## Consequences

The team product is a real engineering project rather than a wrapper, and
saying so early is cheaper than discovering it during a security review. In
exchange, the sequence is unambiguous: the tenant dimension first, the
commercial layer second, and the four items usually listed first — collector,
identity, remote policy, dashboard — are all downstream of it.

Three specific things become harder, and are accepted:

- A shared spend cap needs per-tenant accounting before it is safe to enable,
  because the failure mode of the current one is a denial of service against
  the whole engineering organisation.
- Any networked metrics surface needs authentication and a decision about
  repository names in counters. Loosening `listenMetrics` without both is a
  regression, and the error string is the specification.
- A remote policy channel needs a threat model of its own. It is a control
  plane over every developer's traffic, and the per-session pin must survive
  it, not be removed for it.

Easier: the parts that do port are now named, so work can start on them without
waiting. The analysis core, the pricing tables and the simulator have no
tenancy assumptions to unwind.

We accept the risk that a competitor ships a centralised dashboard sooner by
ignoring these constraints. The position is that a proxy which aggregates an
organisation's API keys and can cut off its engineers is infrastructure, and
infrastructure that fails this way once is not bought twice.

## Alternatives considered

**Ship the dashboard first and retrofit tenancy.** Fastest to a demo, and the
demo is the easy part. It loses because the retrofit lands in exactly the
shared mutable state that has already produced two defects of this class — the
evicting counter map and the process-wide policy revert flag that disarmed
every policy after it. Retrofitting a tenant key into state that a paying
customer is already reading is worse than adding it first.

**Keep the proxy local and centralise only the reporting.** Attractive, and
closest to the current architecture: no shared credential path, no shared cap,
no remote control plane. It loses on the metrics surface as it stands, because
the counters name repositories and the endpoint has no authentication — so
this is not "only reporting", it is a new data-egress decision that ADR-0010
and the consent gate in `replay probe --contribute` already have opinions
about. It remains the most promising direction and is deferred rather than
rejected.

**Declare the enterprise path out of scope permanently.** Consistent with
ADR-0001's refusal of scope creep, and it loses because the demand is real and
the guardrails genuinely are the differentiated part. The objection here is to
sequence and to a false claim of readiness, not to the destination.
