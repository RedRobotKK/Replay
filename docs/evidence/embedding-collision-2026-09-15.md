# Embeddings cannot separate line 42 from line 43

**Measured 2026-09-15 on one machine: 4,027 unique coding-agent prompts, 3,187
constructed distinct pairs, one embedder. The median cosine similarity between
two prompts differing by a single integer is 0.9999. Method below; read the
limits before quoting the curve.**

**What this measures:** whether coding-agent prompts that provably ask
different things fall inside the cosine-similarity thresholds semantic caches
ship with, for one named embedder, on one operator's corpus.

**Named precisely:** this is EMBEDDING collision, not semantic collision. The
ground truth is that two inputs were deliberately made different, not that a
model judged them to differ in meaning. Calling it semantic would claim a kind
of ground truth this measurement does not have.

## The curve

A **collision** is a provably-distinct pair whose cosine similarity is at or
above the threshold: a cache would treat two different questions as one.

| threshold | negation | number | path | aggregate |
|---:|---:|---:|---:|---:|
| 0.85 | 99.3% | 99.9% | 95.3% | **98.6%** |
| 0.90 | 96.5% | 99.1% | 93.1% | **96.6%** |
| 0.95 | 74.0% | 96.6% | 91.6% | **86.9%** |
| 0.97 | 56.5% | 94.7% | 82.3% | **77.3%** |
| 0.99 | 25.0% | 89.8% | 63.4% | **58.9%** |

At 0.95, inside the range implementations ship with, 86.9% of
provably-distinct pairs collide.

| family | n | min | median | p95 | max |
|---|---:|---:|---:|---:|---:|
| negation | 1,200 | 0.7649 | 0.9764 | 0.9969 | 1.0000 |
| number | 1,200 | 0.8348 | **0.9999** | 1.0000 | 1.0000 |
| path | 787 | 0.4662 | 0.9953 | 0.9996 | 0.9999 |
| all | 3,187 | 0.4662 | 0.9947 | 1.0000 | 1.0000 |

## Why this corpus rather than a benchmark

Published semantic-cache guidance benchmarks thresholds on FAQ and question
answering datasets, which are short and well separated.

Coding-agent prompts are neither. They are long, highly repetitive, and they
differ in one token that decides the answer: a file path, a line number, a
flag, a negation. That is the worst case for a similarity metric and it is the
workload now being placed behind caches.

## Method

4,027 unique user prompts, read from 1,991 Claude Code transcripts and 158
Codex rollouts already on disk.

Ground truth is structural rather than judged. A prompt naming a different
file provably wants a different answer. No human labelling and no
LLM-as-judge: using a model to decide whether a model-similarity metric is
safe is circular.

Three families, 3,187 pairs, exactly one meaning-bearing token changed in each:

| family | mutation | n |
|---|---|---:|
| `negation` | polarity inverted by inserting "not" | 1,200 |
| `number` | one integer literal incremented | 1,200 |
| `path` | one real file path swapped for another real path from the same corpus | 787 |

Embedder: `nomic-embed-text`, nomic-bert, 137M parameters, 768 dimensions,
F16, 2,048 context, run locally through Ollama. No network request and no API
spend. Zero embedding failures across 6,374 calls.

## The analysis plan was written before the data was read

The threshold grid, the definition of a collision, the per-family reporting
and both stopping rules were fixed in advance, including the rule that a
near-zero result meant stopping rather than writing a harder pair generator
until a graph appeared.

The other stopping rule was that a high result also meant stopping: reporting
representation collision and concluding nothing about production caches. This
file is that rule being kept.

## What this does and does not license

It licenses one sentence:

> Real coding-agent prompts contain mechanically distinguishable pairs that
> fall within the similarity thresholds semantic caches ship with, for this
> embedder.

No cache made a decision here, no answer was generated, and nothing was
checked for correctness.

## Limits

**The pairs are constructed, not observed.** Mining naturally occurring
distinct pairs from the corpus yielded 84, too few for a curve. Each pair is
therefore a real prompt plus the same prompt with one token changed, and the
second member never occurred in this operator's history. That is weaker than
mined pairs. It is stronger in one respect: the pair differs in one token and
nothing else, so the similarity cannot be explained by unrelated drift.

**One embedder, and a small one.** A larger model may separate numerals
better. No evidence either way was gathered, and this file does not imply any.

**One operator's corpus.** n=1. One person's prompts, projects and habits, the
same limitation that applies to every other figure in this directory.

**A collision is only harm if a cache serves it.** Nothing here serves one.

## The rungs this is the first of

1. **embedding collision**: could these be confused. This file.
2. **actual cache hit**: does a cache confuse them. Not run.
3. **re-execution**: was the served answer wrong. Not run.

Each rung removes an assumption. Nothing above the first is claimed.

## Reproducing

The pre-registration, the four scripts and the full twenty-row curve are kept
with the measurement. The scripts are deliberately not part of the `replay`
binary: they need an embedder, which the zero-third-party-dependency rule does
not permit.
