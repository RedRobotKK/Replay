# Capability map after v0.8.0: context, quota, compaction, state

**2026-10-05, read from the code at `5c87562` and from this machine's records,
not from memory of earlier discussion.** Four questions a user brings and what
the shipped binary can say to each. Every row names where the answer comes
from and how the figure is labelled.

## The real data this map is measured against

One operator's machine: 994 Claude Code transcript files (981 sessions with
usage), 464 Codex rollouts (46 with usage, 6,109 rate-limit events), five
proxy ledger sessions and none with provider rate-limit headers. One
operator, one machine: the evidence documents of 2026-09-06 called this "close
to a case study" and it still is.

## Matrix

| Capability | Exists | Production wired | Observable | Evidence quality | User value today | Missing |
|---|---|---|---|---|---|---|
| Context attribution (what entered, by tool) | yes, `replay context`, `blame`, `replay` | yes | measured from transcripts; byte-to-token fit marked `*` estimated | high; conservation laws in CI | high for Claude Code users | share is of what was attributed, never of the window, by design |
| Expensive and repeated context | yes, `advise` (hot files, large results, tool inputs), `replay` re-read rate, `trim` cap scoring, `diff` cache breaks | yes | measured and estimated, labelled | high | high | staleness in the content sense is not detected anywhere |
| Context approaching a limit | no | no | the window is not in a Claude Code transcript; Codex records it per turn and Replay parses and discards it | none shipped | none | a context-pressure figure; see the compaction rows |
| Prompt optimisation surface | partial: `advise` ranks targets with action text; `trim` names a cap; `prefix` names a prefix break | yes | labelled | high for cost; nothing on pressure or quota | medium | no single session view joining cost, pressure, quota |
| Quota, Anthropic | partial: `statusline` re-displays Claude Code's own rate_limits with reset; proxy ledger records rate-limit headers verbatim | statusline yes; ledger headers written (`record.Quota`) and read only by the unwired `internal/quota` | observed, verbatim | high for what is shown | medium | `internal/quota` (consumption per arm, forecast) unwired by decision; titration measured null |
| Quota, Codex | yes, `codex` and `burn` print used percent and window | yes | observed from rollouts | high; 5,700 events with `resets_at` | high | until today: the reset instant was parsed and never printed, and the credits-based limit of Codex 0.154 was printed as "0% used, window unknown"; both fixed in the commit after `5c87562` |
| Quota forecasting | code exists, `internal/quota.Forecast` | no, by decision | would be calculated | the one Anthropic titration moved the counter zero steps over 3.09M tokens | none | a second account and a reason to trust the slope; project rule forbids forecast wording |
| Quota exhaustion | spend cap and day cap in the proxy, local dollars and tokens | yes | calculated from the ledger | high, measured refusal | high for proxy users | provider-side exhaustion is never predicted |
| Compaction detection | yes: paired Claude Code boundary and summary with sizes; Codex `context_compacted` | yes, `replay context` | recorded; a shrinking prompt alone is labelled inferred since 0.7.0 | high | medium | nothing is said before it happens |
| Compaction prediction | no | no | measurable: on this machine 99 recorded compactions sit at a per-model ceiling (998k on the 1M tier, 194k on haiku); 0 of 967 non-compacting sessions reached 90% of it | measured here, one machine | none shipped | a panel decision on whether and how to expose it; the window tier is ambiguous per model id |
| Compaction management | partial: `context` reports what was dropped and that figures overstate | yes | recorded sizes | high | medium | what survived (post size p50 79k on opus-5, summary p50 17 KB) is not reported as such |
| State preservation, durable scratch | no, by evidence | no | three rooms found the repository's own commit prose and tree resume paused work; a hand-kept state file was net negative | high, negative | none | nothing missing that evidence supports building |
| Recovery after compaction | no | no | the summary that survives is recorded; whether the user repeated information afterwards is not measured anywhere | none | none | an outcome signal, which the 2026-09-29 closeout named as the restart condition for any behavioural claim |
| Model and provider differences | yes: Anthropic exclusive counting, OpenAI inclusive, Codex rollouts, Responses path | yes | measured | high | high | Codex default models unpriced |
| Economic optimisation | yes: `cost`, `ceiling`, `route`, `trim`, `simulate` (SIMULATED), `advise` dollars | yes | measured list price; simulated where stated | high | high | savings wording banned; no quota-aware routing |
| Evidence and provenance | yes: measured/estimated/inferred discipline, frozen mutants, wiring and production matrices, guard reachability | yes | | high | indirect | |

## What this machine's records say about compaction

Measured 2026-10-05 over the Claude Code transcripts above.

| Model | Compactions with a prior reading | Pre-compaction prompt total p10 / p50 / p90 | Post p50 | Summary bytes p50 | Turns from 90% to compaction, p50 (min) | Non-compacting sessions reaching 90% of the median pre size |
|---|---|---|---|---|---|---|
| claude-opus-5 | 86 | 963,705 / 966,870 / 998,021 | 78,834 | 16,987 | 112 (37) | 0 of 255 |
| claude-fable-5-1 | 8 | 165,514 / 961,009 / 965,989 | 96,483 | 21,447 | 61 (3) | 0 of 22 |
| claude-haiku-4-5 | 2 | 193,782 / 193,782 / 193,836 | 59,270 | 10,034 | 24 (24) | 0 of 260 |
| claude-opus-4-8 | 2 | 996,363 / 996,363 / 998,113 | 74,950 | 16,976 | 81 (81) | 0 of 2 |
| claude-sonnet-5 | 1 | 966,356 | 116,109 | 19,232 | 128 (128) | 0 of 430 |

Codex: 10 `context_compacted` events at 0.54 to 0.90 of the recorded 258,400
window; no ceiling, so no threshold prediction there.

The same model id appears with two ceilings (fable at 165k and 961k; sonnet
sessions reach 189k without compacting while one compacted at 966k), so the
tier is not derivable from the id. Claude Code's own statusline already shows
its user a used-percentage of the window. Whether Replay should expose a
compaction signal on top of that, and in what words, went to a twelve-person
panel the same day; its decision is recorded beside this file.

## What changed today, after v0.8.0

- `replay codex` prints each quota window's reset instant and its distance
  from the clock, and reads Codex 0.154's credits-based limit without
  printing a window that was not reported; `replay burn` likewise. Frozen as
  M137.
