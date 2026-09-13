# Roadmap

What ships in each release, the gate for each, and what is deliberately deferred. The requirements behind every line are in [`requirements.md`](requirements.md); the decisions are in [`adr/`](adr/README.md).

The sequence is chosen so the first release costs nothing to run, works for every user regardless of how they authenticate, and produces the output that gets the project noticed. Items are **proposed** until the owner approves PRD v5.

## Gating spikes (before any public claim)

| Spike | Question | Pass condition |
|-------|----------|----------------|
| 1 | Do Claude Code transcripts carry per-message usage with cache read and write counts? | Present on 20 real sessions across two client versions. **Status:** confirmed, on many times the twenty sessions the gate asks for, across two client versions. The count is deliberately not quoted here. The corpus is this machine's own live transcript store, so it grows while you read it, and this row has already carried a withdrawn figure twice — see the [2026-09-06 correction](evidence/calibration-corpus-2026-09-06.md) for why a transcript count is not a session count, and [`evidence/`](evidence/README.md) for readings dated by the run that produced them. **Independence is not met, and no session count fixes it**: every session comes from one machine, one account and one operator |
| 2 | Does the replay engine reproduce as-run cache reads and writes? | At least 95 percent of turns across 20 sessions, mismatches explained. **Status:** met, with margin, and every transcript below the threshold is listed rather than dropped. Neither the rate nor the sample is quoted here, because both move: the corpus grows daily and the engine changes under it, so a figure carrying no read-date is a snapshot presented as a standing fact — which is exactly how this row came to hold a withdrawn number twice. [`evidence/`](evidence/README.md) carries each reading with the date it was taken on. **Independence is not met, and no session count fixes it**: every session comes from one machine, one account and one operator |
| 3 | Does Claude Code honor a base URL override under subscription authentication, within the provider's terms? | Documented answer with a source. **Status: passed.** The LLM gateway docs state that `ANTHROPIC_BASE_URL` without a gateway credential keeps the subscription login active and routes requests through the gateway; a local gateway is a documented configuration. See [`architecture/proxy-protocol.md`](architecture/proxy-protocol.md) |
| 4 | Does adding the context-editing parameter from a proxy leave Claude Code's behavior intact? | A ten-turn session completes with the parameter present. **Status: passed 2026-09-05** ([evidence](evidence/spike-4-real-provider-2026-09-05.md)); the provider applied zero edits on that session, so the parameter is accepted and not yet shown to do anything |
| 5 | Does scoped rehydration hold under adversarial content? | Adversarial corpus never reaches a shell or network tool input |

## v0.1: replay, blame, diff (offline)

Read Claude Code transcripts. Reproduce the provider's caching turn by turn, print the calibration line, then score alternative layouts and rank token sources. Estimated tier only; Anthropic rules; macOS and Linux. (Windows was listed until 2026-09-06 and has never been tested; see the README.)

**Status:** shipped, and the offline path is where most of 2026-09-06 went. Bare `replay` leads with the cost report over the transcript root it discovers, rather than a list of sixteen commands with the one that reports money eleventh; `cost` and `corpus` take that same root as their default; and `--help` is grouped and ranked by value. The cost report states the avoidable figure in tokens as well as dollars and names who the dollars are for, because most readers hold a flat seat and a dollar-only finding is addressed to a minority. It discloses transcript overlap instead of implying the total is exact, and it indexes rather than reparsing — the report over 1,483 transcripts cost 6.3s wall and 19.6s CPU, nearly all of it reparsing unchanged files. Compaction is parsed, so `replay context` now reports by how much its own attribution overstates a session that compacted; across this corpus the client wrote 39 such records at a median retention of 2.55%. `replay route --to` charges for the switch itself rather than comparing two per-turn rates. What breaks the cache is now measured across the whole corpus rather than a sample ([break causes](evidence/break-causes-2026-09-06.md)).

**Gate:** spikes 1 and 2 pass; every output carries tier, calibration, and assumption lines; README shows real output from the maintainer's own sessions.

## v0.2: transparent proxy

`replay serve`: loopback listener, byte-for-byte passthrough for the Anthropic Messages API with streaming, usage capture into a derived-data ledger, measured tier for `replay`, `blame`, and `diff`. No policies yet.

