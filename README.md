# Project Buffy

**Buffy shows you the turn your coding agent's prompt cache broke, what it cost, and what a better context layout would have saved, on sessions you have already paid for.**

It reads the transcripts your agent already writes, reproduces the provider's caching turn by turn, and only then scores alternatives. Later, as a local proxy, it applies the better layout live using only mechanisms the provider itself sanctions, and keeps improving from your own history. Everything runs on your machine. No API calls are spent on analysis. Nothing leaves.

> **Status: v0.1 and v0.2 in development, no release tagged yet.** `replay`, `blame`, `diff`, and `redact` work offline on Claude Code transcripts and have been calibrated against one real session so far; the 20-session corpus the roadmap requires is still pending. `serve` is a byte-for-byte passthrough proxy that records a derived-data ledger; it has been exercised against a fake provider, not yet against the real one. Follow [`docs/ROADMAP.md`](docs/ROADMAP.md).

[![CI](https://github.com/RedRobotKK/Buffy/actions/workflows/ci.yml/badge.svg)](https://github.com/RedRobotKK/Buffy/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/RedRobotKK/Buffy)](https://goreportcard.com/report/github.com/RedRobotKK/Buffy)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

## Why

Coding agents resend the whole conversation on every turn. Three things go wrong quietly:

- **You cannot see what a task cost** until the invoice arrives.
- **Prompt caches break silently.** A timestamp in a system prompt or a reordered tool list can double the bill with no error anywhere.
- **Secrets leave the machine.** API keys and tokens in tool output ride along in the prompt.

Buffy addresses each one locally, without changing how the agent works.

## What it will do

| Command | What you get |
|---------|--------------|
| `buffy replay` | Reproduces your sessions' caching, prints how well it matched the provider's own numbers, then scores alternative layouts in tokens saved |
| `buffy blame` | Ranks which files, tool descriptions, and instructions are eating your prompt tokens across all sessions |
| `buffy diff` | Points at the exact turn where the cached prefix diverged and classifies the cause |
| `buffy doctor` | What Buffy can see on this machine (transcripts, proxy variable, a running proxy, ledger) and the next command to run |
| `buffy corpus` | Calibration summary across every session in a directory, as Markdown with no paths or content, for reporting how well Buffy understands your sessions |
| `buffy serve` | Local proxy: byte-for-byte passthrough that records what the provider charged, so the three commands above run on measured data (policies and guards come later) |

Every number is labeled *estimated* (from transcripts) or *measured* (from the wire), with the calibration that justifies it. Release sequence, gates, and what is deliberately deferred: [`docs/ROADMAP.md`](docs/ROADMAP.md). Full requirements: [`docs/prd/buffy-prd-v5.0.0.md`](docs/prd/buffy-prd-v5.0.0.md).

## Quick start

No proxy, no configuration, no trust required. Build from source until the first release is tagged:

```sh
make build
./bin/buffy doctor                                  # what is on this machine, and what to run next
./bin/buffy replay ~/.claude/projects/<your-project>/
```

Real output from a Claude Code session, on the session in which Buffy itself was written:

```text
Tier: estimated (transcripts only)
Calibration: reproduced provider cache reads on 77/78 turns (7 read more than predicted: a sibling request extended the prefix); 1 cache breaks
Assumption: replayed savings assume the agent would have behaved identically under the alternative layout
Rules: anthropic-2026-09-01; user-content fit 0.469 tokens/byte ±64% from 34 turns; system prefix 39k (measured from the first request's cache read)

  policy                                    prompt tokens  cached share  vs as-run  misses  guardrail
  as-run                                           22.87M           97%          -       0
  ttl-5m0s                                         22.87M           91%       +35%       4  none
  ttl-1h0m0s                                       22.87M           97%        +0%       0  none
  context-edit(keep=6,trigger=239k) *              20.98M           79%     +206%       0  re-read rate and failed edits: unknown until a live trial

  turn 32 at 22:08:56 (+2m30s): read 39k of 228k expected, 189k re-billed
    cause: client re-rendered history after the system prefix (no edit visible in transcript)
    where: message 0 (user text)
    evidence: read 38987 tokens, about the size of the system prefix (38547); the message history was re-billed from the first message
```

Add `--dollars` for a list-price column computed from a dated first-party price table; the output names the table date because prices change and other platforms differ.

The calibration line is the point: Buffy first proves it can reproduce what the provider charged, and only then says anything about alternatives. Every figure is labeled estimated or measured. Here the 1-hour TTL the client chose reproduces as-run to the token, the 5-minute TTL would have cost 35% more across four idle gaps, and context editing would have cost more, not less, for this session shape. How it works: [`docs/architecture/replay-engine.md`](docs/architecture/replay-engine.md).

### Contributing a calibration corpus

The roadmap gate for the first release is calibration on twenty real sessions. If you have Claude Code transcripts, this produces a report that contains no paths, project names, or content:

```sh
make build
./bin/buffy corpus ~/.claude/projects > docs/reviews/calibration-corpus-$(date +%F).md
```

Open it, check that nothing in it identifies your projects, and commit it on a branch.

### Measured numbers with the proxy

Transcripts do not contain the system prompt, tool definitions, or cache markers, so figures from them are labeled estimated. The proxy sees all of it:

```sh
./bin/buffy serve                                  # listens on 127.0.0.1:4000
export ANTHROPIC_BASE_URL=http://127.0.0.1:4000    # in the shell that runs your agent
./bin/buffy replay ~/.buffy/ledger/                # same commands, measured tier
```

What the proxy does and does not do:

- Forwards every request and response byte for byte, including streaming, cache markers, and the `anthropic-beta` and `anthropic-version` headers. It never rewrites a body or removes a header.
- Forwards your credential and never stores or logs it. A Claude subscription stays the active credential when no API key is set, as the client's gateway documentation describes.
- Writes one ledger file per session with block kinds, sizes, labels, timings, and usage. No message text, ever. Ledger files live under `~/.buffy/ledger` with owner-only permissions.
- Binds loopback only, refuses browser-originated requests, and accepts an optional shared token (`--token` or `BUFFY_TOKEN`, sent as `x-buffy-token`).
- Fails open: if anything inside Buffy's own bookkeeping fails, the bytes still flow. If the provider is unreachable you get a 502 that says how to bypass Buffy.
- Off switch: `BUFFY_DISABLED=1` refuses to start; unsetting `ANTHROPIC_BASE_URL` bypasses it entirely.

Added latency, measured on 2026-09-03 with a 46KB request against a local fake provider on a 4-core Xeon, 300 requests after warm-up: p50 48µs, p99 98µs on top of the round trip. Provider latency is three orders of magnitude larger. The method is in [`docs/reviews/proxy-latency-2026-09-03.md`](docs/reviews/proxy-latency-2026-09-03.md).

While it runs, every response's cache read is checked against the expectation from the previous request, and a break is logged the moment it happens with the tokens re-billed and the likely cause. Because the proxy hashes the tool definitions and system prompt of every request, a break caused by a changed prefix is named with certainty, which a transcript can only infer.

After every turn the proxy also re-scores the session's candidate layouts (5-minute and 1-hour TTLs, context editing at two triggers) with the same simulator `buffy replay` uses, from measured usage. This is a dry run: nothing changes on the wire, and the live figures are exactly what `buffy replay` prints for the same ledger. `GET /buffy/status` returns per-session totals (requests, prompt tokens, cached share, breaks, prefix changes, list cost) and the `what_if` rows, each with a `vs_as_run` delta and how a user would turn it on, as JSON. `GET /buffy/metrics` exposes the totals as Prometheus text. Both honor the token when one is set and refuse browser origins. Every ten requests a session gets one log line naming its best candidate or saying none beats what ran.

Guards, all off unless you set them (see `buffy serve -h`):

- `--max-session-tokens` and `--max-day-tokens` refuse the *next* request once a cap is reached, never a response in flight. `--max-session-usd` and `--max-day-usd` do the same at list price from the dated price table (a model not in the table counts as free, and the status endpoint shows the same figure). The refusal is a provider-shaped error the agent shows you; send `x-buffy-override: <reason>` to proceed once.
- `--error-budget 0.3` refuses a session's next request once that share of its prompt tokens carried error content: failed tools, failed edits, repeated identical calls, overflow notices. It trips before the spend cap because an agent stuck on failures burns money on nothing; sessions under ten thousand prompt tokens are never judged. The same figure is `error_share` on `/buffy/status`, and `buffy replay` on the ledger names what failed.
- `--loop-warn` and `--loop-block` count how many times in a row the agent has just made the same tool call with the same input, and add a warning header or refuse the request. A repeated command earlier in the session never counts; only the current run does. `x-buffy-override` passes a block once.
- `--breaker-failures` opens a circuit after consecutive provider failures and answers locally with `Retry-After` until the cooldown passes, so the agent stops burning retries against a provider that is already saying no.
- `--retries` resends a request up to that many times on rate limit, overload, server error, or connection failure, with doubling jittered backoff from `--retry-base` capped at `--retry-max`, and the provider's `Retry-After` in place of the backoff when it fits under the cap. A retry can only happen before any byte of a response has reached the client, never on a client error, and never once a stream has started. The ledger records the count per request; the log names each attempt and its reason.

One live policy, experimental and off by default:

- `--context-edit-trigger <tokens>` asks the provider to clear old tool results server-side once the prompt passes the trigger, keeping the last `--context-edit-keep` (default 6). Buffy adds the provider's `context_management` parameter after the client's own bytes, which stay byte-identical, and only on requests whose client already enabled the context-management beta and set no such parameter itself. The decision is made at a session's first request and pinned. Each edit is logged with body hashes before and after, never content; the ledger records which requests carried it and the provider's applied edits and cleared tokens, so `buffy replay` on the ledger shows what it did. `--policy-file ~/.buffy/policy.json` applies the context-edit candidate `buffy learn` selected instead, read at each session's first request; an explicit trigger flag wins over the file. Either way the decision is pinned on disk, so a session keeps its first decision through a rewritten file or a restarted proxy. A learned policy runs as a bounded trial: `--trial-share 0.5` applies it to half of new sessions by a stable hash of the session id and holds the rest out as controls, and `--guardrail-reread 0.5` reverts it for new sessions once `--revert-after` treated sessions (default two) show a re-read rate after the provider's clears at or above that share. The revert is persisted and survives restarts; a newer `buffy learn` result lifts it. An explicit trigger flag is your own decision and is never split or reverted. `BUFFY_NO_POLICY=1` forces every policy off. The parameter shape follows the provider's documentation and has not yet been exercised against the real provider (roadmap spike 4); the `what_if` rows tell you what it should save before you turn it on, and the `re_reads` figures (file reads that repeated a path already in context, before and after the provider's first clear) tell you whether the agent is paying the savings back by re-reading.

### Learn from your own sessions

```sh
buffy learn ~/.claude/projects/* ~/.buffy/ledger
```

Re-scores the policy catalog (both cache TTLs and context editing at four triggers) over every session with the replay simulator, then selects one with rules built for a corpus of tens of sessions: a minimum number of sessions that actually carry evidence, a margin above noise, a repeat on held-out sessions chosen by a stable hash, and ties to the simpler policy judged on the paired per-session difference (ADR-0006). The verdicts and the selection go to `~/.buffy/policy.json` in a documented format ([`docs/architecture/policy-file.md`](docs/architecture/policy-file.md)). On a small corpus the honest answer is "none", and that is what it says. Reads files only; never the network.

### Get told what to change

```sh
buffy advise ~/.claude/projects/* ~/.buffy/ledger
```

Turns the largest token sources across every session into suggestions with a predicted saving: tool inputs that dominate prompts (long heredocs), tool results that should be truncated, files read again and again, first-turn instruction files that every request re-carries, tool definitions a session never calls (visible in the ledger), and cache breaks to look at with `buffy diff`. Each prediction assumes the target is halved and is stated as a share of prompt tokens first, the scale-free metric, then as tokens across the corpus. Suggestions are tracked to closure: pending until the newest sessions show the target shrinking, then applied, then verified or not verified against the prediction. Written to `~/.buffy/advice.json`.

One client caveat from the gateway docs: with a non-first-party base URL, Claude Code disables MCP tool search unless `ENABLE_TOOL_SEARCH=true` is set. Buffy forwards `tool_reference` blocks unchanged, so setting it is safe. Details: [`docs/architecture/proxy-protocol.md`](docs/architecture/proxy-protocol.md).

## Install

No release is tagged yet. When one is, every release carries `checksums.txt` signed with Sigstore keyless signing, built only by CI from the tag. Verify before running:

```sh
cosign verify-blob --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/RedRobotKK/Buffy/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Until then, build from source as below.

## Development

Requires Go 1.24+, [golangci-lint](https://golangci-lint.run/) v2, and Node 22+ (Markdown lint only).

```sh
make ci       # lint, test, build, docs-lint: exactly what CI runs
make build    # ./bin/buffy
make help     # all targets
```

## Documentation

- [`docs/ROADMAP.md`](docs/ROADMAP.md): what ships when, and why
- [`docs/adr/`](docs/adr/): architecture decision records
- [`docs/architecture/`](docs/architecture/): system design
- [`docs/prd/buffy-prd-v5.0.0.md`](docs/prd/buffy-prd-v5.0.0.md): current requirements; earlier versions kept as history
- [`docs/reviews/`](docs/reviews/): design reviews
- [`docs/HOUSEKEEPING.md`](docs/HOUSEKEEPING.md): how this repository is run

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) first. Bugs and features go through the issue templates. Security reports go through [`SECURITY.md`](SECURITY.md), never a public issue.

## License

Open source under the [Apache License 2.0](LICENSE). See [`NOTICE`](NOTICE) for attribution. The decision is recorded in [ADR-0005](docs/adr/0005-apache-2-license.md).
