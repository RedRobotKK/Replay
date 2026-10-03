# PRD falsification: 24 lenses against the Free / Pro / Enterprise map

**2026-10-03. A product review, not a feature session. Every claim about what Replay
does today is tied to a measured result in this repository; every claim about what
the market does is a lens reasoning from a published position, and is labelled as
such. Nothing here authorizes an experiment or a build.**

The question was fixed by the brief: not "what else could Replay do" but "what will
a serious 2026 developer, platform team, SRE, security team, FinOps team or buyer
reasonably expect, and which of the proposed features should die." Two areas were
pressure-tested on request: agent identity plus delegated authority plus context
engineering plus execution fingerprinting, and simulation before a policy change.

Labels. **MEASURED** means a figure this repository produced under a registered
oracle. **OBSERVED** means read from source or from a run. **LENS** means a panel
member reasoning from the published position of the organisation or role named;
these are arguments, not quotations, and no named person said any of it.
**UNKNOWN** stays UNKNOWN.

---

## 0. What Replay is today, so the review attacks the real thing

From the Release 1.0 certification (`9694f1e`), the qualification pass and the
production-grade gate:

- **Observe.** Reads transcripts and ledgers on disk; prices usage from a dated
  table; attributes cache behaviour per turn; names the turn a cache broke on and
  why. 33 dispatched surfaces, each PRODUCTION-GRADE through the built binary.
- **Control, in the request path only.** `replay serve` caps spend per session and
  per day (100% of over-cap requests refused, 0.33 ms p50, latch, one-shot
  override), stops a looping agent, opens a breaker on provider failure, refuses a
  prefix re-lay above a ceiling, masks secrets. Forward or refuse: there is no
  second allocation. The subscription quota question is a null result (3.09M tokens
  moved the counter by zero steps).
- **Context, as measurement.** `replay context` says what entered the context and
  by whom, estimated versus measured; `trim` scores a tool-output cap over history;
  `prefix` says whether a change voids the cached prefix; `--freeze-prefix` labels a
  tool-set epoch from the exact tools JSON; `replay replay` scores alternative
  layouts against the as-run session.
- **Continuity.** Closed negative three times: a fresh agent resumes from the
  repository alone; commit bodies add nothing; stale state does not fossilise.
  Durable scratch has no interface; the state-ledger prototype has no carrier, by
  decision.
- **Prove.** A claims register with oracles, 116 frozen mutants re-run in CI, a
  production surface gate, black-box binaries, evidence documents with their own
  guards. This is the part no competitor in the kill test has.
- **Identity.** The proxy reads `x-claude-code-session-id` and `x-claude-code-agent-id`
  as client-supplied headers (observed, unverifiable); a distinct-account claim is
  structurally unavailable because no provider issues an account-scoped credential;
  single-tenant state is an architectural boundary (ADR-0015, SP-5/6/8 specified and
  not built).
- **Outcome.** No corpus examined carries a task-outcome signal. Every behavioural
  hypothesis in the research campaign died on that absence.

Those last two facts decide most of what follows.

---

## 1. Pass 1 to 2: the 2026 inventory, with the test applied

The inventory is in the brief. Applying the ten questions to each family yields
four buckets, and the bucket is the finding; the per-item table is section 25.

**Replay must own** (it is the correlation point and nobody else stands there):
request admission with its decision and reason; economic state (spend, caps,
reservation if ever built) keyed to Work and identity; the ledger of what happened
on the wire with provenance bits; context accounting (what entered, why, what it
cost, what was dropped); execution fingerprint; policy dry-run over its own
history; evidence export with reproducibility.

**Replay must integrate with, never own**: identity issuance (OIDC/SPIFFE/IAM),
secrets management, SIEM, OTel backends, durable execution engines, model gateways,
vector or memory stores, document storage, workflow engines, ticketing.

**Replay must measure but may not claim to judge**: outcome. It can carry an
outcome signal supplied by the operator (tests passed, PR merged, ticket closed,
human mark) and compute cost per declared outcome; it cannot judge arbitrary AI
quality, and the repository already proved that no transcript carries the signal.

**Replay must not build**: durable scratch as a product, a memory layer, titration
as a control (no second allocation exists), generic realtime, rollback of agent or
external state, a dashboard platform, a graph platform.

---

## 2. Pass 3: agent identity

