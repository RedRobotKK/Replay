# Preregistration: DeepSeek prefix cache block model

Written 2026-09-29T03:3x UTC, **before** the falsification round was run.
Registered so the model is tested rather than fitted twice.

## The model

Fitted to five rungs in the P2 probe:

    cached_tokens = 128 * max(0, floor(shared_prefix_tokens / 128) - 1)

Two claims:

1. The cache is quantised to **128-token blocks**.
2. The **final block is never served from cache**, so a prefix must span at
   least two blocks before any hit occurs.

## Predictions for rungs not yet measured

The boundary between 0 and 128 cached tokens is the sharpest test, so three of
the five rungs sit on it.

| rung (words) | predicted prompt tokens | predicted cached | what a miss would mean |
| ---: | ---: | ---: | --- |
| 160 | ~215 | **0** | threshold is lower than two blocks |
| 200 | ~265 | **128** | threshold is higher, or not block-shaped |
| 240 | ~315 | **128** | |
| 2048 | ~2375 | **2176** | quantisation breaks down at scale |
| 4096 | ~4750 | **4608** | |

## Falsifiers, declared in advance

- Any rung whose observed cached count is **not a multiple of 128** kills claim 1.
- Rung 160 caching anything, or rung 200 caching nothing, kills claim 2.
- The two large rungs caching the full floor (2304 / 4736) rather than one block
  less kills claim 2 at scale, and would mean the minus-one term is an artifact
  of short prefixes.

A model fitted to five points and confirmed on five more is worth acting on. A
model fitted to five points and never tested is a description of those points.
