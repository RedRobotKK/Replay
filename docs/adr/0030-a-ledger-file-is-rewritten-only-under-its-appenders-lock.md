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

The lock works differently on each platform:

- **Unix** (macOS, Linux, the BSDs): `flock(LOCK_EX)`, blocking, retried on
  `EINTR`. It is advisory, and released when the descriptor closes or the
  process dies. `flock` rather than `fcntl`, because an `fcntl` lock is
  dropped when any descriptor the process holds on the file closes.
- **Windows**: `LockFileEx` with `LOCKFILE_EXCLUSIVE_LOCK` and without
  `LOCKFILE_FAIL_IMMEDIATELY`, so it blocks as `flock` does. It is called
  through `syscall.NewLazyDLL("kernel32.dll")`, since the module takes no
  dependencies, and `kernel32.dll` is a KnownDLL so it cannot be preloaded
  from the working directory. `LockedFile.Close` calls `UnlockFileEx` before
  it closes the handle. The OS also releases the lock on close and when the
  process dies. A `LockFileEx` range is mandatory: a handle that did not take
  the lock cannot read or write inside it. So the lock covers one byte at
  offset 2^62, far past the end of any ledger, rather than the data.
  Unlocked readers (`ReadRecords`, the purge walk) are therefore unaffected,
  and writers still exclude each other. `ERROR_IO_PENDING` from an
  asynchronous handle is waited out through `GetOverlappedResult`.
- **Any other platform** (js, wasip1, plan9, ...): `lockFile` returns an
  error, so `Append` and the purge rewrite refuse instead of writing
  unlocked. It used to return nil.

The lock is advisory on unix. On Windows it is mandatory only for that one
byte, which nothing else touches. A writer that does not use `OpenLocked` is
not excluded: an older `replay` binary (including any Windows build from
before this change, which took no lock), an editor, or a sync client. Neither
is `replay purge --older-than`, which removes whole files and is unchanged. On
a network filesystem that does not provide the lock, `Append` returns the lock
error and the record is not written, where it was written before. `Store`'s
mutex serialises one `Store` only. It is not the cross-process lock and does
not stand in for it.

Purge's rename over a file that other handles hold open needs two more things
on Windows. Both apply only there; unix keeps `os.OpenFile` and `os.Rename`.

- Every ledger handle is opened through `CreateFile` with
  `FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE`, wrapped with
  `os.NewFile`. That covers `OpenLocked` (`Append`'s handle and the rewrite's
  locked handle), the replacement written by `ledger.WriteFile`, and the
  `ReadRecords` and `IsLedgerFile` readers. `os.OpenFile` omits
  `FILE_SHARE_DELETE`, and without it the rename failed with "Access is
  denied", measured on windows-latest.
- The replacement is renamed in with
  `SetFileInformationByHandle(FileRenameInfoEx)`, using
  `FILE_RENAME_FLAG_REPLACE_IF_EXISTS | FILE_RENAME_FLAG_POSIX_SEMANTICS`. The
  old file is unlinked while its open handles stay valid, as on unix, and a
  waiting appender's identity check then sends it to the new file. Where POSIX
  rename is unsupported, it falls back to `os.Rename`, which refuses rather
  than losing anything.

The lock is still held across the rename. It is never released before the
rename: doing so would let an appender write into the file about to be
replaced, which is the original defect. `TestRW8` and `TestXL10` fail if the
order changes. A reader that opens a ledger without the shared mode, for
example through a plain `os.Open` elsewhere in the tree, makes a purge fail
with an error while it holds the file, but loses no record.

Append now pays one lock call (and on Windows one unlock) and two `stat` calls per record beside the open,
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
