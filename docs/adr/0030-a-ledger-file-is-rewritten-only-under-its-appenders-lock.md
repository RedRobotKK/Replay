# 30. A ledger file is rewritten only under the lock its appender takes

**Status:** Proposed
**Date:** 2026-10-30

> ADR number 0029 is reserved for another workstream; this one takes 0030 so the
> two cannot collide.

## Context

`replay purge --session` is an erasure: it removes one session's records from
inside ledger files that may hold others. It reads a file, keeps every line
that is not the erased session's, writes the kept lines to a sibling and
renames the sibling over the original.

`replay serve` appends to the same files from another process, opening each
file with `O_APPEND` for every record. Nothing coordinated the two. A record
appended after the purge read the file and before its rename was in the
original and not in the replacement, so the rename deleted it: a record that
was never matched, exported or reported. Two purges of one file also shared a
fixed temporary name. `TestPG25_ARecordAppendedDuringAnErasureSurvivesIt`
reproduces the first.

No check made without coordination closes this. Re-reading the file's size or
modification time before the rename narrows the window and leaves one between
the check and the rename, and an appender that opened the file before the
rename writes into the replaced file whenever it gets to run.

## Decision

Every writer of a ledger session file takes an exclusive advisory lock on the
open file first, through `ledger.OpenLocked`: `Store.Append` for each record,
and the purge rewrite for the whole read-check-write-rename. After the lock is
granted the opened file must still be the one at the path, or it is reopened,
because a rewrite that renamed a replacement into place leaves any waiting
writer holding a lock on a file nobody reads again.

Under the lock the purge re-reads the file and requires the bytes it matched
during its walk to be an unchanged prefix of what is there now. Anything after
that prefix was appended since, and is carried into the replacement untouched.
Anything else (another rewrite, a truncation) refuses that file with nothing
removed from it.

## Consequences

An erasure no longer deletes a record it did not examine, and two erasures of
one file serialise: the second refuses the file rather than overwriting the
first's result. Records for the erased session that arrive after the walk are
kept, because they were not exported or counted; erasing a session that is
still live needs a second run once it stops.

The lock is advisory. A writer that does not use `OpenLocked` (an older
`replay` binary, an editor, a sync client) is not excluded, and neither is
`replay purge --older-than`, which removes whole files and is unchanged. The
lock is `flock`, which local filesystems on macOS and Linux provide; on a
network filesystem that does not, `Append` returns the lock error and the
record is not written, where it was written before. On Windows, where the
binary refuses to run, no lock is taken.

Append now pays one `flock` and two `stat` calls per record beside the open,
write and close it already did. This is a cost of the fix, not a performance
change, and nothing here was measured as one.

## Alternatives considered

Detecting a concurrent modification by re-stat (size and modification time)
before the rename, and refusing or retrying: smaller, and leaves the window
between the check and the rename, so it cannot be shown correct.

Moving the file aside before reading it, so new appends create a fresh file:
an appender that opened the file before the move still writes into the moved
copy after the purge has read it, so the loss remains, and the purge must then
merge two files.

One lock for the whole ledger directory held by `replay serve` for its
lifetime, with purge refusing while it is held: correct, but it refuses every
erasure while the proxy runs, which is most of the time on a machine that uses
it.
