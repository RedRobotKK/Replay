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
explains why gives a reader something to check, which is the only reason to
believe anything else in it.

## Show HN

**Title:** `Show HN: Replay - find the exact turn your agent's prompt cache broke`

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
> Reading your transcripts needs no account and no key and makes no network
> call.
>
> It is not a binary that never touches the network, and I am not going to say
> it is. `replay proxy` is a proxy, `probe --execute` bills real requests on
> purpose, and `replay upgrade` fetches a release. Those are the four typed
> network commands and you invoke them. What cannot send is the CONTRIBUTION
> path: `internal/observation` builds a file and has no transport in it, which
> an import allowlist test enforces, so a submission cannot leave by accident.
>
> **What it found on my own machine.** 1,950 transcript files, 121 sessions,
> 72,439 requests, over 32 days to 2026-09-12 of which 27 were active. One
> machine, one operator. **833 cache breaks and about 44.2M re-billed tokens,
> read by `replay diff` on the v0.6.0 build on 2026-09-13.**
>
> That figure needs its command and its build attached, and here is why, since
> it is the best thing I can show you about how this project works. `replay
> cost` on v0.5.4 reports 44,214,854 re-billed tokens for the same corpus. Two
> numbers agreeing to 0.03%, and they are not the same quantity. On the current
> build `cost` reports 71.4M and `diff` reports 44.2M, because they count
> different things. I nearly shipped that collision in this post.
>
> I do not know which cause is biggest, and I can show you why. Three readings
> in nine days:
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
> small: 600 breaks at a mean of 29,240. **One developer going to lunch costs
> about seven re-renders**, and 97 of them account for more re-billed tokens
> than 600 re-renders do. That survived a rank flip, which is why it is the
> claim I am willing to make.
>
> **What it will not do.** On a flat seat you are not billed per token, so every
> dollar here is list price for somebody else and the tokens are still yours.
> Whether a re-billed token also burns your rate-limit window is **measured,
> unresolved and published as a null**: 3.09M tokens moved the utilisation
> counter by zero steps, which is a null about the instrument's sensitivity
> rather than a finding that nothing is recoverable. The file says in terms that
> nothing licenses a claim in either direction. Set `promptCacheTtl: 1h`, freeze your tool list, and you
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
> **I am trying to build a business on this and the plan is in the repo, with
> the numbers.** `docs/MONEY-PATH.md` carries a provisional $199 per repository
> per month, a $500,000 post-tax target, and this sentence of mine about the
> first of those: "$199 a month to recover a measured $60 to $90 a month."
> Nothing is for sale today and no price is on any public page. I would rather
> you read that from me than find it.
>
> **The licence is BUSL 1.1, which is not open source.** It converts to Apache
> 2.0 on 2029-09-06. Running it at work, in production, at any scale, in CI is
> free and unrestricted. It forbids two things: offering it to third parties as
> a hosted or managed service, and embedding it in a product that derives
> substantial value from its measurement of agent traffic and cost. Read the
> Additional Use Grant rather than my summary of it.
>
> The tool's own retractions are in `docs/evidence/`. The largest: a 98.8% and
> an 11.0% that were **both retracted** when the instrument turned out to be
> comparing each request against whichever sibling lane wrote last. 4.2% is the
> replacement measurement and it is **a different quantity, not a corrected
> estimate of the same one**, because the retracted figures were shares of a
> broken run's own deficit total. All of it is still on the page. A separate,
> un-retracted 98.8% in a later file is a different measurement again, which is
> exactly why every figure here carries its population.

## What is being asked for, and it is not a star

**The ask is a corpus, not a star.** Stars do not close the one gap that matters.

> If you run agents on a metered bill, `replay cost --contribute` writes 19
> fields to a file and prints the path. It sends nothing and it has nowhere to
> send. You read the file, and if you are happy with it you attach it to a pull
> request. The pool and every contributed file are public.

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
> Reading your transcripts needs no account, no key and no network call. The
> contribution path cannot send at all: it builds a file, and an import
> allowlist test refuses every transport in that package. The proxy, the probe
> and the upgrader do reach the network, you invoke them, and they are listed.
>
> On one machine, over 32 days to 2026-09-12, `replay diff` on the v0.6.0 build
> found 833 cache breaks and about 44.2M re-billed tokens. That is one
> operator's laptop, it is labelled as such everywhere it appears, and widening
> it past one machine is the only thing this launch is for.
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
