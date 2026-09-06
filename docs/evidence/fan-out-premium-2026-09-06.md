# The fan-out premium

**Measured 2026-09-06 on one machine's corpus. Method below; read it before the number.**

When an agent fans out into parallel subagent lanes, the siblings do not share the cache write
for the prefix they have in common. Each one writes it again.

A cache write is billed at **1.25×** and a read at **0.1×**, so a sibling that writes a prefix
pays **12.5× what a sibling that reads it would pay**. Fan out five ways and you buy the same
prefix five times at the expensive rate.

## The number

Against the counterfactual in which the first sibling writes the shared prefix and the rest read
it, real fan-out cost:

| lanes opened together | groups | premium (aggregate) | premium (median group) |
|---|---:|---:|---:|
| 2 or more | 483 | **1.68×** | 1.48× |
| 3 or more | 79 | **2.56×** | 2.24× |
| 5 or more | 24 | **3.34×** | 2.93× |

**It grows with width**, which is what makes it worth knowing: the wider you fan out, the worse
the multiple, so the cost of parallelism is superlinear in exactly the regime people reach for it.

## Method

- **One machine, one operator, one account.** Same corpus limitation as every other figure here,
  and it is the open gap the roadmap names.
- **Deduplicated on `requestId` before anything else.** A subagent lane re-renders its parent's
  requests, so **54.8% of usage records in this corpus are duplicates** (54,636 of 99,747). An
  earlier attempt at this measurement that skipped the dedupe overstated project spend by about
  5×. Any analysis of this corpus that sums usage across files without deduplicating is wrong.
- **A lane's opening request only.** That is where the shared prefix is either written or read.
- **A fan-out group** is lanes under one session whose opening requests fall within a time window
  of each other.
- **Effective tokens** use the provider's own multipliers, 1.25 for a write and 0.1 for a read —
  the same arithmetic the replay engine uses.
- **The counterfactual** is one sibling writing the shared prefix and the others reading it, where
  the prefix is taken as the largest opening prompt in the group.

## Robustness

The window is not doing the work. Across 30, 60, 120 and 300 seconds:

| window | ≥2 lanes | ≥3 lanes | ≥5 lanes |
|---|---:|---:|---:|
| 30s | 1.61× | 2.38× | 3.00× |
| 60s | 1.64× | 2.42× | 3.24× |
| 120s | 1.68× | 2.56× | 3.34× |
| 300s | 1.73× | 2.58× | 3.30× |

The premium moves by less than 0.1× at the 2- and 3-lane thresholds across a tenfold change in the
window. The trend with group size is larger than any sensitivity to how a group is defined.

## What this does not say

- **It is not a saving that is available today.** The counterfactual is arithmetic, not a setting.
  Whether a provider can be made to serve one write and many reads to concurrent siblings is a
  separate question this measurement does not answer, and nothing here should be read as a
  promise that the premium is recoverable.
- **It is not a claim about any provider's correctness.** Writing per sibling may be the only
  thing a concurrent cache can do. The finding is about the shape of the bill, not a defect.
- **The 5-or-more row rests on 24 groups** from one operator's working style. Treat it as a
  direction with a number attached, not as a rate.
- **It does not measure whether fanning out was worth it.** Parallel lanes buy wall-clock time and
  independent context. This prices one side of that trade only.

## A correction to an earlier framing

`docs/PRODUCT-DIRECTION.md` offers, as an example of a publishable sentence, *"Your six sub-agents
cost 4.2x one agent, not 6x."* That was written as an illustration rather than a result, and the
measurement points the other way: against a naive per-agent baseline fan-out may well be
sublinear, but against the cache-sharing baseline it is **super**linear, and the reason is the
same in both cases — the siblings each pay the write. The interesting sentence is not that six
agents cost less than six; it is that they cost about two and a half times more than the same
work would if the cache were shared, and that the multiple gets worse as you widen.

## Reproducing it

Every input is in the transcripts already on disk: `cache_creation_input_tokens`,
`cache_read_input_tokens`, `sessionId`, `requestId` and `timestamp`. No proxy, no key, no network.
Deduplicate on `requestId` first or the result is meaningless.
