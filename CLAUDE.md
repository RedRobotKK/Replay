# CLAUDE.md

Conventions for agents (and humans) working in this repository. Read fully before changing anything.

## Project

Project Replay is a local, byte-transparent proxy between coding agents and model providers that adds cost visibility, prompt-cache diagnostics, secret masking, and spend circuit breakers. Status: pre-MVP. Roadmap in `docs/ROADMAP.md`. Decisions in `docs/adr/`.

## Commands

```sh
make ci          # lint + test + build + docs-lint; must be green before any push
make test        # go test -race ./...
make lint        # go vet + golangci-lint
make build       # ./bin/replay
make docs-lint   # markdownlint over all Markdown
```

## Non-negotiables

- **Transparency first.** The proxy forwards bytes unchanged unless a feature is explicitly enabled. Any transformation must be a deterministic function of its input so the same client message always renders to the same provider bytes within a session.
- **Never touch provider thinking blocks, signatures, or `cache_control` markers.**
- **Never rewrite an earlier turn differently than it was rendered before.** Provider history-binding checks reject edited history.
- **Secrets stay local.** No secret, token, or vault content is ever logged, committed, or sent anywhere but the intended provider request.
- **Handle every error.** `_ =` on an error needs a comment saying why it is safe.
- **No magic numbers.** Timeouts, limits, and thresholds are named constants.
- **Tests prove behavior.** Anything touching request bytes, masking, or caching gets a test that would fail if the invariant broke.

## Repository rules

- Branch from `main` as `type/short-description`. Conventional Commits. Small, single-purpose pull requests.
- Design changes need an ADR in the same pull request (`docs/adr/template.md`).
- User-visible changes need a `CHANGELOG.md` entry under **Unreleased**.
- Docs are linted. Every doc opens with one sentence on what it is for. Delete stale docs; do not leave them to mislead.
- Never commit `.env`, keys, local paths, or generated binaries. `.gitignore` is the source of truth.
- Never push to `main` directly. Never force-push a branch someone else may have checked out.

## Layout

```text
cmd/replay/          command entry point
internal/           private packages (version, later: proxy, masking, spend, dashboard)
docs/adr/           architecture decision records
docs/architecture/  system design
docs/prd/           product requirement documents (history)
docs/reviews/       design reviews
.github/            CI, templates, labels, housekeeping automation
```
