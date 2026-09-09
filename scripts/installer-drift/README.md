# installer-drift

Does the installer people actually run match the one in this repository?

```sh
scripts/installer-drift/check.sh
INSTALLER_DRIFT_STRICT=1 scripts/installer-drift/check.sh   # exit 1 on drift
```

## Why it exists

On 2026-09-09 it did not match. `install.sh` here carried `--no-tui`, merged in
#91; the copy served from `redrobot.jp` was **28 lines behind** and rejected the
flag outright:

```text
✗ unknown option: --no-tui. Try --help.
```

Every reader who runs the one-liner gets the hosted file. So for as long as they
differ, **the version in this repository is the one nobody runs**, and a flag
added here is a flag that does not exist for anyone.

It is the same lesson as `scripts/release-check.sh`, which exists because a
published binary was 75 days behind the tree. The artifact and the source are
different things, and only one of them is what a user meets.

## Why it does not fail the build

A deploy lag is a fact about the website, not a fault in the commit under test.
Failing a pull request for it would put a red cross on work that did not cause
it, and a check that cries wolf gets switched off. It prints loudly and exits 0.

`INSTALLER_DRIFT_STRICT=1` makes it exit 1, for a release checklist where the
answer genuinely gates.

## What it compares

Both hosted paths, `/replay.sh` and `/Replay/install.sh`, against `install.sh`
by SHA-256. On a mismatch it reports the line delta and, more usefully, **the
flags the hosted copy does not accept** — a missing flag is the difference a
reader or a script hits first.
