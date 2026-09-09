# Demo recordings

## `triage.tape` → `triage.gif`

The advise screen as a triage tool: select a finding, open its evidence, mark it
applied. Recorded against this machine's own corpus, so the figures move between
recordings — 1,691 transcripts to 1,738 in a single day.

```sh
go build -o docs/demo/.replay-demo-bin ./cmd/replay
./docs/demo/.replay-demo-bin advise ~/.claude/projects   # fill the advice cache
vhs docs/demo/triage.tape                                 # writes triage.gif
```

The binary is built into `docs/demo/` and is **not committed** — a recording
that picks up whatever `replay` is on `PATH` records the wrong software, which
is exactly what happened the first two times this was rendered.

### What building it found

Three defects, none of which any unit test had caught:

- **`--screen advise` was silently ignored.** The flag is documented as "which
  question to open on" and was honoured only by `--once`; `StartWith` built the
  loop with no initial key, so the interactive path always opened on `cost`.
  Two renders were blamed on the wrong binary before the flag was suspected.
- **The tape recorded an installed `v0.5.4`** rather than the tree, showing a
  release that predated everything the demo existed to show.
- **`ttyd` on `PATH` is a Linux ELF binary.** `~/.local/bin/ttyd` shadows the
  native Homebrew one, and VHS refuses it with "version (`<nil>`) is out of
  date". Render with `PATH="/opt/homebrew/bin:$PATH"` until that is removed.

A demo is a test that watches the product the way a person does. These three
were invisible to nine unit tests over the same package.

## Dependencies

`vhs`, `ttyd` (native, not the ELF one) and `ffmpeg`. All present via Homebrew.

---

[Documentation index](../README.md) · [Repository README](../../README.md)
