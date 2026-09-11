# Gap analysis, 2026-09-07

What a day of measuring this product and its site turned up, consolidated so it
does not have to be re-derived. Every item is either VERIFIED here or marked as
needing a check. Fixed items are kept with their evidence, because what was wrong
is worth more than a shorter list.

## The pattern, stated once

Fourteen defects were found today and **six were the same shape**: a check that
reports success where success and failure are indistinguishable.

- `failclosed_test.go` drove `Masker.Mask`, the layer that was already correct,
  while `Server.mask` discarded the fix and put a credential on the wire.
- `TestGuideCoversEveryCommand` derived its command list by parsing `--help`, so
  a command missing from help was invisible to the test whose job was to notice
  commands missing from help.
- A mutation test's fixture ordered two lanes `[late, early]`, so "keep earliest"
  and "keep last" returned the same answer.
- `TestSharePNGCarriesNoSpendTotal` passed a literal instead of calling the
  function it named.
- A path check was satisfied by `t.TempDir()`, which is already absolute.
- `TestTW3` asserted no trailing whitespace against output that had already been
  trimmed.

None was found by reading. All six were found by mutating the code and watching
the test stay green.

## Fixed today

| | Evidence |
|---|---|
| Masking failed open, forwarding a credential | Reproduced against a live proxy; `Mask` returns the blind-scrubbed body and the caller discarded it. Fixed in 48dba3f |
| `SURFACES.md` claimed "No `filepath.Walk`" in a security document | Seven call sites in non-test code. Corrected |
| `cost` counted agent lanes and called them tasks | 1,613 rows over 114 ids; schema bumped to v2 |
| `CONTRIBUTING.md` told contributors their work was Apache 2.0 "and there is no plan to change it" | BUSL has no inbound clause; the grant is in CLA.md |
| The site said Apache 2.0 in twelve places while the installer it served printed BUSL 1.1 | Corrected and deployed |
| "The 26 installs that exist" | A fetch count wearing the word installs. Measured: 53 fetches from 2 IPs |
| `replay tui --help` exited 1 to stderr | The defect `help.go` exists to prevent, in the newest command |
| The source block panicked on a hand-edited file | Slice bounds, on the one file users are told to hand-edit |
| Retention set to 365 days on a 98%-full disk | My error. Corrected to 120, derived from free space |

## Open, product

- **The released binary prices from a 75-day-old table.** The 2026-09-07 table is
  in the repo and has not shipped. Two-line release. (#15)
- **Grok is unread and the doc dismissing it is wrong.** `~/.grok/sessions` is
  3.80 GB with per-turn `inputTokens`, `cachedReadTokens` and `costUsdTicks` —
  the only local surface reporting a cost figure at all. (#17)
- **`diff` emits no request id**, so its cause cannot be joined to the provider's
  own `cache_miss_reason`. That field is a MEASURED deficit against Replay's
  estimate, and a free calibration source. (#19)
- **Parsing is 94.6% of `cost --per-task`** and `cost.go` reads none of the
  message bodies the parser retains. A usage-only path measured 2.4x. (#9)
- **`TestKillMatrix` reports a stillborn baseline** that cannot be reproduced by
  hand. Until explained, the kill matrix cannot be trusted. (#8)
- **Three security findings** stand: install.sh skips cosign silently when
  absent, `HashedPathLabel` keeps the file extension in clear so `.env` reads
  are visible, and `serve` accepts any upstream URL without the warning
  `doctor` gives. (#14) **Two closed 2026-09-10:** `call_key` is now HMAC'd on
  the response half as well as the request half (`ledger/store.go`), and the
  two key files are checked on every open by `internal/ownerdir` — mode
  verified and tightened, and an unreadable key refused before
  `loadOrCreateKey` can decide to generate a replacement.
- **Four of eight TUI screens still carry example data.** Honest on screen,
  but it bounds what can be shown.
- **The proxy publishes no per-lane cost**, and the lane path has never run with
  real fan-out traffic. 17 ledger records, zero carrying an agent id.
- **Cursor is permanently unreadable** and should be recorded as a closed
  question rather than a backlog item: 29,665 `tokenCount` rows are byte-identical
  zeros, and the same database holds live credentials.

## Open, site

- **The hosted MCP endpoint now advertises the smaller server.** Five tools
  against the local seven. Copy needs to say it is the no-install version.
- **Three different corpus totals are published**: the page says $2,976.12,
  `WHAT-YOU-GET.md` says $3,018.99, today's run says $3,172.41. All true when
  written. The fix is to generate the figure, not to pick one.
- **"Open source" survives in roughly ten places.** BUSL is not OSI open source
  and the repo already says source-available. The hero was corrected; the rest
  is a positioning decision, not a typo.
- **`og-card.svg`**, the non-Replay card, was never checked for the same stale
  claims its sibling carried.

## Open, evidence

- **`wire-families-2026-09-06.md` is wrong** about Grok and builds a conclusion
  on it. (#17)
- **`SURFACES.md` says "one cache model, four readers."** Six agent surfaces are
  present on this machine and two of the unread ones carry usage.
- **Verify that published break-cause figures were not computed on a subset.**
  My own scan tonight covered 123 of 1,675 files and inverted the headline;
  `replay diff` uses the correct walk, so this is a check rather than a claim.

## Open, distribution

Zero acts in five days. One star, 53 install fetches from 2 unique IPs, no
external user ever observed, against ccusage at 18,407 stars reading the same
files. The finding post is drafted and needs rewriting in the author's own words;
`social/launch/show-hn.md` carries retracted figures behind a STOP header.

Every panel today reached the same conclusion independently: **publish a finding,
not a tool.** On that channel the finding scored 706 points and the tools scored
112, 81 and 20.

---

[Documentation index](README.md) · [Repository README](../README.md)
