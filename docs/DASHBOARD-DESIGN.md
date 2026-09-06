# Live dashboard: layout and state inventory

**Status:** design, not built. Nothing here ships until the numbers it renders
are guaranteed by the P1 conservation laws.

**What this is for:** `replay serve` currently prints lines. A person running a
fleet through it cannot see, at a glance, what is being counted, what is being
forwarded blind, and which guard is about to fire. This specifies the surface
that answers that, and the states it has to survive.

## The rule that governs every cell

**Nothing renders that the surface registry does not back.**

`internal/ledger/surface_registry_test.go` already fails a status that outruns
its evidence. The dashboard reads the same statuses, so a row cannot claim
`parsed` for a path the build forwards blind. Two drafts of this design were
written before this rule and both invented capability: one had Replay
overriding an endpoint "to protect conversation branches", which it cannot do,
and one showed `PARSED_CONFIG` against Grok's `/settings`, which Replay never
reads. That decompression was done by a throwaway capture proxy, not by the
product.

A second rule follows from it. **No figure on screen may come from a past
measurement.** A draft rendered `BLIND (N=8)`, taking the sample size from one
capture session and pinning it into a live display, where it would still read 8
after a thousand turns. Live surfaces show live values or they show nothing.

## The width constraint, measured

Box-drawing characters are the wrong charset for this, and the reason is not
aesthetic.

`─ │ ┌ ┬ ┐ ├ ┼ ┤ └ ┴ ┘` are all **East Asian Ambiguous**. In a terminal
configured for a CJK locale they render two cells wide, not one. A frame drawn
from them is 79 columns in one locale and 158 in another, and the second is not
a frame at all. Measured on the previous storyboard: 98 ambiguous-width
characters in three lines of grid.

The same storyboard also sheared without help from the locale. Its data rows
measured 78 cells against 79 for every border, because one header cell lost its
padding space. The design claimed "borders never shear" and the artifact
demonstrating it did.

**So: ASCII and whitespace only.** No box-drawing, no emoji, no glyph outside
printable ASCII in any frame element. Verified at 0 ambiguous-width characters
and one consistent width across all rows.

```text
  time      surface    endpoint                 wire              status
  --------  ---------  -----------------------  ----------------  ---------
  15:06:44  anthropic  api.anthropic.com        messages          parsed
  15:05:58  grok       cli-chat-proxy.grok.com  responses         forwarded
  15:05:44  deepseek   api.deepseek.com         chat/completions  parsed
```

Width budget is **80 columns**. Content is truncated with a trailing `~` rather
than wrapped, because a wrapped cell breaks the row and a truncated one does
not.

## Repaint

One buffer, one write, cursor home plus erase-to-end-of-line per row. Erase
matters: without it a row that shortens leaves the tail of the previous frame
on screen, which is how a stale value survives a repaint and gets read as
current.

Repaint is **time-driven, not event-driven**, at a fixed cadence. An
event-driven repaint under fan-out repaints per request, which is both flicker
and a lock held on the render path while the proxy wants to serve.

## States

Twenty-five, in five groups. The storyboard covered five of them.

### A. Lifecycle

| # | State | What it must show |
|---|---|---|
| 1 | Idle, listening | Listen address, upstream, ledger path. Empty slots, not a spinner |
| 2 | Active | The traffic log, the counters, the armed guards |
| 3 | Shutting down | Session totals, ledger path, what was forwarded blind |

### B. What the proxy may say about a request

Each maps to a registry status. Nothing else is renderable.

| # | State | Registry status |
|---|---|---|
| 4 | Parsed, ledger record attached | `LIVE` |
| 5 | Parsed against a stub, never verified live | `STUB`, and the row says so |
| 6 | Forwarded, read nothing, warned once | `FORWARDED` |
| 7 | Measured and found unmeasurable | `REFUSED` |

State 5 is the one a dashboard would normally hide. It is the OpenAI-compatible
path, and `RELEASE-CRITERIA.md` gates v1.0 on it being verified live or
labelled. A row that renders it identically to state 4 defeats that gate.

### C. Guards

Twenty-six `serve` flags configure these. All of them can fire.

| # | State |
|---|---|
| 8 | Spend cap approaching, per session and per day, tokens and dollars |
| 9 | Spend cap reached: refusing before the next request |
| 10 | Error budget exceeded |
| 11 | Loop warn |
| 12 | Loop block |
| 13 | Breaker open, with cooldown remaining |
| 14 | Retry in flight |
| 15 | **Cap configured but not enforced** |

State 15 is the most important guard state and the easiest to omit. A dollar cap
is set, a request cannot be priced, so it adds nothing to the running total and
the cap can never be reached. The user has no cap and believes they do.
`SpendGuard.CapNotEnforced` already reports this; the dashboard has to surface
it, because a cap that silently is not applied is exactly the class of defect
this project exists to refuse.

### D. Settings and configuration

