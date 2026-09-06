# Commands

Sixteen commands, listed on nineteen lines because three of them have a form worth
showing separately. Most people use three of them.

`replay <command> --help` prints that command's flags to stdout and exits 0, and is always the
authority on what your installed version actually has — this page can drift, the binary cannot. It
worked on none of them until 2026-09-05: every subcommand exited 1 with its usage on stderr, and
`redact --help` tried to open a file called `--help`.

## The three you will use

### `replay <path>`

The default. Give it a transcript directory or a ledger directory and it reproduces the provider's
caching turn by turn, reports how well that reproduction matched the provider's own numbers, then
scores alternative context layouts against what actually ran.

```sh
replay ~/.claude/projects/your-project/     # estimated, from transcripts
replay ~/.replay/ledger/                    # measured, from the wire
```

Add `--dollars` for a list-price column. The output names the date of the price table it used.

### `replay doctor`

Tells you what Replay can see on this machine: transcript directories, whether a proxy variable is
set, whether a proxy is running, whether a ledger exists. It ends by naming the next command worth
running. Start here when something is not behaving.

### `replay cost`

The unit-economics view: what one task cost, and the share of it nobody chose.

```sh
replay cost ~/.claude/projects/            # the summary
replay cost --per-task ~/.claude/projects/ # every session, most expensive first
replay cost --json ~/.claude/projects/     # for a dashboard, or an agent
```

There is deliberately no mean. One very long session drags an average somewhere no real task lives,
so it reports the median and the p90, which is the spread you need before you can price a feature.
The avoidable figure prices tokens the provider re-billed after a cache break: money already spent
twice, not a projection of what a different layout might save. Sessions whose model is not in the
price table are excluded and counted, never treated as free.

`--share` prints a block designed to be posted:

```sh
replay cost ~/.claude/projects/ --share
```

It carries the avoidable rate, the median and p90 task cost, the session count and the break count —
and deliberately not the total. A total tells a reader your monthly burn and lets them infer team
size; it is also the least comparable number in the set, because $3,000 means nothing without
knowing how many engineers spent it. A rate reads the same from a solo developer and a team of
fifty. No paths, no project names. The card goes to stdout and its note to stderr, so
`replay cost <dir> --share | pbcopy` copies exactly what is safe to paste.

A plain `replay cost` run also names the tip jar once, under the figures, when the avoidable amount
is over $5 — at the one moment the tool has just shown you money you already spent twice. It prints
a line; it never opens a browser.

### `replay cost --compare <date>`

The only measurement here that a provider invoice could contradict, which is what makes it the one
worth running.

```sh
replay cost --compare 2026-08-25 ~/.claude/projects/
replay cost --compare 2026-08-25 --predicted -0.2 ~/.claude/projects/
```

It splits sessions at the date and reports cost **per task** on each side. Per task, because total
spend also falls when you simply do less work, and that is how a report like this most easily
misleads. Task volume is printed on both sides so you can judge the mix yourself, and a swing beyond
40 percent warns that the two periods are not comparable work. Fewer than ten tasks on a side prints
no figure at all.

`--predicted` grades a forecast against what happened. The band is wide, because this is an estimate
compared against real spend, and symmetric, because badly beating a prediction is also a failed
prediction: it means the model does not understand the effect it claims.

This is list price against transcripts, not your invoice. It says so in its own output.

### `replay statusline`

Live spend, cache health, and what the misses are costing, in Claude Code's status line.

```sh
replay statusline --install   # prints the settings.json snippet
```

Claude Code already reports what a session has cost. What it cannot report is how much of that was
avoidable, because it counts cache misses in tokens and does not price them. This does, on the JSON
Claude Code already hands a status line, in about 6ms per render. It opens no files and makes no
network call.

It will not price a model that is not in the price table, and when its own figure exceeds what the
session actually cost, the two disagree and it shows the cause without the number.

`--no-color` never emits ANSI escape codes. Colour is already suppressed when the output is not a
terminal; this is for the case where it is a terminal and the escapes are still unwanted — a status
line rendered into a bar that shows them literally, or a log that keeps them.

### `replay rules`

Shows the provider rules in effect and where they came from, or installs a dated document.

```sh
replay rules                                  # what is in effect now
replay rules --update ./rules.json            # install from a file
replay rules --update https://... --dry-run   # validate without installing
```

