# Structural divergence and economic disruption are distinct observables

**E005, frozen 2026-09-26.** A blinded retrospective comparison of Replay's
cache-break detector against Anthropic's `message.diagnostics.cache_miss_reason`
on 709 Claude Code transcripts. **No new experiment follows this file.**

## Method

The provider diagnostic was removed from a working copy by a recursive strip of
136,037 fields across 464,901 lines, verified to leave **zero structured
diagnostic fields**. 938 residual string matches are prose inside message
content (an Azure `livediagnostics` URL, and this project's own notes) and are
not machine-readable as an oracle. Replay's detector was run unmodified.

## Corpus, frozen

| class | count |
|---|---:|
| tools_changed | 1,006 |
| messages_changed | 591 |
| previous_message_not_found | 255 |
| system_changed | 119 |
| unavailable | 48 |
| model_changed | 45 |
| **total** | **2,064** |

Structural (`*_changed`) 1,761, indeterminate 303, and 1,761 + 303 = 2,064
exactly. Every event has a prior request. **1 unreadable file and 380 malformed
lines are reported rather than silently dropped.**

An earlier pass keyed events by `(file, second)`, collapsing 2,064 events into
1,580 keys and losing 484. **That was a bookkeeping error, it is closed, and
event identity is now per-event.**

## The result

| class | n | economic | median missed | **median Δwrite** |
|---|---:|---:|---:|---:|
| tools_changed | 1,006 | 932 | 13,482 | **+5,005** |
| messages_changed | 591 | 163 | 54,125 | **0** |
| system_changed | 119 | 35 | 82,484 | **0** |
| model_changed | 45 | 13 | 243,095 | **0** |

### Erratum, 2026-09-26: what the "economic" column counts

**No value in this file changed. The label was misleading and is corrected here
rather than in the table, which stays frozen.**

The column headed `economic` was computed as:

```python
hit = bool(rb.get((file, second)))   # rb = Replay break boundaries
```

That is: **the oracle event fell in the same second as a break reported by
Replay's own detector.** It is **agreement between two instruments, one of which
is the subject of this study.** It is NOT a measurement of whether provider
usage moved, and it must not be read as one.

The provider-usage observable in this table is **`median Δwrite`**, the change in
provider-reported `cache_creation_input_tokens` across the boundary. That column
is independent of Replay's detector and is unaffected by this erratum, so **the
result stands unchanged**: `tools_changed` moves +5,005 and the other three
classes move 0.

The distinction matters for anyone building on this file. Recomputing the column
as "provider usage shows a write" (`cur.write > 0`) gives 1006 / 590 / 119 / 45,
not the 932 / 163 / 35 / 13 below. Both are meaningful; only the second is what
this table reports.

The definition was recovered verbatim from the executed procedure, which was
never persisted at the time and was reconstructed on 2026-09-26. See
`internal/e005/` for the durable scorer, whose tests pin every number below.

The load-bearing column is the last one. **`tools_changed` is the only class
where provider usage independently shows a rebuild.** Three classes show zero
median change in cache-creation tokens despite the provider reporting
divergence, and `model_changed` does so while carrying the largest median
`cache_missed_input_tokens` in the corpus.

### Ambiguity bounds

375 boundaries carry more than one oracle event in the same second as a Replay
break, so per-event attribution there is not possible. Conservative bounds:

| class | lower | upper |
|---|---:|---:|
| tools_changed | **58%** | 93% |
| messages_changed | 24% | 28% |
| system_changed | 24% | 29% |
| model_changed | 27% | 29% |

**`tools_changed` at its lower bound exceeds every other class at its upper
bound.** The ordering is robust to the ambiguity; the exact rate is not.

## Claim taxonomy

### ESTABLISHED

- The corpus accounting above, to the stated totals.
- Median Δwrite is +5,005 for `tools_changed` and 0 for the other three classes.
- Replay detects `tools_changed` between 58% and 93% of the time **without
  access to the diagnostic**.
- **`cache_missed_input_tokens` is not interchangeable with economic
  disruption**: the class with the largest median missed tokens has the smallest
  observed cache-write movement.

### SUPPORTED, NOT ESTABLISHED

- That Replay measures an economic observable distinct from provider structural
  divergence. Consistent with every measurement here; not proven by them.
- That TTL-class Replay events are a provider-side condition the diagnostic does
  not report. 51 of 102 have no provider diagnostic at all.

### NOT ESTABLISHED

- Per-event causal attribution at the 375 ambiguous boundaries.
- Why `messages_changed` is usually non-material.
- That every structural-only event occurred after a reusable breakpoint, was
  economically harmless, or left provider cache internals unchanged. **These
  require cache internals not observable from durable artifacts.**
- That Replay deliberately ignores structural divergence. It has no such rule.
- That the 68-73% output-discipline result decomposes into these categories.
  Different experiment, unresolved.

## On the word "miss"

**A Replay miss requires a protocol expectation that the event should have been
detected.** Four categories are kept separate and are not collapsed:

1. established Replay detection,
2. economic event without Replay detection,
3. structural divergence without demonstrated economic disruption,
4. indeterminate alignment.

74 of 1,006 `tools_changed` fall in (2). The 618 structural-only events fall in
(3), and calling them misses would assume the conclusion.

## The 62 Replay-only events

51 with no provider diagnostic, 35 `previous_message_not_found`, 16 near a
`*_changed`. **Partially characterised; not causally explained.**

## Falsifiers, sought rather than avoided

What in these artifacts would falsify the distinct-observable theory:

1. `tools_changed` showing no cache-write movement. **Not present**: +5,005.
2. All four classes showing similar Δwrite. **Not present**: one is +5,005,
   three are 0.
3. Replay detection tracking `cache_missed_input_tokens`. **Not present**:
   `model_changed` has 243,095 missed and 29% detection.
4. The class ordering collapsing under the ambiguity bounds. **Not present**:
   58% lower bound beats 29% upper bound.
5. Replay-only events all having a `*_changed` counterpart. **Not present**:
   51 of 102 have no diagnostic.

**No falsifier is present in the corpus.**

## Frozen statement

> E005 supports treating provider-reported structural cache divergence and
> Replay-observed economic disruption as distinct observables. In the observed
> corpus, `tools_changed` events align strongly with independent cache-write
> movement, while several other provider-reported divergence classes commonly
> show none. The evidence supports an economic interpretation of Replay that is
> not reducible to the provider's structural diagnostic. Same-second collisions
> prevent complete per-event causal attribution, so **E005 does not establish a
> universal one-to-one mapping** between provider diagnostics and Replay events.

**E005 does not prove the theory. It is consistent with it and materially
strengthens it.** E005 is closed.

## Durability, added 2026-09-26

E005's blinder and scorer were run as inline heredocs and were never written to
disk, and its corpus sat in a directory designed to be deleted. Both are now
preserved, and the numbers above are reproducible rather than merely recorded.

- **Corpus**: `~/Development/replay-e005-corpus-2026-09-26/`, 722 files,
  fingerprint `1a5ce6f15612e03bf55f28b9e6bfaca3695c929615266bb5f7f44b721637e22b`.
  It is **not** in this repository: `blind/` is 1.8GB of real session
  transcripts, blinded of the provider diagnostic but not of message content,
  and this repository is public.
- **Procedure**: nine steps recovered verbatim from the session transcript and
  archived beside the corpus, unmodified.
- **Scorer**: `internal/e005/`, which re-derives every figure in this file from
  two sanitized artifacts committed as test data.

### Reproducing this file

From a clean checkout, with no setup and no local artifacts:

```sh
go test ./internal/e005/
```

`TestEndToEndReproducesFrozenE005` runs the whole chain, committed fixtures
through the real parser and the real scorer, and asserts every number in this
document. It is part of `make ci`, so the frozen result is checked on every
build rather than on request.

| committed input | sha256 | what it is |
|---|---|---|
| `internal/e005/testdata/events.json` | `2de9f6206ea004a3c86350db786a8cb885a98dc8ad9c3ace16b514a7164b81f9` | 2,064 oracle events, eight fields each |
| `internal/e005/testdata/breaks.json` | `a713793541bc489a5b720d52c45aed0e6b142a7643253612b4eb2445694e0f76` | 866 boundaries, break counts only |

Both hashes are asserted by `TestFixtureProvenanceIsPinned`, so an input cannot
be edited to move a published number without the suite saying so.
`testdata/README.md` records what was stripped from the originals and why.

**Deliberately NOT committed**, and where they live instead
(`~/Development/replay-e005-corpus-2026-09-26/`, fingerprint `1a5ce6f1`):

| artifact | why it is out |
|---|---|
| `blind/`, 1.8GB | real session transcripts; blinding removed the provider diagnostic, not message content, and this repository is public |
| `replay_out.txt` | its `where:` lines quote message and tool-result content |
| `oracle2.json` | no message text, but ten session-derived identifiers the scorer never reads |

`TestFrozenNumbersDependOnTheFixture` is the negative control: it perturbs the
parsed input and requires the result to move, so the assertions cannot pass on
constants while the pipeline is dead.

Two things this file's numbers depend on are **not** recoverable and are
recorded as gaps rather than closed: the `replay` build that produced the
detector arm, and the live transcript tree the oracle arm read, which has since
changed. The oracle arm's output survives; its input does not.
