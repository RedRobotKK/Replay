# Pre-tag review, 2026-09-13: what was fixed and what was not

Three adversarial reviews ran against `corpus-build-identity` before the v0.6.0
tag: the release pipeline, the night's diff, and the launch copy. This records
what came out of them, because a finding that is deferred without a written
record is a finding that was dropped.

**Deferred deliberately on 2026-09-13.** None of the open items blocks the tag.

## Fixed before the tag

| Finding | Why it mattered |
|---|---|
| `publish-shims.yml` triggered on `release: published`, which goreleaser raises with `GITHUB_TOKEN` | GitHub does not start workflows from that token. It would have fired **zero times**, silently, and npm and PyPI would have received nothing |
| The PyPI builder skipped a missing tarball | A partial platform set, and a PyPI filename cannot be reused once taken. The one irreversible failure in the pipeline |
| The refusal left earlier wheels on disk | A refusal that leaves its output behind is not a refusal |
| `Corpus.UnmarshalJSON` refused the retired spelling but never required the new one | A v2 document omitting `rebilledUsd` parsed to zero and passed `Validate`. The defect the refusal was written to close, surviving inside it |
| The cosign exec pin had a comment-only switch arm | It constrained nothing. A second exec call and an env-var-chosen binary both passed. A new `os/exec` permission on shipped code, unbacked |
| Both launch-copy guards hardcoded a dated filename and skipped when absent | A one-day slip silently removed the only check on the Show HN copy |
| The banned-word scan stopped at the heading listing banned words | Everything appended after it was unscanned, and appending is how launch copy grows |
| Seven factual errors in the announcement | Including a privacy overclaim the repo's own source calls false, and an arithmetic claim wrong by 15x |

## Open, and deliberately not done before the tag

### 1. `replay upgrade` does not say whether the signature was checked

`cmd/replay/upgrade.go` prints `✓ Checksum verified` in all three outcomes:
signature verified, cosign absent, signature skipped. `install.sh` prints three
distinct lines. `Fetch` returns `([]byte, error)` and so **structurally cannot**
report which happened.

The test is named `TestSIG1_NoCosignMeansChecksumsOnlyAndSaysSo` and asserts
nothing about anything being said. The code half of that gap is closed; the
observable half is not, so a user still cannot tell which promise they got.

**Fix:** have `verifyChecksumSignature` return a status, print it, and make SIG1
assert it.

### 2. `Pool` got the v2 bump and not the refusal

`PoolSchema` moved to `replay.pool.v2` for the rename. `Pool` has `MarshalJSON`
and no `UnmarshalJSON`, so a v1 pool document read back silently zeroes its
figures. Nothing reads pools today, which is why this waits, but the pool is the
**published** artifact with outside readers, so it is more exposed than the
corpus rather than less.

**Fix:** the same probe refusal, or a comment saying why the asymmetry is
deliberate.

### 3. `docs/WASTE-DEFINITION.md` now describes a shipping field inaccurately

The tier was renamed to **Re-billed**, and it is documented as covering cache
breaks, duplicate re-reads and identical repeated calls. `cost.go` prices
**only** the cache-break member; repeats and errors are tallied separately and
never priced.

Before the rename "Avoidable" was a taxonomy label deliberately broader than the
wire field. Now the label is character-for-character the name of a shipping JSON
key and a printed column, so a customer reading the definition believes
`rebilledUsd` includes duplicate re-reads. It does not.

**Fix:** keep the tier named "Avoidable", and add one clause saying
`rebilledUsd` measures only its cache-break member.

## Two process findings worth more than any single defect

**Do not run `scripts/guard-reachability` and cut a tag in the same tree.** It
writes `false &&` neutralisations into the working tree in place and restores
them seconds later. A reviewer read two of those mid-flight and filed two
findings that did not exist, then retracted both. Suspecting the instrument was
correct, and the instrument was the tree.

**Three defects in one night traced to the same thirty seconds**: `git add -A`
without reading `git status`, and a suite result read too fast. One shipped a
broken build, one committed a `.pyc` and two test fixtures, and one misfiled
seven corrections into a commit about something else.

---

[Design index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md)
