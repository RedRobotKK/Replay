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
Cost per task, across 1505 transcripts at list prices dated 2026-06-24
(caching rules anthropic-2026-09-01).

  total          $3018.99
  median task    $0.65
  p90 task       $2.16
  avoidable      $150.27  (5% of the total)
                 31.4M tokens re-billed

Avoidable is the part nobody chose: tokens re-billed because a prompt cache
broke. It is not a forecast of savings, it is what was already spent twice.

On a subscription seat - Claude Pro or Max, Copilot, Cursor - none of that is
money: you are not billed per token, so the dollars above are list price for
someone who is. The tokens are still yours. They are context the work did not
get, and rate-limit budget spent on nothing. `replay advise` ranks what to cut.
```

Those are one machine's numbers on 2026-09-06, read from the transcripts Replay found on it. Point it
at yours, or give it a directory of your own.

**The avoidable figure is stated twice on purpose.** Most of the people who run this hold a flat seat,
and a dollar figure addressed to someone else reads as a number that does not apply — which is how a
real finding gets dismissed. The tokens apply to everyone: a re-billed token is context the work did
not get, on a window you are rate-limited against either way. Whether a break also burns a
subscription quota the way it burns a bill is measured, unresolved, and
[written up as null](docs/evidence/quota-titration-2026-09-06.md) rather than assumed in either
direction.

`transcripts` counts files, not sessions: a session writes one transcript per agent lane, so a
session that spawned sub-agents contributes several. `replay doctor` reports both figures side by
side. The same fan-out means a sub-agent lane re-renders its parent's requests, so a few requests are
read from more than one file; the report says how many rather than implying the total is exact — 430
of 30,977 requests, 1.4%, on the run above.

## What actually broke the cache

`replay diff` classifies every break, so the money has a cause attached rather than a total. Across
1,506 transcripts on the same machine, 735 breaks and 31.26M re-billed tokens
([method and limits](docs/evidence/break-causes-2026-09-06.md)):

| cause | share of re-billed tokens | shape |
|---|---:|---|
| client re-rendered history after the system prefix | 50.8% | 583 breaks, ~27k tokens each |
| cache expired (gap longer than the TTL) | 33.9% | 39 breaks, ~271k tokens each |
| prefix diverged inside the message history | 7.0% | 102 breaks, ~22k tokens each |
| system prompt or tool definitions changed | 5.8% | 5 breaks, ~361k tokens each |
| model changed between requests | 2.6% | 6 breaks, ~134k tokens each |

The two large causes have opposite shapes, and that matters more than the ranking: a re-render is
frequent and small, a TTL expiry is rare and enormous. One developer going to lunch costs more than a
hundred re-renders.

Read the sampling note in that file before quoting the number. The same measurement over the 40
largest sessions said TTL expiry was 75.2% and re-rendering 2.5% — almost the reverse — because the
largest sessions are the long-running ones, long-running sessions contain long gaps, and long gaps
are what TTL expiry means. Sorting by size selected for the cause.

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

The same instinct applies to the answer as well as the input. `replay route --to` now prices **the
move itself**: the destination model starts cold and has to write the shared prefix again before it
reads any of it, so a cheaper model is not automatically cheaper. It reports the switch cost, the
saving per turn, and the turn on which those cross — and says plainly when that turn lies beyond the
number of turns actually measured, which is the case a comparison of two price-per-token figures
cannot see at all. Dollar figures also carry the age of the table they came from, because a date
tells a reader what was used and only a subtraction tells them it is stale.

## What it does

```sh
replay                             # cost per task across the transcripts on this machine
replay diff      <session>         # where the cache broke, and why
replay advise    <dir>             # what to change, from your own history
replay serve                       # a local proxy, for measured rather than estimated figures

