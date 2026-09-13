# The launch announcement, 2026-09-14

**This supersedes the Show HN section of [`launch-draft.md`](launch-draft.md),
which leads with a cause table this repository retracted on 2026-09-11.** Every
figure below is from [break causes, third
reading](../evidence/break-causes-2026-09-13.md) or
[cost](../evidence/README.md), both read on 2026-09-13 against the same corpus.

Written from **Los Angeles**. Post Monday 2026-09-14.

## The rule that shaped every line of this

The old draft led with "50.8% of re-billed tokens are client re-renders". Three
readings in nine days gave 50.8%, then 42.4%, then 39.7%, and the ranking
flipped. **So the announcement does not lead with a cause ranking.** It leads
with the shape, which held across all three, and it publishes the drift itself.

That is not a defensive choice. A post that shows its own headline moving and
explains why is the only post in this category that will do it, and it is the
entire reason to believe the rest.

## Show HN

**Title:** `Show HN: Replay – find the exact turn your agent's prompt cache broke`

Keep it under 80 characters. No numbers in the title; every number needs a
population and a title has no room for one.

**Body:**

> When a prompt cache is invalidated the provider does not fail. It recomputes
> the prefix, bills it at write rates, returns a normal response, and raises
> nothing. The event is visible only in the usage counters on that response, and
> only to something comparing them against the previous request. In practice the
> first signal is the invoice.
>
> Replay is a local Go binary that reads the transcripts your agent already
> writes and names the turn that re-billed, against which predecessor, and why.
> No account, no key, no network call.
>
> **What it found on my own machine.** 1,950 transcript files, 121 sessions,
> 72,439 requests, 32 days to 2026-09-12. One machine, one operator. 833 cache
> breaks, about 44.2M tokens re-billed.
>
> The interesting part is not which cause is biggest. It is that I do not know,
> and I can show you why. Three readings in nine days:
>
> | read | corpus | client re-render | TTL expiry |
> |---|---|---|---|
> | 2026-09-06 | 1,506 transcripts | 50.8% | 33.9% |
> | 2026-09-11 | 1,816 transcripts | 42.4% | 42.6% |
> | 2026-09-13 | 1,950 transcripts | 39.7% | 43.5% |
>
> The ranking flipped between the first and the second and has not flipped back.
> The corpus grew and the engine took several hundred commits, and the re-runs
> cannot separate those two. So the cause ranking is a snapshot, not a finding,
> and every one of those readings is still published with its date.
>
> **What held all three times is the shape.** TTL expiries are rare and
> enormous: 97 breaks at a mean of 198,268 tokens. Re-renders are frequent and
> small: 600 breaks at a mean of 29,240. One developer going to lunch costs more
> than a hundred re-renders. That survived a rank flip, which is why it is the
> claim I am willing to make.
>
> **What it will not do.** If you are on a flat seat with short single-lane
> sessions, the recoverable figure is zero dollars. That is published as a null
> result, not buried. Set `promptCacheTtl: 1h`, freeze your tool list, and you
> do not need this. It reads Claude Code and Codex transcripts; Cursor's store
> carries no cache fields so there is nothing to read. It refuses to run on
> Windows, because the directory-privacy check it relies on is a no-op there and
> I would rather decline than pretend.
>
> Everything is `[measured]`, `[estimated]` or `[structural]`. If a provider did
> not return a counter it reports **unknown** and refuses to print a number.
> Absence, zero and unknown are three values.
>
> Business Source License 1.1, converting to Apache 2.0 on 2029-09-06. That is
> source-available, not OSI open source, and I am not going to blur it.
>
> ```bash
> curl -fsSL https://replay.doctor/replay.sh | less   # read it first
> curl -fsSL https://replay.doctor/replay.sh | sh
> ```
>
> <https://github.com/RedRobotKK/Replay>

## The first comment, posted within two minutes

