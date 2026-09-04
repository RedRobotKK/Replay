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
