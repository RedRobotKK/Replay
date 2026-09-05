# Commands

Ten commands. Most people use three of them.

Run `replay` with no arguments for the built-in usage text, which is always the authority on flags
your installed version actually has.

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

### `replay learn`

Scores candidate context layouts from your own history and selects one. It refuses to score
alternatives for any model whose calibration looks unreliable, which is the point: a recommendation
built on a session the engine does not understand is worse than no recommendation.

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
| `--breaker-failures` | Open a circuit after consecutive provider failures and answer locally with `Retry-After` until the cooldown passes |
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
  breaks, prefix changes, list cost, and what each alternative layout would have done.
- `GET /replay/metrics` exposes aggregate totals as Prometheus text. It has no per-session rows.

Both honour the token when one is set, and both refuse browser origins.

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
