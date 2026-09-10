# Every flag, and what it becomes on screen

**Status:** partly built. `replay tui` shipped in 0.5.0 with the question
screens, nine of them now, and four read this machine: `cost`, `why`, `doctor`
and `share`. The flag-to-widget mapping below
is what is still unbuilt: no screen renders a flag from a running `replay
serve`, and the archetype renderings that do appear, the four-state threshold
meter on the guards screen and the posture block on the safe screen, are drawn
from example data and say so.

Replay has **80 flags across 14 commands**. `serve` carries 29 of them, `probe`
16. That count is the finding rather than the input: a surface that renders 78
flags as 80 widgets is a worse tool than the command line it replaces, because
it asks the reader to hold the same complexity with less to hold onto.

So the question is not "where does each flag go". It is what kind of thing each
flag is, because **twenty of them are the same kind of thing and collapse into
one component with four states**.

Every flag below was extracted from `cmd/replay/*.go`, not from documentation.
All 80 are classified; none are left over.

## The six archetypes

| Archetype | Flags | What it looks like |
|---|---|---|
| Replaces the surface | 7 | There is no TUI. `--json` means a machine is reading, and drawing a frame for it is wrong |
| Plumbing, shown once | 16 | A header line, never interactive. Where it listens, where it writes, what it talks to |
| Threshold that can fire | 20 | A meter with **four** states: unset, armed, approaching, fired |
| Posture, on or not covered | 14 | A line saying what is on, and more importantly what it does **not** reach |
| Scope of the question | 12 | The query line. What this screen is about, and what it excludes |
| Action with a consequence | 11 | A confirmation, with the consequence named before the key |

## Why a threshold needs four screens, not one

`--max-day-usd 5.00` is not a setting. It is a promise that can be in four
conditions, and three of them are invisible if the surface only prints a number:

```text
  unset        day cap                 -            not set
  armed        day cap                 $0.62 / $5.00      12%
  approaching  day cap                 $4.71 / $5.00      94%   !
  fired        day cap                 $5.00 / $5.00     100%   refusing
```

A fifth condition matters more than any of them and belongs to no flag value:

```text
  ! this cap cannot be reached. 12 requests could not be priced, so they
    add nothing to the total and the limit will never be met.
```

That is `SpendGuard.CapNotEnforced`. The user set a cap and does not have one.
No flag expresses it, which is exactly why the surface has to.

## Why posture is written as what it misses

`--mask` is on or off, and printing "masking: on" is true and useless. What an
operator needs is the boundary:

```text
  masking                 on
    patterns              14
    covers                /v1/messages
    NOT covered           /responses, /v1/chat/completions
```

The second line is the one that changes behaviour. A path Replay forwards
without parsing cannot be masked, and a screen that says only "on" invites
somebody to point a Grok seat at it and assume their secrets are handled.

## Actions name the consequence before the key

`--apply`, `--update`, `--record`, `--contribute` and `--execute` all change
something outside the process. The confirmation says what changes, not whether
you are sure:

```text
  contribute to the corpus

  what leaves this machine   1 measurement, 412 bytes
  what does not              prompts, completions, file paths, project names
  where                      the campaign named on the card
  reversible                 no. A sent measurement cannot be recalled

  [y] send    [n] cancel    [d] show me the exact payload
```

`[d]` exists because a consent screen that cannot show you the payload is
asking you to trust a summary of the thing rather than the thing.

## The full mapping

### Replaces the surface (12)

| Flag | Command | Type |
|---|---|---|
| `--json` | `advise` | bool |
| `--once` | `tui` | bool |
| `--json` | `context` | bool |
| `--json` | `cost` | bool |
| `--json` | `route` | bool |
| `--json` | `trim` | bool |
| `--no-color` | `statusline` | bool |
| `--color` | `tui` | string |
| `--check` | `upgrade` | bool |
| `--version` | `upgrade` | string |
| `--older-than` | `purge` | string |
| `--session` | `purge` | string |

<!-- Both belong here for the same reason `--check` does: each replaces what the
command produces rather than tuning how it behaves. `--older-than` makes purge a
retention policy; `--session` makes it an erasure request; the command refuses if
given both, because they describe different deletions. `--export` and `--yes` are
plumbing and classified with the rest. -->

