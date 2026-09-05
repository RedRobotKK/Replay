<!-- Delete any section that does not apply. Empty headings are worse than no heading. -->

## What

<!-- One or two sentences. What does this change do for a user or maintainer? -->

## Why

<!-- Link the issue or ADR. If there is no issue, the motivation goes here instead. -->

Closes #

## How

<!-- Design notes a reviewer needs. Delete this section for anything mechanical. -->

## Checklist

- [ ] `make ci` passes locally (lint, test, build, docs-lint)
- [ ] Tests added or updated, and they failed before the change
- [ ] Docs updated where behavior changed: README, `docs/`, and an ADR for a design decision
- [ ] `CHANGELOG.md` has an entry under **Unreleased** (skip for docs-only changes and chores)
- [ ] No secrets, tokens, or local paths in the diff
- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)

<!--
One concern per pull request. A mixed refactor-and-feature diff will be asked to split.
Security problems do not go here: see SECURITY.md.
-->
