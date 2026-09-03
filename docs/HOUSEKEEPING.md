# Repository Housekeeping

How this repository stays clean enough to be public. Maintainers own these rules; contributors follow them.

## Branches

- `main` is always releasable. It is protected: no direct pushes, pull requests only, CI green required, at least one review required, linear history.
- Work branches are `type/short-description`. Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`.
- Agent-authored branches use the `claude/` prefix and follow the same rules.
- Branches are deleted on merge. Anything unmerged and untouched for 30 days is a candidate for deletion; the owner is pinged first.

## Commits and pull requests

- Conventional Commits. Squash-merge by default so `main` reads as a changelog.
- One concern per pull request. Mixed refactor-and-feature diffs are split before review.
- Every pull request fills the template. Empty sections are deleted, not left blank.
- No pull request merges with red CI, unresolved review threads, or a missing changelog entry for user-visible change.

## Issues and labels

- Every issue carries exactly one `type:` label and one `status:` label. `area:` labels are optional and can stack.
- The canonical label set lives in `.github/labels.yml`. Change it there, then sync.
- New issues are triaged within two working days: accepted, asked for more information, or closed with a reason.
- The stale workflow marks issues after 45 idle days and closes after 14 more. Labels `status: accepted`, `type: security`, and `pinned` are exempt.

## Releases

- Semantic versioning. Tags are `vMAJOR.MINOR.PATCH`.
- `CHANGELOG.md` is updated in the release pull request by moving **Unreleased** into a dated section.
- Binaries are built by CI from the tag, never from a laptop: the release workflow refuses a tag that is not on `main`, builds every platform with GoReleaser, publishes a software bill of materials, and signs `checksums.txt` with Sigstore keyless signing. The release notes link to the changelog section and show the verification command.
- Security fixes ship as patch releases with a `type: security` changelog entry.

## Documentation

- Every document opens with one sentence saying what it is for.
- Decisions go in ADRs. Design goes in `docs/architecture/`. History (PRDs, reviews) is kept but never edited after the fact; add a new version instead.
- Markdown is linted in CI. Broken links are bugs.
- The README states the project status truthfully. If the product cannot do something yet, the README says so.

## Secrets and privacy

- Nothing under `.env`, `*.key`, `*.pem`, or local paths is committed. `.gitignore` enforces it; secret scanning on GitHub is enabled.
- Logs, issues, and pull requests are redacted before posting. The bug template requires a redaction confirmation.
- A leaked secret is rotated first and cleaned from history second.

## Weekly checklist (maintainer)

- [ ] Triage queue empty of `status: triage` older than two days
- [ ] Dependabot pull requests reviewed and merged or closed
- [ ] Stale branches on origin reviewed
- [ ] CI green on `main`
- [ ] README status line still true
- [ ] `CHANGELOG.md` **Unreleased** reflects merged work