**Who is the agent?** Today: whoever put a string in a header. That is observed
and unverifiable, and the register already bounds it (RPL-C019, NO ENDPOINT).

The lenses split on one axis. The AI security/identity architect and the CISO
(LENS: Microsoft's agent-identity guidance treats agents as first-class principals
with scoped permissions and auditable activity) say an enforcement point that cannot
bind a request to a principal is not an enforcement point. The IAM competitor and
the durable-execution veteran say the same thing from the other side: Replay must
not mint identities, because an identity it mints is one nobody else trusts.

**Convergence.** Replay should be an identity *consumer and binder*, not an issuer:

```text
Human principal        from the IdP (OIDC subject), presented to the proxy
  ↓ delegates to
Agent identity         workload identity (SPIFFE SVID or an IdP-issued agent token)
  ↓ runs
Work                   Replay's own id, minted locally, carried on every request
  ↓ issues
Request                the ledger record, with the three ids above as fields
  ↓ calls
Tool                   name + definition hash (already in the tool-set epoch)
  ↓ reaches
Provider               endpoint + model + rules version (already recorded)
```

Every field is **OBSERVED** (presented on the wire) or **ABSENT**, never inferred.
A request with no presented identity is recorded as `identity: absent`, which is a
third value and not an anonymous principal. The chain becomes part of the
provenance model as fields on the ledger record; it does not become an IAM.

**What this buys, measurably.** Per-identity caps (SP-5/6/8 become real), a
refusal that names the principal, and an audit row that an enterprise can join to
its own directory. **What it cannot buy.** Proof that the header was not forged by
a process on the same host; that is the tenancy boundary ADR-0015 already names,
and it is closed only by a token the proxy validates (`--token` exists for the
proxy's own access; the per-identity token is the Enterprise addition).

**Delegated authority.** Record it; do not adjudicate it. Agent A delegating to
Agent B is a parent Work id on B's Work plus the budget and policy B inherited,
written at delegation time. Whether A *may* delegate is the IdP's question.

Free: record presented identity fields and the Work id. Pro: per-identity caps and
reports on the developer's own machine. Enterprise: token-validated identity,
per-tenant state, delegation records, directory join.

---

## 3. Pass 4: authorization versus economic admission

Today's admission answers "can this request afford to run." The security lenses
want "is this agent authorized to perform this action with this tool under this
authority." The AI-gateway competitor (LENS: gateways already do key-scoped model
allow-lists) and the enterprise platform buyer both say: do not build a policy
engine; evaluate one.

**Decision.** Replay keeps *one* admission point with *two* question families and
an external evaluator for the second:

- economic admission: Replay's own, as shipped;
- action authorization: a policy decision point Replay calls with a fixed input
  document (identity chain, Work, tool name and definition hash, model, provider,
  spend state) and obeys. The evaluator is OPA/Cedar/the customer's PDP; a bundled
  local evaluator for Pro is acceptable only if its language is one of those, not a
  new one.

Replay's contribution is the **input document** and the **decision record**: what
was asked, under which policy version, what was answered, and that the request did
or did not go out. That record is what an auditor wants and what no PDP produces on
its own.

Free: observe (record the input document, no enforcement). Pro: developer-local
policy file, evaluated by the bundled evaluator, with dry-run. Enterprise: external
PDP, signed policy, per-tenant.

---

## 4. Pass 5: runtime defense

The runtime-defense competitor's attack: admission is a gate at the door; the
damage happens inside. Prompt injection and tool poisoning arrive in *responses*
and *tool results*, which Replay forwards unchanged by design (the proxy never
rewrites wire bytes, and the production-grade gate just re-proved it).

**What Replay actually observes**: every request body summary (tool calls, their
hashes, repeats), every response's usage, tool-set changes between requests,
spend rate, request rate, secrets in traffic (the masker), provider failures.

**What it can reliably block today**: identical-call loops (measured, 1,110/1,110),
spend above a cap, prefix re-lays above a ceiling, requests while a provider is
failing, secrets leaving the machine (masking, with documented gaps).

**What it cannot do and must not claim**: detect prompt injection in content it
does not read as content; judge tool-result maliciousness; attribute intent.