Provider numbers change faster than release cycles. Compiled in, a change means a binary release and
every report is quietly wrong until it ships. Loaded from `~/.replay/rules.json`, the same correction
is a download, and every report names the rules that produced it.

Loading is strict, because a stale rules file announces itself in the version string and a wrong one
does not. It refuses an unknown schema, a missing version, an empty match that would match every
model, a negative price, a model marked priced with no price, or a read multiple outside 0 to 1.

There is no background refresh and no default source. A URL is a network request, and the promise
that Replay makes none except to your provider survives only because a person has to type it.

```sh
replay rules --check-prices                   # is the compiled table still right?
```

`--check-prices` answers the question the date alone cannot. Every dollar Replay prints comes from a
table compiled into the binary; before this, the only thing the tool could say was how old that
table was, and age is a prompt to worry rather than information.

It compares against an independent published price database and reports where the two differ. It
never installs anything. That source is a second observer, not an authority: it can be stale, wrong,
or describing a different SKU under a similar name, so a disagreement is a prompt to read the
provider's own page and update the table and its date deliberately — the same shape the rules
document uses for `documented` against `observed`, for the same reason.

Reseller keys are excluded on purpose. The same model appears under Bedrock and Vertex names at
different rates, and comparing a first-party table against a reseller's would report a difference
that is really a different product. A model the source does not name is reported as unchecked rather
than confirmed, separately from any disagreement, because absence of evidence is not evidence of a
difference.

This is the one command that reaches the network on your behalf, and it does so only when typed.

```sh
replay rules --export > rules.json             # the compiled table, as a document
```

`--export` writes the table compiled into this binary in the format `--update` installs. Two uses. An
air-gapped machine can be handed a document without running the binary that generates one. And it is
what makes "the free tier is complete" checkable rather than a claim: the free feed published at
`https://redrobot.jp/Replay/rules/free.json` is this output, so anyone can diff it against the paid
feed and see exactly what the money buys.

Any installed override is deliberately ignored — exporting whatever happened to be installed would
publish one machine's local state as though it were the product.

```sh
replay rules --measure ~/.replay/ledger > measured.json
```

`--measure` reads a ledger and emits a rules document carrying what the wire
actually showed about each model's caching floor, alongside what the provider
documents. It is the content of a maintained feed, and it answers a question no
price page does.

The evidence is asymmetric, and only one direction is sound from what the
ledger records today. A **cold** cache write — one with no cache read — proves
the floor is at or below the number of tokens written, because with no prior
entry what was written is what was cached. That figure is exact.

A **warm** write proves nothing about the floor. `cache_creation_input_tokens`
counts the tokens written by that request, not the size of the cached prefix, so
a turn that reads 20,000 cached tokens and writes 118 more has a 20,118-token
prefix. Reading the 118 as a prefix size errs downward every time, which is the
direction that manufactures contradictions: on this repository's own ledger it
reported opus-5 caching a 118-token prefix and therefore refuting a documented
floor of 512. Warm writes are discarded.

A **lower** bound is not derivable at all yet. It would need the prompt size at
the breakpoint for a marked request that cached nothing, and the proxy records
how many `cache_control` markers a request carried but never where they sat. So
claims come back `unverified` rather than `consistent`: an upper bound over an
open interval agrees with the documented figure without confirming it.

Only the proxy can answer this. Transcripts do not record what the provider
cached, so `--measure` needs sessions that went through `replay serve`.

#### Promotions, and rates only you can see

A vendor promotion is a pricing event, not a fact about traffic. The ledger
records tokens, cache reads and writes, and timings — none of which change
because someone ran a sale — so **a promotion never rewrites the ledger.** It is
a dated row in a rules document, and each request is priced by the rules in
effect at *its own* timestamp:

```json
{ "match": "opus-5", "inputPerMTok": 2.5, "outputPerMTok": 12.5,
  "effectiveFrom": "2026-09-01", "effectiveUntil": "2026-09-30" }
```

A dated row wins over an undated one while it is in effect, and the base rate
returns afterwards. Dates are inclusive and interpreted in UTC, so a promotion
ending on the 30th covers the whole of the 30th. Without this, a report
spanning the end of a promotion prices the entire period at one rate and is
wrong on one side of the boundary whichever rate it picks.

