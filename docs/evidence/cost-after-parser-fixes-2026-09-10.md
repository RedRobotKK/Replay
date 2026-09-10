# Four parser fixes moved no cost figure, 2026-09-10

> **Correction, 2026-09-10 (later the same day).** Every dollar figure below was
> produced while `replay cost` priced one lane per transcript. That dropped
> 21,854 of 60,401 requests on this corpus — 36.2% — concentrated in the 8
> multi-lane files out of 1,812, which are the fan-out sessions. The corrected
> total is **$10,503**, not $3,411/$3,721. The figures here are NOT withdrawn:
> both arms of this comparison used the same pricing, so the comparison stands.
> What does not survive is any absolute total quoted from it.


**What this measures:** whether merging #133, #136, #137 and #138 changed any
number `replay cost` prints, measured by running the pre-merge and post-merge
binaries over the same corpus at the same moment.

## The result

Identical. Every figure.

| | pre-merge (`3c228ee`) | post-merge (`001c81c`) |
|---|---|---|
| tasks | 116 | 116 |
| lanes | 1,745 | 1,745 |
| total | $3,411.92 | $3,411.92 |
| avoidable | $161.93 | $161.93 |
| avoidable share | 4.746% | 4.746% |
| median task | $0.77 | $0.77 |
| p90 task | $3.41 | $3.41 |
| avoidable tokens | 34,337,507 | 34,337,507 |
| unpriced | 6 | 6 |
| duplicated requests | 430 of 36,524 | 430 of 36,524 |

## Why that is the expected answer, not a disappointing one

Both fixes were real, and neither one is *reachable from this command on this
corpus*. Stating why is the whole point of running it:

**#138 corrected image-block measurement, and cost does not read block bytes.**
An Anthropic image carries its payload under `source`, so it measured zero, and
now measures what crossed the wire. But `replay cost` prices the provider's own
`usage` object — `input_tokens`, `cache_creation_input_tokens`,
`cache_read_input_tokens`, `output_tokens`. Block bytes never enter that
arithmetic. The fix changes **attribution**, which is a different report.

**#137 corrected a 1.94× double-count on Codex, and this corpus has no Codex in
it.** `summary.route` reads `first-party API` across all 116 sessions: not one
Bedrock ARN, not one Vertex publisher path, and no Codex transcript. A fix to a
wire family that is absent cannot move a figure derived from the ones present.

## Where the image fix *is* visible

`replay context` over the same session, pre-merge and post-merge:

```text
  unaccounted:      13.3%   133k  x133      <- before
  unaccounted:      13.3%   132k  x132      <- after
```

One block left `unaccounted` and became attributable. That is the fix working,
in the report that reads block bytes, in the predicted direction, at the
predicted size. It is one block because this corpus is nearly all text.

## The delta against the earlier baseline is corpus growth, not code

An earlier run on this machine recorded $3,382.13 across 115 tasks. This one
records $3,411.92 across 116. The difference is **one more session** — work done
on this machine between the two measurements — and not a parser change. That is
the confound this file exists to remove: comparing a figure taken on Tuesday
against one taken on Wednesday measures the week, not the diff.

Running both binaries against the same corpus in the same minute is what
separates them, and it is cheap enough that there is no reason to publish the
other kind of comparison.

## Method

```sh
git worktree add /tmp/replay-old 3c228ee   # main before the four merges
go build -o /tmp/replay-old-bin ./cmd/replay
go build -o /tmp/replay ./cmd/replay       # main after
for b in /tmp/replay-old-bin /tmp/replay; do $b cost ~/.claude/projects --json; done
```

## Limits

**One machine, one corpus, all first-party.** 116 sessions across 1,745 priced
lanes on a single laptop, every model id first-party, six transcripts excluded
because their model is not in the price table.

This measurement says nothing about a corpus that *does* contain Codex
transcripts or image-heavy sessions. On those, both fixes are expected to move
figures, and nothing here bounds by how much — the honest statement is that it
was not measured, because the corpus available cannot measure it.

The 4.746% avoidable share carries the same standing caveat as every other
figure derived from this corpus: it is one operator's caching behaviour, not a
population rate, and pooling across contributors is the only thing that would
change that.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
