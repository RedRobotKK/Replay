# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- **`replay cost` read seven transcripts it had been discarding whole.** The
  Claude Code parser grouped assistant lines into requests by the top-level
  `requestId` and skipped any line without one. Only the `cli` entrypoint writes
  that field: `sdk-cli`, `sdk-ts` and `claude-desktop` write the same
  `message.usage` and the same `message.id` and no `requestId` at all, so those
  files produced no lanes and were reported as unreadable. The message id groups
  the same lines - measured over 1821 transcripts, 28,665 request ids each
  carrying exactly one message id and no message id under two request ids - and
  is now used where the request id is absent, with `Request.IDFromMessage`
  recording which one a request holds.
- **A client's record of a failed call is no longer counted as a request.**
  Claude Code writes an API error as an assistant line with
  `isApiErrorMessage`, a model of the literal string `<synthetic>` and a usage
  object of all zeros. One landing at the head of a lane renamed that lane's
  model to `<synthetic>`, which no price table knows, and took its avoidable
  figure from 1,586,545 tokens to zero with the cost unchanged.
- **A transcript that could not be read is no longer reported as read.** `cost`
  counted transcripts that parsed and priced to nothing, and counted nothing at
  all for transcripts that did not parse, so over a single unreadable file it
  printed "0 were read" about a file with 183 assistant turns in it. The two
  are now separate counts with separate sentences, in the report, in the
  `--max-avoidable-usd` gate and in `--json` as `unreadable`.
- Redaction keeps `message.id`, which it had been dropping. Every fixture here
  is a redacted transcript, so a bug report about an SDK transcript arrived with
  the only identifier the file carried removed.
- **This moves a published figure.** On the corpus this was measured on,
  `replay cost` goes from 116 sessions and $3766.87 to 118 sessions and
  $3771.32, and from disclosing 6 transcripts read but not priced to disclosing
  13 that could not be read at all.
- **`replay privacy` reported a `~/.replay` it could not read as one holding
  nothing.** The store registry returned an empty list for a directory that was
  absent and for one that existed and failed to list, and the command printed
  "Replay has written nothing to this machine" for both - a false absence in the
  command that answers "what do you hold about me". Only `os.IsNotExist` now
  means nothing is there; anything else is reported. The same swallow sat under
  the TUI's safe screen, which already had an unreadable state and no way to be
  told it had one.
- **A store measured over entries it could not walk no longer prints as its
  readable part.** `privacy` and the safe screen both totalled what the walk
  could read, so a store containing an unreadable directory printed smaller than
  it is, or as `0 B` - indistinguishable from a store known to be empty. Both
  now say how many entries went unmeasured.
- **`replay purge` reported an erasure over files it had not examined.** Ledger
  files the walk could not enter and files it could not read were skipped in
  silence, and the run still printed "removed N record(s)" or "Nothing to
  remove". Both are counted, and a sweep that skipped anything says so and says
  that the report does not cover it.

### Added

- **`replay cost --usage <file>` prices a usage export: token counts per
  request, no conversation content anywhere in the file.** Three readers cannot
  use the transcript path — an operator whose security review will not approve a
  tool that reads prompt content, a finance owner who wants a reconciliation
  figure and not a conversation, and anyone whose transcripts were rotated away
  before the question was asked.
- **The cost is Measured and the cause is not, in the same row.** Cache
  arithmetic never needed the content: the expected read is the previous
  request's prompt minus its uncached tail, both provider-reported, so the break
  deficit is a measurement. Why the prefix stopped matching is not — usage and
  timing settle only three causes (expired, model changed, nothing read) and
  every other break is counted with its deficit and its cause marked
  `NOT MEASURED`. The transcript path's fourth answer is located by the
  byte-to-token fit and is not borrowed here, because there are no bytes.
- **The export declares two things it cannot prove and is refused without
  them.** A record missing from the export is indistinguishable from a cache
  break, so the exporter asserts `complete`; undated records cannot be put in
  the order a break is defined against. Without either, the cost still prints
  and the break figures say `NOT MEASURED` with the reason. `fresh + cached_read
  - cached_write` must equal `prompt`, which refuses an export copied from a
  provider that counts inclusively — the error is largest on exactly the
  sessions that cache best.
- `--max-avoidable-usd` refuses to pass when the avoidable figure was not
  measured. `--share`, `--png`, `--compare`, `--contribute` and `--per-lane` are
  refused on this path rather than served with a blank.
- A file that is not readable JSON is refused for that reason, in its own
  words: not JSON at all, a foreign schema, and counts that disagree are three
  different mistakes and a reader sent to check the wrong one is worse off than
  one told nothing. A failed write of either report form is returned and says a
  write is what failed — a truncated JSON document a pipeline parses, or a
  printed report that stops before the NOT MEASURED block, must not exit 0.
