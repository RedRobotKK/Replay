# ADR-0011: Opt-in request rewriting

- **Status:** Proposed
- **Date:** 2026-09-04
- **Related:** [`architecture/rewriting-prompts.md`](../architecture/rewriting-prompts.md)

## Context

Daniel asked for rewriting as an opt-in feature after I argued against it. Opt-in genuinely answers
two of the four objections and does not answer the other two, so the design has to.

| Objection | Does opt-in solve it? |
|---|---|
| Breaks "forwards byte for byte" | **Yes**, the claim becomes "by default", which is already true of `--mask` and `--context-edit-trigger` |
| A model in the request path costs the 48µs headline | **Yes**, you only pay it if you turn it on |
| No verification path | **No.** Has to be designed |
| Cannot fail open | **No.** Has to be designed |

Every risky feature here is already opt-in and off by default, pinned per session and logged with
body hashes. **The pattern exists; this has to fit it rather than invent a new one.**

## Decision

**Ship it in three stages, and never ship stage 3 without the evidence stage 2 produces.**

### Stage 1: offline only. `replay rewrite <transcript>` sends nothing

Reads a session that already happened, prints what it *would* have changed and what the change would
have saved, and exits. **No network, no proxy, no risk.** Same shape as `replay` itself: score the
counterfactual before touching anything.

**This is where most of the value is**, and it is worth building even if stages 2 and 3 never ship.

### Stage 2: deterministic transforms only, live, off by default

`--rewrite` accepts named transforms and **nothing else**. Each must be **semantics-preserving**:
byte-identical content reaching the model, arranged so the cache works.

- `stable-tool-order` — a reordered tool list breaks the prefix. Restoring the previous order is
  provably identical content.
- `breakpoint-on-stable-block` — placing a `cache_control` marker. Metadata only; already on the
  roadmap and unbuilt.
- `hoist-stable-prefix` — moving content that never changes ahead of content that does.

**No model is involved. Nothing is reworded.** These are the transforms whose output can be proven
equal to their input, and they capture most of the available win because **cache breaks are mostly
caused by instability, not by wording.**

Fail-open is straightforward here: if a transform errors, forward the original bytes. That is the
same rule masking and the policy path already follow.

### Stage 3: a user-supplied rewriter, gated on evidence from stage 2

`--rewrite-command <cmd>` pipes the request body through a command **the user chose**. If they want a
model, they bring it, pay for it and accept its latency. **Replay ships no rewriter and embeds no
model**, so the 48µs figure stays true of Replay and the user owns whatever they added.

**And this is the answer to the verification objection.** Replay cannot measure answer quality, but
it can measure whether a rewrite made things *worse* in ways it already tracks:

- **retries** per session (`record.go:57`, aggregated at `state.go:137`)
- **error share**: failed tools, failed edits, repeated identical calls
- **re-read rate**, which is the existing guardrail metric

So stage 3 runs as a **bounded trial with controls**, exactly like the context-edit policy:
`--trial-share` treats a share of new sessions by stable hash and holds the rest out, and a guardrail
reverts for new sessions when treated sessions' rework rate exceeds the controls' by a margin.

**That is not proof the rewrite helped. It is detection that it hurt**, which is the honest half and
is the same standard `replay learn` already holds itself to.

## Constraints that do not move

- **Off by default, at every stage.** Nothing rewrites unless a flag says so.
- **Pinned per session.** A session keeps its first decision, so a config change mid-session cannot
  make a conversation incoherent.
- **Body hashes before and after, on the ledger**, as context-edit already does. **Never the content.**
- **`--rewrite` and `--rewrite-command` must appear in `doctor`** when active, because a user
  debugging strange behaviour has to be told the proxy is changing their request.
- **The README claim becomes explicit**: forwards byte for byte *unless you turned on masking, a
  context-edit policy, or rewriting*, each named.

## Consequences

**Good.** The genuinely valuable transforms, the deterministic ones, ship with every guarantee
intact. The user who wants a model gets a documented seam rather than a fork. And the offline command
delivers the insight without anyone taking a risk.

**Costs.** `--rewrite-command` is a new execution surface: `os/exec` appears in a codebase that
currently has none, and `docs/SURFACES.md` records "no `exec.Command` anywhere" as a checkable
property. **That property dies the day stage 3 ships**, and it should be named as the price rather
than discovered later.

**The honest risk.** A user pipes their request bodies through a command of their choosing. If that
command is careless, their prompts leave the machine and Replay cannot tell. The flag help has to say
so in those words.
