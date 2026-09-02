# Project Buffy

**Buffy shows you the turn your coding agent's prompt cache broke, what it cost, and what a better context layout would have saved, on sessions you have already paid for.**

It reads the transcripts your agent already writes, reproduces the provider's caching turn by turn, and only then scores alternatives. Later, as a local proxy, it applies the better layout live using only mechanisms the provider itself sanctions, and keeps improving from your own history. Everything runs on your machine. No API calls are spent on analysis. Nothing leaves.

> **Status: pre-MVP, design phase.** Nothing here proxies traffic yet. The build, test, and lint pipeline is real; the product is not. Follow [`docs/ROADMAP.md`](docs/ROADMAP.md) for what ships first.

[![CI](https://github.com/RedRobotKK/Buffy/actions/workflows/ci.yml/badge.svg)](https://github.com/RedRobotKK/Buffy/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/RedRobotKK/Buffy)](https://goreportcard.com/report/github.com/RedRobotKK/Buffy)
[![License: BSL 1.1](https://img.shields.io/badge/license-BSL%201.1-blue.svg)](LICENSE)

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

Not yet. When v0.1 lands, the first contact needs no proxy, no configuration, and no trust:

```sh
brew install redrobotkk/tap/buffy      # planned
buffy replay ~/.claude/projects/my-app/
```

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

Source-available under the [Business Source License 1.1](LICENSE). Each version converts to Apache 2.0 three years after its release. Buffy is not OSI open source until that conversion; see the license text for what you may do before then.