**Status: shipped in v0.2.0.** Implemented for the Anthropic Messages API and **verified against the real provider** on 2026-09-05: a ten-turn session, all 200, 1,816,417 prompt tokens captured from provider usage, zero credential strings and zero message content in the ledger ([spike 4](evidence/spike-4-real-provider-2026-09-05.md)). OpenAI chat completions are now read rather than only forwarded: request summarised, usage converted out of inclusive counting, streaming included, guards and ledger applied. **That path is verified against a test stub only, never against a live OpenAI-compatible provider**, and secret masking does not cover it; the proxy warns at runtime on both counts ([spike](evidence/spike-openai-compatible-2026-09-05.md), [surfaces](SURFACES.md)).

**Gate: met.** Spike 3 answered; passthrough hash test green; added latency p99 published (98µs, see [`evidence/proxy-latency-2026-09-03.md`](evidence/proxy-latency-2026-09-03.md)); a real session recorded through the proxy at the measured tier (spike 4, 2026-09-05).

## v0.3: policies, dry-run, guards

Policy catalog using only provider-sanctioned mechanisms (ADR-0003), dry-run scoring of candidates, spend and loop guards, provider circuit breaker, error budget.

**Status:** guards are implemented and tested against a fake provider: spend caps per session and per day (fail closed before the next request, override logged), loop detection (warn or refuse), and the circuit breaker. Dry-run scoring (DR-1) is implemented: the proxy re-scores TTL and context-editing candidates after every turn from measured usage, publishes them on `/replay/status`, and sends nothing extra to the provider; the live figures equal an offline replay of the same ledger by test. `freeze-system-prompt` is implemented as the prefix hash on every ledger record and the certain break cause it gives. `context-edit-tool-results` is implemented as an opt-in flag that adds the provider's `context_management` parameter to admissible requests, pinned per session, logged with hashes, and measured through the provider's applied-edits report on the ledger; it is off by default and stays marked experimental for a measured reason rather than an absence of measurement: spike 4 ran it against the real provider on 2026-09-05 and the provider applied **zero** context edits on a session whose largest prompt was ten times the configured trigger, so the parameter is accepted without breaking anything and is not yet shown to do anything. Retries are implemented as PX-4 specifies: bounded, jittered, honoring `Retry-After`, only on retryable failures, only before any response byte reached the client, off by default. The error budget (SP-4) and list-price dollar caps (SP-1) are implemented. `hold-parallel-siblings` is implemented as `--hold-siblings`: a request whose prefix is in flight and not yet cached waits, bounded, until the first response begins, with the wait recorded per request. Graduation (DR-2) is implemented in `replay learn`. `breakpoint-on-stable-block` is not built. SP-7 has landed: a daily spend-cap refusal names the session that spent the budget, attributed on today's spend rather than the lifetime total, and says the attribution is partial when eviction means the largest survivor cannot be the largest spender. The proxy also records each response's rate-limit headers verbatim on the ledger, which is the only spend figure a flat-seat user has. SP-5, SP-6 and SP-8 — a resolved tenant key, per-tenant caps, and accounting eviction cannot widen — are specified and not built, gated on [ADR-0015](adr/0015-single-tenant-state-is-a-boundary.md): every piece of shared mutable state here is scoped to one human, so a centralised deployment would turn the day cap into an organisation-wide denial of service.

**Gate:** spike 4 passes (done, 2026-09-05); provider history-binding check green in CI against a policy-applied session; guardrail revert tested (done: breaches and the revert flag are now keyed per policy, so evidence gathered against one trigger cannot revert another and reverting one policy no longer disarms the guardrail for the next).

## v0.4: learning and advisor

Nightly local re-scoring, held-out validation, session types, bounded live trials with automatic revert, suggestions tracked from prediction to verified saving.

**Status:** the learning job is implemented as `replay learn` (LN-1, LN-2, LN-4, LN-6 with the metric decision in ADR-0006): catalog re-scoring over all sessions, held-out validation, minimum evidence, margin above noise, paired ties to the simpler policy, intervals in the output, a documented policy file. The proxy reads the policy file at each session's first request and pins the decision on disk (PX-8, `serve --policy-file`). The advisor is implemented as `replay advise` (AD-1 to AD-3): suggestions from the largest token sources with predicted savings on the share of prompt tokens, tracked to closure across sessions. Live trials with automatic revert (LN-5) are implemented on the re-read guardrail: a stable share of new sessions is treated, the rest are controls, and enough breaches revert the policy for new sessions until a newer learning result. Session types (LN-3) are implemented in `learn` on model family and first-prompt size, with one selection per type in the policy file, and the proxy picks by type at a session's first request. Graduation from trial results (DR-2) is implemented in `learn` from the treated and control sessions the trial recorded. Staleness detection (ST-1) is implemented offline: calibration is judged per model with the newest sessions on their own, a model whose newest sessions stopped calibrating is reported as changed provider behavior and has no alternatives scored, and the minimum cacheable prefix is bounded from usage; the lookback window is not refit (ST-2).

