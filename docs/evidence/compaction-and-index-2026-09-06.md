# What compaction discards, and what the transcript index saves

**Measured 2026-09-06 on one machine, 1,520 transcripts. Both figures exist
because they were being quoted in conversation before anything on disk could
support them.**

That is the reason for this file. A documentation pass refused to state a
"127x faster" figure and a "30.5M tokens dropped" figure on the grounds that
neither appeared anywhere in the repository — correctly, because a number
whose only home is a chat log is unsourced however honestly it was obtained.
Both were re-measured to write this, and one of them changed.

## Compaction

39 records carrying `compactMetadata`, every one `trigger: "auto"`.

| | |
|---|---|
| median preTokens | 999,029 |
| median postTokens | 23,218 |
| **median retention** | **2.55%** |
| total pre / post | 32,038,701 / 1,501,091 |
| **discarded from context** | **30,537,610 tokens** |
| **wall clock spent compacting** | **73.5 minutes** |

**2.55% is the median of the per-event ratios. The ratio of the medians is
2.32%.** Both appear defensible and they are not the same statistic; adversarial
review flagged them sitting adjacent without a label, which is how a reader
ends up quoting whichever they saw. The retention figure quoted anywhere should
be the median of ratios, because the question is what a typical compaction
keeps, not what a typical numerator over a typical denominator would be.

Two features of the shape matter more than the averages. Compaction fires at a
ceiling near one million tokens — `preTokens` is within 1% of 1,000,000 on 30
of 39 events — so it is triggered by exhaustion, not by policy. And it takes
103 to 180 seconds each time, which is latency the operator waits through.

One record reports `postTokens` **above** `preTokens` (22,303 against 296,742).
It is kept and clamped rather than dropped, because a single record producing a
negative would silently offset several good ones.

## The transcript index

`replay cost` re-derived every transcript on every run. Transcripts are
append-only and most never change again, so the work was almost entirely
repeated.

| | |
|---|---|
| cold, no index | **6.474s** |
| warm | **0.046s** |
| warm again | 0.044s |
| **speedup** | **141x** |

Measured back-to-back on 1,520 transcripts, timing the whole process. The
figure quoted in conversation before this file existed was "127x (6.2s to
0.049s)" — the same measurement on a slightly smaller corpus, close enough to
be honest and different enough to show why it needed writing down.

The index is keyed on each file's size and mtime, on the price and rules
versions, and on the shape of the record it stores, derived from that struct's
own JSON tags. That last part was added after a defect: `AvoidableTokens` was
added to the cached struct without bumping a hand-written schema literal, and
the same binary then printed 763k tokens on a warm run against 31.4M on a cold
one, with the already-cached dollar column agreeing in both.

## Limits

One operator, one machine, one client version. The compaction figures come from
39 events in 9 transcripts, and 22 of those events are from a single session.
Nothing here says what a different developer's sessions would show, and the
concentration means these are close to a case study.
