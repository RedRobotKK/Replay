# Implementation board: ten passes against the commercial, capacity and routing plan

**2026-10-03. Twelve roles, ten passes, each trying to break the previous one. Every
statement about what Replay does today is tied to a repository artifact; every
statement about a provider's capacity visibility is tied to the quota census or
labelled UNKNOWN. Nothing here is built, committed or pushed. Release 1.0 at
`9694f1e` is frozen and untouched.**

Status words, used exactly: **SHIPPED** (in the certified binary), **MEASURED** (a
registered oracle produced the figure), **DESIGNED** (an ADR or design document,
not built), **PROPOSED** (this plan), **USER-DECLARED**, **PROVIDER-OBSERVED**,
**RUNTIME-OBSERVED**, **INFERRED** (never silently), **UNKNOWN**, **SIMULATED**.

---

## 0. What the repository already decided, which the plan did not read

Three documents constrain everything below and the proposed commercial
architecture contradicts all three.

- **ADR-0023, "Entitlement is a signed document, not an account" (DESIGNED,
  Proposed, 2026-09-13).** No licence server, no account, no callback, no
  identifier that leaves the machine; a document installed from a local file,
  verified offline against a public key compiled into the binary, expiring on a
  date read from the local clock. The transaction that produced it "lives with
  whoever took the money and never reaches the binary."
- **ADR-0013, the x402 rules feed (SHIPPED as the refusal path).** "Replay never
  holds a key and never signs a transaction." The free tier "is not a trial and is
  not degraded." SPONSORS.md: nothing free today ever becomes paid.
- **ADR-0015, single-tenant state is a boundary.** A tenant dimension must enter
  the spend guard, the session table, the metrics surface and the credential path
  before any commercial layer is built on them.

And three measured facts:

- **Quota data census (MEASURED, 2026-09-09).** The Anthropic quota capture path on
  the proxy has recorded **zero** header observations; `retry-after` has never been
  seen; no lockout has ever been observed locally. The only live quota signal in any
  client is Codex's `rate_limits` (6,871 events, 5h window 0 to 52%, 7d window 0 to
  89%). Claude Code sends `rate_limits` percentages to the statusline (RUNTIME-
  OBSERVED, `cmd/replay/statusline.go`), and the subscription titration returned
  null: 3.09M tokens moved the window by zero steps, so no token-denominated
  conversion of a window exists.
- **The proxy fronts one upstream** (`--upstream`, `cmd/replay/serve.go`), never
  rewrites bytes on the wire (re-proven in the production-grade gate), and the
  OpenAI-compatible client half is a STUB except for DeepSeek
  (`docs/PRODUCTION-WIRING.md`). `replay route --to` is a postmortem projection, not
  a router.
- **REVENUE-PIVOT (2026-09-30):** the individual $25/month question is closed on
  the repository's own arithmetic; the asset being sold is the operator's method.

The plan proposes a hosted account hierarchy, Stripe and RevenueCat webhooks into
a canonical entitlement service, a capacity ledger over sources whose capacity is
mostly unknowable, and a router in a binary that cannot translate requests. The
passes below take that apart and keep what is left.

---

## Pass 1: system boundaries

**Question.** What does Replay actually own?

**Attack (all twelve).** The Product Architect and the Killer agree before anyone
else speaks: the plan lists nine systems to own and Replay has the evidence to own
three. The Billing Architect notes that "Replay Account → Organization →
Membership → Subscription" is a SaaS backend and ADR-0023 forbids it. The IAM
Architect: identity issuance is the IdP's. The Routing Architect: a router that
cannot translate request shapes is a selector, not a router. The Financial
Specialist: a "capacity ledger" over unknown capacity is a ledger of guesses. The
Security Architect: a canonical entitlement *service* is a network dependency in
the credential path of a `curl`-installed binary, the exact thing ADR-0023 refuses.

**Contradictions.** Local-first with no account (README, ADR-0023) versus a hosted
entitlement service. Free-tier-complete (ADR-0013) versus "Replay billing expires
while routing is active." Never-rewrite-the-wire (SHIPPED) versus routing across
provider shapes.

**Missing evidence.** Any customer asking for capacity coordination; any provider
capacity API (none integrated, none assumed).

**Snapshot after Pass 1.**