Two windows covering the same dates for the same model are **refused at load**,
not resolved. It would make the price depend on which line came first, and a
figure that depends on file order is not one to act on.

A negotiated account rate is different again: it is private, never appears on
the wire, and Replay cannot observe it. Ignoring it overstates every figure for
anyone who has one, so it can be stated instead:

```json
{ "accountDiscount": 0.85 }
```

The document's price tier then becomes **`declared`** rather than `documented`.
The discount is applied because you asked for it, and labelled because nobody
else can verify it. A multiplier outside 0 to 1 is refused — a negative one
would turn spend into savings, and both are far more likely to be a typo than a
deal.

#### When a feed asks to be paid

A rules feed may answer `402 Payment Required` with machine-readable terms
([x402](https://x402.org)). Replay reads those terms, prints them, and installs nothing:

```sh
replay rules --update https://redrobot.jp/Replay/rules/latest.json
replay rules --update https://... --x402-json   # the same terms, as JSON
```

**Replay will not pay, and cannot.** It holds no key and contains no code that can sign a
transaction; a test fails the build if any appears. Two reasons. A binary people install with
`curl | sh` onto machines holding provider credentials must not also be a wallet, because that turns
every supply-chain risk into a wallet compromise. And paying is a decision rather than a step: an
agent pays from a wallet its operator funded and budgeted, and that operator authorised the agent,
not this tool.

So the flow for an agent with a wallet is to fetch the document itself and install it from a file:

```sh
replay rules --update ./rules.json
```

`--x402-json` prints the seller's terms as JSON — amount, network, payee, asset — alongside an
explicit `"paid": false`, so a spending policy can decide. The command exits **2**, distinct from
`1`, so a script can tell "this resource costs money" from "this is broken" without parsing prose.
Nothing is blocked either way: the compiled rules are complete and every command works on them.

`--update`, `--export` and `--check-prices` each do a different thing and are
refused together rather than silently preferring one. `replay rules --update
<url> --export` used to exit 0 with a price table on stdout, having fetched and
installed nothing — a script reading that as a successful update would be wrong
and never find out.

A redirect from `https` to plain `http` is refused mid-chain, not just on the
URL you typed, and the provenance recorded in the installed document names the
address the bytes actually came from rather than the one you asked for.

The reasoning is [ADR-0013](../adr/0013-x402-rules-feed.md).

### `replay serve`

Starts the local proxy on `127.0.0.1:4000`. Point your agent at it with
`export ANTHROPIC_BASE_URL=http://127.0.0.1:4000` and every figure moves from estimated to measured.
Flags are below.

## The rest

### `replay blame`

Ranks what is eating your prompt tokens **within one session**: which files, and which blocks.
Run it per session; `replay advise` is the one that aggregates. Useful when the bill is high and you do not know which part of your setup is
responsible.

### `replay diff`

Points at the exact turn where the cached prefix diverged, and classifies why. Use it when `replay`
reports cache breaks and you want to know what broke them.

### `replay corpus <dir>`

Produces a calibration summary across every session in a directory, as Markdown, with no paths, no
project names and no message content. It is designed to be safe to share, and it is how you would
contribute a calibration report without handing over your work.

```sh
replay corpus ~/.claude/projects > corpus-report.md
```

Open it and check that nothing in it identifies your projects before you share it. It judges
calibration per model with the newest sessions separated, so a provider changing its behaviour shows
up as a provider change rather than as silent drift.

### `replay context <transcript|dir>`

What entered a session's context, by tool, ranked by share of content bytes.

```sh
replay context ~/.claude/projects/some-project/     # ranked attribution
replay context <transcript> --top 30                # more than the default 12
replay context <transcript> --json                  # for a script
```

```text
Session facfd32e  998k tokens of content entered this context

  mcp__claude-in-chrome__javascript_tool     31.2%   311k  x814  *
  mcp__claude-in-chrome__computer            27.6%   275k  x634  *
  assistant                                  21.6%   216k  x643
  Bash                                        3.6%    36k   x64  *
```

`blame` ranks what is eating prompt tokens across a whole lane and is the one to reach for when the
bill is the question. `context` answers a narrower one: for this session, where did the content come
from. The columns are share of content bytes, the bytes themselves, and how many times that source
appeared. A `*` marks a figure estimated through the byte-to-token fit rather than measured on the
wire.

**`--top` truncates silently, including under `--json`.** The default is 12, so a script reading the
JSON gets twelve rows and no indication that more existed. Pass a larger `--top` when the output is
being parsed rather than read.

### `replay learn`

Scores candidate context layouts from your own history and selects one. It refuses to score
alternatives for any model whose calibration looks unreliable, which is the point: a recommendation
built on a session the engine does not understand is worse than no recommendation.

`--min-sessions` is how many sessions with evidence a candidate needs before it can be selected
(default 5). Raising it is the conservative direction: fewer selections, each on more evidence.
Lowering it lets a layout be chosen on a handful of sessions, which is how a recommendation ends up
describing last Tuesday rather than how you work.

`--out` names the policy file written on selection (default `~/.replay/policy.json`); `--out -`
writes none and prints only.

### `replay redact <transcript.jsonl>`

Writes a redacted copy of a transcript to standard output. Use it before attaching a transcript to a
bug report.

```sh
replay redact session.jsonl > redacted.jsonl
```

### `replay advise`

Aggregates across sessions and suggests changes worth making, then tracks whether a suggestion was
later borne out. Each suggestion is `pending`, `verified` or `not verified` on a subsequent run.
Unlike `blame`, this one does look across sessions.

`replay advise <dir> --guards` suggests spend caps from your own session spread using Tukey's upper
fence, `Q3 + 1.5*IQR`. It prints the quartiles, the median and the session count behind them, refuses
below ten sessions rather than dressing up a guess, and refuses a sample with no spread, because a
fence over identical sessions sits on the typical session and would refuse ordinary work. Print-only:
it cannot be combined with `--apply`, because a spend cap the tool set for you is a refusal you did
not choose.

`--apply` proposes the one setting the evidence can decide and shows the diff; on its own it only
describes the change, and `--yes` is what actually writes it. Two steps rather than one because a
tool editing your settings unasked is a different thing from a tool suggesting an edit.

`--out` names the advice file that tracks whether a suggestion was later borne out
(default `~/.replay/advice.json`). Pass `--out -` to keep no state, which makes each run independent
and gives up the `verified` / `not verified` follow-up that the tracking exists for.

### `replay route <dir> --to <model>`

What switching models would change, in two halves, only one of which it will answer without evidence.

The structural half prints unconditionally: break-even trim thresholds and the cache-read inversion
boundary, built from read multiples, write penalties and a price ratio. No token count enters them,
so no tokenizer can move them. Above the inversion boundary a model that costs more per input token
can be cheaper per turn, because its cache-read multiple is better.

The dollar half needs sigma, the ratio between two tokenizers on the same content, and sigma is
measured from both sides of your ledger or the figure is suppressed. There is no fallback of 1.0 and
no constant on a rate card: at a 99% cached share a comparison can break even at sigma = 1.0627, so a
plausible-looking 1.15 would not be a safety margin, it would be the deciding vote cast by a number
nobody measured.

### `replay trim <dir> --cap <bytes>`

What a per-block byte cap on tool output would have saved, and what it would have cost you.

Savings are priced as cache reads, which is what a resent byte is, with the fresh-input figure printed
beside it and the ratio between them, because a token-share report implies the larger number and it is
about ten times too big. The harm probe then asks, for every region the cap would remove, whether the
agent later depended on it: a later `Edit` whose `old_string` sat only in the removed part, a re-read
of the same path, a quote of a removed line.

The probe is a **lower bound** and prints its own blind spots. `Write` has no `old_string`, line
numbers carried into a later `Read` are invisible to it, and removing test failures produces *fewer*
later edits, which it would score as a saving rather than as damage.

Nothing is trimmed and no request is touched. The live trimmer does not exist, and this command is
how that was decided: on the development corpus the whole prize was $4.70.

### `replay probe --model <id>`

Measures where a model's prompt cache actually starts, by sending requests
designed to find out.

This is the only command that originates a billable request. Everything else
reads files or forwards what an agent already sent; this creates traffic with
your credential and spends your money. So it **plans by default and sends
nothing**, and `--execute` is what makes it run:

```sh
replay probe --model claude-opus-5                 # what it would do, and what it would cost
replay probe --model claude-opus-5 --execute       # actually send them
```

The key is read from `ANTHROPIC_API_KEY` in the environment and is never a
flag. A credential on a command line is recorded in shell history and visible
in the process table to every other user on the machine. `ANTHROPIC_BASE_URL`
is honoured, so probes can be sent through `replay serve` and land in the
ledger.

**Why this exists.** Ordinary traffic never sends a small prompt with a cache
breakpoint on it — nobody caches a 600-token prefix by accident. Passive
measurement across a real ledger produced one loose bound from four sessions:
floor at most 36,635, which agrees with a documented 512 and confirms nothing.
The evidence that would tighten it has to be made on purpose.

**How it works.** A probe is one request carrying a cache breakpoint at a known
prefix size. If the provider writes a cache entry the floor is at or below that
size; if it writes nothing the floor is above it. Bisect. Each probe's content
is unique, because repeated content caches on the first probe and is then
*read* by every later one — and a read tests nothing, so the run would cost
full price and learn nothing.

| Flag | What it does |
|---|---|
| `--model` | The model to measure. Required |
| `--min`, `--max` | Bracket where the floor could be. Defaults 0 and 65536 |
| `--resolution` | How narrow a bracket is narrow enough, in tokens. Default 512 |
| `--relative` | Stop within this fraction of the answer instead of a fixed width. `--relative 0.1` is "within ten percent", which is the same statement at every scale — 128 tokens is a quarter of 512 and two thousandths of 65,536 |
| `--max-probes` | How many billable requests the run may make. Default 16 |
| `--confirm` | Agreeing answers required before a boundary is believed. Default 2 |
| `--candidates` | Plausible floors to test before searching between them. Defaults to `512,1024,2048,4096`; empty disables it |
| `--prior` | A documented floor to test before searching. Defaults to the compiled table's figure for the model; `-1` disables it |
| `--execute` | Actually send them. Without it, only the plan is printed |

**`--confirm` multiplies against `--max-probes`.** Every confirmation is a
billable request, so 16 probes at 2 confirmations buys 8 bisection decisions,
not 16. The plan says how many decisions the budget affords, and warns before
anything is sent if that cannot reach the resolution asked for.

**What it reports is a bracket, not a value.** The floor is above one size and
at or below another; the exact value inside that gap was never tested, and
naming it would claim precision the probes did not buy. Two results are not
brackets at all and are reported as findings in their own right: a prefix that
cached at one size and failed at a larger one, and a prefix that cached on one
request and not the next. Neither is averaged away — a floor that holds
sometimes is not a floor, and that is the most interesting thing a run can
find.

It also infers **block granularity** from the greatest common divisor of the
sizes actually cached, and stops bisecting below it: if writes land on
1024-token blocks then no floor between multiples of 1024 is observable, and
probing for it spends money distinguishing sizes the provider treats as the
same. Probe points are deliberately offset from the power-of-two grid, because
a clean bisection proposes 32768, 16384, 8192, whose divisor is an artifact of
the search rather than a fact about the provider.

**It tests the plausible answers before the space between them.** Every
documented floor in the table is a power of two, and a new model almost
certainly lands on one too. Testing that list directly is O(1) in the size of
the range where bisection is O(log n) — which matters most for an undocumented
model, where the search window has to be wide. Candidates are tried nearest the
middle of the remaining bracket first, so one that is refuted still halves the
space rather than shaving an end off it, and the answers they give narrow the
bracket like any other. A model that breaks the pattern is still measured
correctly; it costs the few probes spent finding that out.

**It tests the documented figure first.** A published minimum is a hypothesis,
and the cheapest experiment tests the hypothesis rather than bisecting the space
that contains it. The run probes the documented size, then the size just below
it — because "the floor is exactly 512" predicts both that 512 caches and that
511 does not. When the documentation is right that settles it in two decisions,
where a blind bisection of 0–2048 needs nine. When it is wrong, the prior is
refuted by its own probe and the bisection continues from the bracket those
answers established, having spent one probe to find out.

**Probe content is varied CJK, and both words matter.** Measured on this API,
English `filler` repeated with a trailing space is about 3.4 characters per token, so a character is
a blunt dial and the reachable token counts are sparse. Varied Han ideographs
count at almost exactly 2 tokens each — 200 runes to 420 tokens, 201 to 422,
202 to 424 — perfectly linear, which is what makes a three-token bracket
reachable at all. *Varied* is load-bearing: a repeated character compresses, so
`あ` a hundred times counts 60 tokens and the hundred-and-first adds nothing,
because the tokenizer merges the run.

The ratio is learned from the first probe rather than assumed, so a model whose
tokenizer differs corrects itself, and every probe still verifies its own size.

Feed the result into a rules document with `replay rules --measure`.

### `replay version`

Version and build commit.

## Flags on `serve`

All guards are off unless you turn them on. Nothing below changes a single byte on the wire; they
decide whether a request goes out at all.

### Spend

| Flag | What it does |
|---|---|
| `--max-session-tokens`, `--max-day-tokens` | Refuse the *next* request once the cap is reached. Never a response already in flight |
| `--max-session-usd`, `--max-day-usd` | The same, priced from a dated table. A model not in the table counts as free |

A refusal arrives as a provider-shaped error your agent will show you. Send
`x-replay-override: <reason>` to proceed once.

### Failure

| Flag | What it does |
|---|---|
| `--error-budget 0.3` | Refuse the next request once that share of a session's prompt tokens carried error content: failed tools, failed edits, repeated identical calls, overflow notices |
| `--loop-warn`, `--loop-block` | Count how many times in a row the agent has made the same tool call with the same input, then warn or refuse |
| `--breaker-failures`, `--breaker-cooldown` | Open a circuit after consecutive provider failures and answer locally with `Retry-After` until the cooldown passes (default 30s) |
| `--retries`, `--retry-base`, `--retry-max` | Resend on rate limit, overload, server error or connection failure, with doubling jittered backoff |

The error budget is designed to catch a stuck agent long before a spend cap would, because an agent
looping on failures wastes money before it has spent much. Note that when both would refuse the same
request, the spend cap is evaluated first and its message is the one you see. Sessions under ten thousand prompt tokens are never judged.

Retries have two rules worth knowing. A rate limit, overload or server error is retried, because the
provider answered and said to try again. A **transport** failure is only retried when the connection
never opened: if any byte of the request has already gone out it may already have been billed, so it
is not resent. And nothing is ever retried once a byte of the *response* has reached the client.

### Secrets

| Flag | What it does |
|---|---|
| `--mask`, `--mask-patterns`, `--mask-entropy` | Detect and mask secrets in traffic, using a maintained pattern set, your own patterns, and an optional entropy heuristic |
| `--rehydrate` | With `--mask`, restore placeholders in responses. On by default. Turning it **off** leaves the placeholders in place, which is how you evaluate coverage: whatever the agent then trips over was masked, and whatever still works was not |
| `--project` | With `--mask`, the directory under which file-edit tool inputs may receive real secrets. Defaults to the current directory. It is the boundary that stops a rehydrated secret being written into a file outside the project you are working in |
| `--rehydrate-scope` | With `--mask`, where a pattern's secrets may be restored, as `name=dest[,dest]` with `dest` one of `text`, `edit`, `tool:NAME` or `none`. `name=*` sets the default (`text,edit`). Repeatable. Narrowing a scope keeps a secret out of destinations that persist it — a file write, a named tool — while still letting the agent read it in conversation |

The README names the patterns covered and has a section on what masking does **not** catch. Three
things worth knowing before you rely on it: bare hex and lowercase secrets are invisible to the
entropy test by design, because a 32-character Twilio token and a git SHA are the same shape and
masking every hex run would corrupt the diffs your agent reads; those are caught only when a name
like `TOKEN=` or `"api_key":` sits beside the value; and if the vault cannot be written, the request
is forwarded **unmasked** with a `MASKING FAILED` log line that is easy to miss in a backgrounded
proxy.

### Access

| Flag | What it does |
|---|---|
| `--listen` | Address to bind. Loopback only |
| `--upstream` | Provider base URL. Default `https://api.anthropic.com`. This is how the proxy is pointed at an OpenAI-compatible provider, or at another proxy, and it is the only setting that changes where your traffic goes — so it is worth reading twice |
| `--token`, or `REPLAY_TOKEN` | Require `x-replay-token` on every request |
| `--ledger` | Where ledger files are written. Default `~/.replay/ledger`, owner-only |

Browser-originated requests are refused regardless.

### Live policy, experimental

| Flag | What it does |
|---|---|
| `--context-edit-trigger`, `--context-edit-keep` | Ask the provider to clear old tool results once the prompt passes the trigger, keeping the most recent N |
| `--policy-file` | Apply the candidate `replay learn` selected, read at each session's first request |
| `--trial-share`, `--guardrail-reread`, `--revert-after` | Run a learned policy as a bounded trial: a share of new sessions get it, the rest are controls, and it reverts automatically if the re-read rate crosses the guardrail |
| `--hold-siblings` | Make a request whose prefix is already in flight wait for the first response to begin, so parallel sub-agents read the cache entry instead of all paying to write it |

`REPLAY_NO_POLICY=1` forces every policy off. `REPLAY_DISABLED=1` refuses to start at all.

## Endpoints

While `serve` is running:

- `GET /replay/status` returns per-session totals as JSON: requests, prompt tokens, cached share,
  breaks, prefix changes, list cost, and what each alternative layout would have done. It
  also reports `caps`, which spend limits are configured, as booleans rather than values, so a
  diagnostic that cannot see `serve`'s flags can tell whether a blind dollar cap is already
  covered by a token one.
- `GET /replay/metrics` exposes aggregate totals as Prometheus text. It has no per-session rows.

Both honour the token when one is set, and both refuse browser origins.

### The metrics

Twenty-one series. Thirteen carry no label; the eight that do are labelled by a bounded
operational category. **None is labelled by session or by model**, deliberately: a session
label is an unbounded, short-lived identifier, which multiplies cardinality in the scraper
and puts session ids in front of anything that can read the endpoint. The per-session view
is `/replay/status`, which you ask for.

| Metric | Type | Label | Counts |
|---|---|---|---|
| `replay_requests_total` | counter | `class` | Requests handled, by outcome class |
| `replay_prompt_tokens_total` | counter | — | Prompt tokens the provider processed |
| `replay_cache_read_tokens_total` | counter | — | Prompt tokens served from cache |
| `replay_cache_write_tokens_total` | counter | — | Prompt tokens written to cache |
| `replay_cached_share` | gauge | — | Cache reads over prompt tokens |
| `replay_cache_break_total` | counter | `cause` | Cache reads that fell short, by cause |
| `replay_cost_usd_total` | counter | — | List-price cost since start |
| `replay_cost_usd_day` | gauge | — | List-price cost for the current UTC day. A gauge because it resets at midnight, and a counter that resets makes every `rate()` wrong |
| `replay_cost_unpriced_requests_total` | counter | — | Requests the rules could not price. Independent of the doctor's unenforced-cap warning, which also needs a dollar cap configured |
| `replay_unparsed_requests_total` | counter | — | Requests on a path this build cannot read. **Excludes** `/v1/chat/completions`, which is read |
| `replay_unmasked_requests_total` | counter | — | Requests the masker does not cover. This is what `/v1/chat/completions` increments |
| `replay_refused_total` | counter | `guard` | Requests refused locally, by guard |
| `replay_upstream_errors_total` | counter | `status` | Provider responses with an error status |
| `replay_retries_total` | counter | — | Requests resent after a retryable failure |
| `replay_held_total` | counter | — | Requests held behind a sibling with the same prefix |
| `replay_held_milliseconds_total` | counter | — | Time spent in that hold |
| `replay_masked_total` | counter | `pattern` | Secrets replaced, by pattern name. Never a secret or a placeholder |
| `replay_rehydrated_total` | counter | `destination` | Placeholders restored, by destination |
| `replay_rehydration_denied_total` | counter | `destination` | Placeholders left in place, by destination |
| `replay_policy_applied_total` | counter | `policy` | Requests carrying a Replay-added parameter |
| `replay_request_latency_seconds` | summary | — | Request received to response finished |

Expressions for alerting on these are in [alerting.md](alerting.md).

The token, cache and break counters are lifetime totals. They were once summed over the
live session map, which evicts past 256 sessions, so they under-reported by whatever had
been dropped and could fall between two scrapes; a falling counter reads to Prometheus as
a reset, which makes every `rate()` over it wrong on a busy machine and right on an idle
one. Fixed 2026-09-05.

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
