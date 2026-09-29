# Anthropic efficiency: the surface map

Canonical inventory, opened 2026-09-29 at `2732809`. **Zero API spend. No
production behaviour changed.**

This page is the index and the synthesis. The detailed row-by-row inventories,
with evidence classes and source URLs, are four reconnaissance documents in
`../experiment/anthropic-recon/`. Findings and corrections live in
`ANTHROPIC-EFFICIENCY-FINDINGS.md`; the rules that bind any claim are in
`ANTHROPIC-EFFICIENCY-CONTRACT.md`; what would have to be true to test anything
is in `ANTHROPIC-EFFICIENCY-TEST-MATRIX.md`.

## Coverage, and an honest gap

| sweep | inventory | DOCUMENTED rows |
| --- | --- | --- |
| entitlement, quota, product, enterprise | `../experiment/anthropic-recon/entitlement-quota.md` | yes, 20 URLs fetched |
| API, cloud resellers, Claude Code controls | `../experiment/anthropic-recon/api-cloud-claudecode.md` | yes |
| context, compaction, prompt caching | `../experiment/anthropic-recon/context-compaction-cache.md` | **none** |
| model, reasoning, tools, agents | `../experiment/anthropic-recon/model-reasoning-tools-agents.md` | **none** |

Two of the four sweeps had **every** network fetch refused by a server-side
classifier that was unavailable, 28 attempts between them. Both recorded zero
DOCUMENTED rows rather than citing pages nobody opened, and both listed the URLs
a re-run must fetch. **Half this map's provider-documentation column is
legitimately empty and is marked so.** It is not a summary of what the docs say;
it is a record of what was and was not read.

## The eight surfaces, and where each stands

| # | surface | strongest evidence | control point |
| --- | --- | --- | --- |
| 1 | **entitlement regime** | DOCUMENTED: authentication, not plan, decides metering | REQUIRES USER POLICY |
| 2 | **quota dimensions** | DOCUMENTED for the API; the subscription unit is **UNKNOWN** | PROVIDER CONTROLLED |
| 3 | **context and compaction** | OBSERVED in full from transcripts; provider mechanics UNKNOWN | OBSERVABLE + MEASURABLE |
| 4 | **prompt caching** | OBSERVED per request; the cache key is UNKNOWN, provider-side only | OBSERVABLE + MEASURABLE |
| 5 | **model** | OBSERVED from the compiled price table; guidance is DOCUMENTED but unvalidated | MEASURABLE + CONTROLLABLE |
| 6 | **reasoning** | one control, effort, is real; the rest UNKNOWN | MEASURABLE + CONTROLLABLE |
| 7 | **tools and subagents** | parent/child attribution works; tool and synthesis tokens do not | partially OBSERVABLE |
| 8 | **inter-agent communication** | nothing measured; the open research question | UNKNOWN |

## What decides everything else: the entitlement regime

A single fact reorganises this map. **Authentication rather than subscription
plan decides how usage is metered**, and an API key takes precedence over a
subscription login. A stray environment variable silently moves a machine into a
per-token regime.

The consequences are not cosmetic:

- **Usage-based Enterprise has no subscription usage limit at all.** Everything
  bills at API rates and a spend limit is the only ceiling. An optimization
  aimed at a five-hour window is meaningless there.
- **Prompt-cache lifetime follows the credential**, one hour on a subscription
  and five minutes on an API key. Two sweeps reached this independently.
- **Microsoft Foundry returns none of Anthropic's rate-limit headers**, which
  removes header-based measurement on that platform entirely.
- **Bedrock splits quota ownership**: Anthropic gates tokens per minute, AWS
  enforces requests per minute.

This is why contract invariant A4 exists, and why **credential state is now a
required field on any benchmark taken in this repository**. A run that does not
record it cannot say which regime it measured.

## What Replay can observe today, verified in-repo

| observable | where | note |
| --- | --- | --- |
| compaction events and metadata | `internal/transcript/claudecode.go` | 67 events on this machine, all `auto` |
| cache creation split by TTL | `Usage.Create5m` / `Create1h` | 80% of writes are 1h here |
| cache read and creation per request | `Usage` | |
| thinking tokens | `Usage.ThinkingTokens` | per lane |
| effort | transcript request line | the one controllable reasoning parameter |
| parent versus child lane | `isSidechain`, parent chain | corroborated by the proxy agent id |
| serving identity | `internal/ledger` | added 2026-09-29 |
| cache state as three values | `Response.CacheState()` | absent is not zero |

## What Replay cannot observe

| not observable | why |
| --- | --- |
| subscription allowance consumed | no published unit, no readout |
| whether cached tokens reduce that allowance | provider-side |
| the prompt-cache key | provider-side, likely permanently |
| tool-definition tokens | ledger-only, not in a transcript |
| synthesis and verification tokens | intent labels, no wire marker |
| quality degradation across compactions | not observable at all |
| eight of ten cache-break causes | require a proxy, not a transcript |

## The two facts that bound any future claim

1. **The subscription allowance unit is unpublished.** Only relative multipliers
   exist. Whether cached tokens draw it down at a reduced rate is UNKNOWN, and
   if they do not, this repository's largest documented lever does not transfer
   to a subscription seat.
2. **Measurement instruments here are themselves unreliable until checked.** One
   sweep found 54.8% of usage records duplicated across files, with undeduplicated
   spend overstated roughly fivefold. Re-running this document's own compaction
   count keyed on record uuid moved it from 69 to 67. The check mattered more
   than the correction.

## Status

**Reconnaissance only.** Nothing here authorises an optimization, a runtime
change, or an experiment. The test matrix classifies **nothing** as SAFE TO
AUTOMATE, deliberately, so that a later change has to argue its way into that
class with evidence.
