# Replay

**Your coding agent is billing you twice.** Replay finds where, and what it cost.

When a prompt cache breaks, the provider silently re-bills the whole conversation history at write
prices. Nothing tells you. Replay replays your sessions against the provider's caching rules and
prints the number.

```sh
curl -fsSL https://redrobot.jp/replay.sh | sh
```

Then, with no proxy, no configuration, no account and no arguments:

```sh
replay
```

```text
Cost per task, across 1443 transcripts at list prices dated 2026-06-24
(caching rules anthropic-2026-09-01).

  total          $2976.12
  median task    $0.65
  p90 task       $2.25
  avoidable      $149.14  (5% of the total)

Avoidable is the part nobody chose: tokens re-billed because a prompt cache
broke. It is not a forecast of savings, it is what was already spent twice.
```

Those are one machine's numbers, read from the transcripts Replay found on it. Point it at yours, or
give it a directory of your own.

`transcripts` counts files, not sessions: a session writes one transcript per agent lane, so a
session that spawned sub-agents contributes several. `replay doctor` reports both figures side by
side.

---

## Every number says how it was obtained

This is the part that matters, and it is enforced in code rather than promised in a README.

| Tier | Meaning |
|---|---|
| **measured** | Read from the provider's own usage counters, via the proxy |
| **estimated** | Derived through a byte-to-token fit, printed with its error bar |
| **structural** | A property of the request shape, not a measurement |

Nothing prints without one. `replay route --to <model>` **refuses to give a dollar figure** for a
model pair it has not measured, rather than guessing — which is the behaviour a tool that wants to
be trusted has to have, and the behaviour that makes it less impressive on first run.

## What it does

```sh
replay                             # cost per task across the transcripts on this machine
replay doctor                      # what is on this machine, and what to run next
replay cost      ~/.claude/projects/    # the same report, over a directory you name
replay context   <session>         # what is filling your context, ranked
replay blame     <session>         # which content cost the most, carried across turns
replay diff      <session>         # where the cache broke, and why
replay advise    <dir>             # what to change, from your own history
replay serve                       # a local proxy, for measured rather than estimated figures
```

Full reference: [`docs/guide/commands.md`](docs/guide/commands.md).

## Footprint

- **No account, no telemetry, no first-run prompt.** Nothing to opt out of.
- **The binary originates two network requests, both of which you type**: `rules --check-prices`
  fetches a public price table, and `probe --execute` sends billable measurement requests to your
  own provider on your own key, after printing the plan and asking. The proxy forwards your own
  traffic and nothing else. Every outbound and on-disk surface is enumerated in
  [`docs/SURFACES.md`](docs/SURFACES.md), including the ones that were wrong in earlier versions
  of this file.
- **The ledger never stores message text.** It stores block kinds, sizes, timings and usage counts.
- **Apache 2.0**, no dependencies. `go.mod` is three lines.

## How far to trust it

The engine reproduces the provider's own cache reads on **97.46%** of compared turns across 1450
transcripts — but those transcripts come from **78 distinct sessions on one machine, one account and
one operator**. A session writes one transcript per lane, so subagents multiply the file count
without adding an independent draw. Read the sample as 78, not 1450.

Earlier versions of this document said "1363 sessions" while counting files, overstating the
independent sample roughly twentyfold. The correction, with the reasoning, is in
[`docs/evidence/calibration-corpus-2026-09-06.md`](docs/evidence/calibration-corpus-2026-09-06.md).
Every evidence file is dated and never edited after the fact; corrections are new files, and there
are several.

**The open gap is independence, and no amount of data from this machine closes it.** That is
stated in the roadmap rather than buried.

## Who should not use this yet

- You do not use a coding agent that keeps transcripts. There is nothing to read.
- You want a savings forecast. Replay reports what was already spent, not what you will save.
- You want a number without a caveat. Most figures here carry one, because most of them earn one.

## How the tests work here

The project's governing rule is [ADR-0014](docs/adr/0014-checks-must-be-able-to-fail.md): **a check
is not evidence until it has been observed to fail.** Roughly twenty defects in a single day shared
one shape — a verification that could not fail — so the rule is now mechanical.

`internal/mutation` keeps **28 real past defects frozen as re-runnable mutants**, each with the
named test that must catch it. `go test -tags mutation ./internal/mutation/` re-applies them all.
It has already caught a false kill (a mutant the compiler rejected, scored as caught), a test that
hung instead of failing, and a catalogue entry naming a test that was not actually load-bearing.

This is error seeding, not mutation analysis: the denominator is 28 chosen edits, not a generated
operator population, and a first run is a kill by construction. The value is temporal — it asks
whether each guard still exists and still discriminates on a tree that has moved.

## Documentation

Start at [`docs/`](docs/README.md), indexed by why you came. Highlights:

- [Commands](docs/guide/commands.md) — every flag, and what it refuses to do
- [What you get](docs/WHAT-YOU-GET.md) — and the three levers worth more than this one
- [Surfaces](docs/SURFACES.md) — every file and endpoint touched
- [Evidence](docs/evidence/) — dated measurements, including the corrections
- [ADRs](docs/adr/) — the decisions, including the ones that were reversed

## Contributing

Issues and pull requests welcome; see [CONTRIBUTING.md](CONTRIBUTING.md). The project is early and
the [roadmap](docs/ROADMAP.md) says plainly what is unfinished.

If it saved you something, [FUNDING.md](FUNDING.md) says how to say so. The tool is free; the
measurements behind it are real API spend.

## License

Apache 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
