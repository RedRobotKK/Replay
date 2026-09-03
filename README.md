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
| `buffy corpus` | Calibration summary across every session in a directory, as Markdown with no paths or content, for reporting how well Buffy understands your sessions |
| `buffy serve` | Local proxy: byte-for-byte passthrough that records what the provider charged, so the three commands above run on measured data (policies and guards come later) |

Every number is labeled *estimated* (from transcripts) or *measured* (from the wire), with the calibration that justifies it. Release sequence, gates, and what is deliberately deferred: [`docs/ROADMAP.md`](docs/ROADMAP.md). Full requirements: [`docs/prd/buffy-prd-v5.0.0.md`](docs/prd/buffy-prd-v5.0.0.md).

## Quick start

No proxy, no configuration, no trust required. Build from source until the first release is tagged:

```sh
make build
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

Guards, all off unless you set them (see `buffy serve -h`):

- `--max-session-tokens` and `--max-day-tokens` refuse the *next* request once a cap is reached, never a response in flight. The refusal is a provider-shaped error the agent shows you; send `x-buffy-override: <reason>` to proceed once.
- `--loop-warn` and `--loop-block` count how many times in a row the agent has just made the same tool call with the same input, and add a warning header or refuse the request. A repeated command earlier in the session never counts; only the current run does. `x-buffy-override` passes a block once.
- `--breaker-failures` opens a circuit after consecutive provider failures and answers locally with `Retry-After` until the cooldown passes, so the agent stops burning retries against a provider that is already saying no.

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
