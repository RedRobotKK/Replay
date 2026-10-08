# External security review: provenance correction, scope, and reviewer brief, 2026-10-08

**This file does not commission or perform an external security review. No
mechanism to engage a third-party reviewer exists in this environment. This
file prepares the artifacts that make commissioning one, when Daniel does it,
fast and unambiguous: the exact scope, a ready-to-send brief, and the exact
evidence RELEASE-CRITERIA.md requires back.**

## 1. Provenance correction, re-verified rather than trusted

[`docs/evidence/security-review-2026-09-04.md`](security-review-2026-09-04.md)
opens: "An external reviewer read the code and then ran the proxy end-to-end
against a fake upstream." Re-checked today rather than taken on the strength
of its own first sentence:

```
$ git log --follow --diff-filter=A --format='%H %an %ae %ad' -- docs/evidence/security-review-2026-09-04.md
2e6a0615e9daa311055a5d5919074e90ddd9f128 Daniel Saito 1215165+saitodaniel@users.noreply.github.com Fri Sep 4 15:17:28 2026 -0700
```

The document was authored and committed by Daniel Saito. There is no second
party in the commit history, no reviewer identity, no engagement record, and
no independent account of what was tested. **This is not external security
review evidence, whatever its own first sentence says, and it is not treated
as closing that gate anywhere in this pass.** Every finding and fix it records
is real work and stays in the tree unmodified; what is corrected is the claim
about who performed it, which this file does not change (frozen evidence) and
instead documents the correction of in one place, here, for whoever reads
RELEASE-CRITERIA.md next.

RELEASE-CRITERIA.md's own "Where this stands today" table already states this
correctly as of 2026-09-13 ("An external security review, published | **Not
commissioned**") and nothing in this pass weakens or reverses that line.

## 2. What RELEASE-CRITERIA.md requires

> An external security review, published | **Not commissioned.** Needs a third
> party and has weeks of lead time

Two words carry the whole requirement: **external** (a party with no stake in
the finding being clean) and **published** (the report, not a summary of it,
reaches the same place the claim is made). A review commissioned and then
summarized in-house, or performed and not published, would still leave this
gate open.

## 3. Scope, derived from what this repository actually is

Not invented for this file: this is the attack surface RELEASE-CRITERIA.md's
own Security section and README.md's Footprint section already describe,
consolidated into one brief a reviewer can act on directly.

**In scope:**

1. **The local HTTP proxy** (`internal/proxy`), loopback-bound. `Host` header
   validation, `Origin`/`Sec-Fetch-Mode` browser guard, `/replay/healthz`
   (ungated by design), `/replay/status` and `/replay/metrics` (unauthenticated
   unless `--token` is set).
2. **Secret masking** (`internal/masking`), specifically: the entropy
   heuristic's actual false-negative rate against real secret shapes, the
   vault's AES encryption and its key-file placement (open finding 3: the key
   sits beside the ciphertext, bounded by a TTL since 2026-09-13 but not
   removed), and whether masking still fails open on any new code path since
   2026-09-10's fixes.
3. **The ledger** (`internal/ledger`), specifically: that no message content
   reaches disk under any transcript shape this build parses, that both
   halves of a record (request and response `call_key`) are HMAC'd and not
   reversible offline, and directory/file permission enforcement
   (`internal/ownerdir`).
4. **The OpenAI-compatible path** (`/v1/chat/completions`), labelled
   `EXPERIMENTAL, UNMASKED` — specifically whether that label is accurate and
   sufficient, and whether anything on that path silently does more than the
   label admits.
5. **The installer** (`install.sh`), specifically checksum verification
   (finding 5, fixed 2026-09-04: anchored `grep`, fail-closed) and the
   Windows refusal path (`cmd/replay/platformguard.go`).
6. **The self-update path** (`replay upgrade`), which fetches and executes a
   binary from `github.com` — the one outbound path in this tool whose
   compromise model is "attacker controls what gets executed," not merely
   "attacker reads what left the machine."
