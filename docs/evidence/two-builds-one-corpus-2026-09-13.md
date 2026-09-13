# Two builds, one corpus, 2026-09-13

**Measured directly, minutes apart, on the identical transcript root.** Not
inferred from two readings taken on different days against a growing corpus,
which is what every previous statement of this problem rested on.

`v0.5.4` was built from the tag into a detached worktree. The current build is
`corpus-build-identity` at `a8e1c99`. Both were pointed at
`~/.claude/projects` within the same minute.

| | v0.5.4 | current build | ratio |
|---|---:|---:|---:|
| sessions | 119 | 121 | 1.02 |
| lanes | 1,949 | 1,951 | 1.00 |
| **requests read** | **45,791** | **72,858** | **1.59** |
| total at list | $4,236.17 | $12,630.61 | **2.98** |
| re-billed | $211.42 | $347.53 | 1.64 |
| **re-billed share** | **4.99%** | **2.75%** | **0.55** |
| re-billed tokens | 44,214,854 | 71,369,167 | 1.61 |
| models it declined to price | 6 | 0 | |

The corpus grew by two sessions between the readings that produced the older
published figures and today. **Two sessions cannot move a total by 2.98x.** The
binary did.

## What changed, and it is not a defect

The current build **reads 27,067 more requests out of the same files** and
prices six model pairs the older build declined to price. Both are
improvements. A larger denominator that is more complete is the correct
direction for this instrument to move.

That is exactly why this file exists. **The improvement moved the headline in
the direction that makes the product look worse**, and an improvement nobody
recorded is indistinguishable from a regression nobody noticed.

## The number that must not be published without this file beside it

The figure in circulation is **"about 5% of my own bill was re-billed"**. It is
in `docs/MONEY-PATH.md`, in the PRD, and in a measurement post drafted for
launch. On the build that is about to ship, **the same corpus reads 2.75%**.

Both are honest readings. Neither is wrong. **A launch post that prints 5% next
to a binary that prints 2.75% is handing a reader an arithmetic discrepancy in
the one project whose claim is that its arithmetic is checkable.**

## The coincidence that is the real trap

| Source | Figure |
|---|---|
| v0.5.4 `cost`, re-billed tokens | 44,214,854 |
| current build `diff`, re-billed sum | about 44,202,000 |
| current build `cost`, re-billed tokens | 71,369,167 |

**The first two agree to within 0.03% and are not the same quantity.** One is an
older build's `cost` total; the other is the current build's `diff` summed over
break events. `cost` and `diff` count different things, and on the current build
they differ by 1.6x.

[Break causes, third reading](break-causes-2026-09-13.md) cites "about 44.2M
re-billed tokens" from `diff`, and so does the launch announcement. A reader who
cross-references that against the older published 44.2M would conclude the
figure is stable across builds. **It is not. It is a collision.** Both documents
now say which command and which build produced it.

## What this is the third instance of

Issue #284 was filed because two builds priced one corpus at $4,088.49 and
$11,969.37 under one rules label. `PricingDigest` was added to the corpus and
watch payloads so a pooled figure could tell two arithmetics apart.

This is the same phenomenon, measured deliberately rather than discovered, and
it is the argument for that field stated as a measurement rather than as a
worry. **A pool that sums a v0.5.4 submission and a v0.6.0 submission is summing
two instruments.**

## What to do before Monday

1. **Every dollar or share figure on a public surface is re-read on the shipping
   build, or it carries the build that produced it.** There is no third option.
2. The 5% claim is retired in favour of **2.75%, on 121 sessions, read
   2026-09-13 on the v0.6.0 build**, with 4.99% kept visible beside it and this
   file cited.
3. `cost` and `diff` figures are never printed in the same sentence without
   naming which produced which.

## What this does not establish

One machine, one operator. The ratios are facts about this corpus under these
two builds and are not a general statement about how much any build differs
from any other. The 2.98x is not a correction factor and must never be used as
one.

---

[Evidence index](README.md) ·
[Break causes, third reading](break-causes-2026-09-13.md) ·
[The launch announcement](../paper/announcement-2026-09-14.md)
