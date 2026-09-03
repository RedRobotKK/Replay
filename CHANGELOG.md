# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `buffy replay`, `buffy blame`, and `buffy diff`: offline analysis of Claude Code transcripts. Reproduces the provider's cache reads turn by turn (the calibration line), classifies every cache break with a cause and location, attributes prompt tokens to content with per-turn sums that match provider usage exactly, prices visible error classes, and scores TTL and context-editing policies with the as-run behavior reproduced first. Every figure is labeled estimated or measured.
- `buffy corpus`: calibration summary across every session in a directory as a Markdown report (session id prefixes, counts, match rates, fit numbers, break causes; no paths or content).
- `buffy redact`: strips content from a transcript while keeping structure, sizes, usage, tool names, and equality of values (salted, so nothing is confirmable from the output), for bug reports and fixtures. A redacted session analyzes identically to the original; the repository fixture is a redacted real session.
- `buffy serve`: loopback passthrough proxy for the Anthropic Messages API. Byte-exact forwarding with immediate flushing for event streams, credentials forwarded and never persisted, gzip responses forwarded compressed, browser-origin rejection, optional shared token, fail-open bookkeeping, and a 502 that names the bypass when the provider is unreachable. Records a derived-data ledger (structure, sizes, labels, usage; no text) per client session under `~/.buffy/ledger`.
- Guards for `buffy serve`, off by default: token caps per session and per UTC day (fail closed before the next request, override header logged), loop detection over identical tool calls (warn header or refusal), and a provider circuit breaker with `Retry-After`. Refusals use the provider's error shape so agents show them.
- `replay`, `blame`, and `diff` accept ledger files and report at the measured tier, with the system prompt and tool definitions visible as the first context message.
- Spike 3 answered from documentation: subscription-authenticated Claude Code routes through `ANTHROPIC_BASE_URL` and stays on the subscription when no gateway credential is set. The gateway protocol facts the proxy must honor are recorded in `docs/architecture/proxy-protocol.md`.
- Release automation: GoReleaser builds for Linux, macOS, and Windows on amd64 and arm64 from a pushed tag, with a software bill of materials and Sigstore keyless signing of the checksum file; the workflow refuses tags that are not on `main`.
- Repository scaffold: Go module, `buffy` command skeleton with `version` and `help`, Makefile, CI on Linux, macOS, and Windows, golangci-lint, Markdown lint, Dependabot, issue and pull request templates, label set, weekly stale-issue housekeeping.
- Governance and community documents: Apache 2.0 license and NOTICE (ADR-0005; the scaffold's initial BSL 1.1 draft was replaced before any release), contributing guide, code of conduct, security policy, support guide.
- `docs/ROADMAP.md`, `docs/HOUSEKEEPING.md`, ADR process with ADR-0001, PRD v4.0.0 and its adversarial review under `docs/`.
- PRD v5.0.0 (`docs/prd/buffy-prd-v5.0.0.md`): replay-first product, two-tier truth labels, provider-sanctioned policy catalog, scoped rehydration, gating spikes, and the release sequence. ADR-0002 to ADR-0004 record the decisions. Red/blue review of the full design under `docs/reviews/`.

### Changed

- README and roadmap now describe the replay-first sequence (`replay`, `blame`, `diff`, then `serve`).

[Unreleased]: https://github.com/RedRobotKK/Buffy/compare/main...HEAD