**Convergence.** Replay owns *behavioural anomaly on the signals it already has*:
spend velocity against the session's own history, request-rate bursts, tool-set
drift inside a session, a new tool appearing mid-session, repeated failures, a
sudden model change. Each is a guard with a measurable false-refusal rate on
history (the dry-run in Pass 9 is how it is tuned). Content inspection belongs to a
dedicated security product, and Replay should hand it the decision record and the
request summary over the same PDP interface as authorization. Enterprise gets
quarantine: freeze a Work (refuse all its requests) on a signal from the PDP or an
operator, which is the one runtime action the proxy can take with certainty.

---

## 5. Pass 6: context engineering

The context-engineering specialist's claim is the strongest "missing" finding and
it is only half missing. Replay already measures what entered the context, by
source, with the estimated/measured split; whether anything was cleared or
compacted; the cost of each source across every prompt that carried it; and the
layouts that would have changed the bill. What it does not do:

- say **why** a block entered (no policy or selection record exists in the
  transcript; UNKNOWN by construction, unless the runtime tells Replay through a
  header or a marker);
- say what was **dropped or compressed**, beyond "nothing was compacted" (Claude
  Code's compaction leaves no diff; a compaction event is observable, its content
  delta is not);
- tag **staleness** (no timestamp on a block's origin, no freshness signal);
- attribute **success** to context (no outcome signal).

The Redis-style framing (LENS: live state, memory, records as the production
context problem) pulls toward owning a context store. The killers are unanimous
against it: a context store is a memory product, and the repository's own
evidence says continuity from the tree is already at ceiling.

**Convergence.** Context engineering in Replay is *accounting and comparison*, not
assembly: a per-request context manifest (source, bytes, measured or estimated
tokens, first-seen request, carried count, cost integral, dropped-at request if a
compaction is observed) and the ability to compare two strategies over history.
That is an extension of what `context`, `blame` and `replay` already compute, with
one new primitive, the manifest, and one new observable, the compaction event.
Free: the manifest. Pro: strategy comparison and the trim/compaction simulators.
Enterprise: a context *policy* (which sources may enter, by classification) is a
data-governance question and belongs with Pass 15, not here.

---

## 6. Pass 7: artifacts

Unanimous. Artifacts stay external references: type, URI or path, content hash,
producing request id, size. Replay records that a commit, file, deployment or
dataset was produced and by which request; it never stores the bytes. The Work
model gains `Artifacts []Ref`, nothing more. The document-store exclusion in Pass 23
stands.

---

## 7. Pass 8: reproducibility (`replay investigate`)

Every builder lens ranks this above any dashboard, and the killers do not object,
because it is the thing the assembled stack cannot do: Temporal replays a workflow's
history, OTel shows a trace, neither reconstructs *the admission decision, the
budget state, the policy version, the context manifest and the fingerprint at the
moment of one request*.

Replay already reconstructs cache behaviour per turn from the record alone. The
feature is that reconstruction widened to every field the ledger holds, keyed by
event or request id, rendered as one document. It is deterministic because the
ledger is append-only and the rules are versioned. It is Pro, because it saves
debugging time, and Enterprise adds signing and retention.

**Boundary.** It reconstructs what Replay recorded. Model reasoning is UNKNOWN;
the content of a dropped block is UNKNOWN; anything not on the wire is UNKNOWN,
and the document says so in those words.

---

## 8. Pass 9: simulation before a policy change

This survived every attack, and the reason is structural: *Replay is already a
replay engine.* `replay replay` scores policies against the as-run session, `route`
prices a model switch over history, `trim` scores a cap, `ceiling` places a budget
halt, `learn` selects a policy on held-out sessions. The brief's example is the
existing machinery pointed at admission and authorization policy over the ledger.

**What survives.** `replay simulate --policy <file> <ledger|transcripts>`: for every
historical request, evaluate the proposed economic rule and, for Pro and
Enterprise, the proposed authorization rule, and report requests affected,
refusals that would have fired, estimated spend delta at list price, sessions that
would have been cut mid-work, and the policy version hash. Deterministic, offline,
and every number carries the population it was computed on, which is the house
rule.

**What the killers removed.** "Continuation-risk candidates" and "estimated
savings" as a single figure. The first needs an outcome signal and is UNMEASURED
until the operator supplies one; the second is list price on recorded usage and
must be labelled as such, never as "savings" (RPL-C013). The investor lens adds
the honest caveat: a simulator is only as valuable as the policies people actually
change, and the number that sells it is "requests that would have been refused,"
not "dollars saved."

Pro: per-developer, over their own corpus. Enterprise: organisation-wide, over the
fleet ledger, with signed policy versions.

---

## 9. Pass 10: testing agents

Strategically valuable and mostly already built inward-facing. Replay has
mutation testing, fixtures, deterministic reproduction and black-box binaries for
*itself*. Turning that outward means: a `replay serve --inject` mode (provider
failure, quota exhaustion, tool failure, latency) so an agent's behaviour under
each can be tested in CI; policy tests (`replay simulate` with assertions); and
replay tests (run a recorded session's requests through a policy and assert the
decisions). The DX lens wants this in Free for the injection flags and Pro for the
assertion harness; the agent-runtime competitor points out that runtimes own
*their* test harness and Replay should only own the control-behaviour tests, which
is exactly the boundary.

---

## 10. Pass 11: topology graph

Consume, never build. Replay knows which tools and providers a Work touched (names,
hashes, endpoints) and can emit that as edges. The system-of-record for "what can
this agent affect" is the platform's inventory or CMDB, and the graph product is a
category of its own. Enterprise gets an edge export; nothing gets a graph UI.

---

## 11. Pass 12 and 13: incident response and rollback

**Incident.** DETECT and EXPLAIN exist (guards, `investigate`). FREEZE is the one
new action Replay can take with certainty: refuse every further request of a Work
or identity until released. CHECKPOINT is Replay's own state only. ESCALATE is an
integration (ticket, pager, SIEM). REVIEW is `investigate`. RESUME is lifting the
freeze.

**Rollback.** The guarantee ends at Replay's own state. Replay can restore its
ledger view, its caps, its policy version to a checkpoint. It cannot roll back an
agent's semantic state (continuity research says the repository is that state) and
it cannot reverse an external side effect (a commit, a deployment, an email). The
product word is **freeze and resume**, never rollback; ABORT means "refuse from
here," not "undo."

---

## 12. Pass 14: versioning

Already partly solved and worth naming as foundational. The ledger schema is
versioned with an add-only rule and old readers preserve unknown fields (P3 decision,
`SchemaVersion` 2); rules and price tables are dated; the binary version and commit
are in the corpus payload. Missing: a policy version hash on every decision record,
a checkpoint schema version, and a stated compatibility promise (N reads N+k by
ignoring, never by reinterpreting). "Which policy version caused this action" is the
Enterprise audit question and it is one field away.

---

## 13. Pass 15: data governance

Replay already stores no message text; the ledger carries kinds, sizes, timings,
usage, tool names and labels, and the masker runs on traffic. That is the right
minimum and the brief's rule ("do not assume raw prompts") is already the design.
Missing for Enterprise: classification tags on sources (so a context policy can
refuse a source class), retention and deletion by Work or identity (`purge` exists
by window and session), residency (deployment, not code), customer-managed keys for
the vault, and a legal-hold flag that suspends purge. All are table stakes an
enterprise buyer asks for; none changes the primitive.

---

## 14. Pass 16: human and agent

The brief's states (CONTINUE, REDUCE, DEFER, ASK HUMAN, REFUSE, ABORT) map onto what
the proxy can actually do: CONTINUE forwards, REFUSE and ABORT refuse, DEFER holds
with `Retry-After` (the breaker already does this), ASK HUMAN is a refusal whose
reason carries an approval handle that `x-replay-override` already resolves once.
REDUCE needs a second allocation and does not exist. So the state machine is real
for five of six, and the approval flow is the override header given a record and
a timeout. Free: manual pause (freeze) and resume. Pro: interactive approval from
the TUI. Enterprise: policy-driven escalation through the PDP.

---

## 15. Pass 17: multi-agent delegation

Record the chain as fields: parent Work id, inherited budget, inherited policy
version, delegating identity. Enforce only what the proxy can see: a child Work's
spend counts against the parent's cap when the parent says so. Responsibility is a
governance assertion the record supports and Replay does not adjudicate.

---

## 16. Pass 18: outcome measurement

The repository's hardest-won fact: no transcript carries an outcome. The economics
lens wants cost per successful outcome; the FinOps competitor says every cost tool
claims it and none can measure it. Replay's honest version: an **outcome signal
input** (a header, a CLI mark, a CI webhook: `replay outcome <work> success|failure
--by tests|merge|human`) recorded on the Work, and every economic figure then
reported *per declared outcome* with the declaration's source. Rework, retries, tool
failures, human interventions and latency are all observable and should be
reported beside cost. Replay never infers outcome; a Work with no declaration is
"outcome: undeclared," a third value.

---

## 17. Pass 19: autonomy level

Useful as a *policy input*, dangerous as a Replay-owned ontology. Record the level
the operator declares for an identity or Work (L0 to L5) and let policy require
stronger identity, approvals and evidence at higher levels. Replay does not infer
a level from behaviour.

---

## 18. Pass 20: discoverability

Already Replay's strength and already disciplined: `doctor` and `agents` find
Claude Code, Codex, Grok, Ollama, and name Cursor, AnythingLLM, OpenClaw and Oracle
as detected but not read, with the maturity column in `PRODUCTION-WIRING.md`.
Keep the three words (observed, inferred, unknown) and extend discovery to MCP
servers configured on the machine (the `prefix` reader already parses the config).
Free.

---

## 19. Pass 21: policy as code

Yes, and only as files in Git: YAML or JSON with a schema, a version hash on every
decision, `replay simulate` as the test, CI validation (`replay policy check`),
signing for Enterprise. No API-first policy store; the file is the record.

---

## 20. Pass 22: supply chain and the execution fingerprint

The fingerprint already half-exists: tool-set epoch from the exact tools JSON,
`cc_version` hashes, rules version, price-table date, binary version and commit,
model and provider on every record. Make it one object on every request record:

```text
fingerprint
  agent        presented identity + declared version
  runtime      client version as observed (cc_version)
  model        requested and responded
  provider     endpoint
  tools        epoch hash + per-tool definition hashes
  policy       version hash
  replay       version + commit
  rules        version + price-table date
```

"What changed between these two executions" is a diff of two fingerprints, and it
is the supply-chain question answered without a registry. Free, because it costs
nothing and is the foundation of `investigate` and `simulate`. Unauthorized tool
registration becomes a policy rule over the fingerprint (a tool hash not on the
allow-list is refused), which is Enterprise.

---

## 21. Pass 23: never build

| Never build | Instead |
|---|---|
| vector database or memory layer | record references and context manifests; integrate with whatever store the runtime uses |
| workflow or durable-execution engine | emit Work and checkpoint events; integrate with Temporal-class engines by correlating their run id to Replay's Work id |
| message broker or WebSocket platform | the in-path proxy and a file ledger; export events to the customer's bus |
| SIEM | decision records and anomaly signals exported in OCSF or as OTel logs |
| IAM | consume OIDC and SPIFFE identities; validate tokens; never mint |
| secrets manager | mask on the wire; integrate with the vault the customer has |
| APM | OTel export of spans and metrics; no storage tier |
| agent framework | runtime-neutral by construction; no SDK that owns the loop |
| model gateway | one proxy in front of the gateway or the provider; no routing table of its own beyond the experiment commands |
| document store | artifact references with hashes |
| dashboard platform | a TUI and JSON; dashboards are the customer's |

---

## 22. Pass 24 and the kill test

**The assembled stack** (runtime + Temporal + LiteLLM + OTel + IAM + SIEM + FinOps +
MCP + observability) provides: durable execution, model routing, traces, identity,
alerting, chargeback by key, tool protocol. It does **not** provide, anywhere in
the chain, a single point that knows for one request: who presented, which Work,
what it would cost at list, whether it was admitted and why, what context it
carried and from where, what the exact tool set and policy version were, and that
the answer can be reconstructed offline and re-run under a proposed policy. Each
component has part of this; none correlates it, none enforces on the correlation,
and none proves its own figures with frozen defects.

So the thesis the brief offers is the right one, with two words changed:
*a runtime-neutral correlation, admission and proof layer in the request path.*
"Recovery" is dropped from the thesis because Replay's recovery is freeze and
resume of its own state; "economics" stays because the admission is economic
first; "authorization" stays as evaluation, not issuance.

**Vulnerability check.** If Replay were only a dashboard, memory, event stream,
budget or tracing, the stack wins. It is none of those. The two things the stack
cannot assemble are the decision record with its fingerprint, and the replay of
history under a new policy with evidence discipline. Those are what to build.

---

## 23. The 2030 test

Five most likely reasons Replay failed by 2030, and the redesign each forces:

1. **Providers moved the proxy inside.** Anthropic, OpenAI and the gateways ship
   budgets, loop detection and audit at the API key. Redesign: the ledger and
   `simulate` must work from provider-side usage exports and gateway logs as inputs,
   not only from Replay's own proxy; the proxy is one source, the correlation is the
   product.
2. **No outcome signal ever arrived**, so "cost per outcome" stayed a slogan and
   the economic story collapsed to "cheaper tokens." Redesign: make the outcome
   declaration a first-class, trivially cheap input from day one (a CI step, a
   header) and report per declared outcome from the first release that has it.
3. **Enterprise bought the identity story from the IdP vendor** and saw Replay as
   one more agent. Redesign: never compete on identity; be the thing the IdP's agent
   identity is *presented to*, with the decision record the IdP cannot produce.
4. **Free adoption died under paywalled measurement.** Redesign: everything that
   measures is Free, forever; only comparison, simulation, enforcement at
   organisation scale and proof export are paid.
5. **The category was never named narrowly enough to be owned.** "Agent
   infrastructure" has a hundred vendors. Redesign: the narrow category is
   *admission decision records for AI work*, with the replay-under-policy as the
   killer feature; measure the competitor count in that category before the next
   naming decision (the category-killer rule).

---

## 24. Required outputs 1 to 12

**1. Missing capabilities.** Identity binding and chain (consume, bind, record);
action authorization via an external PDP with a fixed input document; decision
records; execution fingerprint as one object; context manifest with compaction
events; `investigate` (deterministic reconstruction); `simulate` (policy dry-run
over history); outcome declaration input and per-outcome reporting; freeze and
resume as a Work state; policy as files with version hashes and `policy check`;
behavioural anomaly guards on existing signals; failure injection for agent CI;
artifact references; delegation fields; retention, deletion, legal hold,
classification tags; MCP server discovery.

**2. Killed capabilities.** "Quota titration" (no second allocation; null result;
rename to spend caps and reservation if reservation is ever built); "generic
realtime" and "advanced realtime" (in-path interception is the product; keep the
words); "durable scratch" and "basic continuity" as features (no interface, closed
negative; keep checkpoint of Replay state only); "memory"; "advanced dashboards"
as a tier item (the TUI and JSON are the surface; dashboards are the customer's);
"recovery automation" beyond freeze and resume; "multi-region" as a Replay feature
(it is a deployment property of a file ledger); "agent/MCP governance" as a
separate line (it is the fingerprint plus policy); any rollback wording.

**3. Free changes.** Everything that measures: identity fields on records, the
fingerprint, the context manifest, decision records in observe mode, discovery
including MCP servers, OTel export, failure-injection flags on `serve`, freeze and
resume by hand, the outcome declaration input. The current map paywalls context
engineering and continuity optimisation; the measurement half of context must be
Free or nobody will install it.

**4. Pro changes.** `simulate` over the developer's own corpus; `investigate`;
strategy comparison (context, trim, compaction, route) with populations stated;
local policy files with the bundled evaluator and dry-run; interactive approvals
from the TUI; per-identity caps on one machine; the agent control-behaviour test
harness; multi-agent analysis as delegation-aware reports. Remove "quota
titration," "advanced realtime," "advanced dashboards," "recovery automation."

**5. Enterprise changes.** Token-validated identity and per-tenant state (SP-5/6/8);
external PDP, signed policy, organisation-wide `simulate`; quarantine on a PDP or
operator signal; decision-record retention, deletion, legal hold, CMK for the
vault; signed `investigate` documents and evidence export; edge export for the
customer's inventory; chargeback by identity and Work with the population stated.
Keep RBAC, SSO/SCIM only as the configuration surface of Replay itself, never as
a product line. Keep private deployment; drop "multi-region" as a feature.

**6. External integrations.** OIDC/SPIFFE for identity; OPA/Cedar/customer PDP for
authorization; Temporal-class engines by run-id correlation; LiteLLM and gateways
by sitting in front of or behind them and by ingesting their logs; OTel for spans,
metrics and logs; OCSF or OTel logs to the SIEM; FinOps tooling by exporting per-
identity, per-Work cost with population; MCP by discovery and tool hashes; CI by
the outcome webhook and `policy check`.

**7. New core primitives.** `Identity` (presented fields, never inferred);
`Decision` (input document, policy version, answer, reason, forwarded or not);
`Fingerprint`; `ContextManifest`; `ArtifactRef`; `Outcome` (declared, with source);
`Freeze` as a Work state; `PolicyVersion`. `Attempt` and `Quota` from the proposed
list survive only as the existing retry record and the existing spend state;
`Checkpoint` is Replay-state only.

**8. New invariants.** Absent identity is a third value, never anonymous; a
decision record exists for every request the proxy saw, forwarded or refused; a
fingerprint is immutable once written and diffs are computed, never stored; a
simulation reports the population and never a "saving"; an outcome is declared or
undeclared, never inferred; freeze is monotone until released; policy evaluation
is deterministic for a given input document and version; nothing on the wire is
rewritten; retention and legal hold are mutually exclusive on the same Work.

**9. New failure modes.** Identity presented but token invalid (refuse, record);
PDP unreachable (fail closed for Enterprise, fail open with a record for Pro, and
the choice is a policy field); fingerprint drift mid-session (a tool hash changes:
warn, then refuse under policy); delegation cycle; parent cap exhausted by a child;
simulation over a corpus whose rules version differs from the policy's (report, do
not reconcile); outcome declared twice (last wins, both recorded); freeze while a
request is in flight (the response is delivered, the next request refused, exactly
the spend-cap rule); legal hold during purge (purge refuses and says why).

