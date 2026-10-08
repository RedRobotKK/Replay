# GTM-02 re-verification and the v1.0 public-surface retrofit, 2026-10-08

**Independent re-verification of REPLAY-GTM-02's three P0 findings, done by
re-fetching the live surfaces rather than trusting the prior report, plus the
in-repository fixes that followed and the two findings that do not have an
in-repository fix.**

## 1. The banned forensic-week offer, re-verified live on replay.doctor

Fetched `https://replay.doctor` directly on 2026-10-08. It currently reads:

> "The week is the only thing for sale: you get the invoice annotated turn by
> turn... As of 2026-09-13 the week is $22,000, quoted at $18,000 to $25,000
> by scope." With a "Talk to Daniel" call to action.

This is exactly the offer [ADR-0028](../adr/0028-replay-is-a-product-with-a-hosted-service.md)
(2026-10-05) retired: "the fixed-fee forensic engagement is not offered and is
not to be proposed again." **Confirmed live, confirmed banned, confirmed not
fixed by this ADR's own landing** — ADR-0028 changed no public copy, only the
decision record.

**Also found, not in the original GTM-02 scope: the same offer was live in
this repository's own `README.md`**, under "What it costs" — "There is one
paid thing that does exist and it is not a tier: a week of the maintainer's
attention over your own corpus. Nobody has bought one." That sentence
directly contradicted the heading two paragraphs above it, "Nothing, and
nothing is for sale today," and predates ADR-0028 by three weeks. **Fixed in
this pass**: the paragraph is replaced with a one-line statement that the
offer is withdrawn, citing ADR-0028, so the page agrees with itself. No test
pinned the old text; `go test ./cmd/replay/... ./internal/regression/...`
stays green.

**Not fixed, and cannot be fixed from this repository:** replay.doctor's own
copy. No `site/`, `web/`, `landing/`, CMS config, or static-site generator
config exists anywhere in this tree (`find . -iname '*site*' -o -iname
'*landing*' -o -iname '*web*'`, checked, returns nothing relevant), and no
deployment config (`wrangler.*`, `netlify.*`, `vercel.*`, `*.toml`) exists
either. The live page is deployed from somewhere this agent has no access to.
**Corrected copy for whoever controls that deployment**, to replace the
pricing section verbatim:

> There is nothing paid today. Every command works, on every model, with no
> account and no key. If a paid capability is ever added it will be
> something that does not exist today.

Remove the "Talk to Daniel" forensic-week CTA and the $22,000 / $18,000 /
$25,000 figures entirely; they describe a withdrawn offer.

## 2. Replay vs. Replay Doctor: not two products being confused, one product under two live names

GTM-02 framed this as Replay (the repository) accidentally being confused
with a separate product called Replay Doctor. **That framing does not survive
a repository-wide check.** Evidence, read directly rather than assumed:

- `packaging/npm/package.json`: `"name": "replay-doctor"`, `"homepage":
  "https://replay.doctor"`. This is the packaging for *this* repository's own
  binary (`packaging/npm/bin/replay-doctor.js` wraps the same release
  artifact `cmd/replay` produces).
- The npm registry confirms it is live, not just configured: `replay-doctor`
  and its four platform packages (`@replay-doctor/darwin-arm64` etc.) are
  published, `dist-tags.latest = "0.8.0"`, with version history back to
  0.6.0. This is not reversible by a README edit.
- `plugins/replay/skills/replay-doctor/SKILL.md` is titled "# Replay Doctor"
  in its heading and switches to "Replay reads the transcripts..." in its own
  first body sentence — the same inconsistency, inside one first-party file.
- `plugins/replay/.claude-plugin/plugin.json` has `"name": "replay"` and a
  description starting "Replay Doctor: at the end of each session...".
- This repository's own `README.md`, title `# Replay`, described itself as
  "Replay Doctor" in its lead sentence and in its author attribution, until
  this pass.

**Conclusion: there is one product. It is published under two names across
different channels** — "Replay" on GitHub, in `go install`, and as the CLI
binary name (`replay`); "Replay Doctor" on npm and at the replay.doctor
domain, which is also the canonical public website. Neither name is a typo
and neither is unused; both are already live and at least one (the npm
package) is not cheaply renamed. **This is not a bug this session can fix by
picking a winner** — it is a real, open brand decision with external,
already-committed consequences on both sides, and making it unilaterally
would be exactly the kind of unauthorized scope expansion the operating
brief warns against.

**Fixed in this pass, narrowly:** `README.md`'s internal self-contradiction
(a document titled "Replay" calling itself "Replay Doctor" in its own first
sentence) is resolved by using "Replay" throughout, with one explicit
sentence noting the npm/web name and that it is the same binary. This does
not resolve the brand question; it stops the one file that unambiguously
contradicted its own title from doing so.

