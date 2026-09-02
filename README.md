# Project Buffy

**A local, transparent proxy that shows you what your coding agent is spending, keeps your secrets on your machine, and stops runaway loops.**

Buffy sits between an agent (Claude Code, Aider, custom loops) and the model provider on `localhost`. It forwards requests byte-for-byte by default, and layers in per-session cost visibility, prompt-cache diagnostics, local secret masking, and spend circuit breakers.

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

## What it will do (v0.1 to v0.3)

| Release | Capability |
|---------|------------|
| v0.1 | Byte-transparent passthrough for the Anthropic Messages API and OpenAI chat completions. Per-session cost and cache-hit dashboard. Cache-break detector that names the first divergent byte between turns. |
| v0.2 | Deterministic local secret masking with a persistent encrypted vault. Dollar-denominated circuit breakers per session and per day. |
| v0.3 | Opt-in context tools exposed over MCP. |

Details, acceptance criteria, and what is deliberately deferred: [`docs/ROADMAP.md`](docs/ROADMAP.md).

## Quick start

Not yet. When v0.1 lands the install will be one command and one environment variable:

```sh
brew install redrobotkk/tap/buffy      # planned
buffy serve
export ANTHROPIC_BASE_URL=http://127.0.0.1:4000
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
- [`docs/prd/`](docs/prd/): product requirement documents (history)
- [`docs/reviews/`](docs/reviews/): design reviews
- [`docs/HOUSEKEEPING.md`](docs/HOUSEKEEPING.md): how this repository is run

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) first. Bugs and features go through the issue templates. Security reports go through [`SECURITY.md`](SECURITY.md), never a public issue.

## License

Source-available under the [Business Source License 1.1](LICENSE). Each version converts to Apache 2.0 three years after its release. Buffy is not OSI open source until that conversion; see the license text for what you may do before then.