**10. New security boundaries.** The identity token the proxy validates (a new
secret at rest, owner-only like the vault key); the PDP connection (a new outbound
destination, which RPL-C022's enumerated set must gain); the policy file (signed in
Enterprise, because it now decides actions); the decision-record store (contains
identities and tool names, so it inherits the ledger's disclosure and purge rules);
the quarantine signal (an inbound control, authenticated); failure injection
(must be impossible to enable on a production listener without an explicit flag
that `doctor` reports).

**11. New customer outcomes, measurable.** Refusals that name a principal (count
per identity per day); policy changes tested before deployment (requests affected,
refusals that would have fired, population); incidents reconstructed without
reading prompts (time to the decision record, in minutes); cost per declared
outcome with the declaration rate stated; tool-set drift caught before a request
leaves (count); supply-chain diffs between two executions (fields changed); freeze
to resume time.

**12. Competitive red team, per capability.** Durable execution: Temporal,
Restate, Microsoft's durable task framework. Model routing and key-scoped
budgets: LiteLLM, Portkey, Kong AI gateway, the providers' own consoles. Traces
and metrics: OTel plus any backend. Identity: Okta, Entra, SPIFFE. Runtime
content defense: Lakera-class and the cloud vendors' agent security products. SIEM:
Splunk, Sentinel, Chronicle. FinOps: Vantage, Finout, the cloud cost tools.
Memory: the runtime vendors and the vector stores. None of them: the decision
record with fingerprint, simulate-under-policy over history, and evidence
discipline on the tool's own figures.

---

## 25. Final decision table

| Capability | Need | Replay owns? | Integration? | Free | Pro | Enterprise | Evidence needed | Priority |
|---|---|---|---|---|---|---|---|---|
| Identity fields on every record, absent as a third value | yes | yes (bind, record) | OIDC/SPIFFE issue | yes | | | RED on a record with and without a presented identity; black-box through the proxy; mutation on the binder | 1 |
| Token-validated identity, per-tenant state | enterprise | yes | IdP | | | yes | SP-5/6/8 oracles; concurrency across tenants; forged header refused | 2 |
| Decision record per request | yes | yes | | yes (observe) | yes | yes | one record per request forwarded or refused; mutation removes it; `investigate` reads it | 1 |
| External PDP for action authorization | enterprise | evaluate only | OPA/Cedar/PDP | | local file | external, signed | PDP unreachable fail-closed test; policy version on the record | 2 |
| Execution fingerprint | yes | yes | | yes | | | fingerprint diff between two recorded executions; tool hash drift refused under policy | 1 |
| Context manifest and compaction event | yes | yes | | yes | | | manifest reconciles to the measured prompt tokens (conservation law); compaction event observed | 1 |
| Context strategy comparison | pro | yes | | | yes | | populations stated; no saving word | 2 |
| `investigate <event>` | yes | yes | | | yes | signed | deterministic twice; UNKNOWN fields printed as UNKNOWN | 1 |
| `simulate --policy` over history | yes | yes | | | own corpus | fleet, signed | affected-request count reproduces on the same corpus; rules-version mismatch reported | 1 |
| Outcome declaration and per-outcome reporting | yes | input only | CI, tickets | yes | yes | yes | undeclared is a third value; cost per declared outcome carries the declaration rate | 1 |
| Freeze and resume (incident) | yes | yes | pager, tickets | manual | TUI | PDP-driven | freeze is monotone; in-flight response delivered; next request refused | 2 |
| Behavioural anomaly guards on existing signals | yes | yes | security products for content | | yes | yes | false-refusal rate on history by `simulate`; mutation on each guard | 2 |
| Failure injection and control-behaviour tests | pro | yes | runtimes' own harnesses | flags | harness | | injection impossible on a production listener without the flag; `doctor` reports it | 3 |
| Policy as files, `policy check`, signing | yes | yes | Git, CI | | yes | signed | version hash on every decision; CI refuses an invalid file | 2 |
| Delegation fields and inherited caps | yes | record | IdP for the authority | yes | yes | yes | child spend counted against parent; cycle refused | 3 |
| Artifact references | yes | refs only | the stores | yes | | | hash recorded, bytes never stored | 3 |
| Retention, deletion, legal hold, classification, CMK | enterprise | yes | KMS | | | yes | purge refuses under hold and says why | 3 |
| MCP server discovery | yes | yes | MCP | yes | | | observed/inferred/unknown preserved | 3 |
| OTel and SIEM export | yes | emit only | OTel, OCSF | yes | | yes | exported span count equals ledger count | 2 |
| Durable scratch, memory, continuity product | no | no | runtimes | | | | closed negative, Rooms VII to IX | kill |
| Quota titration | no | no | | | | | null result; no second allocation | kill |
| Generic realtime, dashboards, rollback, multi-region, graph UI | no | no | customer's | | | | | kill |

---

## 26. The final product map

```text
REPLAY
│
├── OBSERVE     ledger record (usage, model, provider, tool hashes), identity fields,
│               fingerprint, context manifest, discovery (observed / inferred / unknown)
├── CONTROL     one admission point in the request path: spend caps, loop, breaker,
│               prefix ceiling, masking, anomaly guards, freeze
├── CONTEXT     the manifest and its conservation law; compaction events;
│               comparison of strategies over history
├── CONTINUE    checkpoint of Replay state only; Work id; freeze and resume;
│               nothing semantic, nothing external
├── OPTIMIZE    replay, route, trim, ceiling, learn, simulate: history under a
│               proposed policy, populations stated, no "saving"
├── AUTHORIZE   the input document and the decision record; evaluation by an
│               external PDP or a bundled local evaluator; identity consumed, never minted
├── GOVERN      policy as signed files with versions; per-tenant state; retention,
│               deletion, legal hold; delegation records; chargeback by identity and Work
├── PROVE       claims register, frozen mutants, production-grade gate, investigate,
│               evidence export; every figure with its population and its oracle
└── RECOVER     freeze, resume, abort (refuse from here); never rollback
```

Minimum primitives per branch: OBSERVE needs the record, identity fields,
fingerprint, manifest. CONTROL needs the decision record and the freeze state.
CONTEXT needs the manifest and the compaction event. CONTINUE needs the Work id and
the Replay-state checkpoint. OPTIMIZE needs the ledger and the policy file.
AUTHORIZE needs the input document, the policy version and the PDP interface.
GOVERN needs the tenant dimension, the policy signature and the hold flag. PROVE
needs what it already has plus signing. RECOVER needs freeze and nothing else.

---

## 27. The answer

**The smallest product that could plausibly become a major AI-infrastructure
company without years of commodity work is the admission decision record and the
replay of history under a proposed policy, both carrying an execution fingerprint
and a presented identity, proven by the evidence discipline Replay already has.**

Concretely: one proxy in the request path that writes, for every request, who
presented, which Work, what it would cost, what the tool set and policy were, and
whether it went out and why; one command that reconstructs any of those records;
one command that re-runs the history under a different policy and says what would
have been refused, on which population; and a Free tier where every bit of that is
measured and only comparison, enforcement at organisation scale and proof export
are paid. Identity is consumed from the IdP, authorization is evaluated by the
customer's PDP, execution is left to the runtime, memory is left to the runtime,
traces go to OTel, alerts go to the SIEM. Replay owns the correlation, the
decision, and the proof, and nothing else.

Everything else in the proposed map is either already a surface of that product,
a measured negative, or someone else's category.