7. **This pass's own change**: `internal/cachemodel`'s new
   `RulesForModel`/`ClassifyBreakForModel` dispatch (see
   [RELEASE-CRITERIA.md](../../RELEASE-CRITERIA.md) and
   [docs/ROADMAP.md](../ROADMAP.md)) touches no secret-handling, network, or
   permission code; it is in scope only in the sense that any production
   change is, and a reviewer should not need to spend material time on it.

**Out of scope**, stated so a reviewer does not spend the engagement there:
the hosted hosted-service design (ADR-0028, not yet built), the Windows port
(refused on evidence, not shipped), replay.doctor's and redrobot.jp's
deployment (not in this repository), and anything requiring real OpenAI API
access (separately gated; see RELEASE-CRITERIA.md's second-provider row).

## 4. The threat model a reviewer is being asked to test against

Already stated candidly in `docs/evidence/security-review-2026-09-04.md`'s own
closing section and restated here because a reviewer brief should not require
opening a second file to find the thing being tested:

**Replay is designed to be safe to run on a machine you trust, on a loopback
interface, by one operator. It is explicitly not designed to be hardened
against a compromised host.** The vault's key-file placement is the clearest
instance: if the host is compromised, masking bounds the exposure window
(TTL) but does not prevent it, and the code says so. A reviewer should treat
"what does a compromised host gain" as a different, explicitly out-of-scope
question from "what does a hostile local process or a hostile remote peer
gain," and test the second, not the first.

## 5. Reviewer brief, ready to send

> Replay is a local CLI/proxy (Go, ~200 source files) that reads or
> intermediates LLM API traffic to measure prompt-cache behavior. It never
> stores message content and masks secrets in transit, on request. We are
> requesting a focused external security review ahead of a 1.0 release,
> scoped to: the local HTTP proxy's network-facing guards, the secret-masking
> path end-to-end (detection, vault encryption, key storage, TTL/eviction),
> the ledger's on-disk record format (content-free by design; we need you to
> try to break that), the self-update binary-fetch-and-execute path, and the
> installer's checksum verification. Threat model: a trusted single operator
> on a trusted host; a hostile local process or hostile LAN peer reaching the
> loopback listener; NOT a compromised host (out of scope by design, and
> already disclosed as such). Deliverable: a published report, in the
> reviewer's own name, independent of the maintainer, that we link from
> RELEASE-CRITERIA.md and docs/evidence/ without editing its substance.
> Existing internal findings (docs/evidence/security-review-2026-09-04.md) are
> provided as a starting map, not as a substitute for independent testing —
> please re-verify rather than take them on trust, the same standard this
> project holds itself to.

## 6. Exact evidence required to close the gate

RELEASE-CRITERIA.md's gate closes when, and only when, all of the following
exist:

1. A **published** report, reachable at a stable URL, authored in the
   reviewer's own name or firm's name — not summarized, not paraphrased into
   this repository's voice.
2. Independent **provenance**: the reviewer is identifiable and is not Daniel
   Saito, RedRobot K.K., or any account controlled by either (the exact defect
   this file exists to prevent recurring).
3. The report's own statement of what was actually tested (not merely read),
   matching or explicitly narrowing section 3's scope above.
4. For each finding: a severity, and either a fix landed in this repository
   with a commit reference, or an explicit, dated decision to accept the risk
   — the same discipline `docs/evidence/security-review-2026-09-04.md`
   already holds itself to for its own (internal) findings.
5. A link from `RELEASE-CRITERIA.md`'s Security section to the published
   report, replacing (not deleting — this project does not delete frozen
   evidence) the current "Not commissioned" status with the new one.

## 7. What this environment can and cannot do about it

This is a non-interactive coding session with no mechanism to contact,
engage, pay, or receive a report from a third party. It can prepare exactly
what sections 2 through 6 above contain, and did. It cannot commission the
review itself. **The dependency is Daniel's: find and engage a reviewer, using
section 5 as the brief, and section 6 as the acceptance criteria.** Nothing in
this repository is blocked on anything further from this environment.

---

[Evidence index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md) ·
[security-review-2026-09-04.md](security-review-2026-09-04.md)
