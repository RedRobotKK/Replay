# Half the TUI describes nobody, 2026-09-08

**What this measures:** which TUI screens show the reader their own machine and
which show an illustration, checked by rendering all ten against a real corpus
and reading what came back.

## The count

| Screen | Shows |
|---|---|
| `cost` | **this machine** |
| `why` | **this machine** |
| `doctor` | **this machine** |
| `share` | **this machine** |
| `live` | **this machine** |
| `context` | example data |
| `advise` | **this machine** (wired 2026-09-08) |
| `guards` | example data |
| `model` | example data |
| `safe` | example data |

**Was five of ten; `advise` is now wired, so four remain.** Each carries the notice `[NOTE] example data: describes nobody,
shows only the shape.` — so nothing here is dishonest. It is simply not the
reader's.

It matters more than the ratio suggests, because `install.sh` opens this surface
automatically on a successful install. The first thing a new user sees is a
screen about a fictional session, on a machine whose real sessions are sitting
one directory away.

## The mechanism, exactly

`cmd/replay/tui.go` dispatches on a key. Five screens have a case that computes
something and passes it in:

```go
case 'd': sc := tui.DoctorScreen(cur)
case 'c': sc := tui.CostScreen(cur, tick, loop.Cursor())
case 'l': ... tui.LiveScreen(liveState(), time.Now())
case 'w': ... tui.WhyScreen(opened, blameFor)
case shareK: ... tui.ShareScreen(st)
```

The other five have no case at all, so they reach:

```go
default:
    return tui.Frame{Key: k, Lines: tui.Outcome(k).Lines}
```

`Outcome` renders a canned illustration for any key without a dedicated branch.
Nothing is broken. **Those screens were never wired to a caller that measures.**

## The data already exists, and is good

The same corpus, through the command-line equivalents, gives specific answers:

```text
replay context   Bash  43.2%  209k  x106
replay advise    Bash inputs are 28% of prompt tokens
replay trim      12 blocks over the cap, 73k bytes removable
```

So this is not a missing analysis. It is an analysis that runs everywhere except
the surface the installer opens.

## Why it was built this way, which is not laziness

`internal/tui` does no I/O, deliberately: that boundary is what lets every frame
be tested without a disk. Screens are fed by the caller, which is why `Machine`
exists and why `doctor` works.

And `measured.go` states the rule the tests enforce: a screen is Measured only
when **every** figure on it came from somewhere a reader could go and check. *"A
screen that measures three numbers and invents a fourth is an example screen,
because the notice is what tells a reader how much to trust, and a partial one
tells them wrong."*

That rule is right, and it is why the fix is not to delete the notice. The notice
is accurate. The fix is to give those five screens a caller that measures, at
which point they earn the right to drop it.

## What wiring one costs

Per screen: a case in the dispatch that runs the analysis, a constructor in
`internal/tui` taking the result, an entry in the provenance test naming its
source, a regenerated golden SVG, and a row in `TUI-FLAG-SURFACE.md`. The five
Measured screens are the worked example for all of it.

`share` is the exception and should stay as it is. Its own comment gives the
reason: it previews a card built from this machine's figures, and an example one
would be *"handing the reader somebody else's number to post under their own
name."* Given nothing, the right screen is the one that says so.

## Method and limits

Ten screens rendered with `replay tui --once --screen <name> -color never`
against a one-session fixture, `HOME` and `REPLAY_TRANSCRIPTS` pinned, and
classified on whether the frame carried the example-data notice. The
command-line comparison used the same fixture.

One corpus of one session. A larger corpus would change the figures those screens
would show and not which of them show figures at all, because the dispatch does
not consult the corpus size.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
