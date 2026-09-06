# Does a cache break burn a subscription quota the way it burns a bill?

**Measured 2026-09-06, one account, one machine. The answer is: not measurable
with this instrument at this budget, and here is what that cost to establish.**

## The question

A provider bills a prompt-cache write at 1.25x and a read at 0.1x, so a break
costs a metered user 12.5x a read. Whether a flat-seat SUBSCRIPTION limit counts
the same way is undocumented, and it decides whether Replay has anything to say
to the majority of its users, who spend no dollars at all.

## Design

Better than a token-volume contrast, because the confound is removed by
construction. A fresh `claude -p` writes a ~159,000 token prefix cold. The same
prompt with `--resume` READS a ~159,000 token prefix. Same model, same size,
same shape - only warm against cold differs. Ten of each, back to back.

## Result: 3.09M tokens moved the counter by zero

| | |
|---|---|
| write tokens | 1,012,229 |
| read tokens | 2,078,382 |
| 5h utilization | 0.25 -> 0.25 |
| steps moved | **0** |

The counter carries two decimals, so one step is 1% of the window. Three million
tokens of haiku traffic did not produce one.

## What that refutes

**An earlier calibration in this repo, which was wrong.** On the first probe,
four requests carrying ~475,000 write tokens coincided with the 5h figure moving
0.20 to 0.21, and that was recorded as "one step is approximately 475,000
tokens". This run says otherwise: more than twice that volume moved nothing.

The earlier step was almost certainly not caused by the probe. The counter is
ACCOUNT-WIDE, and the same account was running an interactive Claude Code
session throughout. The red-team review named this exact hazard before the run,
and the run confirmed it: passive attribution of an account-wide counter to
individual requests does not work.

Any figure derived from that 475,000 calibration should be treated as void.

## What it establishes

**The classifier was broken, and it was found in live data rather than by
reasoning.** Of the 21 requests, 14 carried BOTH a cache read and a cache
creation, and the shipped rule - `Wrote = CacheCreation > 0` - scored every one
of them as a write. Zero were classified as reads.

The shape is ordinary. A resumed turn reads 159,434 tokens and extends the
prefix by 76. That is a read with a rounding error attached, and calling it a
write puts the cheapest requests in the expensive arm - biasing the ratio toward
1.0, which is the answer that says subscriptions do not charge like the bill,
reached by misclassification rather than by evidence.

Fixed: a request joins an arm only when one kind of cached token holds 90% of
the cached total. Mixed requests are excluded from both, because a request that
is 40% write and 60% read carries no clean contrast and averaging it in is how a
null gets manufactured.

## What would be needed

The budget, honestly stated. To move the counter 20 steps per arm at haiku
volumes would take well over 30M tokens per arm on this evidence, which is a
large share of a 5-hour window and would risk locking the operator out - to
measure lockout. That is not a reasonable experiment to run on a working
account.

Three routes are cheaper than brute force:

- **A quiet account.** The account-wide confound is the largest error term and
  it is removable, not reducible. Thirty minutes of confirmed idle before each
  arm, with any movement voiding the block.
- **A larger per-request payload.** Steps are what carry information, and a
  request writing ten times more prefix produces them ten times faster for the
  same request count.
- **Opus rather than haiku.** If the limit is weighted by model - untested, and
  worth testing on its own - haiku is the slowest possible way to move it.

## Status

The instrument is honest and the question is open. `Compare` refuses on this
data, correctly, and says which arm is short. Nothing here licenses a claim in
either direction: a ratio near 12.5 and a ratio near 1.0 remain equally
consistent with what has been measured.
