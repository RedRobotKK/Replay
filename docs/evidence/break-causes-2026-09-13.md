# Break causes, third reading, 2026-09-13

**The launch reading.** Taken because `docs/paper/launch-draft.md` still carries
the 2026-09-06 table, and that table's own file has carried a retraction since
2026-09-11. Posting a cause ranking on Hacker News that this repository has
already retracted two files away would be the single worst thing this project
could do, because publishing its own corrections is the whole claim.

**Corpus:** 1,950 transcript files, 2.0 GB, 121 sessions, 1,937 lanes, 72,439
requests, spanning **2026-08-12 to 2026-09-12**, 32 calendar days of which 27
were active. One machine, one operator, one account. Read 2026-09-13 by
`replay diff` at the **estimated** tier (transcripts only, no proxy).

## The reading

**833 breaks, about 44.2M re-billed tokens.**

| Cause | Breaks | Share of re-billed tokens | Mean per break |
|---|---:|---:|---:|
| cache expired (gap longer than the TTL) | 97 | **43.5%** | 198,268 |
| client re-rendered history after the system prefix | 600 | **39.7%** | 29,240 |
| prefix diverged inside message history | 116 | 8.2% | 31,163 |
| system prompt or tool definitions changed | 12 | 5.9% | 218,416 |
| model changed between requests | 8 | 2.7% | 148,750 |

**Precision limit, stated before the conclusion.** `replay diff` prints
re-billed tokens rounded to the nearest thousand and has no `--json`, so these
shares are summed from rounded values. Worst-case rounding is ±0.11 points on
TTL expiry and ±0.68 on re-render, against a gap of 3.8 points. The ranking
survives its own error bar; a gap under one point would not have.

## The ranking is not stable, and this is the third reading that says so

| Read | Corpus | client re-render | TTL expiry |
|---|---|---:|---:|
| 2026-09-06 | 1,506 transcripts | **50.8%** | 33.9% |
| 2026-09-11 | 1,816 transcripts | 42.4% | 42.6% |
| **2026-09-13** | **1,950 transcripts** | **39.7%** | **43.5%** |

Re-render has fallen 11.1 points across nine days and three readings. TTL expiry
has risen 9.6. **They crossed between the first reading and the second and have
not crossed back.**

What the re-runs cannot separate: the corpus grew by 444 transcripts and the
engine took several hundred commits between the first reading and this one.
Whether the movement is the workload changing or the classifier changing is not
answerable from these three numbers, and nothing here claims it is.

**So no cause ranking should be published as a finding.** Each reading is its
reading on its date. That sentence is the finding.

## What did hold across all three

The **shape**, which never moved:

- **TTL expiry is rare and enormous:** 97 breaks at a mean of 198,268 tokens.
- **Client re-render is frequent and small:** 600 breaks at a mean of 29,240.
- The ratio of means is **6.8x**, and it was about 10x on 2026-09-06.

One developer going to lunch still costs more than a hundred re-renders. That
is the claim worth making, it survived three readings and a rank flip, and it
does not depend on which cause is first.

## What this does not measure

- One machine, one operator, one account. It is not a population and the
  `replay pool` roster has one member.
- Estimated tier. No proxy ran, so every figure rests on a byte-to-token fit
  whose error on large lanes runs from ±29% to ±170%
  ([band sensitivity](rerender-band-sensitivity-2026-09-11.md)).
- The re-render class is **inferred** from a cache-read mismatch on a turn whose
  persisted messages are byte-identical to the previous prefix. It is not
  observed as a byte diff, because the bytes that differ were never written to
  the transcript. That inference is the weakest link in the table and it is the
  class with the most breaks.

---

[Evidence index](README.md) ·
[The reading this supersedes](break-causes-2026-09-06.md) ·
[Band sensitivity](rerender-band-sensitivity-2026-09-11.md)
