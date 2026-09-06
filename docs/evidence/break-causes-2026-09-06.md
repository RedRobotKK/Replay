# What actually breaks the cache, and what a layout change could fix

**Measured 2026-09-06 across 1,506 transcripts on one machine: 735 cache breaks,
31,264,349 tokens re-billed. Method below; read the sampling note before the
number.**

## Result

| cause | breaks | tokens re-billed | share | mean per break |
|---|---:|---:|---:|---:|
| client re-rendered history after the system prefix | 583 | 15,876,000 | **50.8%** | 27,232 |
| cache expired (gap longer than the TTL) | 39 | 10,586,000 | **33.9%** | 271,436 |
| prefix diverged inside the message history, block unknown | 102 | 2,193,349 | 7.0% | 21,503 |
| system prompt or tool definitions changed | 5 | 1,807,000 | 5.8% | 361,400 |
| model changed between requests | 6 | 802,000 | 2.6% | 133,667 |

Grouped by what could plausibly address them:

- **Prefix layout and ordering: 63.6%** of re-billed tokens.
- **Timing and TTL: 33.9%.**

The two families have opposite shapes and that matters more than the ranking. A
re-render is frequent and small - 583 of them at about 27,000 tokens each. A TTL
expiry is rare and enormous - 39 of them at about 271,000 each, ten times the
size. A single developer going to lunch costs more than a hundred re-renders.

## The sampling note, which is the point of this document

This measurement was first run on the **40 largest sessions**, and it produced
almost the opposite answer:

| | 40 largest | full corpus |
|---|---:|---:|
| cache expired (TTL) | **75.2%** | 33.9% |
| system prompt or tools changed | 15.2% | 5.8% |
| client re-rendered history | 2.5% | **50.8%** |

On that sample the conclusion was "three quarters of the money is TTL expiry;
prefix layout addresses 15% and is not worth building". On the full corpus the
conclusion reverses: layout-addressable causes are 63.6%.

The bias is obvious in hindsight and was predictable in advance: the largest
sessions are the long-running ones, long-running sessions contain long gaps, and
long gaps are what TTL expiry means. Sorting by size selected for the cause.

It was flagged as a caveat at the time and the conclusion was stated anyway. The
caveat should have blocked the conclusion rather than accompanying it, because
the full run cost five seconds. **A stated limitation is not a substitute for
removing it when removing it is cheap.**

## Method

- `replay diff` over every `*.jsonl` under `~/.claude/projects`, 1,506 files,
  parsing the cause and re-billed figure it already reports per break. No new
  analysis: the classifier has been in the tool since v0.1 and every cause is
  one of seven named constants.
- Causes are the tool's own, not re-derived here. `CauseRerendered` is
  explicitly "client re-rendered history after the system prefix (no edit
  visible in transcript)" - the transcript shows no edit, so this is inference
  from the provider's cache read, not an observed diff.
- Re-billed figures are as printed, rounded by the report to three significant
  figures, so the total carries that rounding.
- Zero unpaired lines in the parse: every cause line had a re-billed figure
  attached.

## Limits

- **One operator, one machine.** n=1 at the level that matters. Another
  developer with different session habits would very plausibly invert this
  again, exactly as the 40-largest sample did.
- **This corpus is fan-out heavy**, and `client re-rendered history` is the
  cause most likely to be produced by sub-agent lanes re-rendering a parent's
  history. A single-lane user may see almost none of it. That would move 50.8%
  somewhere else.
- **A re-render is not obviously fixable by a memory layout.** It is a client
  behaviour, not a prompt-assembly choice, and nothing here establishes that a
  proxy could prevent it. The 63.6% is what layout could plausibly ADDRESS, not
  what any design has been shown to RECOVER.
- Skipped: sessions `replay diff` could not calibrate. They are excluded rather
  than counted as zero, per the tool's usual rule.

## What this does and does not license

It licenses ordering the work: prefix stability is the larger pool by tokens,
and TTL is the larger pool per event. Both are worth attention and the second
is much cheaper to act on - a session idle at 55 minutes holding a 700,000 token
warm prefix is about to re-bill all of it, and saying so is a notification
rather than an architecture.

It does not license any claim that a particular layout recovers any of it. That
requires a live trial with a control arm, which is what `--trial-share` and the
graduation rules in `replay learn` exist for.
