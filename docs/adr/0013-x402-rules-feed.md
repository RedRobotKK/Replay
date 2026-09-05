# 0013. Sell a maintained rules feed over x402, and never hold a key to do it

**Status:** Accepted
**Date:** 2026-09-05

## Context

Every dollar Replay prints comes from a price table compiled into the binary and
dated by `PriceTableVersion`. On the day this was written that table was **73
days old**, and until `rules --check-prices` shipped the same day, the only thing
the tool could say about it was how old it was.

That is the product. Not the analysis — the analysis is commoditised and
[ADR-0005](0005-apache-2-license.md) said so. What decays without ongoing work,
and what nobody else is doing, is **keeping a dated, verified statement of what a
provider actually charges and how its cache actually behaves.**

The corpus already found one disagreement worth money: the published minimum
cacheable prefix is 512 tokens and observation bounds it at at most 40,563. A
rules document that carries `documented` beside `observed`, and marks the field
`contradicted` when they disagree, is a thing a person would pay for and a thing
that requires somebody to keep looking.

**x402** — HTTP 402 with machine-readable payment terms, now a Linux Foundation
protocol with Anthropic, Cloudflare, Stripe and Visa in it — is the only payment
rail an agent can complete without a human. That matters here specifically: the
consumer of a rules document is `replay cost` running unattended inside somebody
else's agent loop, at 3am, with nobody to click a checkout.

Its adoption is also, honestly, tiny: ~$28K/day of real commerce across 100,000
sellers, roughly half of the activity gamified. **This is a bet on the rail
existing later, not a revenue plan for this quarter**, and it is recorded as one.

## Decision

**Replay sells a maintained rules feed. It does not sell the tool, the analysis,
or any capability the tool already has.**

### What stays free, permanently

The compiled rules ship in the binary and are complete. Every command works with
them. `rules --check-prices` compares them against a public database at no
charge. **A user who never pays anything gets a tool that is honest about the age
of its own numbers and fully functional.**

The free tier is not a trial and is not degraded. If it ever becomes one, the
argument for the paid tier collapses with it, because the paid tier's value is
freshness and freshness is worthless if the base is broken.

### What is sold

A dated rules document, per provider, that is:

- **re-verified continuously**, not on release cadence
- **corrected within hours** of a provider changing a published number
- carrying **`documented` against `observed`** per field, with a status of
  `untested`, `unverified`, `consistent` or `contradicted`
- carrying the **cache heuristics** the corpus measures and nobody publishes:
  minimum cacheable prefix bounds, block granularity, read multiples, TTL
  behaviour

The last line is the one worth paying for. Prices can be scraped. **The
observed-versus-documented column can only be produced by replaying real
traffic, which is what this project does and what a scraper cannot.**

### Replay never holds a key and never signs a transaction

This is the part that is not negotiable.

The binary understands a 402: it reads the payment terms, reports them as
structured data, and stops. **It does not pay.** It ships no wallet, stores no
key, and has no code path that can move money.

Two reasons, and the second is the load-bearing one.

A tool that signs transactions is a tool whose supply chain is a wallet
compromise. Replay is a single binary people `curl | sh` onto machines that hold
provider credentials; adding key custody to that is a category of risk the
project has no business taking on for a rules document.

And **paying is a decision, not a step.** x402's model is that an agent pays from
a wallet its operator funded and budgeted. That operator authorised the agent,
not Replay. Replay's job is to make the terms legible so the agent's own policy
can decide — the same line drawn everywhere else in this project: report, and
let the person or their agent choose.

## Consequences

- **The service is a separate work**, as [ADR-0012](0012-dual-licensing-deferred.md)
  required. The binary stays Apache 2.0 with nothing gated.
- **A paid feed must never become a hostage.** If the service disappears, every
  installation keeps working on compiled rules. That is why the free tier is
  complete rather than crippled, and it is a constraint on the business, not a
  courtesy.
- **The observed column is the moat and the obligation.** Selling it means
  continuing to measure. A feed that stops being verified is fraud with a
  subscription attached.
- **Trust runs the wrong way for a paid feed.** A document that sets prices is a
  document that can misprice. Every fetched feed is validated by the same strict
  loader as a free one — unknown schema, missing version, empty match, negative
  price, or a read multiple outside 0 to 1 are all refused — and the installed
  version is named in every report that used it.
- **Revenue is not expected soon.** The rail is early and Replay has no users
  yet. This exists so the shape is decided before either changes.

## What was built

