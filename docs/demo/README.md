# Demo recordings

## `triage.tape` → `triage.gif`

The advise screen as a triage tool: select a finding, open its evidence, mark it
applied.

### Provenance

`triage.gif` was rendered by `docs/demo/render.sh` from the released **v0.9.0**
binary (`replay version` prints
`replay 0.9.0 (06de1b2, built 2026-10-10T05:54:31Z)`; binary sha256
`aa2d0a800c5b4c23f8562c62fcf2e0efc05f8f0e81104887ea6d91c86781d0a5`, the
darwin/arm64 build from the signed release archive) against the repository's
redacted fixture `internal/transcript/testdata/session-redacted.jsonl` (sha256
`f74e411ff848ea9c63338ab696e65b7a82c2aa63a8b44db324e89e1742fe4faa`), placed under
a throwaway `HOME`. No transcript on the recording machine is read, so the
figures on screen are the fixture's, they do not move between recordings, and
they are not the maintainer's own usage. Every screen in the GIF is the binary's
real output; nothing was composed or edited by hand.

```sh
docs/demo/render.sh                      # builds the binary from this tree
docs/demo/render.sh path/to/replay       # records a released binary instead
```

The script prints the fixture hash, the binary hash and the binary's version
line before it records, so the recording's provenance is checkable. The binary
is copied to `docs/demo/.replay-demo-bin` and the throwaway home to
`docs/demo/.home`; neither is committed.

GitHub may show only the first frame of a GIF, so that frame has to be the
advise screen and not the shell typing the command. VHS records the typing and
the launch before the TUI first draws, and how many frames that takes varies
from run to run (156 and 165 in two consecutive re-renders). `render.sh`
therefore finds the first frame whose content area is populated and drops
everything before it, rather than dropping a fixed count. If it finds none it
exits non-zero. `DEMO_LEADIN_FRAMES=N` forces a count.

### Reproducibility

Re-rendering does not give a byte-identical file: the frame count and the
timing differ between runs (249 and 250 frames in two re-renders from a clean
clone, against 258 committed), and the screen's "as of" line carries the render
time. What is reproducible is the inputs (binary hash, fixture hash, tape) and
the screens shown. The commands the tape types are the ones in the list below.

### What the recording shows

1. The advise list opens on the fixture: "Showing 2 of 3 changes worth making,
   across 1 transcript(s)".
2. `j`, `j`, `k` move the cursor between findings.
3. `enter` opens the evidence for "Bash inputs are 28% of prompt tokens":
   6,601,667 prompt tokens, 14.1% of prompt tokens if applied, status pending.
4. `esc` returns to the list, `a` marks a finding applied, `q` quits. The
   marking is written under the throwaway home, not the viewer's.

### Defects in the previous recording

The GIF shipped before 0.9.0 failed the same provenance test this one is held to:

- It was recorded from a development build, not a release.
- It was recorded against the maintainer's private corpus (1,691 transcripts at
  the start of a day, 1,738 at the end), so the figures could not be reproduced
  by anyone else and changed between renders.
- It had no recorded binary or input hash.

### What building it found

Three defects, none of which any unit test had caught:

- **`--screen advise` was silently ignored.** The flag is documented as "which
  question to open on" and was honoured only by `--once`; `StartWith` built the
  loop with no initial key, so the interactive path always opened on `cost`.
- **The tape recorded an installed `v0.5.4`** rather than the tree.
- **`ttyd` on `PATH` is a Linux ELF binary.** `~/.local/bin/ttyd` shadows the
  native Homebrew one, and VHS refuses it with "version (`<nil>`) is out of
  date". `render.sh` puts `/opt/homebrew/bin` first.

## Dependencies

`vhs`, `ttyd` (native, not the ELF one) and `ffmpeg`. All present via Homebrew.

---

[Documentation index](../README.md) · [Repository README](../../README.md)
