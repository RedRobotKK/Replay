# `replay cost` prices one lane per transcript and drops the rest

**Date:** 2026-09-10
**Status:** measured, not fixed. The fix changes every dollar figure this tool
has published.
**Tier:** estimated (transcripts only)

## How it surfaced

Phase C added a cost column to `replay burn`, pricing every request in every
lane. On this machine the two surfaces then disagreed:

```text
replay cost   $3,721   38,186 requests
replay burn  $10,499   60,404 requests
```

2.8× on the same corpus, in the same minute, from the same price table.

## The mechanism

`replay cost` prices `rep.Policies()`, and `rep` is a `LaneReport` built from
`MainLane(session)` — one lane per transcript file (`cmd/replay/cost.go:534`,
`internal/analysis/report.go:111`). `replay burn` iterates `sess.Lanes` and
prices all of them.

Neither is a bug in arithmetic. They are two different denominators, and only
one of them is the bill.

## Measured, corpus-wide

Walking every `*.jsonl` under `~/.claude/projects` and counting requests two
ways:

| | |
|---|---:|
| files walked | 1,812 |
| files that parse | 1,803 |
| **files with more than one lane** | **8** |
| requests in main lanes only | 38,547 |
| requests across all lanes | 60,401 |
| **dropped by main-lane pricing** | **21,854 (36.2%)** |
| distinct request ids | 59,967 |
| ids appearing in more than one lane | 428 |

The two independent counts corroborate: `cost` reports 38,186 requests against
38,547 main-lane requests here, and `burn` reports 60,404 against 60,401.

## The dropped requests are not duplicates

The obvious explanation is that a session's in-file lanes are the same requests
as the separate `<session>/subagents/agent-*.jsonl` files, counted twice. They
are not.

On session `facfd32e`, whose transcript carries 17 lanes and whose `subagents/`
directory holds 1,019 lane files:

- the main file holds **9,278 distinct request ids** across its 17 lanes, of
  which the main lane is **942**;
- 200 sampled lane files hold **2,583 distinct request ids**;
- **overlap between the two sets: zero.**

The 428 ids that do appear in more than one lane are the known ~1.1% re-render
`replay cost` already discloses in its overlap note. They do not account for
21,854.

## What this means

The undercount is concentrated: **8 files out of 1,812**. Those eight are the
fan-out sessions, and they are where the money is — `docs/evidence/avoidable-concentration-2026-09-10.md`
already found 25 fan-out sessions holding ~95% of avoidable spend. That is why
36% of requests is 2.8× of dollars.

`replay blame` has been disclosing this the whole time, per session:

```text
Scope: 1 of 17 lanes; 8336 requests in this session's other lanes are not counted here.
```

`replay cost` has no equivalent line. The disclosure exists on the surface that
looks at one session and is absent from the surface that reports the total.

## What is NOT established here

- **Which figure is right.** Pricing all lanes is closer to what a provider
  billed, and that is an argument, not a measurement. Nothing here re-derives
  the bill from an invoice, and the tool has never claimed the estimated tier is
  one.
- Whether `burn`'s all-lane figure double-counts anything other than the 428
  known re-renders. The zero-overlap check covers one session and a 200-file
  sample of its lanes, not the corpus.
- Whether `MainLane` selection is right for the reports that legitimately want
  one lane. `blame` and `diff` answer "what filled this conversation", and the
  main lane is arguably the correct scope for both; `cost` answers "what did
  this cost", where it is arguably not.

## Why this is filed rather than fixed

Changing what `cost` prices changes:

- every dollar figure in this repository's evidence files, including the
  calibration corpus series and the concentration analysis;
- the share card, which publishes a rate derived from these totals;
- the corpus contribution payload's `totalUsd`, `medianTaskUsd` and
  `avoidableUsd`, which have already been written to disk by anyone who ran
  `--contribute`;
- `--max-avoidable-usd`, which is a CI gate somebody may have tuned.

A 2.8× change to the product's headline number is a decision about published
figures, not a refactor. It is recorded here with its measurement so that the
decision is made against evidence rather than against a recollection.