Implemented 2026-09-05 and verified end to end against a stub facilitator.

**Buyer side, in this repository.** `replay rules --update <url>` handles a
`402` by parsing the terms, printing them, and installing nothing. `--x402-json`
emits them as JSON with an explicit `"paid": false`, and the command exits `2`
rather than `1` so a spending policy can distinguish "this costs money" from
"this is broken". `internal/cachemodel/x402.go` reads terms and cannot do
anything else; there is no code path that constructs a payment.

`replay rules --export` writes the compiled table as an installable document.
This is what makes the free tier's completeness checkable: the published free
feed *is* that output, and both `scripts/preflight.sh` and the site's
`npm run check:rules` fail when the two differ.

**Seller side, in the site repository.** `src/lib/x402.ts` quotes terms and asks
a facilitator to verify a presented payment; `/Replay/rules/latest.json` is the
paid resource and `/Replay/rules/free.json` the free one. Three properties it
keeps, each with a test that was made to fail before it was trusted:

- Only a strict `true` from the facilitator opens the gate. A truthy value is
  not consent, and treating one as consent makes the paywall free.
- A facilitator that is unreachable, erroring or returning non-JSON is **our**
  fault, answered `502`. Reporting our outage as `402` tells a buyer their good
  payment was rejected and invites them to pay again.
- A half-configured seller is not a seller. Any missing setting answers `503`
  and points at the free feed, because a quote naming a payee we do not control
  loses the buyer's money.

**Configuration.** The seller reads five values from the environment, and quotes
nothing unless all five are present:

| Variable | Value |
|---|---|
| `X402_PAY_TO` | `0x2733E9BE752848D578937fDB6029D7c739dc89Cb` |
| `X402_NETWORK` | `base` |
| `X402_ASSET` | USDC on Base, `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` |
| `X402_FACILITATOR` | facilitator base URL |
| `X402_PRICE_ATOMIC` | price per fetch, in atomic units (USDC has 6 decimals, so $2.50 is `2500000`) |

The receiving wallet is deliberately separate from the donation addresses in
[FUNDING.md](../../FUNDING.md): donations are gifts and this takes payment for a
product, so keeping them apart keeps the accounting honest and lets either be
rotated without disturbing the other. Both are pinned by
`cmd/replay/funding_address_test.go`, because a payment address is the one
string in this repository where a single wrong character sends money to a
stranger with no recourse.

**Verify is not settle, and this was got wrong first.** The initial
implementation called the facilitator's `/verify` and served the document on a
positive answer. `/verify` is a dry run: it checks that the buyer's signed
authorization *would* succeed and broadcasts nothing. Only `/settle` submits
the transfer. A seller that verifies and serves has given the document away —
the buyer keeps their funds, the seller is never paid, and every sale is a
silent giveaway. Worse, the response carried a hardcoded `settled: true` that
no code path could justify.

The order is now verify, then settle, then serve, and nothing is served on a
settlement that did not confirm. Settlement also consumes the authorization's
on-chain nonce, which is what stops one captured `X-PAYMENT` header buying an
unlimited number of fetches.

**The feed refuses to sell a document identical to the free one.** Today the
paid document *is* the free one, because the corpus-backed `observed` claims
are not published yet. Rather than quote a price for something the buyer
already has free at a published URL, `differsFromFree` compares the two and
answers 503. It is a function rather than a comment because the previous
version documented the same situation in prose and quoted a price anyway. When
a genuinely maintained feed exists, the guard returns true on its own and the
endpoint starts selling.

**What is not built.** Settlement is delegated entirely to the facilitator; the
edge asks for a verdict and never watches a chain itself. There is no
replay cache beyond the on-chain nonce, so a buyer who pipelines concurrent
requests inside a settlement window is bounded by the chain rather than by us.

## Alternatives considered

**Charge for the tool.** Rejected by ADR-0005 and again by
[ADR-0012](0012-dual-licensing-deferred.md). Nothing has changed.

**A subscription with accounts and API keys.** Rejected: it needs a human to sign
up, which is exactly the consumer this cannot reach, and it means holding
customer records for a project whose main claim is that it stores nothing.

**Let Replay pay automatically from a configured wallet.** Rejected above. The
convenience is real and the risk is a binary with key custody, distributed by
`curl | sh`.

**Do nothing.** Viable, and nearly chosen. The reason not to is that the price
table's staleness is a real defect with a real cost to users, and fixing it
properly is ongoing work that has to be paid for by somebody.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
