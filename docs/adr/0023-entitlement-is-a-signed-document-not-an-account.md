# 23. Entitlement is a signed document, not an account

**Status:** Proposed
**Date:** 2026-09-13

## Context

The recurring commercial proposal is some arrangement of: a hosted server, an
account bound to a user name, a sign-up with an opt-in, and a subscription the
server checks before the tool works. It is the shape almost every developer tool
has, so it is the shape people reach for, and asking for it is reasonable.

[`../MONEY-PATH.md`](../MONEY-PATH.md) section 4 designs the alternative and then
says, correctly, that the design should be recorded as a decision before anything
is built on it rather than inferred from two adjacent ADRs. This is that record.

**Three published promises constrain the answer, and they were made deliberately.**

- [`../../README.md`](../../README.md) states local operation with no account and
  no network call as a footprint guarantee. `internal/observation` cannot import
  `net/http`, and `TestO7_ThisPackageCannotSend` walks the imports to keep it
  that way.
- [`../../SPONSORS.md`](../../SPONSORS.md): nothing free today ever becomes paid,
  and a paid capability will be something that does not exist today.
- [ADR-0015](0015-single-tenant-state-is-a-boundary.md): a multi-tenant Replay
  must introduce a tenant dimension into the spend guard, the session table, the
  metrics surface and the credential path **before** any commercial layer is
  built on them, and must not present a shared deployment as a configuration of
  the local one.

There is also a fact about the product rather than the promises. The asset is
that every figure says how it was obtained. A measurement behind a paywall is
unfalsifiable to the person who has not paid, which is the same as not making the
claim. So the paywall cannot sit in front of a measurement, and an account that
the tool must reach to work puts a network dependency in the credential path of a
binary people install with `curl`.

## Decision

**Entitlement is a signed document the user installs from a local file. It is
verified offline against a public key compiled into the binary and expires on a
date read from the local clock. There is no licence server, no account, no
callback, and no identifier that leaves the machine.**

Four consequences follow, and each answers one part of the usual proposal.

**A user name is not the binding, because it cannot be.** An entitlement names
what was bought and when it lapses. It does not need to name a person, and naming
one would create the only piece of personal data this tool has ever held. What
binds a document to a buyer is the transaction that produced it, which lives with
whoever took the money and never reaches the binary.

**Opt-in already exists and stays where it is.** The corpus contribution is opt-in
today, gated on a consent file the user writes themselves, and it sends nothing:
`cost --contribute` writes a file and a person moves it. Entitlement does not
touch that path and must not become a reason to add one.

**Client and server already exist, split along the line that matters.** There are
two MCP servers and the split is not an accident of implementation:

| | Hosted, `redrobot.jp/mcp.json` | Local, stdio |
|---|---|---|
| Answers questions about | the world: what a model costs, what its cache floor is, what a tool set weighs | this machine: what you have spent, what Replay can see here |
| Needs user data | none, which is why it can be hosted at all | all of it, which is why it can never be |
| Can be sold | yes | no |

Anything sellable as a service is on the left. That is a small surface on
purpose, and it is the whole of the legitimate hosted product.

**The subscription is a repository line item, not a seat.** `MONEY-PATH.md`
carries the arithmetic: at the measured 2 to 3 percent realistic recovery, a seat
needs roughly $1,000 a month of metered token spend before $25 a month closes,
and for a flat-seat user the recoverable figure is zero dollars with a published
null result behind it. Per repository closes without asking anyone to believe a
savings claim.

## The blocker, which was already removed, and where the line actually belongs

**Corrected 2026-09-13, the day this ADR was written, after a review found it.**
The paragraph here originally said that `crypto/ed25519` was banned and that
this ADR was "the conversation the list exists to force." That conversation had
already happened. `crypto/ed25519` was allowlisted for `internal/feed` in
`c69c875` on **2026-09-07**, six days earlier, for verifying a signed vendor
feed, and the blanket ban was replaced by `TestFeedVerifiesAndNeverSigns`, which
reads the tree for construction paths rather than trusting the import list.

So this ADR opened by describing a blocker that was not there. It is worth
saying plainly why that happened rather than quietly deleting it: the claim came
from `MONEY-PATH.md` section 4, which was written before the change and never
re-read against the code, and I repeated it because it was in a document rather
than because I had looked. **A published document is not evidence about the
tree, and this project spends most of its time finding exactly that mistake in
other people's numbers.**

What survives is the reasoning, and it is unchanged: verifying a signature does
not move money, signing does, so the line belongs at the operation rather than
the package. `TestX402_AllowlistIsMeaningful` still bans `crypto/ecdsa`,
`crypto/elliptic`, `crypto/ecdh` and `math/big`, and those are still the right
bans.

**One objection this ADR must not leave unanswered.** The guard is a file walk
asserting that `ed25519.Sign` and `ed25519.GenerateKey` appear in no non-test
file. That is a syntactic control on a semantic property, and it is defeated by
`var sign = ed25519.Sign`, by any value of type `crypto.Signer`, or by an
ordinary refactor that nobody intended as an evasion. A check that can be made
not to fail by accident is the defect class ADR-0014 exists to name. **Keep the
property and change the mechanism: assert on the built binary's symbol table
rather than on the source AST.** That is unbuilt, and it is a precondition for
the entitlement key existing rather than a nice-to-have.

**An HMAC token is rejected explicitly.** A symmetric check ships the
verification key inside the binary, so anyone who reads it can mint entitlements.
That is not a weaker fence. It is a fence with the gate drawn on it.

The entitlement signing key lives off the build machine and off CI, and it signs
entitlements only. Release signing is separate and already exists: Sigstore
keyless through cosign over `checksums.txt`, which `install.sh` dies rather than
warns on.

## What this decision does not authorise

Recorded so it is not read as permission.

- **No hosted service that receives transcripts, ledgers or per-session data.**
  The hosted MCP answers questions about the world. If a proposal needs the
  user's data to reach a server, it is the local server's job and the answer is
  no.
- **No multi-tenant deployment before ADR-0015's tenant dimension exists.** SP-5,
  SP-6 and SP-8 are specified and unbuilt for that reason.
- **No entitlement check in a request path.** `replay serve` sits in a credential
  path, and a payment check there is a new failure mode in the worst possible
  place. Entitlement gates capabilities that do not exist today, never the proxy.
- **No gating of anything that works in a release someone already has.**

## Consequences

The rail stops mattering, which is the point. Stripe, an invoice paid by bank
transfer, GitHub Sponsors and the existing x402 endpoint are identical to the
tool: each produces a signed file by whatever means, and the file is what the
binary reads. An enterprise that cannot pay a crypto rail and a machine agent
that can pay nothing else are served by the same mechanism.

Offline verification also means an air-gapped buyer is a supported case rather
than an exception, which is usually the hardest thing to retrofit and is free
here.

The cost is that a lapsed entitlement cannot be revoked remotely. That is
accepted: the expiry date is in the document, the document is short-lived, and
remote revocation would require the callback this decision exists to avoid.

---

[ADR index](README.md) · [The money path](../MONEY-PATH.md) · [Roadmap](../ROADMAP.md)