**Gate:** synthetic-corpus selection test passes (done); policy file format documented (done: [`architecture/policy-file.md`](architecture/policy-file.md)).

## v0.5: secret masking

Named pattern set, HMAC-derived placeholders, persistent encrypted vault, scoped rehydration (ADR-0004), per-session masked report.

**Status:** implemented as an opt-in flag: the named pattern set with user patterns and an opt-in entropy heuristic (MK-1), HMAC placeholders under a per-install vault key (MK-2, keyed per install rather than per project until a project identity exists), a vault encrypted at rest under an owner-only key file rather than the operating system keychain (MK-3, partial), scoped rehydration across stream chunks and inside tool-call JSON with per-pattern scope and a logged destination for every placeholder (MK-4 to MK-6), and the per-request masked report on the ledger and status (MK-7). Precision and recall are reported by the corpus test (1.00 and 1.00 on 15 positives and 15 negatives); the adversarial corpus test holds shell, network, unknown tools, and paths outside the project closed. Spike 5 is answered by that test on fixtures, not yet by traffic from a real agent.

**Gate:** spike 5 passes; precision and recall published for the pattern set (done for the repository corpus).

## Open, and measured rather than assumed

**Does a cache break cost a flat seat anything?** It decides whether Replay has a case to make to the majority of the people who run it, and no provider documents it. It was measured on 2026-09-06 with matched arms — ten cold writes and ten warm reads of a ~159,000 token prefix, same model, same size, only warm against cold differing — and 3.09M tokens moved the 5h utilisation counter by **zero** steps ([titration](evidence/quota-titration-2026-09-06.md)). A ratio near 12.5 and a ratio near 1.0 remain equally consistent with what has been measured, and the tool says so instead of picking one.

Two things came out of the attempt that outlive the null. The instrument now refuses rather than reports, naming which arm is short: the first estimator took the median of non-zero deltas, and simulation showed it returning exactly 1.00 whether the truth was 12.5 or 1.0, because the counter moves in whole hundredths. And live traffic settled a documentation question — a subscription session returns none of the documented `tokens-remaining` headers, but does return `anthropic-ratelimit-unified-5h-utilization`, `-7d-utilization` and `-representative-claim`.

Closing it needs a quiet account rather than a bigger budget: the account-wide counter is the largest error term and it is removable, not reducible. Until then, the report states the waste in tokens as well as dollars, which is a statement that holds either way.

## v0.6: the pool can tell two builds apart

Corpus submissions carry the binary that priced them.

**Status: shipped in v0.6.0.** Defect #284: two builds read one transcript
directory on one machine on one day and reported $4,088.49 and $11,969.37, both
stamped `rulesVersion: anthropic-2026-09-01`. The label was honest, the provider
had changed nothing, and the code had. A submission now carries `binaryVersion`,
`commit` and `pricingDigest`, the last computed from the price table, the
caching floors, the unknown-model fallback and any loaded rules document, so it
moves when any of them moves. `RulesVersion` keeps its published meaning.

**Gate: met.** Every pricing input is mutated in test and the digest is required
to move; the roster carries the build; the published field count and byte bound
are pinned by a test that names the documents quoting them.

---

## The path to 1.0

Four gates in [`../RELEASE-CRITERIA.md`](../RELEASE-CRITERIA.md) are unmet. The
releases below are the order they come off in, chosen by what unblocks what
rather than by what is easiest.

**These are gates, not dates.** Two of the remaining four depend on people who
are not the maintainer: a security reviewer with a calendar, and contributors
who do not exist until the launch produces them. Putting a date on either would
be the kind of claim this project spends its time refusing in other people's
numbers. A version ships when its gate is met, and the cadence rule in
RELEASE-CRITERIA still applies: cut when the changelog holds something a user
would act on, and do not let `Unreleased` run past roughly twenty entries.

