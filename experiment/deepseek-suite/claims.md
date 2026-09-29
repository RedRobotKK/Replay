# Claims matrix

Every claim is bounded to the condition it was tested under. None is stated as a
universal property of the provider. `deepseek-flash` on `/v1/chat/completions`
unless the claim names otherwise.

| # | Claim | Direct evidence | Derived | Replication | Confounds | Falsifier | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| C1 | Cached tokens follow `128 * max(0, floor(n/128) - 1)` under the tested condition | 35 rungs, provider-reported `prompt_cache_hit_tokens` | formula fit | 17 retrospective + 18 prospective, discriminating rungs only | prefix content held constant; token counts provider-reported, never estimated | any cached count not a multiple of 128, or a discriminating rung matching a rival | **SUPPORTED** |
| C2 | The published 64-token storage unit does not describe this model | same 35 rungs | H-D1 0/18, H-D2 4/22 prospective | two independent rung sets | vendor page is dated 2024-08-02, describes V2/MLA, quotes $0.014/M against today's $0.006 | a rung matching a 64-token formula | **SUPPORTED for flash in 2026-09.** Says nothing about V2 |
| C3 | One leading space destroyed cache reuse | A1, hit 0 against a control of 2048 | none | prior observation plus A1 | none material | a leading space that still serves cache | **SUPPORTED** |
| C4 | Matching is prefix-wise, not whole-string | A1: trailing space added still hit 2048; a mid-prefix change hit 896 = 7x128 | block arithmetic | single run | change position approximate | a trailing change that zeroes the hit | **SUPPORTED** |
| C5 | Parallel requests issued before population completed did not reuse cache | A4: sequential B hit 3200, concurrent B hit 0, with `B dispatched before A returned` from wall-clock | none | A4 plus the A2-arm of the earlier arms run | dispatch order is not arrival order; controlled by timeline, not by code order | a concurrent B that hits | **SUPPORTED** |
| C6 | Sampling parameters are not part of cache identity | A6: temperature, max_tokens, top_p, frequency_penalty, presence_penalty, stop all hit 3200 against a 3200 control | none | single run, 6 arms | `reasoning_effort` was held constant across all A6 arms, so A6 does NOT cover it | any listed parameter zeroing the hit | **SUPPORTED, scope limited** |
| C7 | An unknown key the endpoint ignores is not part of cache identity | A10 arm C: junk key hit 3200 | none | single arm | none material | a junk key that breaks reuse | **SUPPORTED** |
| C8 | `reasoning_effort` IS part of cache identity, in both directions | A10 arms A and B: adding it hit 0, removing it hit 0, against 3200 controls | none | two directions, one run | whether it changes the cache key or the serialised prompt is NOT distinguished | toggling it while reuse survives | **SUPPORTED**; mechanism undetermined |
| C9 | The cache is model-scoped, not account-global | A8: flash to v4-pro 0, v4-pro to flash 0, against working controls on both | none | both directions | same key, same endpoint, same prefix | cross-model reuse | **SUPPORTED** |
| C10 | Cross-dialect reuse occurs only when `reasoning_effort` is absent from both bodies | A9: absent 3200, present 0, same prefix and order | none | A9 two arms, plus A7 refuting and the original observation confirming | byte-identity of the serialised prompt across dialects is ASSUMED | reuse with the parameter present, or no reuse without it | **SUPPORTED**; supersedes both earlier readings |
| C11 | 484 saved responses reconstruct to $0.24876 DERIVED | provider `usage` blocks | published rates at each call's own timestamp | two independent paths agreed to 9 decimals on the 479-response subset | 5 calls NOT_OBSERVED, so this is a LOWER bound | a conservation violation, or a path disagreement | **SUPPORTED as a lower bound** |
| C12 | The reconstruction lies inside what the balance can distinguish | balance $46.01 and $45.77, raw bodies persisted for the later reads | interval $(0.230, 0.250)$, open both ends | one full-session test; an earlier single-batch test is weakened by lag | settlement lag; the $46.01 anchor has no raw body | derived outside the band after settlement converges | **SUPPORTED, provisional until settlement converges** |
| C13 | The account balance moved during a period containing no inference | $45.83 to $45.78 with no billable call in the window | none | one window | corroborated by zero response files written in the window | a balance that never moves without inference | **SUPPORTED** |
| C14 | Settlement converges and then stops | 13 polls over 24.1 minutes, all $45.78, zero changes | none | one window | none material | a change during a quiet window after convergence | **SUPPORTED** |
| C15 | Per-request cent truncation does not explain the reconciliation | max per-request cost $0.0056997, 0 of 479 reach $0.01 | truncation predicts a $0.00 session total | arithmetic over the whole corpus | none | any request reaching a cent | **REFUTED (the truncation hypothesis)** |
| C16 | Reasoning configuration changes correctness and cost differently by task class | lookup 18/18 vs 17/18; aggregation 24/24 vs 7/24 | none | one run per cell, n=18 and n=24 | the first attempt was INVALID (a line-number task mislabelled as lookup) | a class where the setting does not move accuracy | **SUPPORTED for the two tested classes** |
| C17 | Once the prefix is cached, disabling reasoning is worth about 1.5x, not 20x | arms A3 $0.00313 vs A4 $0.00211 | 39x output tokens, 1.48x total | one run | prefix size and output length drive the ratio; not general | a cached workload where it approaches the output ratio | **SUPPORTED for the tested shape** |

## Explicitly NOT claimed

- Nothing here establishes behaviour for `deepseek-v4-pro` beyond C9. All cache
  geometry is flash.
- Cache TTL is **NOT_OBSERVED**. The vendor says hours to days; nothing here ran
  long enough.
- Whether C8 works by changing the cache key or the serialised prompt is
  **NOT_OBSERVED**. A10 rules out "the key is the raw body" and no further.
- The 2024 vendor figure may be accurate for the model it describes. C2 says
  only that it does not predict flash.
