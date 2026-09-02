# Contributing to Buffy

Thanks for helping. This page is the whole process; if something here is wrong or missing, fixing it is a welcome first pull request.

## Before you start

- **Bugs and features start as issues.** Use the templates. A pull request with no linked issue is fine for typos, docs, and one-line fixes; anything larger needs the discussion first so nobody builds the wrong thing.
- **Security problems never go in a public issue.** Read [`SECURITY.md`](SECURITY.md).
- **Design changes get an ADR.** If your change alters how Buffy handles traffic, secrets, caching, or storage, add a record under [`docs/adr/`](docs/adr/) in the same pull request. The template is there.

## Development setup

```sh
git clone https://github.com/RedRobotKK/Buffy.git
cd Buffy
make ci
```

You need Go 1.24 or newer and golangci-lint v2. Node 22 is only used for Markdown lint. `make help` lists every target.

## Branches and commits

- Branch from `main`. Name branches `type/short-description`, for example `feat/cache-break-detector` or `fix/sse-chunk-boundary`.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, `ci:`. The subject is imperative and under 72 characters. The body says why, not what.
- Keep pull requests small and single-purpose. A reviewer should be able to hold the whole change in their head.
- Do not rewrite history on a branch someone else has checked out. Merge `main` in rather than rebasing once a pull request is open.

## Pull request expectations

The template lists the checklist. In short: `make ci` is green, behavior changes have tests, decisions have docs, and the changelog has an entry under **Unreleased** for anything a user would notice.

Reviews aim to respond within two working days. A maintainer merges with squash unless the branch history is deliberately curated.

## Code style

- Go: `gofmt` and the linters in `.golangci.yml` are the style guide. Clear names over clever ones. Comments explain why, never what.
- Errors are handled where they occur. Discarding an error with `_ =` needs a comment explaining why it is safe.
- No magic numbers. Limits, timeouts, and thresholds are named constants.
- Anything that touches secrets, request bytes, or provider history must be deterministic and covered by a test that proves it.

## Documentation

Docs live in `docs/` and are linted in CI. Every document opens with one sentence saying what it is for and who should read it. Delete docs that are no longer true rather than leaving them to mislead.

## License of contributions

By contributing you agree that your contribution is licensed under the repository's [license](LICENSE), including its scheduled conversion to Apache 2.0.