The gap in the storyboard. There are 26 flags, three consent decisions and a
masking configuration, and a person cannot currently see any of them at once.

| # | State | What it must show |
|---|---|---|
| 16 | Settings view | Every flag, its value, and **where the value came from**: default, flag, or environment |
| 17 | Corpus consent: undecided | Never asked. Not the same as refused |
| 18 | Corpus consent: granted | With the file path, so it can be revoked |
| 19 | Corpus consent: refused | Remembered, and stated, so nothing re-asks |
| 20 | Corpus consent: unreadable | The file exists and its ownership cannot be trusted |
| 21 | Update consent | The same four, independently |
| 22 | **Ownership not verified** | Windows: the mode-bit check cannot run, so consent state is parsed but its provenance is unestablished |
| 23 | Masking | On or off, pattern count, entropy threshold, and **which paths it does not cover** |

Provenance in state 16 is the point of it. A flag showing `--max-day-usd 5.00`
tells you the number. It does not tell you whether you set it or whether it is
a default you have never thought about, and those are different situations for
somebody deciding whether their cap is real.

State 22 exists because of a defect fixed on 2026-09-06: the consent gate read
Unix mode bits, Windows has none, and every consent decision on that platform
was refused. `Decision.OwnershipChecked` now reports whether the check ran. A
dashboard that renders "granted" identically on both platforms throws that
distinction away again.

### E. Empty and degraded

| # | State | Behaviour |
|---|---|---|
| 24 | Not a TTY | One plain line per event, no repaint, no cursor control. Piping into a file must produce a readable log, not escape sequences |
| 25 | Terminal narrower than 80 | Drop columns in a fixed order: wire, then endpoint, then surface. Never wrap, never shear |

Also required and not numbered because they are failures rather than states:
port already in use, ledger directory unwritable, upstream unreachable at start.
Each names the path or address it failed on.

## Rendering: the states worth drawing

### Idle

```text
  replay serve                                                    v0.4.0

  listening   127.0.0.1:4000
  upstream    api.anthropic.com
  ledger      ~/.replay/ledger            (writable)

  traffic
  time      surface    endpoint                 wire              status
  --------  ---------  -----------------------  ----------------  ---------

  no requests yet. point a client at the listen address above.

  [s] settings   [q] quit
```

### Active, with a blind path and a cap that is not enforced

```text
  replay serve                                                    v0.4.0

  listening   127.0.0.1:4000        sessions  3
  upstream    api.anthropic.com     billed    1,204,881 prompt tokens
  ledger      ~/.replay/ledger      spend     $2.41 of $5.00 day cap

  traffic
  time      surface    endpoint                 wire              status
  --------  ---------  -----------------------  ----------------  ---------
  15:06:44  anthropic  api.anthropic.com        messages          parsed
  15:06:41  openai     api.openai.com           chat/completions  stub
  15:05:58  grok       cli-chat-proxy.grok.com  responses          forwarded
  15:05:44  anthropic  api.anthropic.com        messages          parsed

  notes
  ! the day dollar cap is NOT being enforced: 12 requests could not be
    priced, so they add nothing to the total and the cap cannot be reached.
  - grok /responses is forwarded unread: no ledger record, no spend cap,
    no masking and no loop detection apply to it.
  - openai is parsed against a stub and has never been verified live.

  [s] settings   [q] quit
```

The two lines under `notes` that most dashboards would not print are the two
worth printing.

### Settings

```text
  replay settings                                                 v0.4.0

  caps                       value            from
  max-session-tokens         unset            default
  max-day-tokens             unset            default
  max-session-usd            unset            default
  max-day-usd                5.00             flag
  error-budget               0.15             default

  consent                    state            checked
  corpus contribution        refused          yes    ~/.config/replay/...
  update checks              undecided        yes    never asked

  masking                    on
    patterns                 14
    entropy threshold        4.20
    NOT covered              /responses, /v1/chat/completions

  [esc] back
```

`from` is the column that makes this worth having.

## What is deliberately not here

No sponsorship banner, no donation link, no `replay sponsor`. Two drafts pinned
one above the data. `5ad67cc` removed an unearned tip ask from the installer
and the reasoning has not changed: a permanent ask above the numbers is rent
charged on the thing the user came for.

No throughput figure in megabytes until it counts real bytes. A draft
incremented a hardcoded `0.118` per packet, which is a constant rendered as a
measurement.

No "critical fault" language for vendor behaviour that is by design. Grok's
static rate-limit headers are not a fault.

## Build order

1. The width-safe column formatter, with a test that fails on any
   ambiguous-width character and on any row whose display width differs from
   the header's.
2. The non-TTY path, first, so the fallback is never an afterthought.
3. States 1, 2, 6 and 15. The blind path and the unenforced cap are the reason
   this surface earns its place.
4. Settings, states 16 to 23.
5. Everything else.

Gated on P1: the conservation laws have to hold before a number is worth
rendering at this size.

---

[Documentation index](README.md) · [Repository README](../README.md)
