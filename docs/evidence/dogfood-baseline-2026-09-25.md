# Dogfood baseline: the anchor a forward optimization claim has to beat

**Frozen 2026-09-25, before any optimization was applied.** Build
`v0.6.2-7-g6871022`, command `replay cost ~/.claude/projects`, list prices dated
2026-09-07, caching rules `anthropic-2026-09-05`.

The decision to dogfood forward is right. The measurement design is the part that
can go wrong, and it has gone wrong here before.

## Provenance, added 2026-09-26

**This measurement cannot be reproduced, and that is a property of it rather
than a gap to be closed.** No figure below changed.

| | |
|---|---|
| Measured | 2026-09-25, before any optimization was applied |
| Build | `v0.6.2-7-g6871022` |
| Command | `replay cost ~/.claude/projects` |
| Prices / rules | list dated 2026-09-07, `anthropic-2026-09-05` |
| Population observed | 127 sessions, 2,229 agent lanes, 94,466 requests |
| Dominant session | `eec05948`, over half of everything priced |

**Source-tree identity is partial.** The population counts and the dominant
session name identify what was observed. The tree itself was never snapshotted,
because the command reads the operator's live transcript directory.

**Demonstrated divergence.** The same command, on 2026-09-26, returned **482
sessions, 2,587 agent lanes and $18,105.79** against the frozen 127 sessions and
$17,738.82. That reading exists only to show the population changed. **It is
not a reproduction, not a correction, and not the forward comparison this file
pre-registers, which remains unrun.**

### What is still guaranteed, and what is not

| claim | status |
|---|---|
| The four components sum to $17,738.82 | **reproducible**, arithmetic, pinned by `internal/dogfoodbaseline` |
| The three rate metrics follow from that block | **reproducible**, same |
| The block itself | **not reconstructible**: its input population no longer exists |
| The 10,000-byte trimming figures | **not reconstructible**, same reason |

`internal/dogfoodbaseline` deliberately reads nothing from disk, and a test
enforces that by checking its imports. A harness pointed at today's tree would
measure a different corpus and call it the 2026-09-25 baseline, which is the
error this file was written to avoid.

### Erratum: the document prints two totals

The block gives **$17,738.82**, which is what its four components sum to. The
sentence about session `eec05948` gives **$17,738.65**, seventeen cents lower.
The frozen figures are not edited to resolve this. The discrepancy is recorded
here and pinned by a test so a reader meets it deliberately, and **the three
rate metrics are computed from $17,738.82**.

## The trap this file exists to avoid

`track()` in the advisor once inferred that a change had been made from the very
number it then used to score that change. A target drifting 30% to 22% across two
sessions of unrelated work returned VERIFIED with an 8-point realized saving to a
reader who had changed nothing. That produced the 20-against-1 figure, which this
repository later established was **corpus drift, not failed advice**.

A forward dogfood comparison walks straight back into it. **Next month is
different work.** If the totals fall, that may be because less work happened. If
they rise, that may be because more did. Comparing totals before and after an
optimization measures the calendar.

## The baseline

```text
127 sessions, 2,229 agent lanes, 94,466 requests

  cache write    $2,660.30
  cache read    $13,880.70
  uncached           $2.04
  output         $1,195.78
  total         $17,738.82
  median task        $0.78
  p90 task          $12.80
  re-billed        $668.43   (4% of total)
                  114.0M tokens re-billed
```

One session, `eec05948`, is **$9,016.27 of $17,738.65**, just over half of
everything priced. Any aggregate here is close to a reading of one session.

## The three metrics that carry the claim

Rates, not totals. These are what a forward comparison is allowed to use.

| Metric | Baseline | Direction that means improvement |
|---|---|---|
| Re-billed share of total spend | **3.768%** | down |
| Cache-write share of cached spend | **16.083%** | down |
| Read-to-write ratio | **5.22x** | up |

They are chosen for one reason: **a stable prefix moves spend from write to
read.** That is the physical signature of the thing being optimized, and it does
not move merely because the next month contains more or less work.

Supporting figure, for the trimming lever specifically: at a 10,000-byte cap the
current corpus holds **2,343 blocks over cap, 17.54M bytes removable, worth
$93.48 at cache-read prices, against 92 proven cases where the agent later needed
the removed content.**

## What may be optimized, and what may not

**Compaction may not.** `replay trim` states it on screen: trimming does not delay
auto-compaction, because `/v1/messages/count_tokens` is untrimmed and the client's
accounting keeps using untrimmed sizes, so a trimmed session hits the threshold at
the same point. **No compaction-management benefit may be claimed**, forward or
backward, and this file does not create one by being forward-looking.

What may be optimized is prefix stability and context volume.

## The levers, ranked by what the corpus says they are worth

**1. Tool output volume. This is the big one and it is behavioural.**

`replay context` on the live corpus:

```text
eec05948   Bash                       53.4%   533k   x1278
facfd32e   javascript_tool            30.4%   304k    x634
581b6292   Bash                       45.7%   440k    x760
```

`581b6292` is the session that produced this file. **Nearly half of its context is
the output of its own shell commands.** The lever is not a setting, it is bounding
what a command prints: piping through `head`, selecting fields, and not dumping a
file to read three lines of it.

**2. Tool-set stability.** `replay prefix --before F --after F` exits 1 when a
tool-server document change invalidates the cached prefix. It is a CI gate and it
is free. It reports the set change honestly and does not claim the next request
will miss, because the mid-conversation-tool-changes beta may preserve the prefix
and whether it is on is not readable from a diff.

**3. `--context-edit-trigger`**, which the product recommends over trimming
because it is provider-sanctioned, is excluded from the history-binding check,
invalidates only from the earliest cleared block, and the provider reports what it
did.

## Protocol for the forward claim

1. This baseline is the anchor. It is not re-derived later from a moved corpus.
2. Interventions are recorded with the date they started, so a comparison window
   has a defined boundary.
3. Comparison uses `replay cost --compare`, on the three rate metrics above.
4. **A movement in a rate metric is reported with the session count and the
   workload mix behind it.** A rate can shift because the work changed character,
   and that possibility is stated every time rather than assumed away.
5. Totals may be reported for context. They may not carry the claim.
6. The dominant session, `eec05948` at over half of all spend, is reported
   separately as well as included, because an aggregate it dominates is really a
   reading of it.

## What a positive result would license

> Between the baseline and date D, the re-billed share of spend moved from 3.768%
> to X% across N sessions, while these interventions were in force.

It would not license a causal claim. **One operator, one machine, no control arm,
and the interventions are applied by the person who wants them to work.** The
honest form names the interventions and the movement, and leaves attribution open.

## Status

Baseline frozen. No intervention applied yet. No claim made.
