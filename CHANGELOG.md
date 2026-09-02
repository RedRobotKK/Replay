# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Repository scaffold: Go module, `buffy` command skeleton with `version` and `help`, Makefile, CI on Linux, macOS, and Windows, golangci-lint, Markdown lint, Dependabot, issue and pull request templates, label set, weekly stale-issue housekeeping.
- Governance and community documents: license (BSL 1.1 converting to Apache 2.0), contributing guide, code of conduct, security policy, support guide.
- `docs/ROADMAP.md`, `docs/HOUSEKEEPING.md`, ADR process with ADR-0001, PRD v4.0.0 and its adversarial review under `docs/`.
- PRD v5.0.0 (`docs/prd/buffy-prd-v5.0.0.md`): replay-first product, two-tier truth labels, provider-sanctioned policy catalog, scoped rehydration, gating spikes, and the release sequence. ADR-0002 to ADR-0004 record the decisions. Red/blue review of the full design under `docs/reviews/`.

### Changed

- README and roadmap now describe the replay-first sequence (`replay`, `blame`, `diff`, then `serve`).

[Unreleased]: https://github.com/RedRobotKK/Buffy/compare/main...HEAD