replay cost                        # the cost report on its own, over the same discovered root
replay context   <session>         # what is filling your context, ranked
replay blame     <session>         # which content cost the most, carried across turns
replay route     <dir> --to <model>   # what a switch changes, including what the switch costs
replay doctor                      # what is on this machine, and what to run next
```

`replay cost` and `replay corpus` take a directory, but no longer require one: with no argument they
read the transcript root `replay doctor` already discovers, and say on stderr which root that was. The
argument still wins when you give it. This is not a convenience — a first command that needs a path
the reader does not know yet is a command they do not run.

`replay --help` lists all seventeen, grouped and ordered by what they are worth rather than
alphabetically, because the list is what a person reads before they know which of them matters. Full
reference: [`docs/guide/commands.md`](docs/guide/commands.md).

`replay context` now says when its own answer is incomplete. Claude Code records a compaction with the
prompt size before and after it, and nothing here was reading that field, so a session that compacted
was attributed as though everything it ever loaded were still present. It is not: the attribution
describes what remains, and the report now names how many compactions fired, how many tokens the
client says they dropped, and therefore by how much the ranking above it overstates. Where the
compaction recorded no size, it says that instead of guessing — an unmeasured overstatement is still
worth declaring.

## Footprint

- **No account, no telemetry, no first-run prompt.** Nothing to opt out of.
- **One ask, at most once every thirty days.** If `cost` has just found more than $5 you paid
  twice, it prints one line about the tip jar. That is the only time this tool asks you for
  anything. It opens no browser and sends nothing: `~/.replay/tip.json` holds the date it last
  asked and a random local seed, and the seed only picks which of two wordings you see. Once a
  month, because asking every run would train you to skip the last paragraph, and the last
  paragraph is often where the caveat is.
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

The second open gap is the one the flat-seat framing above rests on. A metered user is re-billed for
a broken cache; whether a subscriber's rate-limit window is charged the same way is undocumented, so
it was measured: matched cold-write and warm-read arms, 3.09M tokens, and the utilisation counter
moved **zero** steps. That is a null result and it is published as one. It also voided an earlier
figure in this repository — a counter step attributed to four probe requests, on an account-wide
counter that an interactive session was moving at the same time. The instrument now refuses rather
than reports: it names which arm is short instead of dividing anyway, after simulation showed the
first estimator returning exactly 1.00 whether the true ratio was 12.5 or 1.0.

## Who should not use this yet

- You do not use a coding agent that keeps transcripts. There is nothing to read.
- You want a savings forecast. Replay reports what was already spent, not what you will save.
- You want a number without a caveat. Most figures here carry one, because most of them earn one.
- **You are on Windows.** See below.

## Platform support: macOS and Linux only

**Replay is not supported on Windows, and it has never been tested there.**

That is a stronger statement than "we have not got to it", and it is the honest
one. The CI matrix has listed a Windows job for a long time and that job has
never run a single test: it failed earlier, at `go vet`, on a helper that only
existed in a file tagged `//go:build unix`. The compile error was fixed on
2026-09-06, and the first test run it ever produced failed fourteen tests.

Several of those are not portability chores. `TestS5_TheFileIsOwnerOnly` and
`TestU2_PermissionMustBeExplicit` assert Unix file-mode semantics that Windows
does not have, and the guarantees they check are guarantees this tool makes
about your ledger and your masking vault. Until there is a Windows story for
those, a Windows build would be a binary that runs while quietly not keeping
its promises. That is worse than not shipping one.

**macOS and Linux are tested on every push**, with `go vet` and
`go test -race`. WSL works, because it is Linux.

## How the tests work here

The project's governing rule is [ADR-0014](docs/adr/0014-checks-must-be-able-to-fail.md): **a check
is not evidence until it has been observed to fail.** Roughly twenty defects in a single day shared
one shape — a verification that could not fail — so the rule is now mechanical.

`internal/mutation` keeps **52 real past defects frozen as re-runnable mutants** (M1 to M52), each
with the named test that must catch it. `go test -tags mutation ./internal/mutation/` re-applies them
all.
It has already caught a false kill (a mutant the compiler rejected, scored as caught), a test that
hung instead of failing, and a catalogue entry naming a test that was not actually load-bearing.

This is error seeding, not mutation analysis: the denominator is 52 chosen edits, not a generated
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
