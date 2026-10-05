# Purge hazard, 2026-10-05

**Recorded separately from the TTL work on purpose. Fixed as its own unit
in `e9da6a8`, 2026-10-05; the proof is at the end of this record.**

**Command shape.** `replay purge <dir> --older-than <window> --yes`
enumerates the entries directly inside `<dir>`, keeps those whose name ends
in `.jsonl`, and removes the ones older than the window. It does not
consult the store registry and does not check that `<dir>` is a ledger
directory.

**Affected files.** Pointed at `~/.replay` itself, it would remove
`measurements.jsonl` (probe readings the reader paid for) and
`interventions.jsonl` (the record of every change Replay made to the
reader's settings, which the registry marks as never subject to a
retention window). Pointed at a transcript directory, it would remove the
reader's own session transcripts.

**Why they matter.** Both are evidence: one is the only record of what a
model billed for a known request, the other the only record that an
intervention happened, when, and what it predicted. Neither is derived
from anything else on disk.

**Expected safe behaviour.** `purge --older-than` acts only on ledger
session files: a directory that the registry names as a ledger store (or
a `ledger-<name>` sibling), and within it only files the ledger wrote.
Anything else is refused by name with the reason.

**Minimal RED test, not yet written.** In `cmd/replay/privacy_test.go`:
create a temporary home with `.replay/measurements.jsonl` and
`.replay/interventions.jsonl` older than the window; run
`runPurge([]string{home + "/.replay", "--older-than", "1d", "--yes"})`;
assert both files still exist and the output names the refusal. The test
fails today because both files are removed.

**Proposed isolated fix.** Before enumerating, resolve `<dir>` against the
registry: accept the ledger store and `ledger-*` siblings and the archive;
refuse any other directory with "purge acts on ledger directories; <dir>
is not one". One commit, one mutant, no change to the TTL unit.

## Proof, e9da6a8

- RED: `TestPG10_OlderThanRefusesADirectoryTheRegistryDoesNotNameAsALedger`
  against the previous code: "a directory the registry does not name as a
  ledger must be refused", both files removed.
- GREEN: `isLedgerDir` from the registry; the refusal names what the
  command acts on, before anything is read.
- Positive control: `TestPG11_OlderThanStillActsOnLedgerDirectoriesAndOnlyThose`:
  ledger, ledger-grok and archive still purge; vault, policy.json, ledgerx
  and notes are refused with their files intact; eight name cases pinned.
- Mutation: 8 hand mutations on the guard (removed, inverted, exact match
  dropped, prefix dropped, prefix without the dash, purgeable check
  dropped, dir check dropped, every store accepted), 8 killed; M145 frozen
  and killed through the harness.
- Regression: full suite clean, lint 0 issues, 3 introduced guards, 0
  survived. Two fixtures renamed to ledger-shaped paths with their
  assertions unchanged.
- Not changed: `--session`, which walks the directory given; the registry
  contract; anything on the TTL path.
