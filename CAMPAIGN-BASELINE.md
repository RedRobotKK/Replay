# Campaign baseline and provenance

**Established 2026-09-30 by forensic inspection, after the fact.** The ordering
is stated plainly below rather than presented as if the baseline had been
established first.

---

## Honest note on ordering

The instruction to establish provenance before fixing arrived **after** the fix
had been made and after the campaign had been implemented. This record is
therefore a reconstruction from git history rather than a reading taken at the
time. The history is intact and every claim below is checkable, but the
sequence was: fix, campaign, then provenance.

| Event | Commit | Time |
|---|---|---|
| R10 evidence file added | `436276e` | 17:17:54 |
| Orphan guard fixed by linking it | `2687d9e` | ~17:25 |
| Claim register implemented | `87ca7c3` | ~17:56 |
| Verification report | `2bf3b68` | ~17:59 |
| **This provenance record** | — | after all of the above |

---

## 1. Provenance of `docs/evidence/r10-outcome-trial-2026-09-30.md`

**Created by this session's own earlier work. Not pre-existing, not another
agent.**

```
commit 436276e
author RedRobot <git@redrobot.jp>
date   2026-09-30 17:17:54 -0700
subj   evidence: R10 outcome trial stopped at ceiling, treatment arm never run
 docs/evidence/r10-outcome-trial-2026-09-30.md | 100 +++++++++++
 1 file changed, 100 insertions(+)
```

That commit added the evidence file **and nothing else**. It did not touch
`docs/evidence/README.md`, which is the section index the orphan guard checks.

At the parent `afb7f29`:

- the file is **absent** from the tree;
- `docs/evidence/README.md` contains **zero** references to it.

## 2. The failure is campaign-induced, proven empirically

Not inferred from the diff. The parent tree was exported with `git archive` to
a scratch directory, leaving the repository untouched, and the guard was run
against it:

| Tree | `TestNoOrphanedDocuments` |
|---|---|
| `afb7f29`, the parent | **ok** |
| `436276e`, the R10 commit | **FAIL** |

**Classification: CAMPAIGN-INDUCED. Not a pre-existing regression, and not
counted as one.**

Precision worth keeping: it was induced by this session's **earlier R10 work**,
which preceded the verification campaign. It was never a property of the
repository as handed over.

## 3. Exact baseline

| | |
|---|---|
| Command | `go test ./...` |
| Result at first run | **35 ok, 1 FAIL** |
| Failing package | `github.com/RedRobotKK/Replay/cmd/replay` |
| Failing test | `TestNoOrphanedDocuments` |
| Failure reason | `docs/evidence/r10-outcome-trial-2026-09-30.md` is linked from nothing, so nobody will find it |
| Pre-existing? | **No** |
| Working tree at the time | dirty, with changes that are **not** from this session, see section 5 |
| Result after linking the file (`2687d9e`) | **36 ok, 0 FAIL** |
| Result after the campaign (`2bf3b68`) | **37 ok, 0 FAIL**, the extra package being `internal/claims` |

**The guard was not weakened at any point.** The repair was to the commit that
broke it.

## 4. Discovery agents modified nothing

Checked two ways:

- The five sweeps were `Explore` agents, whose tool set excludes `Edit`,
  `Write` and `NotebookEdit`.
- `find -newermt` across the sweep window, 17:20 to 17:30, returns **no file**.

**Discovery was read-only in fact, not only by intent.**

## 5. Working-tree changes that are not this session's

Three files are modified and uncommitted. They split cleanly, and **none of
them has been committed by this session**:

| File | Provenance |
|---|---|
| `docs/WORK-STATE.md` | **Not this session.** mtime 2026-09-27 10:57:52, three days earlier. Content is the Grok/stateledger investigation |
| `docs/design/UNWIRED-LOG.md` | **Mixed.** Section 15 (`internal/stateledger`) is pre-existing and absent from HEAD. Section 16 (`internal/claims`) is this session's |
| `internal/regression/unwired_packages_test.go` | **Mixed.** The `internal/stateledger` entry is pre-existing and absent from HEAD. The `internal/claims` entry is this session's |

Verified: the stateledger entry appears **0 times in HEAD** and **1 time in the
working tree**, so it is somebody's in-flight work and not something this
session introduced.

**Nothing was discarded, stashed or overwritten.** The two mixed files were
left uncommitted precisely because committing them would have swept up another
session's work.

## 6. Open decision, not taken here

Per the instruction that a campaign-induced failure is not a regression and the
document's place in the evidence surface is a later decision:

**The R10 evidence file is currently linked from `docs/evidence/README.md` and
the suite is green.** If the file should not be in the final evidence surface,
removing it and its index row restores the pre-campaign state exactly. Nothing
in the claim register depends on it.

The alternative, reverting `2687d9e` to restore the red baseline, would leave
the suite failing without recovering any information this record does not
already contain.