**One long-lead item starts now, not at 0.9.** An external security review has a
scheduling lead time measured in weeks. Commissioning it is 0.7 work even though
publishing it is 0.9 work, because a gate whose clock starts when you reach it
is a gate you reach late.

## v0.7: the tool states what it cannot see

The theme is coverage honesty. Every release so far widened what Replay reads;
this one makes it say precisely where it stops.

- **The surface detection layer.** `cmd/replay/othersurfaces.go` extended to the
  agent surfaces a 2026 reader might be spending on, under the rule in
  `AGENTS_STATE.md`: verify on a real machine, mark everything else UNVERIFIED,
  and ship no path that has not been seen. A detector that invents a directory
  is worse than no detector, because it tells a reader their bill has no blind
  spot when it does.
- **The OpenAI-compatible path labelled `EXPERIMENTAL, UNMASKED`** wherever it is
  offered. This closes a 1.0 gate the honest cheap way rather than the expensive
  way: the path has only ever run against a test stub and masking does not cover
  it, and a label costs nothing while an unlabelled path implies a parity that
  does not exist. 0.8 upgrades the label to coverage.
- **`--json` on every command that prints a figure**, asserted by
  `scripts/surface-drift/drift.py`, which already exercises every documented
  surface and is currently used only for drift.
- **A mutation score with a denominator.** The 75 frozen mutants are a
  regression catalogue, not a score. Killed over generated, with the equivalent
  mutants named and the date attached, in the format every other figure in the
  README already uses. This project's own rule is that a figure carries its
  population, and this one does not.
- **Corrections as a dated collection**, the way `docs/evidence/` already works,
  rather than one generated page sliced out of a README heading. The retraction
  record is the most quotable asset the project owns and it currently has no
  per-entry URL and no per-entry date.
- **Distribution a reviewer will accept.** A Homebrew tap and a documented
  `go install`, standing beside the install script rather than replacing it. The
  people most likely to refuse a pipe into a shell are the people most likely to
  audit the signing, which is the project's best work.
- **Provenance attestation** on release artifacts. The Sigstore identity proves
  the tag ran the workflow; provenance proves what went into the build, and the
  SBOM is already generated and unattested.
- **Commission the external security review.** Lead time, not deliverable.

**Gate:** no surface ships a path that has not been observed on a real machine;
the OpenAI label appears everywhere the path is offered; the mutation figure
carries its denominator or is withdrawn.

## v0.8: someone else's machine

The theme is independence. Everything up to here was measured on one machine,
one account, one operator, and the roadmap has said so in every spike row since
2026-09-06.

- **The corpus stops being one machine.** Spikes 1 and 2 are marked met with the
  standing caveat that independence is not, and no session count fixes it. This
  is the one item on the path that the maintainer cannot do alone: it needs
  contributed corpora from people who are not him, which is what the launch and
  the 0.6 pool work exist to make possible. Until `replay pool` holds
  submissions from more than one `sourceTag`, every headline figure is a fact
  about one laptop.
- **Windows: settled 2026-09-13, and not the way this line used to describe.**
  The fourteen failing tests were fixed on 2026-09-10 and the job has been
  green and blocking since. That turned out to be the problem rather than the
  solution: Windows is green because the promise is switched off there.
  `internal/ownerdir` reports 100% statement coverage on ubuntu and 40% on
  windows, `modeIsChecked()` returns false so `tighten` never chmods or
  re-stats, and twenty-two tests across the ledger, the vault, the consent gate
  and the contributor secret skip with "Unix permission bits". A blocking green
  check whose greenness comes from disabling what it checks is exactly what
  ADR-0014 forbids.
  The binary now refuses on Windows, so "unsupported" is a thing the program
  does rather than a line in a README. The route that made it reachable is also
  closed: install.sh refused Windows and then offered a release archive that
  does not exist and a `go install` that works, which handed a reader a one
  line path to the binary the project says must not ship.
  **The port is not refused on difficulty.** An owner-only DACL is reachable
  from the standard library with no new dependency, since `syscall` and
  `unsafe` are already on the import allowlist. It is refused on evidence:
  `guard reachability` and `frozen mutants` both run on ubuntu only, so every
  refusal in an ACL layer would ship unmutated, and an unmutated guard is
  indistinguishable from an absent one. A Windows leg on those two jobs is what
  reopens this, and it is written into RELEASE-CRITERIA as the condition.