**Left open, named for Daniel:** which name is canonical going forward is a
strategic decision (SEO, the already-published npm package, the already-live
domain, the GitHub org name) and is recorded here as a decision this
repository's own evidence cannot make for him, not as a defect.

## 3. The headline figures: provenance traced, not reconciled

The public figures on replay.doctor and redrobot.jp/replay, re-read live on
2026-10-08: **$16,078.68 total, $631.77 re-billed (rounds to "4%" on the
page), 123 sessions, 2,176 agent lanes, 106.7M tokens, data read 2026-09-21,
build 0.6.2.**

A repository-wide search (`grep -rln "16,078\|16078.68\|631.77\|2,176\|106.7"
docs/ CHANGELOG.md`) finds **zero matches**. No dated evidence file in
`docs/evidence/` carries this total, this re-billed figure, this lane count,
or this token count.

The nearest reading of what appears to be the same growing, single-machine
corpus is
[`seeded-intervention-prereg-2026-09-25.md`](seeded-intervention-prereg-2026-09-25.md):
**123 sessions, 105.2M tokens, $620.47 re-billed, 4.18%, read 2026-09-17.**
Same session count, close but not identical token count, a different dollar
figure, a different percentage, and a date four days earlier than the
website's.

**What this can and cannot establish.** The operator's transcript corpus is
explicitly documented elsewhere as growing while it is read (`docs/ROADMAP.md`
row 1: "the corpus is this machine's own live transcript store, so it grows
while you read it"). A reading taken 2026-09-21 on a growing corpus differing
from one taken 2026-09-17 is not on its face contradictory — more data
between the two dates could explain a token-count increase. But:

- No frozen evidence file exists for the 2026-09-21 reading at all. It cannot
  be verified, only guessed at, and this project's own standing rule
  (`RELEASE-CRITERIA.md`: "no headline figure without the instrument that
  produced it being checked first") forbids publishing a figure nobody can
  check.
- The percentage moved the wrong direction for pure corpus growth to be the
  whole explanation (4.18% on 2026-09-17 vs. ~3.93% implied by $631.77 /
  $16,078.68 on 2026-09-21): if new sessions were added at roughly the
  corpus's existing re-billed rate, the percentage should hold roughly
  steady, not visibly drift, across four days. This is not proof of an
  error, but it is exactly the kind of drift this project's own doctrine
  says must be checked before publication, and it was not.
- This machine (this git worktree) cannot regenerate the 2026-09-21 reading:
  it would require the operator's own, larger, growing transcript store as
  it stood on that date, which is explicitly outside this repository
  (`~/replay-experiment/` and the operator's live transcript directories are
  protected paths this session does not touch) and cannot be reconstructed
  after the fact.

**Disposition, per the rule this project already states for exactly this
situation ("if provenance cannot be established, remove the unsupported
precision from public claims rather than inventing a reconciliation"):**

The $16,078.68 / $631.77 / 2,176-lane / 106.7M-token / 2026-09-21 figure set
is **UNSUPPORTED** — it has no corresponding evidence file and cannot be
independently checked — and should be removed from public surfaces. The
$620.47 / 4.18% / 123-session / 105.2M-token / 2026-09-17 figure set is
**HISTORICAL**, sourced to a frozen, dated, checkable evidence file, and may
continue to be published as what it is: a dated reading of one operator's
corpus, not a current or growing total.

**Corrected copy for replay.doctor and redrobot.jp/replay**, to replace the
current headline statistics block:

> On one operator's own corpus, read 2026-09-17: 123 sessions, 105.2M tokens,
> $620.47 re-billed at write prices (4.18%). One machine, one account — not a
> population estimate. [Full reading](https://github.com/RedRobotKK/Replay/blob/main/docs/evidence/seeded-intervention-prereg-2026-09-25.md).

This is a decrease in precision and in the size of the headline dollar
figure from what is live today. That is the correct direction: the larger,
more recent figure is the one with no evidence behind it.

**Not fixed, and cannot be fixed from this repository:** this is the same
external-deployment limitation as finding 1. The corrected copy above is
prepared; it is not live anywhere.

## 4. What this leaves for the release gate

GTM/PUBLIC SURFACES is not closable to GREEN from this repository alone.
Two of its three findings require someone with access to replay.doctor's and
redrobot.jp's actual deployment to publish the corrected copy above. The
third (the Replay/Replay Doctor identity question) requires a naming
decision only the operator can make. The in-repository half of each finding
(the README self-contradiction, the stale forensic-week offer in README.md)
is fixed and verified green in this pass.

---

[Evidence index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md) ·
[ADR-0028](../adr/0028-replay-is-a-product-with-a-hosted-service.md) ·
[seeded-intervention-prereg-2026-09-25.md](seeded-intervention-prereg-2026-09-25.md)
