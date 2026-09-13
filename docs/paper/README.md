# Paper and launch drafts

Outward-facing writing about the measurements. Both documents draw on the dated
files under [evidence](../evidence/README.md), and neither is published.

| Document | What is in it |
|---|---|
- [**The launch announcement, 2026-09-14**](announcement-2026-09-14.md) - the Show HN and Product Hunt copy actually being posted, built on the 2026-09-13 re-read. Supersedes the Show HN section of launch-draft.md, which leads with a retracted cause table.
| [Launch drafts](launch-draft.md) | Show HN and LinkedIn copy, with a table mapping every figure to the corpus and provenance tier it came from, and a record of the numbers that were struck |
| [Preprint](replay-preprint.tex) | An arXiv-style write-up of the cache-invalidation attribution study. **Not compiled** — no LaTeX toolchain was available where it was written, so it is checked structurally only |

## Why the figures are scoped per corpus

Four corpora appear across these documents and they are not interchangeable.
The cause taxonomy, the keepalive result and the lane-isolation share come from
three different sets of transcripts at two different provenance tiers, so a
sentence of the form "we analysed N sessions and found all of this" is false
for every N.

The launch draft records two numbers that were struck before publication and
where each came from. Neither was invented. Both arrived without a population
attached, at which point the nearest available population was assumed to be
ours — which is the failure mode
[reference-distribution.md](../design/reference-distribution.md) makes
`Population` a mandatory field to prevent.

---

[Documentation index](../README.md) · [Evidence](../evidence/README.md) · [Repository README](../../README.md)