- **Masking covers the OpenAI-compatible path**, upgrading 0.7's label to actual
  coverage, and spike 5 answered by traffic from a real agent rather than by
  fixtures.
- **Caching rules for a second provider**, which 1.0 has required since this file
  was written. It needs published provider rules and a calibration corpus for
  them, so it starts here and lands when it calibrates.
- **Multi-tenant spend accounting (SP-5, SP-6, SP-8)** only if a user asks. All
  three are specified, unbuilt, and gated on ADR-0015 because every piece of
  shared mutable state is scoped to one human today. Recorded here so that
  building them stays a decision rather than drift.

**Gate:** the pool holds corpora from more than one operator, and the figures on
the website say how many; Windows is resolved in one direction or the other.

## v0.9: nothing unexamined

The theme is the security posture, which is the last gate with real uncertainty
in it.

- **Finding 3, the vault key boundary.** The remaining half of the oldest open
  finding: the key file sits beside the ciphertext, so within the TTL the vault
  is plaintext-equivalent to anyone who can read the directory. This is a
  structural decision rather than a bug, because the obvious fix is the OS
  keychain and reaching it needs `os/exec`, which
  `TestX402_ExecIsConfinedToTheMutationHarness` keeps out of every ordinary
  build on the grounds that it can call anything. RELEASE-CRITERIA already names
  the acceptable outcomes: move the key, or say plainly in the README that
  masking is a transit control and not storage. Either closes the gate. Deciding
  which is the work.
- **The external security review published**, with its findings open in the
  tracker rather than summarised.
- **Reproducible builds verified, not just signed.** Releases are signed and
  carry an SBOM; `-trimpath` and `mod_timestamp` are set. Nobody has rebuilt a
  published tag from source and compared the bytes. A reproducibility claim
  nobody has tried to falsify is exactly the kind of claim this project refuses
  elsewhere.

**Gate:** finding 3 closed or explicitly scoped in the README; the review
published; one shipped tag independently rebuilt to identical bytes.

## v1.0

Every box in [`../RELEASE-CRITERIA.md`](../RELEASE-CRITERIA.md) ticked, and
"nearly" still does not count.

That file's four gates, restated as they will read when they are met: the vault
key boundary resolved; the OpenAI-compatible path exercised against a live
provider or labelled; Windows supported or removed; and no headline figure
published without something having first tried to falsify the instrument that
produced it.

1.0 does not mean feature complete. `replay recall` is designed and unbuilt,
`breakpoint-on-stable-block` is unbuilt, ST-2 does not refit the lookback
window, and marking a session important has no client mechanism. All of those
are roadmap items and none of them are gates, which is recorded here so they
cannot be smuggled in later as blockers.

What 1.0 does mean is that the tool's claims about itself have all been
checked by somebody other than the person who wrote them.

---

## After 1.0

A feature list this far out would be fiction, and this file is not the place to
start writing any. What follows is the part that is knowable: the obligations a
1.0 creates, and the evidence that decides the direction.

### v1.x: what a 1.0 actually commits you to

The release that costs the most is the one after the promise. These are standing
obligations rather than features, and none of them are optional once the number
has a 1 in front of it.

- **Compatibility surfaces, named.** Before 1.0 ships, this file has to say which
  things are covered by the version number. The candidates are the corpus
  submission schema (`replay.corpus.v1`), the pool document
  (`replay.pool.v1`), the ledger format, the policy file, and every `--json`
  output the 0.7 work adds. The corpus schema has already survived two additive
  changes without moving, which is the behaviour a contract should have; the
  others have never been tested by a change. An unnamed compatibility surface is
  one you break by accident and find out about from a user.
- **A deprecation policy, which does not exist.** There are 30 verbs and no
  stated procedure for retiring one. The cheapest version is a sentence: what
  warning a command prints, for how many minor releases, before it is removed.
- **The price table goes stale on the provider's schedule, not ours.**
  `PriceTableStaleDays` is 60, prices have moved several times a year, and the
  cache multiples that the advice turns on move with them. This is the one
  maintenance cost that recurs forever and does not scale with users, and it is
  precisely what the x402 rules feed exists to fund. A 1.0 that ships with a
  table nobody has budgeted to maintain is a 1.0 with a shelf life.
