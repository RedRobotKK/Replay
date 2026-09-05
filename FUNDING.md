# Funding

Replay is free, Apache-2.0, and has no paid tier. What is not free is the
measurement behind it: the corpus that makes its figures measured rather than
estimated is real API spend, and adding each new provider costs the same again.

If the tool found money you had already paid for once, a share of that back is
what keeps it maintained.

## Card

[buymeacoffee.com/saitodaniel](https://buymeacoffee.com/saitodaniel) — coffees
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

## How these were checked, and what checking cannot cover

Each address was verified on 2026-09-05 with an implementation self-tested
against known-good vectors *before* it was trusted on these:

- The two EVM addresses carry an **EIP-55 checksum** — the mixed case is the
  checksum, not styling, so a lowercased copy is a weaker string. Both verified
  against the specification's own test vectors.
- The Bitcoin address is **Base58Check**, verified by double-SHA256 against a
  decoder first tested on known addresses. It is P2SH.
- The Solana address is a valid Base58 Ed25519 public key, 32 bytes, and lies on
  the curve — which rules out roughly half of possible typos.

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
