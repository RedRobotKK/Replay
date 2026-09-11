# Funding

Replay is free to run, under the Business Source License 1.1. Every command works, on every model, with no
account, no key and no network call, and that does not change. What is not
free is the measurement behind it: the corpus that makes its figures measured
rather than estimated is real API spend, and adding each new provider costs the
same again.

If the tool found money you had already paid for once, a share of that back is
what keeps it maintained.

## What building it cost

Replay was not written from documentation. Every figure in it was calibrated
against my own agent work, and that work is the cost.

Measured on 2026-09-07, by Replay, on the machine it was built on:

| | |
|---|---|
| Claude Code | 1,599 transcripts · 115 sessions · 32,188 requests |
| At list prices | **$3,107.57** |
| Median task | $0.63 · p90 $2.14 |
| Re-billed by broken caches | $152.92 (5%), 32.5M tokens |
| Codex | 610,551,532 tokens across 148 sessions |

**That $3,107.57 is not money I paid, and I am not going to pretend it is.** It
is what the same work would cost someone billed per token. I was on a
subscription, so what I actually paid was the monthly fee, month after month,
plus the time. Replay prints that same caveat every time it prints a dollar,
and a funding page that quietly dropped it would be the one place the tool's
own rule did not apply.

What the number does show is the size of the corpus. 32,188 real requests
across 115 sessions is why Replay's cache figures are measured rather than
estimated, and it is why the $152.92 line exists at all: the tool found five
percent of my own bill re-billed before it found anyone else's.

Adding each new provider costs the same again. The Codex support in this
release came out of 148 sessions and 610 million tokens of my own logs, and the
thing it found, that Codex rebases its own token counter on compaction and
hides 34% of the spend, is not in any documentation. It came out of the corpus.

## If it was useful

If Replay found something on your machine that you had already paid for,
a share of that back is what keeps it going:
[buymeacoffee.com/saitodaniel](https://buymeacoffee.com/saitodaniel)

There is now one optional paid thing, described under **Paid: the rules feed**
below. It is a subscription to *maintenance* of the price and cache table, not
a feature gate: the same table ships compiled into the binary, is published for
free at a stable URL, and stays complete. **Nothing in Replay is behind the
feed** — subscribing buys maintenance, never access, and an expired subscription
costs you a fresher table and nothing else.

Two promises that go with it, stated in full because the sentence this replaces
said "nothing in Replay is ever behind it" and read wider than the feed it meant:
**sponsorship gates nothing**, and **nothing that is free today ever becomes
paid**. See [SPONSORS.md](SPONSORS.md) and [the money path](docs/MONEY-PATH.md).

## Card

[buymeacoffee.com/saitodaniel](https://buymeacoffee.com/saitodaniel), coffees
at $5. The CLI suggests a number after a cost run, rounded to whole coffees
because that is what the page can actually charge.

## Crypto

**Read the network column.** The same token exists on several chains and they
are not interchangeable. Sending on the wrong one is how people lose money that
no amount of goodwill gets back, and none of these addresses can return it.

| Asset | Network | Address |
|---|---|---|
| USDC | Avalanche C-Chain | `0x585ef883e750694E4ba1463bc20820e9C4fBF369` |
| BTC | Bitcoin | `3HzfvNb1iKjeKsRMgMSttP1oqJzyHULhGu` |
| cbBTC | Base | `0xdaC0fCFa02b20aF55e6e34e931fB169a0C8Ddb98` |
| BTC | Solana | `F7XcHFFGe4uJUTrQJUELwfC4VzPYNvy9th1Yx3jVz6zc` |

## Paid: the rules feed

Every dollar figure Replay prints comes from a table of prices and cache floors
per model. That table is compiled into the binary, and it goes out of date on
the provider's schedule rather than on ours. Prices have moved several times a
year, and the cache read multiples that Replay's advice actually turns on move
with them.

Keeping it current is ongoing work. So it is sold, to agents, over
[x402](https://x402.org), which is HTTP 402 with machine-readable terms:

| | |
|---|---|
| Free feed | `https://redrobot.jp/Replay/rules/free.json` |
| Paid feed | `https://redrobot.jp/Replay/rules/latest.json` |
| Receiving wallet | `0x2733E9BE752848D578937fDB6029D7c739dc89Cb` (USDC on Base) |
| Asset | `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` (the USDC token contract on Base) |

That second row is not somewhere to send anything. It is the token the terms
name, and it is published for the same reason as the addresses: paying the right
address in a worthless lookalike token loses the money just as completely, so a
buyer should be able to check the asset against Circle's own published contract
before paying.

**Not selling yet.** The paid feed currently contains exactly the free feed, so
the endpoint refuses to quote a price for it and answers 503 instead. That
refusal is enforced by a comparison in code, not by a note in a document. It
lifts by itself once the feed carries something the free one does not.

**What the money buys.** Anyone can read a provider's price page. What they
cannot read is the part Replay measures: the minimum cacheable prefix each
model actually enforces, whether the documented figure survives contact with
real traffic, and the cache read multiple. Those carry a `documented` against
`observed` claim, from replaying a real corpus. The paid feed is that table kept
current; the free feed is the same table as it stands today.

**The free tier is permanently complete, and you can check that.** It is not a
sample or a trial. `replay rules --export` generates it from the binary, the
published file is that output, and the site repository's `npm run check:rules`
fails its build if the two ever differ. This repository's `scripts/preflight.sh`
runs the same comparison locally. Diff the two URLs and see exactly what the
money buys.

**Replay cannot pay, by design.** It holds no key and contains no code that can
sign a transaction. `cmd/replay/x402_test.go` fails the build if any appears.
Two reasons: a binary people pipe from `curl` onto machines holding provider
credentials must not also be a wallet, and paying is a decision rather than a
step. An agent with a funded wallet fetches the paid document itself and
installs it from a file:

```sh
replay rules --update ./rules.json
```

Pointing `--update` at a paid URL prints the terms and installs nothing:

```sh
replay rules --update https://redrobot.jp/Replay/rules/latest.json --x402-json
```

That emits the payment terms as JSON and exits **2**, distinct from 1, so a
spending policy can tell "this costs money" from "this is broken" without
parsing prose. The full reasoning is in
[ADR-0013](docs/adr/0013-x402-rules-feed.md).

## How these were checked, and what checking cannot cover

Each address was verified on 2026-09-05 with an implementation self-tested
against known-good vectors *before* it was trusted on these:

- The three EVM addresses carry an **EIP-55 checksum**. The mixed case is the
  checksum, not styling, so a lowercased copy is a weaker string. Both verified
  against the specification's own test vectors.
- The Bitcoin address is **Base58Check**, verified by double-SHA256 against a
  decoder first tested on known addresses. It is P2SH.
- The Solana address is a valid Base58 Ed25519 public key, 32 bytes, and lies on
  the curve, which rules out roughly half of possible typos.

**Solana addresses have no checksum.** Base58Check protects the Bitcoin address
and EIP-55 protects the EVM ones. That format has neither, so a single wrong
character can still decode to a valid-looking key nobody controls. It is the one
row here that a reader cannot verify for themselves, and it is worth a small
test transaction before a large one.

## Why there is a test pinning these

`cmd/replay/funding_address_test.go` pins every address. Changing one, in any
documentation file, fails the suite.

A payment address is the only string in this repository where one wrong
character sends money to a stranger with no recourse, and quietly altering one
inside an otherwise plausible pull request is a documented attack on
open-source projects. It works because a 42-character hex string is exactly what
a reviewer's eye slides over. The test puts any change somewhere a reviewer
will look, next to a comment explaining why it matters.

---

[Repository README](README.md)