<!-- Both belong here rather than under a posture or a threshold. `--check`
replaces the act of upgrading with a report about it, and `--version` replaces
"whatever is latest" with a tag the reader chose: each swaps what the command
produces rather than tuning how it behaves. `--dry-run` is already classified
elsewhere in this file. -->

### Plumbing, shown once (20)

| Flag | Command | Type |
|---|---|---|
| `--max-avoidable-usd` | `cost` | float |
| `--peek` | `since` | bool |
| `--before` | `prefix` | string |
| `--after` | `prefix` | string |
| `--contribute-dir` | `probe` | string |
| `--export` | `rules` | bool |
| `--install` | `statusline` | bool |
| `--ledger` | `serve` | string |
| `--listen` | `serve` | string |
| `--metrics-listen` | `serve` | string |
| `--card` | `cost` | string |
| `--out` | `advise` | string |
| `--out` | `learn` | string |
| `--png` | `cost` | string |
| `--tone` | `cost` | string |
| `--policy-file` | `serve` | string |
| `--project` | `serve` | string |
| `--screen` | `tui` | string |
| `--token` | `serve` | string |
| `--upstream` | `serve` | string |

### Threshold that can fire (20)

| Flag | Command | Type |
|---|---|---|
| `--breaker-cooldown` | `serve` | duration |
| `--breaker-failures` | `serve` | int |
| `--cap` | `trim` | int |
| `--context-edit-keep` | `serve` | int |
| `--context-edit-trigger` | `serve` | int |
| `--error-budget` | `serve` | float64 |
| `--guardrail-reread` | `serve` | float64 |
| `--loop-block` | `serve` | int |
| `--loop-warn` | `serve` | int |
| `--max-age` | `probe` | duration |
| `--max-day-tokens` | `serve` | int |
| `--max-day-usd` | `serve` | float64 |
| `--max-probes` | `probe` | int |
| `--max-session-tokens` | `serve` | int |
| `--max-session-usd` | `serve` | float64 |
| `--retries` | `serve` | int |
| `--retry-base` | `serve` | duration |
| `--retry-max` | `serve` | duration |
| `--revert-after` | `serve` | int |
| `--trial-share` | `serve` | float64 |

### Posture, on or not covered (15)

| Flag | Command | Type |
|---|---|---|
| `--compare` | `cost` | string |
| `--dir` | `burn` | string |
| `--write` | `agents` | string |
| `--dry-run` | `rules` | bool |
| `--guards` | `advise` | bool |
| `--hold-siblings` | `serve` | duration |
| `--mask` | `serve` | bool |
| `--mask-entropy` | `serve` | bool |
| `--mask-patterns` | `serve` | string |
| `--per-task` | `cost` | bool |
| `--predicted` | `cost` | float64 |
| `--rehydrate` | `serve` | bool |
| `--relative` | `probe` | float64 |
| `--share` | `cost` | bool |
| `--trend` | `probe` | bool |

### Scope of the question (12)

| Flag | Command | Type |
|---|---|---|
| `--pooled-at` | `pool` | string |
| `--candidates` | `probe` | string |
| `--dollars` | `main` | bool |
| `--max` | `probe` | int |
| `--min` | `probe` | int |
| `--min-sessions` | `learn` | int |
| `--model` | `probe` | string |
| `--per-lane` | `cost` | bool |
| `--prior` | `probe` | int |
| `--resolution` | `probe` | int |
| `--to` | `route` | string |
| `--top` | `context` | int |

### Action with a consequence (11)

| Flag | Command | Type |
|---|---|---|
| `--apply` | `advise` | bool |
| `--check-prices` | `rules` | bool |
| `--confirm` | `probe` | int |
| `--contribute` | `probe` | string |
| `--execute` | `probe` | bool |
| `--measure` | `rules` | string |
| `--record` | `probe` | string |
| `--update` | `rules` | string |
| `--x402-json` | `rules` | bool |
| `--yes` | `advise` | bool |
| `--yes` | `probe` | bool |
| `--metered` | `ceiling` | bool |
| `--subscription` | `ceiling` | bool |
| `--day-ceiling` | `ceiling` | float64 |
| `--flat-rate` | `ceiling` | float64 |

## What this does not answer

Which of these screens is reachable, and how. The flags are a complete
inventory; the navigation between screens is not designed here, and guessing at
it would be the third draft of this surface inventing structure ahead of
evidence.

Nothing renders until the context-budget conservation laws hold.

---

[Documentation index](README.md) · [Repository README](../README.md)