- **Security response with an audience.** The SLA in `SECURITY.md` currently has
  one reader. After 1.0 it has reporters who will hold it to the letter, which is
  why the escalation path went in before the launch rather than after the first
  missed acknowledgement.

### The three doors, and which one opens is not up to the maintainer

Every roadmap past here forks on one fact that does not exist yet: whether
anybody else contributes a corpus. The 0.6 pool work and the launch exist to
find out. All three of these are acceptable outcomes and only one of them is a
failure of nerve.

**Door A. The pool stays at one operator.** The honest response is to say so on
the front page and keep the tool as what it demonstrably is: a personal
instrument that reads your own transcripts and is very good at it. No pooled
percentile, no "teams like yours", no benchmark. This door is only a failure if
it arrives and the site keeps implying the other one.

**Door B. Corpora arrive from people who are not the maintainer.** Then the
pooled benchmark becomes the product rather than a supporting claim, and the
work is distributional: percentiles that survive a small n, a stated method for
refusing a figure when the population cannot carry it, and the order-statistics
discipline the 0.5 benchmark work already had to learn once. The thing that
makes this defensible is already built, which is that every roster row is a file
a reader can download and re-hash.

**Door C. A team pays for a forensics week.** Then ADR-0015 gets revisited and
SP-5, SP-6 and SP-8 unblock, because the reason they are specified and unbuilt
is that every piece of shared mutable state here is scoped to one human and a
centralised deployment would turn a day cap into an organisation-wide denial of
service. That is a real architecture change and it should follow a paying user,
not precede one. `docs/design/forensics-week-qualification.md` already describes
how to find out quickly that a week is not worth selling, which is the part most
people skip.

**How a buyer is recognised is already decided, and it is not an account.**
[ADR-0023](adr/0023-entitlement-is-a-signed-document-not-an-account.md): a signed
document installed from a local file, verified offline against a key compiled
into the binary, expiring on a date read from the local clock. No licence server,
no callback, no identifier that leaves the machine. The recurring proposal in
this category is a hosted service with an account bound to a user name, and the
reason that is refused is not taste: the identifier would be the only personal
data this tool has ever held, and the binary sits in a credential path where a
network dependency is a new failure mode. The two MCP servers already split along
the line that matters, the hosted one answering questions about the world and the
local one about this machine, and only the first can ever be sold.
[`MONEY-PATH.md`](MONEY-PATH.md) carries the tiers, including what an enterprise
buyer is told no about.

### What would justify a 2.0

Not features. A major version is a promise broken on purpose, and there are only
two honest reasons to break one here: a published data format has to change in a
way that cannot be additive, or a provider or platform gets dropped. The corpus
schema absorbed six new fields across two releases without moving, so the bar is
not theoretical. If a 2.0 happens it should be nameable in one sentence, and the
sentence should be about what stopped working.

### What stays a not goal

Restated so that a busy year does not quietly adopt them: translating between
provider API shapes, replacing server-side compaction or context editing, any
server component, and Cursor agent mode. Padded cache slots are rejected
outright by ADR-0001, which is the rule that the tool measures and does not act.
Vector store, agent to agent messaging, virtual filesystem, a Rust sidecar and a
web dashboard remain deferred until a user asks, and "a user asked" means a user
asked rather than a maintainer imagining one.

### What would end it

Worth writing down while it is cheap to write down.

The open measured question in this file is whether a cache break costs a flat
seat anything. Measured on 2026-09-06 with matched arms, 3.09M tokens moved the
utilisation counter by zero steps, and a ratio near 12.5 and a ratio near 1.0
remain equally consistent with what has been observed. **If that resolves toward
1.0, the majority of people who run this tool are not losing money**, and the
case Replay makes to them evaporates even though every figure it prints stays
correct. The tool would still be right and would still be worth much less.

Closing it needs a quiet account rather than a bigger budget. Until then the
report states the waste in tokens as well as dollars, which is a statement that
holds either way, and this paragraph stays here so that the outcome is a result
rather than a surprise.

## Deferred until a user asks

Vector store, agent-to-agent messaging, virtual filesystem, Rust sidecar, web dashboard. Padded cache slots are rejected outright (ADR-0001).

## Not goals

Translating between provider API shapes. Replacing server-side compaction or context editing. Any server component. Cursor agent mode.

---

[Documentation index](README.md) · [Repository README](../README.md)