**This is the most important 200 words of the day** and it exists because two
independent audience models named the same worst case: a reader finds
`docs/MONEY-PATH.md`, pairs a provisional per-repository price with the revenue
target and the BUSL licence, and posts all three together. The reading requires
no bad faith.

**A document volunteered cannot be used to expose the person who volunteered
it.** So it goes first, in the author's own voice:

> Author here. Three things before anyone has to dig for them.
>
> **The corpus is one machine.** Mine. `replay pool` has exactly one member and
> the roster says so. Every population claim in this post is a claim about one
> operator's laptop, and the single thing I want from today is a second machine.
>
> **I am trying to build a business on this and the plan is in the repo.**
> `docs/MONEY-PATH.md` has the pricing thinking, the revenue arithmetic and the
> parts that do not work. It is unflattering in places. I would rather you read
> it from me than find it.
>
> **The licence is BUSL 1.1, which is not open source.** It converts to Apache
> 2.0 on 2029-09-06. Running it at work, in production, at any scale, in CI is
> free and unrestricted. The one thing it forbids is reselling it as a hosted
> service.
>
> The tool's own retractions are in `docs/evidence/`, including a 98.8% that
> became 4.2% the same day, with the wrong figure still on the page.

## What is being asked for, and it is not a star

**The ask is a corpus, not a star.** Stars do not close the one gap that matters.

> If you run agents on a metered bill, `replay cost --contribute` writes about
> 19 numbers to a file, prints the exact curl line, and sends nothing on its own.
> You read the file first. The pool and every contributed file are public. That
> is the only thing on this page I actually want.

## Product Hunt

Different audience and the models are blunt about it: a large share of that
audience cannot run a terminal binary at all. **Expect it to underperform and do
not spend the day's energy there.** Ship it because it costs an hour.

**Tagline (60 char max):** `Find the exact turn your AI agent's cache re-billed you`

**Description:**

> Prompt caches break silently. The provider recomputes the prefix, bills it at
> write rates, and returns a normal response. Nothing errors. The first signal
> is the invoice.
>
> Replay is a free local Go binary that reads the transcripts Claude Code and
> Codex already write, and names the turn that re-billed, which predecessor it
> broke against, and why.
>
> It sends nothing. No account, no key, no network call. An import allowlist
> test in the repository enforces that rather than a promise in a README.
>
> On one machine over 32 days it found 833 cache breaks and about 44.2M
> re-billed tokens. That is one operator's laptop, it is labelled as such
> everywhere it appears, and widening it past one machine is the only thing this
> launch is for.
>
> Every figure carries its population and its date. The corrections stay on the
> page with the wrong number still readable.

**First comment:** the same three disclosures as the HN comment, shortened. The
maker comment carries the licence and the one-machine caveat, in that order.

## Lines that must not appear anywhere

Carried from the deliverable rules and the ban list, because launch copy is
where they get broken:

- No "save", "savings", "optimise", "unlimited", "typical", "should".
- **No percent-of-savings and no forecast.** The re-billed figure is what was
  already spent twice, never a prediction of what a change recovers.
- No em-dashes. No "it's not just X, it's Y".
- No figure without its population and its date **in the same sentence**.
- No cause ranking presented as a finding. See the top of this file.
- No vote asks, no astroturfing, no fabricated proof.

## The thresholds, already committed

Pre-registered on 2026-09-12 in
[launch-prediction-2026-09-14](../evidence/launch-prediction-2026-09-14.md):
front page 25%, at least one non-maintainer corpus in 7 days 15% to 25%,
Product Hunt top five 0.00% across 40,000 draws.

**Not recorded as evidence of anything:** upvotes, rank, badges, comment counts,
stars. **Recorded:** distinct source IPs fetching `/replay.sh`, `replay pool`
membership delta, issues opened by a non-maintainer.

The only comment category that counts as evidence is a stranger reporting their
own number.

---

[Launch drafts](launch-draft.md) ·
[Break causes, third reading](../evidence/break-causes-2026-09-13.md) ·
[Pre-registered KPIs](../evidence/launch-prediction-2026-09-14.md)
