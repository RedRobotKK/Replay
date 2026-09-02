# Project Buffy

**Buffy shows you the turn your coding agent's prompt cache broke, what it cost, and what a better context layout would have saved, on sessions you have already paid for.**

It reads the transcripts your agent already writes, reproduces the provider's caching turn by turn, and only then scores alternatives. Later, as a local proxy, it applies the better layout live using only mechanisms the provider itself sanctions, and keeps improving from your own history. Everything runs on your machine. No API calls are spent on analysis. Nothing leaves.

> **Status: v0.1 in development.** `replay`, `blame`, `diff`, and `redact` work offline on Claude Code transcripts and have been calibrated against one real session so far; the 20-session corpus the roadmap requires is still pending. Nothing proxies traffic yet. Follow [`docs/ROADMAP.md`](docs/ROADMAP.md).

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
| `buffy serve` | Local proxy: byte-for-byte passthrough, measured numbers, safe policy application, spend and loop guards |

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