- `internal/usage` is wired into the binary for the first time
  (UNWIRED-LOG #8): the export decodes into `usage.Entry` and `Validate` runs on
  every record at the door.

- [The first run, end to end](docs/guide/first-run-journey.md): install to first
  finding to acting on it to verifying the change, as one document, with every
  block pasted from a run against the redacted session this repository ships so a
  reader can reproduce it. It names the three places the route does not complete
  on a new machine, including that the verification step in step 7 has not been
  observed to fire on any corpus this project holds.

### Changed

- **The installer prints the ending it actually performs.** Two "Next:" commands
  were printed unconditionally, and then, on a terminal, the script `exec`d
  `replay tui` over the top of them: the reader was told to run one thing and
  handed another, and the surface that took the terminal was named nowhere in the
  output. The exec was kept and the printed line moved. On a terminal the block
  now names `replay tui`, says it opens on `cost`, and offers the two commands as
  what to type after quitting; off a terminal — CI, a `Dockerfile RUN`, cron, a
  container built without `-t` — nothing opens and the two commands are the next
  step exactly as before. `--no-tui` and `REPLAY_NO_OPEN=1` are unchanged.
## [0.5.4] - 2026-09-09

### Changed

- **Every TUI screen now reads this machine.** `advise`, `context`, `guards`,
  `model` and `safe` fell through to a canned illustration, because they had no
  case in the dispatch and so no caller that measured. They carried an
  example-data notice, so nothing was dishonest — it was simply not the reader's,
  on the surface `install.sh` opens after a successful install.
- Each keeps three states rather than two: data is Measured; sessions read with
  nothing found is Measured and says how many; no sessions at all is
  `not measured here`, never "example data". An absence and an illustration are
  different claims, and only one is about the reader.
- `guards` refuses a cap below ten sessions, because a threshold from fewer is a
  threshold from noise. `model` names no target: pricing a switch to a model the
  reader did not choose would invent the question as well as the answer.

### Fixed

- Narrow-terminal overflows went 88 to 18 across the release, as measured output
  replaced the illustrations and the `advise` saving row began dropping its
  status column when the terminal cannot hold it — `storyboard.go` scene 25,
  implemented.
## [0.5.3] - 2026-09-08

### Added

- `replay cost --max-avoidable-usd <n>` fails the build when measured avoidable
  spend exceeds a ceiling, so CI can gate on it without parsing prose. It derives
  nothing of its own: the figure is the one `cost` already measures.
- **It refuses to pass on a corpus that priced nothing.** `cost` exits 0 with no
  transcripts, deliberately, but asking for a ceiling is asking to assert spend
  is under a number, and that cannot be asserted over nothing. A CI runner has no
  transcripts, so without this the gate would go green having measured nothing.
- Excluded unpriced transcripts are counted in the failure, because the total the
  ceiling was compared against has holes and the real figure is higher.
## [0.5.2] - 2026-09-08

### Added

- `replay prefix --before <file> --after <file>` gates a change that voids the
  cached prefix, exiting non-zero so CI can act on it. A prefix break is the
  rarest cause in the corpus and the most expensive per event: 5 breaks,
  1,807,000 tokens, a mean of 361,400 — higher than a TTL expiry. It watches the
  tool set rather than the system prompt, because the 30-lane trial of
  2026-09-06 found the system prompt never moved and the tool set always did. It
  reads only the two files it is handed and refuses `settings.json` outright.
- `replay since [--peek]` reports what ran and what it cost since you last
  looked. No account, no network, no pairing: the answer is already on disk. A
  first run says it has no previous look rather than reporting zero sessions
  since a window that does not exist, and a quiet window says nothing happened
  rather than printing `$0.00`.
- `docs/CLI.md`, every command and every flag, generated from the binary and
  diffed in CI so it cannot drift from what the tool accepts.
- `scripts/ledger-bench`, before and after for each product that meters agent
  spend, against a committed fixture with every figure pinned.
- `scripts/tui-audit`, 72 renders across four widths and two locales checking
  for overflow, palette drift, unclosed colour and control characters.

### Fixed

- The TUI width is a measurement rather than a constant. `BudgetCols = 80`
  became `Cols()`: COLUMNS, then `TIOCGWINSZ`, then 80. Reading COLUMNS alone
  would have been a fix that mostly does not fire, because a shell maintains it
  for its own line editing and does not export it.
- Prose is wrapped at the single output boundary, with the original indent
  carried onto each continuation. Table rows are deliberately left alone:
  wrapping destroys the alignment that makes a table, and truncating silently
  drops a column's value.
- Narrow-terminal overflows went from 88 to 30 across the nine screens. What
  remains is 22 table rows, 4 rules and 4 unbreakable tokens, and no prose.

### Changed

- `docs/evidence/surface-census-2026-09-08.md` records every agent store on one
  machine, opened and measured. Two earlier claims are retracted: Grok was
  documented as keeping no local transcript and keeps 3.8 GB; Cursor was ruled
  out on a base-URL argument and holds 118 readable transcripts. Neither carries
  usage, so the conclusion held and both stated reasons were wrong.
- A `$406.07` figure, reported as the only measured dollar figure on the
  machine, is withdrawn. The fields it was derived from do not exist.
## [0.5.1] - 2026-09-08

Recorded after the fact: this release was tagged without a changelog entry, and
a release nobody wrote down is one nobody can audit.

### Added

- `replay mcp` answers an agent's questions mid-session over JSON-RPC on stdio,
  serving one MCP vocabulary from the binary and saying which answers need a
  network.
- `replay agents` writes a boot block naming where a project keeps its records.

### Fixed

- The MCP snippet was not valid JSON on Windows: a path like `C:\Users` made
  `\U` an invalid escape. Paths are marshalled rather than interpolated.
- Figures that were never measured stopped being reported as passing results.
## [0.5.0] - 2026-09-07

The first release with an interactive surface, the first carrying a fix somebody
outside the project reported, and the first under a new licence.

### Changed — licence

- **Replay is now under the Business Source License 1.1**, converting to Apache
  2.0 on 2029-09-06 ([ADR-0016](docs/adr/0016-business-source-license.md)).
  Running it inside your own organisation stays free and unrestricted —
  commercially, in production, in CI, modified for internal use, and its output
  is yours for any purpose. The one thing the grant withholds is reselling
  Replay itself as a hosted service or embedding it in a product sold on its
  analysis. **v0.4.0 and everything before it remain Apache 2.0 permanently**;
  a licence binds a version, not a project. Under the OSI definition this
  project is no longer open source, it is source-available, and the README no
  longer claims otherwise. The name is now reserved explicitly in NOTICE: a
  fork may take the code, not the name.

### Added

- **`replay tui`**, a question-first surface. Eight questions, one keystroke
  each, and every screen prints the command that produced it so a reader can
  copy the line and stop needing the screen. Three screens read this machine
  (`doctor`, `cost`, and `why` once a session is chosen); the rest say
  `example data` above their figures rather than letting anyone take them for
  their own. Row selection with `j`/`k`, `H`/`G` and enter, a `?` overlay
  carrying the full vocabulary, and a footer of three keys rather than a wall
  of them. Piped output is plain lines with no escape sequences.
- **Corpus discovery searches everywhere it might be**, returns every root
  holding transcripts rather than the first, and honours `REPLAY_TRANSCRIPTS`.
  A user with sessions in two places was previously reported on one of them,
  which is worse than no total because it is wrong and looks right. When
  nothing is found it names every path it searched and tells apart a machine
  that never ran the agent from one that has run it and recorded nothing.
- **A frozen-defect harness** (`internal/regression`): one register entry per
  defect this project has found, each carrying what it looked like from outside
  and what would be true again if it returned. Eighteen new mutants are frozen
  in `internal/mutation/testdata/mutants.json` so the score is reproducible
  rather than a number in a commit message.
- **`replay codex` reads OpenAI Codex rollout logs**, the first surface beyond
  Claude Code. Codex writes two token counters and they are not the same
  quantity: `last_token_usage` is the per-turn delta and `total_token_usage` is
  a running total that Codex **rebases when the context compacts**. Summing the
  deltas is what was paid for; reading the running total is what is still in
  context. Measured across 148 local rollouts the two agree on 146 and diverge
  on exactly the 2 that compacted — 610,551,532 tokens billed against
  401,450,788 reported, so 34% of the spend is invisible to the client's own
  counter. Both figures are kept and named rather than reconciled into one.
  Discovery searches `sessions/` **and** `archived_sessions/`, which on the
  machine this was built against is 148 files rather than 27.
- **The Codex rate-limit signal is now readable.** Every session opens with a
  `token_count` event whose `info` is null, because nothing has been spent yet,
  and which carries `rate_limits` instead: used-percent over a five-hour and a
  seven-day window, each with a reset time. Refusing that record as malformed
  costs the only live quota signal this project has found in any client. The
  Anthropic surface reports utilization on the wire only, and a titration moved
  3.09M tokens through it for zero counter movement; the Grok CLI's
  `x-ratelimit-*` headers did not shift once across 8 calls and 940KB. This one
  moves, and it needs no proxy. How heavily a cached read weighs against it is
  measured as far below one and no further: the corpus fits four weightings
  equally well and cannot separate them.
- **A surface registry** where a support claim cannot outrun its evidence.
  `LIVE` requires a captured fixture on disk, so a surface cannot be promoted by
  editing a string.

### Fixed

- **`--check-prices` never compared the cache-read price it parsed.** Reported
  by [@roy-tong](https://github.com/roy-tong) in #54 with an exact diagnosis:
  the field was read from the price database and had no other use anywhere in
  the repository, tests included. "No disagreement" was therefore silent on the
  field this cost model leans on hardest. Following the same gap one field
  sideways found the cache-write multiplier equally unchecked and larger: a
  cached read costs 0.1x input where a write costs 1.25x. Both are compared now,
  and a field the other database does not carry is recorded as absent rather
  than compared against zero.
- **A Windows user could not record a consent decision at all.** The gate read
  Unix permission bits; Windows has none and Go synthesises `0666` for any
  writable file, so every consent file a user had just written was refused,
  citing permissions that do not exist there. No corpus opt-in, no update
  consent, silently. `Decision.OwnershipChecked` now reports whether the check
  could run, so "verified as this user's" stays distinguishable from "not
  verifiable here".
- **Spend eviction was not least-recently-used.** It broke ties by wall-clock
  timestamp, which is an LRU only if the clock can separate two records. Under a
  burst on a coarse clock it became evict-anything, and the entry it dropped
  could be the heavy, still-active session whose spend is the reason attribution
  exists. Now keyed on a monotonic counter.
- **A routing error band was describing the estimator, not the corpus.**
  Comparing a model against itself returned a ratio of exactly 1.0 with a band
  of ±85%, which a quantity known to be exactly 1 cannot honestly carry. The
  band was a turn-weighted mean of per-session errors, and a mean does not
  shrink with sample size, so no amount of corpus could ever sharpen it. Work
  deferred on "we need more data for sigma" was deferred for a reason that was
  never true.
- **The ledger read in filename order rather than write order.** `Glob` sorts by
  name, so a session whose records landed in two files came back in the wrong
  sequence, and every test asserting "the first record carries the policy"
  passed on which filename happened to sort first.
- **golangci-lint reports every issue it finds.** The default `max-same-issues:
  3` and `max-issues-per-linter: 50` are display caps, not filters: the run
  always failed on everything it found, and everyone reading the output saw 38
  issues where there were 119.
- **Grok is no longer filed under the OpenAI-compatible wire.** It posts to
  `/responses` at `cli-chat-proxy.grok.com`, not `/v1/chat/completions` at
  `api.x.ai`, and nothing in this build parses that path. It is `FORWARDED`, on
  its own row, with the capture behind it.
- **`isolateHome` sets `USERPROFILE` as well as `HOME`.** `os.UserHomeDir` reads
  `USERPROFILE` on Windows, so tests built on that helper ran against the
  runner's own home and failed on empty output: a harness bug wearing a product
  bug's clothes for as long as the job stayed red.
- **`docs/SURFACES.md` said Windows was built** after the `goos` line had
  already dropped it.
- **Every TUI screen displayed `v0.4.0` as a string literal.** The version is
  injected at link time precisely so it cannot drift, and no screen read it, so
  a 0.5.0 binary would have reported 0.4.0 on every screen it drew. The header
  padding was also derived from the literal's width, so any other version would
  have sheared the right edge of every header at once.
- **The surface wrote cursor addressing into pipes.** `enter` and `leave`
  already refused to emit alternate-screen sequences when the terminal had not
  taken raw mode, with a comment naming the failure exactly — "writing escape
  sequences into a pipe produces a file full of control codes". The per-frame
  paint did not share the gate, so redirecting `replay tui` to a file, a pager,
  or an agent reading on someone's behalf wrapped every line in control codes.
- **A test asserting that every screen leads with a figure had never once
  exercised a screen.** It scanned the first four rows for a digit, and row zero
  was the header carrying `v0.4.0` — so every screen passed on the version
  number in its own title bar. Removing the hardcoded version is what exposed
  it. The window now skips the header, and the one screen that genuinely opened
  without a figure was fixed rather than the test loosened.

### Changed

- **The context report says when its estimate borrowed a constant.** A session
  that fitted its own turns measures its own ratio; one with no fittable turn
  falls back to an English prose average, and both used to print the same
  footnote. At 2.29 bytes per character for Japanese against roughly 1.0 for
  ASCII, that is a difference worth stating.
- **Two conservation laws run in CI.** Every token the provider billed lands in
  exactly one named bucket, and the same holds inside each agent lane. Three of
  the eleven mutations written against them passed the pre-existing suite.

### Fixed after tagging

- **The release pipeline's tool pins did not pin the tools.** Both
  `cosign-installer` and `sbom-action` are pinned by commit SHA, which fixes the
  action and leaves the binary it downloads free to move: v0.4.0 was signed with
  cosign v2.5.2 and built its SBOM with syft v1.42.3, and the identical workflow
  a day later pulled cosign v3.0.6 and syft v1.51.1. cosign v3 defaults to the
  new bundle format, ignores `--output-signature` and `--output-certificate`, and
  the release failed on `create bundle file: open : no such file or directory`.
  Both tools are now pinned to the versions that produced every release so far.
  The format matters beyond this failure: every copy of `install.sh` in the wild
  looks for `checksums.txt.pem` and `checksums.txt.sig`, and an installer that
  finds cosign but no signature refuses to install rather than proceeding
  unverified. Publishing bundles instead would break them.

### Known limits

- Windows is still not supported. The job passes and that is not the same thing:
  `Decision.OwnershipChecked` is false there, and the tests asserting Unix mode
  semantics skip with the reason stated. Skipping honestly is not passing.
- Five of the eight TUI screens carry example data and say so. `guards`, `model`
  and `safe` need a running proxy or are limited by the estimator floor above.
## [0.4.0] - 2026-09-06

A minor release. The theme is that the proxy's own instruments were wrong about
concurrent sessions, and now are not.

**If you build from `main`, update.** Two data races were introduced earlier
today and are fixed here. They did **not** affect v0.3.0 or any earlier tag;
only a build taken from `main` between the two merges carries them.

**Windows is declared unsupported**, which is a statement of fact rather than a
change: it has never been tested there. See the README.

### Added

- **The cost report states the waste in tokens as well as dollars, and names who the dollars are for.** The avoidable line carried a dollar figure only, and most of the people who run this hold a flat seat, where a broken cache costs no money at all — so the headline finding was addressed to a minority and read as inapplicable to everyone else. It now prints the re-billed token count beside the dollars and, when there is any, a paragraph saying plainly that on a subscription seat the dollars are list price for somebody else while the tokens are still yours: context the work did not get, and rate-limit budget spent on nothing.

- **`replay --help` is grouped and ranked by value.** Sixteen commands in one flat list put `redact` and `probe` in front of a reader who had not yet found `diff`. The list is now four groups — start here, look closer, corpus and calibration, setup and maintenance — with the commands that answer the question people arrive with at the top. A test asserts both halves: that no command was lost in the regrouping, and that the front-door commands still precede the specialist ones, because a ranking nothing checks reverts to insertion order the next time someone adds a command.

- **Compaction is parsed, and `replay context` says when its own answer is incomplete.** Claude Code writes a record carrying `compactMetadata` with the prompt size before and after the compaction, and nothing here read it, so a session that compacted was attributed as though everything it ever loaded were still in context. It is not. The report now names how many compactions fired, how many tokens the client says they dropped, and by how much the attribution above therefore overstates — and where a compaction recorded no size it says that rather than guessing, because an overstatement that cannot be measured is still worth declaring. Measured across this corpus: 39 such records, median 999,029 tokens before against 23,218 after, a retention of 2.55%.

- **The proxy captures what the provider says a request spent against a rate limit.** Every response's `anthropic-ratelimit-*` and `x-ratelimit-*` headers, plus `retry-after`, are recorded verbatim on the ledger entry. Verbatim because the formats disagree between providers and across header families, and a parser that guesses wrong fails silently; the ledger's job here is to preserve the reading, not to interpret it. These are the only figures a flat-seat user has, since they have no invoice to compare against.

- **`internal/quota`: whether a cache break burns a subscription quota the way it burns a bill.** The method is subtraction — the drop in the provider's own "remaining" between two consecutive requests is what the later one spent — with no thresholds, risk levels or lockout forecasts, none of which the data supports. Live traffic settled a documentation question along the way: a subscription session returns none of the documented `anthropic-ratelimit-tokens-remaining` headers. It returns `anthropic-ratelimit-unified-5h-utilization` and `-7d-utilization`, on two windows at once, with a `-representative-claim` naming which one currently binds. The population this package exists for is only visible through that second family.

- **`replay route --to` charges for the switch itself.** The destination model starts cold and has to write the shared prefix again before it reads any of it, so a model that is cheaper per turn can still be the wrong move. The report now prints the one-time switch cost, the saving per turn, and the turn on which those cross — and, when that turn lies beyond the number of turns actually measured, says so rather than recommending the switch. Payback turns are rounded up and floored at 1, because a partial turn repays nothing and a payback reported a turn early is advice to switch on a task that loses money. With no turns to divide by, or no saving to divide, it reports no payback instead of manufacturing a quotient.

- **A spend-cap refusal names the lane that spent the budget (SP-7).** A day cap that trips with the reason "daily spend cap reached" is an outage with no operator action attached: it says something stopped, not that the indexing lane did it. The refusal now names the largest session still accounted for and its share. It is attributed on the day's spend rather than the lifetime total, because a lane that ran all of yesterday has a large lifetime figure and may have spent nothing today. When the session table has evicted enough that the largest survivor is not the largest spender, it says the attribution is partial rather than blaming a small lane for someone else's overrun.

- **ADR-0015: single-tenant state is a boundary**, with SP-5 to SP-8 in `docs/requirements.md` specifying the tenant key that would cross it. The engine is portable to a team; the shared mutable state is not. The spend guard's counters, the evicting session table, the metrics surface and the credential path are each scoped to exactly one human, so centralising them today turns the day cap into an organisation-wide denial of service. SP-7 has landed; SP-5, SP-6 and SP-8 are specified and gated on the ADR, and SP-6's acceptance test is written to fail against the current guard on purpose.

- **New evidence: [what actually breaks the cache](docs/evidence/break-causes-2026-09-06.md).** 735 breaks and 31.26M re-billed tokens across 1,506 transcripts, classified by the tool's own cause constants: client re-render 50.8%, TTL expiry 33.9%. The document is kept as much for its sampling note as its result — the same measurement over the 40 largest sessions said TTL 75.2% and re-render 2.5%, almost the reverse, because the largest sessions are the long-running ones and long gaps are what TTL expiry means. The caveat was stated at the time and the conclusion drawn anyway; the full run cost five seconds.

- **New evidence: [the quota titration](docs/evidence/quota-titration-2026-09-06.md), and it came back null.** Matched arms — ten cold writes and ten warm reads of a ~159,000 token prefix — moved 3.09M tokens and the 5h utilisation counter by zero steps. Published as null, and it voids an earlier calibration in this repository that attributed a counter step to four probe requests on an account-wide counter an interactive session was moving at the same time. The estimator was replaced too: simulation of the measured instrument showed the first one returning exactly 1.00 whether the true ratio was 12.5 or 1.0, because the counter moves in whole hundredths and the median of each arm is then 1 in every possible world. It was degenerate rather than noisy, and it would have reported "subscriptions do not charge like the bill" with total confidence and nothing behind it.

- **`replay` with no arguments reports what your agent work cost.** The install line is `curl | sh` and the next thing a person types is `replay`, which answered with a list of sixteen commands — `cost`, the one that reports money, eleventh. It now runs the cost report over the transcript directory `defaultTranscriptRoots` already finds, then names the other commands underneath it. A machine with no discoverable transcripts still gets the command list, because a report over nothing is not a finding; that branch is tested rather than assumed. `replay <path>`, `replay --help` and every subcommand are unchanged. Six mutants, six caught, and M51 is frozen in `internal/mutation`.

- **The share card names the billing route.** `replay cost --share` derives `first-party API`, `Bedrock`, `Vertex` or a mix from the model ids already on each request, and labels the Bedrock and Vertex routes `metered` because neither offers a flat seat. A bare first-party id is deliberately **not** labelled: the same string is emitted by an API key and by a subscription, so a billing claim there would invent the precondition the product rests on — a cache break re-bills tokens, and a flat seat is not billed per token. The route is a category and never an identifier; a real Vertex id carries the caller's GCP project and region and a Bedrock ARN their account number, and none of that reaches the card. Eight mutants (M29–M36) are frozen against it, including one that prints the raw ids, which is caught by both the leak test and the card's identifier test — the latter only after it was rebuilt to cover the new field. Across the 1,471-file measured corpus every observed id was first-party: not one Bedrock ARN or Vertex publisher path.

- Staleness detection (ST-1): `replay corpus` and `replay learn` judge calibration per model with the newest sessions on their own. When a model's earlier sessions calibrated and three of its newest five each fall below the threshold, the report says the provider's behavior changed, `replay learn` scores no alternatives for that model, and the minimum cacheable prefix is bounded from usage next to the rules file's value. The lookback window is not inferable from usage and stays in the rules file (ST-2).
- `replay serve --mask-entropy`: an opt-in entropy heuristic for credentials the pattern set does not name, judged by length, mixed character classes, class transitions, path shape, and Shannon entropy, reported as pattern `entropy` and scored on its own corpus. Placeholders it creates rehydrate under the same scope rules.
- `replay serve --hold-siblings <duration>`: the `hold-parallel-siblings` policy from the v0.3 catalog. A request whose tools and system prompt are already in flight and not yet cached waits, up to the given bound, until that first response begins streaming, so parallel sub-agents read the cache entry instead of all writing it. A prefix with a response begun within the short cache lifetime holds nobody, a leader that fails releases its siblings, and the wait is recorded per request as `held_ms` on the ledger, the log, status, and metrics. Off by default.
- Scoped rehydration in `replay serve --mask` (v0.5, ADR-0004): placeholders in responses are restored in assistant text and in file-edit tool inputs whose path is under `--project`, never in shell, network, or unrecognized tool inputs, edits outside the project, or thinking blocks; `--rehydrate-scope name=dest,dest` widens or narrows that per pattern and `--rehydrate=false` keeps masking only. Streams are rewritten as they pass, with a placeholder split across deltas or chunks restored whole and a tool call's input held until its block ends; escape-spelled placeholders fail closed. Every response's restored and denied placeholders are logged, recorded on the ledger, and counted on status and metrics by destination. An adversarial corpus test holds the line for shell, network, unknown tools, and paths outside the project.
- Secret masking in `replay serve --mask` (v0.5, experimental): a named pattern set plus user patterns replace secrets in outbound bodies with placeholders derived by HMAC under a vault key, changing only the matched bytes and never thinking blocks or signatures; the vault persists encrypted at rest; ledger records, status, and metrics count what was masked by pattern name. The pattern corpus test reports precision and recall.
- Graduation in `replay learn` (DR-2): when the ledger holds sessions the proxy treated with a policy and sessions it held out as controls, the two arms are compared on cost per token of new content, and the policy graduates only with five sessions per arm, arms separated above noise, and a realized saving of at least half the prediction. The verdict and both arms' figures go to the policy file and the report.
- `replay serve` picks the learned policy by session type at a session's first request, classified from the model and the prompt's size, and records the type on the pin. A type with no selection falls back to the overall one.
- Session types in `replay learn` (LN-3): sessions are typed by model family and first-prompt size, both known at a session's first request, and the policy file carries one selection per type under the same rules next to the overall one, so a policy that helps large-prefix sessions and hurts small ones is selected for the first and withheld from the second.
- Security review fixes: `REPLAY_NO_POLICY=1` and a restart without a policy source now also stop sessions an earlier process pinned on; retries resend only requests that never connected, never one that may already have been billed; a request the summarizer cannot read never receives the context-edit parameter; ledger call keys are keyed by the ledger secret so a guessed call cannot be confirmed from the file; the advice file holds file base names, never full paths, and search patterns are no longer treated as file reads; in-memory session tables are bounded; the breaker's probe slot is given back only by the probe and a client that left before any response is not counted; the stream parser stops buffering an endless line; the client's own forwarding headers pass through; a pin with invalid parameters is ignored and file-derived strings are sanitized before logging; ledger file names for sanitized session ids carry a hash so two ids never share a file.
- Live trials in `replay serve` (LN-5): a policy read from the policy file is applied to a `--trial-share` of new sessions by a stable hash and held out from the rest as controls, both pinned; `--guardrail-reread` reverts it for new sessions once `--revert-after` treated sessions reach that re-read rate after the provider's first clear. The revert is persisted under the ledger directory, survives restarts, and is lifted by a newer `replay learn` result. `/replay/status` reports the arms, breaches, and any revert.
- `replay advise`: turns the largest token sources across all sessions into suggestions with a predicted saving on the scale-free metric (share of prompt tokens) and tracks each to closure: pending, applied when the newest sessions show the target shrinking, then verified or not verified against the prediction. Kinds: dominant tool inputs, oversized tool results, hot files, first-turn instruction content, tool definitions never called (ledger sessions), and cache breaks. The ledger now records tool definition names and sizes, never their text, so the last kind is possible.
- `replay serve --policy-file`: applies the context-edit candidate `replay learn` selected, read at each session's first request. The decision and its parameters are pinned per session and persisted under the ledger directory, so a rewritten policy file or a restarted proxy never changes a running session (PX-8). A stale file (other schema or rules) or one that selects nothing applies nothing and says so in the log. `/replay/status` shows `pinned_policy` per session.
- `replay learn`: re-scores the policy catalog over every transcript and ledger session with the replay simulator and selects a policy under rules sized for a personal corpus: minimum sessions with evidence, a margin above noise, a repeat on held-out sessions chosen by a stable hash, and ties to the simpler candidate on the paired per-session difference (ADR-0006). Writes a versioned, documented policy file to `~/.replay/policy.json`; says "none" when nothing qualifies. Reads files only.
- `replay replay`, `replay blame`, and `replay diff`: offline analysis of Claude Code transcripts. Reproduces the provider's cache reads turn by turn (the calibration line), classifies every cache break with a cause and location, attributes prompt tokens to content with per-turn sums that match provider usage exactly, prices visible error classes, and scores TTL and context-editing policies with the as-run behavior reproduced first. Every figure is labeled estimated or measured.
- `replay doctor`: reports transcripts found, the proxy variable and whether Replay answers there, and the ledger, with the next command to run.
- `make test` and CI report statement coverage.
- `--dollars` on `replay`: a list-price cost column per policy from a dated first-party price table (`cachemodel.PriceTableVersion`); models not in the table get no dollar figure. Effective-token math now uses the model's own cache-read multiple.
- `replay corpus`: calibration summary across every session in a directory as a Markdown report (session id prefixes, counts, match rates, fit numbers, break causes; no paths or content).
- `replay redact`: strips content from a transcript while keeping structure, sizes, usage, tool names, and equality of values (salted, so nothing is confirmable from the output), for bug reports and fixtures. A redacted session analyzes identically to the original; the repository fixture is a redacted real session.
- `replay serve`: loopback passthrough proxy for the Anthropic Messages API. Byte-exact forwarding with immediate flushing for event streams, credentials forwarded and never persisted, gzip responses forwarded compressed, browser-origin rejection, optional shared token, fail-open bookkeeping, and a 502 that names the bypass when the provider is unreachable. Records a derived-data ledger (structure, sizes, labels, usage; no text) per client session under `~/.replay/ledger`.
- File re-read rate: `replay` reports how many file reads repeated a path already in context and what they cost in prompts, and when the ledger shows provider context edits, the rate after the first clear next to the rate before it. `/replay/status` carries the same figures per session as `re_reads`. This is the guardrail metric for the context-edit policy.
- Experimental live policy in `replay serve`: `--context-edit-trigger <tokens>` (with `--context-edit-keep`, default 6) asks the provider to clear old tool results server-side by adding the `context_management` parameter to requests whose client enabled the context-management beta and set no such parameter itself. The parameter is spliced after the client's bytes, which stay byte-identical; the decision is pinned per session at its first request; every edit is logged with body hashes before and after, never content; the ledger records which requests carried it and the provider's applied edits and cleared tokens; `REPLAY_NO_POLICY=1` forces every policy off. Off by default and unverified against the real provider until spike 4.
- Dry-run scoring in `replay serve`: after every turn the proxy re-scores the session's candidate layouts (cache TTLs, context editing) with the same simulator `replay replay` uses, from measured usage, and publishes them per session on `/replay/status` as `what_if` rows with a `vs_as_run` delta and a live-reachability note. Nothing is sent to the provider; the live figures equal an offline replay of the same ledger, by test. A summary line is logged every ten requests.
- Prefix-change detection in `replay serve`: each record carries a content-free hash of the tool definitions and system prompt as sent, so a cache break caused by a changed prefix is named with certainty rather than inferred, and `/replay/status` counts `prefix_changes` per session. The offline diff uses the same hash on ledger data.
- Live cache-break detection in `replay serve`: each response's cache read is classified against the previous request the moment it arrives, logged with the deficit and likely cause, and recorded on the ledger entry. `/replay/status` (JSON per-session totals) and `/replay/metrics` (Prometheus text) endpoints.
- Dollar caps and an error budget in `replay serve`, off by default: `--max-session-usd` and `--max-day-usd` cap list-price spend the way the token caps do; `--error-budget <share>` refuses a session's next request once that share of its prompt tokens carried error content, with the same override header. `/replay/status` shows `error_share` per session.
- Retries in `replay serve`, off by default: `--retries` resends on rate limit, overload, server error, or connection failure with doubling jittered backoff, honors `Retry-After` under a cap, never retries a client error or a started stream, records the count on the ledger, and exposes `replay_retries_total`.
- Guards for `replay serve`, off by default: token caps per session and per UTC day (fail closed before the next request, override header logged), loop detection over identical tool calls (warn header or refusal), and a provider circuit breaker with `Retry-After`. Refusals use the provider's error shape so agents show them.
- `replay`, `blame`, and `diff` accept ledger files and report at the measured tier, with the system prompt and tool definitions visible as the first context message.
- Spike 3 answered from documentation: subscription-authenticated Claude Code routes through `ANTHROPIC_BASE_URL` and stays on the subscription when no gateway credential is set. The gateway protocol facts the proxy must honor are recorded in `docs/architecture/proxy-protocol.md`.
- Release automation: GoReleaser builds for Linux, macOS, and Windows on amd64 and arm64 from a pushed tag, with a software bill of materials and Sigstore keyless signing of the checksum file; the workflow refuses tags that are not on `main`.
- Repository scaffold: Go module, `replay` command skeleton with `version` and `help`, Makefile, CI on Linux, macOS, and Windows, golangci-lint, Markdown lint, Dependabot, issue and pull request templates, label set, weekly stale-issue housekeeping.
- Governance and community documents: Apache 2.0 license and NOTICE (ADR-0005; the scaffold's initial BSL 1.1 draft was replaced before any release), contributing guide, code of conduct, security policy, support guide.
- `docs/ROADMAP.md`, `docs/maintainers.md`, ADR process with ADR-0001, PRD v4.0.0 and its adversarial review under `docs/`.
- PRD v5.0.0 (`docs/requirements.md`): replay-first product, two-tier truth labels, provider-sanctioned policy catalog, scoped rehydration, gating spikes, and the release sequence. ADR-0002 to ADR-0004 record the decisions. Red/blue review of the full design under `docs/evidence/`.
- **A break says which tools changed.** "System prompt or tool definitions
  changed" covered every prefix change and named a cause that mostly did not
  happen. Two narrower causes join the vocabulary, `tool definitions changed`
  and `system prompt changed`, and the new ledger field `cause_detail` names the
  tools: *"added 39 tool(s): mcp__claude_ai_Calendly__... ; removed 1 tool(s):
  WaitForMcpServers"*. The break log line also names the lane.
- **Per-lane breakdowns on `/replay/status`**: `context_by_lane`,
  `re_reads_by_lane`, `what_if_by_lane`. The existing keys keep their shapes and
  now mean the main loop specifically.
- **A pre-flight deficit warning**, off by default. Warns before a changed
  prefix is re-billed and refuses only against a ceiling the operator set.
  Consent is config, never a request header. A ceiling inside the estimate's own
  ±15% band suppresses the refusal rather than deciding it.
- **A throughput guard for `rescore`**, which re-walks the whole lane per
  request. Measured at 17.8 ms of proxy CPU for a 200-request session; the guard
  asserts a growth ratio, so it catches a regression without a fragile
  wall-clock bound. **The cost is measured, not fixed.**

### Changed

- **`replay cost` and `replay corpus` default to the transcript root the binary already discovers.** Both required a directory argument while `replay doctor` had, one command earlier, printed exactly the path they needed. A first command that asks for a path the reader does not have yet is a command they do not run. With no argument they now read `$CLAUDE_CONFIG_DIR/projects` (or `~/.claude/projects`), confirmed to contain at least one transcript before it is offered, and name the root on stderr so the report never leaves you guessing what it read. An explicit argument still wins, and a machine with nothing to find gets the usage error rather than an invented default.

- **`replay cost` keeps an index instead of reparsing every transcript.** Over 1,483 transcripts the report cost 6.3s wall and 19.6s CPU, and almost all of it was reparsing files that had not changed since the last run. Priced units are now cached in `~/.replay/cost-index.json`, keyed by the price table and rules versions so a rules change invalidates the whole file rather than serving figures computed under the old ones. Per entry the file's size **and** modification time must both match: a restore from backup, a checkout, or clock skew each produce a changed file under an unchanged timestamp. A corrupt or unreadable index is a cache miss, never an error — an optimisation that can fail the command it accelerates is a worse trade than the work it saves.

- **The tip ask is rate-limited to once every thirty days.** It printed on every run of `replay cost` over the threshold, which turns a thank-you into a nag and trains the reader to skip the last paragraph of a report whose last paragraph sometimes matters. State lives in `~/.replay/tip.json`. A finding at least three times the amount at the last ask re-opens it early, because a materially larger number is new information rather than a repeat. Two wordings are split by a per-machine random seed — random and local, because a hostname or a hardware id would be an identifier — so which arm a machine gets is stable without anything being tracked.

- `replay <transcript|dir>` runs the replay analysis without naming the subcommand, so the tool is no longer `replay replay <path>`. A first argument that names something on disk is taken as the analysis's own argument; a subcommand name always wins, a leading flag is never a path, and a mistyped command is still an error rather than a silent path. `replay replay <path>` keeps working.
- First calibration corpus on real traffic ([`docs/evidence/calibration-corpus-2026-09-03.md`](docs/evidence/calibration-corpus-2026-09-03.md)): 11 real sessions, 402 compared turns, 99.00% of provider cache reads reproduced, every session above the 95% threshold and every mismatch classified. Spikes 1 and 2 now have real evidence across two client versions; the corpus is one project on one machine, so the twenty-session half of the gate is still open.

- **The project is now Replay** (was Buffy). This renames everything a user touches: the binary and module path (`github.com/RedRobotKK/Replay/cmd/replay`), the state directory (`~/.buffy` becomes `~/.replay`), the endpoints (`/replay/status`, `/replay/metrics`), every metric name (`replay_*`), the headers (`x-replay-token`, `x-replay-override`, `x-replay-warning`), the environment variables (`REPLAY_DISABLED`, `REPLAY_NO_POLICY`, `REPLAY_TOKEN`, `REPLAY_UPSTREAM`), and the masking placeholder prefix (`REPLAY_SECRET_`). Nothing is migrated: move `~/.buffy` to `~/.replay` by hand if you were running the old name, and note that placeholders in an existing vault carry the old prefix and will no longer be recognised. Done before any tagged release, so no published artifact is affected.

- CI runs the test suite once more under the latest released Go, not only the version pinned in `go.mod`. The shutdown bug above reached `main` because nothing exercised a newer standard library; only the release pipeline, which uses the toolchain GoReleaser requires, caught it.
- README and roadmap now describe the replay-first sequence (`replay`, `blame`, `diff`, then `serve`).
- Ledger schema 2: records carry provider-named usage fields and a typed break cause. Files written by schema 1 are skipped by the reader rather than misread; delete `~/.replay/ledger` from earlier builds.

[Unreleased]: https://github.com/RedRobotKK/Replay/compare/main...HEAD

### Fixed

- **`replay doctor` and `replay cost` reported 90 and 1477 over the same directory, one second apart.** Neither figure was wrong and nothing on screen said they were counting different things. Claude Code writes one transcript per session at `<project>/<sessionId>.jsonl` and one more per sub-agent lane at `<project>/<sessionId>/subagents/agent-*.jsonl`; measured on the machine that produced those two lines, 91 of the first and 1403 of the second. doctor printed the session count only because its glob was never recursive — its number was right, and it described 6% of what the next command would read. doctor now reports both, the session count and the transcript-file total, with the reason they differ, and prints the second only when it differs from the first. The total is asserted against `transcriptFiles`, the walk the other commands actually do, rather than against a literal that could drift away from it. This is the share-card lesson recurring in a second place; M48–M50 are frozen against it.
- **`replay cost` used both words for one thing inside one page of output.** The headline said "transcripts" and the two lines beneath it said "sessions" about the very same files.

- **The cost total was presented as exact while double-counting a small share of requests.** A sub-agent lane re-renders its parent's requests, so the same request appears in more than one transcript file and was priced once per file. The report now counts distinct request ids alongside the total and discloses the overlap in its own sentence, with the direction of the error stated: measured with the real parser over 1,484 files, 430 of 30,716 requests, 1.4%. Deduplicating silently would have been the wrong fix — the overlap is a real property of how the client writes transcripts, and a reader who does not know it exists cannot judge any per-file figure.

- **The fan-out premium was corrected the same day it was published.** Presented as a measured discovery that parallel cost is superlinear, it is substantially an identity: observed is `1.25 * sum(P_i)` and the counterfactual `1.25 * P_max + 0.1 * P_max * (k-1)`, which divide into an empirical dispersion ratio times `1.25k / (1.25 + 0.1(k-1))`. The second factor holds no data and exceeds 1 for every `k > 1`, so **no corpus can produce a premium below 1** — a quantity that cannot come out low is not evidence. Measured values sit at 88–99% of that ceiling, leaving the dispersion ratio (≈0.9, flat) as the only empirical content. The offered robustness — barely moving across 30–300s grouping windows — was the same error twice, since neither `k` nor the multipliers depend on the window. The cost is real and the number stays; the claim around it changed, on the evidence doc, the site, and `PRODUCT-DIRECTION.md`. The "4.2x" in that document was never measured at all, and is within rounding of the arithmetic ceiling for six lanes (4.29).

- `replay serve` shut down cleanly only when no client held an unused connection. A coding agent keeps pooled connections open without a request on them, and those never become idle, so Ctrl-C waited the full five-second grace period and then exited non-zero with `context deadline exceeded`. Connections with no request in flight are now closed at once, turns in flight still get the grace period, and a turn that outlasts it is closed rather than holding the proxy open. Ctrl-C with a pooled connection open went from 5.006s and exit 1 to 4ms and exit 0.
- **The proxy compared each agent lane against a different one.** `prefixHash`,
  `last`, `model`, `lastSeen`, `context`, `re_reads` and `what_if` were
  session-wide single slots, so in any session running sub-agents each lane was
  measured against whichever sibling wrote last. This forged cache breaks and
  overruns, blamed lanes for model changes belonging to siblings, and let a
  quiet sub-agent erase a busy one's context breakdown. `cachemodel`'s own
  vocabulary always said lane: `ReadFirst` is documented as "the first request
  in a lane" and the proxy gated it on a session-wide request count. Now one
  `laneState` per lane.
- **Opening a sub-agent lane counted as a cache hit.** An unseen lane's usage is
  the zero value, so the expected read was 0, an opening request read 0, and the
  two matched exactly and scored "reproduced". Every sub-agent lane in a fan-out
  contributed a fabricated hit and inflated the cached share on
  `/replay/status`. It produced no error and no anomaly, only a better-looking
  success metric.
## [0.3.0] - 2026-09-05

A minor release rather than a patch: it adds a second provider path.

### Added

- **OpenAI-compatible requests are read, guarded and ledgered.** `/v1/chat/completions` is parsed, streaming included, so the spend cap, error budget and loop detector apply to Cursor and DeepSeek traffic. (This line named Grok until 2026-09-06 and was wrong: Grok posts to /responses at cli-chat-proxy.grok.com, which this build forwards and does not parse.) This family reports no usage on a stream unless the client sets `stream_options.include_usage`, and clients do not, so Replay sets it when the client left it unset (ADR-0003's first admissible kind). **Verified against a test stub and never against a live provider**, and secret masking does not cover this path; the proxy warns about both at runtime and counts `replay_unmasked_requests_total`.
- **A normalised usage record** (`internal/usage`) with the provider's own payload kept verbatim on the ledger. Anthropic counts exclusively and OpenAI inclusively, so an adapter that copies rather than subtracts double-counts the cache by an amount that grows with the hit rate; `FromInclusive` subtracts and `Validate` refuses a record whose parts do not add up.
- **Rules documents carry claims**: what a provider documents beside what replaying real traffic showed, with the verdict derived from the two. A file that writes its own `status` is refused. `contradicted` is the value no provider dashboard will show you.
- **Cost on `/replay/metrics`**: `replay_cost_usd_total`, `replay_cost_usd_day`, and a count of traffic the rules could not price. The endpoint carried no cost figure at all before.
- `replay doctor` reports which guards fired, what today cost, and warns when a dollar cap cannot be applied to some traffic, naming `--max-day-tokens` as the fix rather than describing it.
- `replay route`, `replay trim` and `replay advise --guards`, and [an alerting guide](docs/guide/alerting.md) for the twenty-one metrics.

### Fixed

- **Four counters were summed over a session map that evicts** past 256 sessions, so they lost whatever had been dropped and could fall between scrapes. Prometheus reads a falling counter as a reset, so every `rate()` over them was wrong on a busy machine and right on an idle one. Measured: 2,688,000 prompt tokens reported against 8,064,000 observed.
- **The error budget divided one agent lane's errors by every lane's tokens.** A quiet sub-agent rescoring after a busy one dropped the numerator to zero, so the guard stopped seeing the errors it exists to catch.
- **Policy trial breaches were counted process-wide**, so evidence gathered against one trigger reverted a different one; and the reverted flag was global, so reverting any policy disarmed the guardrail for every policy after it.
- **The installer verified the download and never the result.** It now runs the binary before reporting that it installed one.
- Flag hoisting was value-blind and separated a flag from its value, breaking every value-taking flag placed after a path.

### Changed

- CI is green for the first time since before v0.1.0.