| OWN | INTEGRATE | OBSERVE | NEVER OWN |
|---|---|---|---|
| request admission in the request path (SHIPPED) | identity from the IdP | provider rate-limit signals where they exist | accounts, memberships, a subscription service |
| the ledger record and decision record | authorization decisions from a PDP | runtime quota readings (Codex rollouts, Claude statusline) | billing state machines (Stripe's) |
| execution fingerprint, context manifest (PROPOSED, from the PRD review) | OTel, SIEM exports | provider usage ledgers (Grok usage.json reconciliation, SHIPPED) | request translation between provider shapes |
| replay and simulate over the own ledger (SHIPPED machinery) | a model gateway for routing execution | user-declared plans and budgets | a capacity marketplace, a credential vault, memory, durable execution |
| entitlement *verification* of a signed document (DESIGNED) | the seller's Stripe, on the seller's side | | RevenueCat (no mobile product exists) |

Killed in this pass: Replay Account, Organization, Membership as Replay-hosted
objects; the canonical entitlement *service*; RevenueCat.

---

## Pass 2: account and identity

**Question.** With no account, what are the objects, and who may do what?

**Attack.** The IAM Architect tries to restore a Person object: "someone bought
it." ADR-0023 answers: the buyer is known to the seller's transaction, never to the
binary; the document names what was bought and when it lapses. The UX Lead asks
how an organisation shares an entitlement: by installing the same document on
each machine, which is the design's stated shape. The Security Architect asks who
may add credentials: the operating-system user who can write the environment or
the keychain; Replay adds nothing and holds a reference.

**Validated chain (PROPOSED fields on the SHIPPED record).**

```text
Human principal    OBSERVED if presented (OIDC subject on x-replay-principal), else ABSENT
Agent              OBSERVED if presented (workload identity or client header), else ABSENT
Work               Replay-minted id, always present once the proxy sees a request
Request            the ledger record (SHIPPED)
Tool               name and definition hash (SHIPPED as the tool-set epoch)
Provider           endpoint, model, rules version (SHIPPED)
Credential         CredentialRef only: an env var name, a keychain item, a file path
Entitlement        the signed document's id, product and expiry (DESIGNED)
Capacity source    a provider plus a credential ref plus a declared plan (PROPOSED)
```

**Who does what.** Authenticates: nobody to Replay; the OS user is the trust
boundary (ADR-0015). Authorizes actions: the customer's PDP (Enterprise) or a local
policy file (Pro); Replay evaluates and records. Owns AI capacity: the provider
account holder, who is not an object Replay knows. Routes: nobody yet (Pass 6).
Modifies policy: whoever can write the policy file; in Enterprise the file is
signed. Views billing: the seller's dashboard, not Replay. Views AI usage: anyone
who can run the binary against the ledger. Delegates: recorded as a parent Work id
and inherited caps; adjudicated by the IdP.

**Killed.** Person, Account, Organization, Membership as Replay objects.
**Narrowed.** Subscription becomes Entitlement, a document. Identity fields stay
OBSERVED or ABSENT; "UNKNOWN" is reserved for a field the runtime could have sent
and did not.

---

## Pass 3: Stripe and RevenueCat

**Question.** Where does the money machinery live, and what does the binary store?

**Attack.** The Billing Architect designs the webhook pipeline and the Killer
asks where it runs. Not in the binary (ADR-0023). So it runs on the seller's side,
and the seller is one person with no robot-held signing key wanted
(`operating-at-scale.md`, gap 2: "entitlement issuance either leaves a ten month
revocation lag or puts the signing key on a robot"). The Reliability Engineer
attacks the eight proposed states: `PAST_DUE`, `GRACE`, `CANCELED_PENDING_EXPIRY`,
`REVOKED` are Stripe's facts about a payment relationship; the binary can only
observe a document. The Mobile Architect: RevenueCat is an adapter for Apple and
Google subscriptions; there is no Replay mobile product, so the adapter has no
input.

**Source of truth.** Stripe is the source of truth for the payment relationship.
The signed document is the source of truth for what the binary may do. Nothing
reconciles them at runtime because nothing connects them at runtime; the
reconciliation is the issuance job.

**What the binary knows (DESIGNED).** `ABSENT`, `VALID` (signature verifies,
expiry in the future by the local clock), `EXPIRED`, `INVALID` (signature or
schema). That is the entire state machine on the client. The seller-side states
collapse into one act: issue a document, or do not. Grace is a document issued
with a later expiry; revocation is the absence of the next document; a refund or
chargeback is likewise non-issuance. Short-lived documents (a month) replace a
revocation list, and that is the resolution of gap 2: the signing key is used
once a month by a human, not continuously by a robot.

**Webhook pipeline, seller side only.** `event → verify Stripe signature → persist
raw event id and payload hash → idempotency on event id → normalize to
(customer, product, period) → decide: issue or not → sign document → deliver →
audit row`. Duplicate and out-of-order events are harmless because the decision is
idempotent on (customer, period): the document for a period either exists or does
not. Failed payment: no document for the next period. Trial: a document with a
short expiry. Account deletion: nothing to delete in Replay; the seller deletes
the Stripe customer.

**What Replay stores.** The document, under `~/.replay/`, registered in
`stores.go` so `privacy` discloses it and `purge` can remove it; nothing else.
**What Replay never stores.** A customer id, an email, a payment state, a webhook
secret.

**Killed.** RevenueCat (no input exists); the canonical entitlement service as a
runtime component; six of eight client-side states.
**Narrowed.** "Stripe adapter" becomes a seller-side issuance script that the
repository may hold but the binary never links; it is `scripts/`, not `cmd/`.

**Open.** The ten-month revocation lag is solved by monthly issuance, which is a
monthly human action; whether that scales past a few hundred documents is
operating-at-scale's question and stays open.

---

## Pass 4: AI capacity discovery, or what Replay can honestly know

**Question.** For each source, what can be observed?

**Attack.** The Provider Integration Specialist fills the table from the census and
the code; the Killer strikes every cell that is not observed.

| Source | Plan | Capacity type | Remaining capacity | Reset | Price | Source of the value | What routing could use |
|---|---|---|---|---|---|---|---|
| Claude (Max) via Claude Code | USER-DECLARED | 5h and 7d windows, percent used | RUNTIME-OBSERVED **only when Claude Code sends `rate_limits` to the statusline**; never seen on the proxy (census: 0 header observations) | RUNTIME-OBSERVED `resets_at` | included; the token-to-window conversion is **UNKNOWN** (titration null) | statusline JSON per render | "near 100% used" as an exhaustion signal; nothing finer |
| Claude via API key | USER-DECLARED | metered | **UNKNOWN**; `anthropic-ratelimit-*` headers would be PROVIDER-OBSERVED but none has ever been recorded | UNKNOWN | list price (SHIPPED, dated table) | proxy headers, if they ever appear | a 429 as an exhaustion event; spend at list |
| Codex (ChatGPT subscription) | USER-DECLARED | 5h and 7d windows, percent used | RUNTIME-OBSERVED from rollouts (SHIPPED: `burn` prints the windows) | RUNTIME-OBSERVED | included | rollout `rate_limits` | exhaustion signal; coarse headroom in percent |
| ChatGPT Free | USER-DECLARED | message caps | **UNKNOWN** | UNKNOWN | included | nothing readable | nothing |
| Grok Free | USER-DECLARED | undocumented | **UNKNOWN**; usage.json is a usage ledger, not a capacity | UNKNOWN | **UNKNOWN** (`costUsdTicks` scale unverified) | vendor ledger (reconciled, SHIPPED) | nothing |
| DeepSeek credits | USER-DECLARED balance | prepaid | **UNKNOWN** unless the user declares and decrements by Replay's own usage; no balance API is assumed | n/a | list price (DeepSeek rows in the table) | ledger usage via the proxy's OpenAI path (SHIPPED for DeepSeek) | a declared budget, consumed by measured usage |
| Local models (Ollama) | n/a | hardware | always available; latency MEASURED from logs (SHIPPED) | n/a | $0 cash | server logs | always eligible for work that fits the model |

**Findings.** Two of seven sources carry any live headroom signal, both as a
percentage of a window whose token size is unknown. Four are UNKNOWN. One is
"always". No source exposes remaining capacity in requests or tokens. Therefore:

- "remaining capacity" cannot be a number Replay computes for any subscription;
- the only honest states are `EXHAUSTED` (window at 100%, or the provider answered
  429), `HEADROOM_PERCENT` (Codex, Claude via statusline), `DECLARED_BUDGET_REMAINING`
  (DeepSeek, from the user's number minus measured usage), `UNKNOWN`, `ALWAYS`;
- UNKNOWN is treated as "available until refused, and never as free of charge."

**Snapshot.** The capacity object is an **observation log with provenance**, not a
balance. Routing on capacity reduces to routing on exhaustion and on declared
budgets. "Maximize included capacity" is implementable only as "prefer the included
source until it reports exhaustion," which is the same as what a user does by hand.

---

## Pass 5: the capacity ledger

**Question.** What is the minimum accounting Replay must own?

**Attack.** The Financial Specialist separates seven ledgers and the Killer deletes
five. Replay commercial money: not in the binary (Pass 3). Actual provider cost:
never known; every dollar is list price (REFUTED claim RPL-C012, SHIPPED
disclosure). Unknown cost: already a value (`unpriced`, SHIPPED). Subscription
entitlement: a document (Pass 3). Prepaid credits: a user-declared number.

**What survives, and most of it is shipped.**

```text
CapacityObservation   append-only: source, value, unit, provenance, observed_at, expires_at, credential_ref
Usage                 the ledger record's usage, per request, per provider (SHIPPED)
ListCost              usage priced by the dated table (SHIPPED), or unpriced (SHIPPED)
DeclaredBudget        user number, with its declaration time (PROPOSED)
Reconciliation        provider ledger versus Replay reading, MATCH / DIFFERS / UNAVAILABLE (SHIPPED for Grok)
```

No reservations: there is nothing to reserve against an opaque window, and the
one place reservation would matter, a declared prepaid budget, is handled by the
spend cap that already exists. No refunds, no corrections beyond appending a
later observation. Idempotency is the provider request id (SHIPPED join rule,
RPL-C020). Concurrency: the spend guard's documented one-request overshoot under
concurrency (MEASURED, qualification QT-8) is the ledger's concurrency model and
it is acceptable for a cap, unacceptable for a balance, which is one more reason
not to pretend to hold a balance.

**Killed.** Reservation, consumption-as-debit, refund, correction, versioned
balances.
**Narrowed.** The ledger is observations plus usage plus declared budgets; the
only derived figure is "declared minus measured, at list, population stated."

---

## Pass 6: routing

**Question.** Should Replay route?

**Attack.** The Routing Architect designs the explainable decision; the Provider
Specialist and the Killer attack its inputs and outputs. Three facts end the
gateway version in one pass:

1. **The clients are provider-bound.** Claude Code speaks to Anthropic, Codex to
   OpenAI, Grok CLI to xAI. None accepts a routing decision; each can at most be
   pointed at a base URL of its own provider's shape. A Claude Code request cannot
   be sent to DeepSeek without translating the body, and translation is the
   gateway's category (LiteLLM, Portkey), which the PRD review placed in NEVER
   BUILD and the never-rewrite invariant forbids.
2. **Capacity is UNKNOWN for most sources** (Pass 4), so "route to the source with
   capacity" is "route to the source that has not refused yet."
3. **Quality routing needs an outcome signal**, which no corpus carries (MEASURED
   absence). "Route simple work to cheaper providers" presumes Replay can tell
   simple from hard; it cannot, and must not infer it.

**What survives.** A **decision**, not a redirect: for a request that an
OpenAI-compatible client could send to several same-shape endpoints behind a
gateway, Replay can evaluate policy (cash budget, declared budgets, exhaustion,
model allow-list) and answer with the eligible set, the exclusions and reasons, and
a recommended target, as a header the gateway may honour and as a decision record.
Execution stays with the gateway. For provider-bound clients the same evaluation
yields a **refusal or a pass** (the SHIPPED admission), never a redirect.

**Explainability.** Every decision record carries: request id, candidate sources,
each exclusion with its reason and the evidence's provenance, the selected source,
the policy version hash, the capacity observations consulted with their ages, the
list cost at stake. This is the PRD review's Decision primitive with capacity
fields added; it is reconstructible because every input is in the ledger.

**Killed.** Replay as the executor of cross-provider routing; request translation;
quality-based routing; "preserve premium capacity for difficult work."
**Narrowed.** Routing becomes **admission with a recommendation**, enforceable only
as refuse-or-forward on the one upstream the proxy fronts, advisory elsewhere.

---

## Pass 7: billing times capacity

**Question.** Can the Replay entitlement and the user's provider subscriptions be
confused, and what happens when states go stale?

**Attack.** Each case, with the surviving architecture:

- *User cancels Replay, keeps provider credentials.* The document expires; the free
  tier is complete (ADR-0013), so the proxy, caps, loop guard, cost, everything
  measured keeps working. Only the paid capability (Pass 9 names it) stops. Nothing
  is spent because of the expiry.
- *User cancels Claude, Replay holds a CredentialRef.* The next request gets the
  provider's 401 or 403, relayed unchanged, recorded as a provider refusal; the
  capacity source transitions to `REFUSED_BY_PROVIDER` with that evidence. Replay
  spends nothing; it never had the credential, only its name.
- *Replay entitlement expires while the proxy is forwarding.* Forwarding is free and
  continues; a paid feature that was mid-run (a fleet simulation) finishes its
  current invocation and refuses the next, with the document's expiry in the
  message. There is no "routing active" state that money depends on, because
  routing execution is not Replay's (Pass 6).
- *Provider billing state changes (plan downgraded, credits exhausted).* Observed
  only through the provider's own refusals and, where present, the runtime's window
  percentages; Replay never assumes a plan from a declaration past its declared
  time, and a declaration carries its date so staleness is visible.

**Invariant adopted.** *No Replay commercial state may change what the proxy
forwards.* Entitlement gates features that compute, never the path that forwards.
A test holds it: with an EXPIRED document installed, the full proxy black-box
suite must pass unchanged.

**Killed.** Any coupling between entitlement and admission.

---

## Pass 8: security and secrets

**Question.** Can Replay work without becoming a vault?

**Attack.** The Security Architect lists what the plan would store and the panel
strikes each.

| Item | Replay stores | Never stores | Boundary |
|---|---|---|---|
| provider API keys and OAuth tokens | a `CredentialRef` (env var name, keychain item, file path) | the secret | the OS user's keychain or environment; masking on the wire already catches secrets in traffic (SHIPPED) |
| entitlement document | the document, owner-only, registered in `stores.go` | the buyer's identity | public key compiled in; expiry by local clock |
| seller-side Stripe webhook secret | never (not in the binary) | | seller's host |
| policy files | the file; Enterprise: the signature | | signed with a key the customer holds; a policy decision names the version hash |
| identity token presented to the proxy | validated, hash recorded | the token | rotation is the IdP's; the proxy rejects an expired token with a recorded refusal |
| ledger, decision records, exports, backups | kinds, sizes, timings, usage, tool names, identity fields, reasons | message text, secrets | the existing disclosure and purge rules extend to decision records |
| support access | nothing to access; the operator holds the files | | |

New outbound destinations, which RPL-C022's enumerated set must gain before any
is wired: the PDP (Enterprise). The seller's issuance script is not the binary and
adds nothing to the set. Replay functions without ever holding a provider secret:
the proxy already forwards whatever credential the client sent, unchanged, and the
capacity reader needs none.

**Killed.** A Replay-held credential store; webhook secrets in the client.

---

## Pass 9: UX and onboarding

**Question.** Is this easier than switching tools by hand?

**Attack.** The UX Lead walks a user who has Claude Code, Codex, Grok CLI, a
DeepSeek key and Replay. Setup today (SHIPPED): install, run `replay`, see the
cost report; `doctor` finds Claude Code, Codex, Grok and Ollama unaided. For the
capacity feature the honest flow is:

1. `replay doctor` shows each discovered source with its capacity state and
   provenance (Codex: 41% of 7d used, from rollouts; Claude: 83% of 5h, from the
   statusline reading 12 minutes ago; Grok: UNKNOWN; DeepSeek: UNKNOWN until
   declared).
2. `replay capacity declare deepseek --budget 42 --as-of today` for the one source
   that is a number only the user knows.
3. Nothing else, because nothing else can be routed: the user still opens the tool
   whose provider they want.

Is that easier than switching by hand? It makes the *decision* visible in one
place; it does not make the switch. The Killer's verdict stands: as a product, "use
the capacity you already have" collapses to a status line, and the previous review
killed dashboards as a differentiator. As a measurement it is cheap and belongs in
Free, on `doctor`, `since` and the statusline, which already carries the Claude
windows.

**Killed.** The routing-policy onboarding, credential connection flows, "first
simulation" of routing.
**Narrowed.** Capacity becomes a line in existing surfaces plus one declaration
command.

---

## Pass 10: the production kill test

**A. Can Stripe plus RevenueCat plus a gateway do everything important?** Stripe does
billing; a gateway does routing and key-scoped budgets; RevenueCat has no role.
What they do not do is the decision record with capacity provenance and the replay
of history under a policy, which is the surviving core and is independent of
capacity coordination.
**B. Can users get the same result without Replay?** For routing, yes, by hand or
by a gateway. For "what did my spend across tools actually buy and where did the
cache break," no; that is the certified product.
**C. Can Replay know enough capacity to route honestly?** No. Two of seven sources
carry a coarse percentage; four are UNKNOWN; none is token-denominated.
**D. Does the user benefit from capacity coordination?** No evidence. No customer
has asked; the $25/month question is closed; the only measurable benefit would be
avoided cash spend, which requires a cash path Replay can refuse, which is the
existing cap.
**E. Is credential management too dangerous?** Not with references only; it is
dangerous the moment a store exists, so the store is NEVER BUILD.
**F. Does configuration exceed value?** For routing, yes. For a capacity line in
`doctor`, no: zero configuration for two sources, one declaration for a third.
**G. Does Replay become a gateway?** If it routes, yes; so it does not route.
**H. Can every decision be explained?** Yes, for decisions Replay makes
(admission, refusal, recommendation); the record is specified in Pass 6.
**I. Can historical commercial and routing state be reconstructed?** Commercial: the
document's issuance history lives with the seller; the binary can show which
document was valid when. Routing: every decision record is in the append-only ledger.
**J. Can a customer run Replay without trusting it with secrets?** Yes, today and
in the surviving design.
**K. Can the smallest implementation prove the commercial hypothesis?** The
hypothesis "developers pay to coordinate capacity" cannot be proven by building a
router that cannot route; it can be falsified cheaply (section 8 below).
**L. Smallest feature that makes a real customer say "I would pay"?** **Nothing yet,
for capacity coordination.** The repository's own revenue memo says the software
has not found a payer and the operator's method has. The candidate with the best
odds is not in this plan: `replay simulate --policy` over the customer's real
ledger, because it answers "what would this cap or model policy have done to my
last 10,000 requests" with a population, and that is a number a team can take to a
budget meeting. It is Pro in the PRD review and untested with any buyer.

---

## Final architecture

### A. Executive verdict

**Survived.** Entitlement as a signed, short-lived document (ADR-0023), with the
seller-side issuance script as the only Stripe integration. Admission in the
request path, the decision record with capacity provenance, the fingerprint, and
`simulate` over the own ledger. Capacity as an observation log with provenance,
shown on existing surfaces. Identity consumed and bound, never minted.

**Died.** Replay accounts, organisations, memberships, a subscription service, a
canonical entitlement service, RevenueCat, the capacity ledger as a balance,
reservations, Replay-executed cross-provider routing, request translation,
quality-based routing, "preserve premium capacity," a credential store, any
coupling between entitlement and forwarding.

**Narrowed.** Routing to admission-with-recommendation (refuse or forward on the
one upstream; advise elsewhere). Capacity to five states (EXHAUSTED,
HEADROOM_PERCENT, DECLARED_BUDGET_REMAINING, UNKNOWN, ALWAYS). Client-side
entitlement to four states (ABSENT, VALID, EXPIRED, INVALID).

**Unknown.** Whether anyone pays for any of it; whether providers will ever expose
capacity on the wire; whether monthly document issuance scales; the token size of
any subscription window.

### B. Canonical ownership matrix

| Capability | Replay owns | External source | Observe only | Never build |
|---|---|---|---|---|
| billing, subscription state | | Stripe, seller side | | a billing service in the binary |
| entitlement | verification of a signed document | seller's issuance | | account, licence server |
| mobile subscriptions | | | | RevenueCat (no product) |
| identity | binding of presented fields | IdP, SPIFFE | | issuance |
| authorization | input document, decision record, local file (Pro) | PDP (Enterprise) | | an IAM |
| provider credentials | CredentialRef | OS keychain, env | | a vault |
| AI capacity | observation log with provenance, declared budgets | runtimes, provider refusals | windows, 429s, provider ledgers | a balance, a marketplace, reservations |
| routing | admission and recommendation with reasons | gateway executes | | translation, quality routing |
| request admission | yes (SHIPPED) | | | |
| economic accounting | list cost, caps, unpriced disclosure (SHIPPED) | | | actual provider cost (unknowable) |
| provenance, fingerprint, context manifest | yes | | | |
| historical replay, simulation | yes | | | prediction of the future |
| OTel, SIEM | emit | backends | | storage |
| MCP | discovery, tool hashes, a read-only server (SHIPPED) | | | |
| runtime execution, memory, durable execution | | runtimes, Temporal-class engines | | |

### C. Commercial architecture

```text
Stripe (seller's account, seller's host)
   ↓ webhook, signature verified, event-id idempotent
Issuance script (scripts/, never linked by the binary)
   ↓ decides per (customer, product, period): issue or not
Signed entitlement document (product, period, expiry; no person)
   ↓ delivered to the buyer; installed from a local file
replay entitlement install <file>   (DESIGNED)
   ↓ verified offline: compiled public key, local clock
Entitlement state in the binary: ABSENT | VALID | EXPIRED | INVALID
   ↓ gates computing features only
Replay authorization of paid features. Never the forwarding path.
```

Source of truth: Stripe for the payment relationship; the document for what the
binary may do; no runtime link between them.

### D. AI capacity architecture

```text
Provider (Anthropic, OpenAI, xAI, DeepSeek, local)
  ↓
Capacity source = provider + CredentialRef + declared plan (USER-DECLARED, dated)
  ↓
Capacity evidence = observations: window percent (RUNTIME-OBSERVED), 429 / 401 (PROVIDER-OBSERVED),
                    declared budget (USER-DECLARED), usage at list (MEASURED), nothing (UNKNOWN)
  ↓
Capacity state = EXHAUSTED | HEADROOM_PERCENT | DECLARED_BUDGET_REMAINING | UNKNOWN | ALWAYS, each with
                 value, source, observed_at, expires_at, confidence (observed = 1, declared = stated age)
  ↓
Policy (file, versioned): cash budget, per-source caps, allow-lists, "UNKNOWN is not free"
  ↓
Request decision: forward | refuse | recommend(source), with exclusions and reasons
  ↓
Decision record (append-only, reconstructible)
```

### E. Data model, minimum

Keep: `Request` (SHIPPED record), `Work`, `Decision` / `DecisionRecord` (one object),
`ExecutionFingerprint`, `ContextManifest`, `Outcome` (declared), `ArtifactRef`,
`Provider`, `CredentialRef`, `CapacitySource`, `CapacityObservation`,
`RoutingPolicy` + `PolicyVersion` (one file, one hash), `Entitlement` (the
document).

Delete: `Account`, `Organization`, `Membership`, `Subscription` (seller side),
`Decision` as separate from `DecisionRecord`.

### F. State machines

- **Entitlement (client):** ABSENT → VALID (install) → EXPIRED (clock) ; any →
  INVALID (bad signature); VALID → VALID (newer document replaces). Idempotent on
  document id.
- **Capacity source:** UNKNOWN → HEADROOM_PERCENT (observation) → EXHAUSTED (100% or
  429) → HEADROOM_PERCENT (reset time passed or new observation); DECLARED →
  DECLARED_BUDGET_REMAINING (usage) → EXHAUSTED (zero); any → REFUSED_BY_PROVIDER
  (401/403). Observations are append-only; the state is the latest unexpired one.
- **Request:** received → summarized → decided (forward | refuse) → forwarded →
  responded | failed → recorded. SHIPPED.
- **Decision:** evaluated once per request; the record is immutable; an override
  is a second decision referencing the first.
- **Provider attempt:** sent → ok | retryable (retried under the documented rules,
  SHIPPED) | failed → breaker state (SHIPPED).
- **Routing:** not a state machine; a pure function of (request summary, policy
  version, capacity states, caps).
- **Reconciliation:** per provider ledger when one exists: MATCH | DIFFERS |
  UNAVAILABLE (SHIPPED for Grok).

Concurrency: the spend guard's rule stands (in-flight responses are not counted
until they return; one-request overshoot measured). Capacity states are read at
decision time and never locked, because they are advisory except at EXHAUSTED.

### G. Billing event pipeline (seller side)

`external event → verify signature → persist (event id, payload hash, received_at)
→ idempotency on event id → normalize to (customer, product, period) → decide
issue / no issue → sign → deliver → audit → monthly reconciliation of issued
documents against Stripe's active subscriptions`. Failure: a webhook delivery
lost is recovered by the monthly reconciliation; a duplicate is a no-op; an
out-of-order cancel-then-create resolves by period, not by order.

### H. Capacity provenance

Every value: `value, unit, source (RUNTIME-OBSERVED | PROVIDER-OBSERVED |
USER-DECLARED | UNKNOWN | MEASURED), observed_at, expires_at (window reset, or
declaration age limit), provider, credential_ref, confidence`. UNKNOWN has no
value and no confidence. Nothing is inferred.

### I. Routing decision record, example

```text
Request:        req_01a0 (session 7f3c, Work w-19)
Candidates:     claude-max (Claude Code, included), codex (included), deepseek (declared $42), ollama (local)
Policy:         P17 (hash 9c2e…): cash budget $5/day; prefer included; UNKNOWN is not free
Capacity:       claude-max HEADROOM_PERCENT 17% of 5h, RUNTIME-OBSERVED 12 min ago, resets 14:00
                codex HEADROOM_PERCENT 59% of 7d, RUNTIME-OBSERVED 3 min ago
                deepseek DECLARED_BUDGET_REMAINING $31.40 (declared 2026-10-01; usage at list since)
                ollama ALWAYS
Entitlement:    VALID (pro, expires 2026-11-01)
Excluded:       claude-max : provider-bound client; this request's shape is OpenAI chat
                ollama     : model allow-list excludes local for this Work
Selected:       deepseek
Reason:         eligible shape; declared budget remains; cash today $1.10 of $5.00 at list
Enforcement:    forward (this proxy fronts deepseek); recommendation header set for the gateway
Reconstruct:    replay investigate req_01a0
```

Every line is a field on the record; none is prose only.

---

## 6. TDD and mutation plan

For every primitive: RED test on the binary (black-box, `internal/blackbox`),
implementation, GREEN, a frozen mutant in the catalogue killed by the E2E test and
by the black-box check, failure injection where a dependency exists, a concurrency
test where requests can overlap, a reconstruction test (`investigate` reproduces
the record).

| Scenario | RED oracle | Mutation that must die |
|---|---|---|
| duplicate Stripe webhook (seller script) | second delivery issues no second document | drop the event-id check |
| out-of-order billing events | period decides, not order | sort by arrival |
| stale / expired entitlement | EXPIRED by local clock; paid feature refuses, forwarding unchanged (full proxy suite green with EXPIRED installed) | compare expiry to a cached time |
| invalid document | INVALID, refused, reason names the signature | skip verification |
| provider credential revoked | 401 relayed, source → REFUSED_BY_PROVIDER, no spend | treat 401 as retryable |
| unknown quota | state UNKNOWN; policy "UNKNOWN is not free" refuses a cash path above budget | map UNKNOWN to ALWAYS |
| stale quota | an observation past `expires_at` is not consulted; decision says "no current observation" | ignore expiry |
| provider outage | breaker opens after N, 503 locally, recovers (SHIPPED, re-proven) | existing catalogue |
| provider price change | a new table date changes list cost and the cost index key (SHIPPED, `TestC037R6`) | existing |
| routing fallback | second eligible source recommended when the first is EXHAUSTED; reasons recorded | skip exclusions |
| concurrent routing | 100 concurrent decisions produce 100 records, each reconstructible | drop the per-request record |
| concurrent capacity consumption | declared budget overshoot bounded by in-flight count (document the bound, as QT-8 does) | count before the response |
| policy change | version hash on every record changes; `simulate` reports the delta on the population | reuse the old hash |
| historical simulation | same corpus, same policy → identical counts twice; "saving" absent from output | nondeterministic order |
| account deletion | nothing to delete; `purge` removes the document and records | leave the document |
| tenant isolation | a second tenant's identity cannot read or be counted against the first (SP-5/6/8) | drop the tenant key |
| secret redaction | no CredentialRef resolves to a secret in any record, export or log | log the resolved value |
| audit reconstruction | `investigate` on any decision reproduces every field, UNKNOWN printed as UNKNOWN | fill UNKNOWN with a default |

---

## 7. Production gates

| Gate | Test | Expected | Fail condition | Evidence |
|---|---|---|---|---|
| unit | package tests for document verification, capacity state machine, policy evaluation | green | any red | `go test ./...` |
| integration | seller script against Stripe's test-mode fixtures | idempotent issuance | a second document per period | script tests with recorded events |
| E2E | `TestE2E_*` for each new subcommand (`entitlement`, `capacity`, `simulate`) | green through `dispatch` | missing or red | wiring gate (SHIPPED) |
| mutation | frozen mutants for every new deciding site | killed by the harness and by the black-box binary | a survivor | `TestFrozenMutantsStillDie`, `TestBB_ProductionGrade` |
| race | `-race` full suite | 0 races | any | CI matrix |
| security | no secret resolved into any record; new outbound destination in RPL-C022's set before it is wired | scans green | a secret or an unlisted host | `TestC022_*`, redaction tests |
| billing | EXPIRED document leaves the proxy suite green | 33/33 black-box unchanged | any proxy check changes | `internal/blackbox` with the document installed |
| reconciliation | Grok-style MATCH/DIFFERS/UNAVAILABLE for any provider ledger | three values, never a sum | a sum | existing GK tests, extended |
| black-box | every new surface through the built binary with invalid input and JSON | PRODUCTION-GRADE | BLOCKED or UNMEASURED | production-grade matrix |
| production surface | matrix regenerated, 36 surfaces if three are added | ALL WIRED SURFACES PRODUCTION-GRADE | INCOMPLETE | `docs/evidence/production-grade-matrix.md` |

---

## 8. Commercial falsification experiments

Each is cheap, pre-registered, and has a kill threshold; none requires building
the router.

1. *Users already solve this with gateways.* Survey 20 multi-provider developers;
   if ≥ 14 run a gateway or a wrapper already, kill coordination.
2. *Users do not care about unused capacity.* Ship the Free capacity line in
   `doctor`; if fewer than 10% of `doctor` runs on opted-in machines reach a
   declared budget within 30 days, kill the declaration feature.
3. *Users refuse to connect providers.* With references only there is nothing to
   connect; the test is whether anyone declares a DeepSeek budget: see 2.
4. *Users cannot understand capacity state.* Five-user think-aloud on the `doctor`
   line; if fewer than 4 of 5 explain HEADROOM_PERCENT and UNKNOWN correctly, kill
   the line's wording and retest once.
5. *Routing quality is worse than manual.* Not testable without an outcome signal;
   therefore not built.
6. *Unknown quotas make the system unreliable.* Already established (Pass 4); the
   design assumes it.
7. *Provider restrictions prevent useful routing.* Already established (clients are
   provider-bound).
8. *Configuration exceeds benefit.* Count commands to the first decision record on
   a fresh machine; kill if more than 3.
9. *Users want optimization but not Replay.* Offer `simulate --policy` to ten
   existing users for 30 days; kill if fewer than 3 change a cap because of it.
10. *Users want Replay only as observability.* Same ten users: if no one enables a
    cap after seeing a simulation, the paid tier is observability-only and the Pro
    hypothesis dies.

---

## 9. Build order

**P0, must build (and the only paid candidate):** `replay simulate --policy` over
the own ledger for spend caps and model allow-lists, with population stated and
no "saving" word; the Decision record with policy version hash on every proxy
decision; the entitlement document verification (`replay entitlement install`,
four states) because without it nothing can be sold; the seller-side issuance
script. Reason: these are the smallest things that can test whether anyone pays.

**P1, after P0 evidence:** the Free capacity line (Codex and Claude windows,
declared DeepSeek budget) on `doctor`, `since` and the statusline; the capacity
observation log; `UNKNOWN is not free` as a policy rule on the existing cap.
Reason: cheap, honest, and it tests experiments 2, 4 and 8.

**P2, optional:** the recommendation header for OpenAI-compatible clients behind a
gateway; identity token validation and the tenant dimension (Enterprise).

**LATER:** a PDP integration; signed policy; the fleet simulator.

**DO NOT BUILD:** accounts, organisations, memberships, a subscription service, a
canonical entitlement service, RevenueCat, a credential store, a capacity balance
with reservations, cross-provider request translation, quality routing, premium
capacity preservation, any routing executor. Reason: each is a category someone
else owns, or depends on a signal that does not exist, or contradicts a published
promise.

---

## 10. The customer question

**If I am a developer spending $100 to $1,000 a month across several AI providers,
why would I install Replay tomorrow instead of continuing to choose providers by
hand or using a gateway?**

From the customer's side: not to route, because Replay cannot move my Claude Code
request to DeepSeek and I would not want a tool that pretended to know my Grok
quota. I would install it because in one command it tells me what each tool
actually cost at list price, which turns were paid twice because a cache broke and
why, and which of my tools is close to its window, with every figure saying where
it came from; and because when I turn on a spend cap, it stops the next request
instead of the tenth, and I can see exactly which request it stopped and why. A
gateway gives me a key-scoped budget and a dashboard; it does not tell me why the
money went, and it cannot read the two subscriptions I use most.

**The single feature that would make that answer materially stronger:** `replay
simulate --policy` against my own last 10,000 requests, so a cap or a model
allow-list is a number ("would have refused 212 requests in 9 sessions, $37 at
list, population: 10,000 requests over 31 days") before it is a risk. That is what
changes a cautious developer from "I will not turn that on" to "I will."

**The evidence that would prove it is worth paying for:** experiments 9 and 10
above, pre-registered: ten existing users, thirty days, the count who change or
enable a cap because of a simulation they ran, with a kill threshold of three.
Anything short of that is a feature people admire and do not buy, which is the
pattern the repository's revenue memo already documents.

---

## 11. Required final list

1. **FINAL VERDICT.** The commercial billing, capacity and routing plan is not
   justified as proposed. Billing survives only as ADR-0023; capacity survives as
   measurement with provenance; routing dies as execution and survives as
   admission with reasons.
2. **SURVIVING PRODUCT.** The certified instrument, plus a signed entitlement,
   plus `simulate --policy` and decision records, plus a Free capacity line.
3. **KILLED.** Accounts, orgs, memberships, subscription service, canonical
   entitlement service, RevenueCat, capacity balance and reservations, routing
   executor, translation, quality routing, premium preservation, credential store,
   entitlement coupled to forwarding.
4. **NARROWED.** Routing → admission + recommendation; capacity → five observed
   states; entitlement → four client states; identity → bind, never mint.
5. **CANONICAL ARCHITECTURE.** Sections C and D.
6. **SOURCE-OF-TRUTH MATRIX.** Section B.
7. **DATA MODEL.** Section E.
8. **STATE MACHINES.** Section F.
9. **SECURITY MODEL.** Pass 8.
10. **BILLING MODEL.** Pass 3 and section G.
11. **AI CAPACITY MODEL.** Pass 4 and sections D and H.
12. **ROUTING MODEL.** Pass 6 and section I.
13. **TDD / MUTATION PLAN.** Section 6.
14. **PRODUCTION GATES.** Section 7.
15. **COMMERCIAL FALSIFICATION.** Section 8.
16. **BUILD ORDER.** Section 9.
17. **EXACT PRD CHANGES.** Delete the Person/Account/Organization/Membership/
    Subscription hierarchy; replace with "Entitlement document (ADR-0023)" and a
    seller-side issuance appendix. Delete RevenueCat. Replace "Mr. Cheapo" with
    "capacity visibility with provenance" in Free and "admission with
    recommendation" in Pro; delete every routing sentence that implies execution or
    quality. Add the five capacity states and the UNKNOWN-is-not-free rule. Add the
    invariant that entitlement never changes forwarding. Add `simulate --policy` as
    the P0 paid candidate with its falsification experiment. Add the PDP to the
    RPL-C022 destination set as a prerequisite. Move "What-if the next 1,000
    requests" to "what would have happened to the last N," SIMULATED, never
    predicted.
18. **OPEN QUESTIONS.** Will any provider put capacity on the wire; does monthly
    issuance scale past a few hundred documents; will a single buyer run a
    simulation and change a cap; what the token size of a subscription window is
    (null result stands).
19. **THE SINGLE SMALLEST NEXT IMPLEMENTATION.** `replay simulate --policy
    <file> <ledger>` over the existing ledger for the existing spend cap: for every
    recorded request, evaluate the cap as the proxy would have, and print requests
    that would have been refused, sessions cut, list dollars involved and the
    population; RED through `dispatch`, a frozen mutant on the evaluation, black-box
    on the binary, and the ten-user experiment behind it. No new storage, no new
    network, no new secret.
