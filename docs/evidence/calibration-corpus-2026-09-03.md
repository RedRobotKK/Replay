# Calibration Corpus

How well the replay engine reproduces the provider's cache reads across 11 sessions found on one machine on 2026-09-03. **Provenance: these are the development sessions for this repository itself** — one main session and ten sub-agent sessions from a single project on a single machine, not a diverse sample of real-world usage. They are real provider traffic with real usage numbers, so calibration means something; they are not the twenty independent sessions the roadmap gate asks for, and the totals below say so. Rows carry a session id prefix, never a path, project name, or content.

| Session | Client | Tier | Requests | Compared | Matched | Breaks | Match rate | Fit tokens/byte | Fit ±% |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|
| 83303506 | 2.1.258 | estimated | 320 | 319 | 315 | 4 | 98.7% | 0.445 | 16 |
| 83303506 | 2.1.259 | estimated | 20 | 19 | 19 | 0 | 100.0% | 0.561 | 47 |
| 83303506 | 2.1.259 | estimated | 13 | 12 | 12 | 0 | 100.0% | 0.629 | 68 |
| 83303506 | 2.1.259 | estimated | 13 | 12 | 12 | 0 | 100.0% | 0.719 | 58 |
| 83303506 | 2.1.259 | estimated | 10 | 9 | 9 | 0 | 100.0% | 0.795 | 71 |
| 83303506 | 2.1.259 | estimated | 8 | 7 | 7 | 0 | 100.0% | 0.552 | 18 |
| 83303506 | 2.1.259 | estimated | 7 | 6 | 6 | 0 | 100.0% | 0.753 | 85 |
| 83303506 | 2.1.259 | estimated | 6 | 5 | 5 | 0 | 100.0% | 0.573 | 48 |
| 83303506 | 2.1.259 | estimated | 6 | 5 | 5 | 0 | 100.0% | 0.691 | 53 |
| 83303506 | 2.1.259 | estimated | 5 | 4 | 4 | 0 | 100.0% | 0.528 | 33 |
| 83303506 | 2.1.259 | estimated | 5 | 4 | 4 | 0 | 100.0% | 0.527 | 159 |

## Totals

- Sessions: 11 (0 below the 95% threshold)
- Compared turns: 402, matched: 398, breaks: 4
- Overall match rate: 99.00%
- Fewer than 20 sessions: the roadmap gate for spikes 1 and 2 is not met by this corpus alone

## Per model

Calibration by the model of each session's first request, with the newest 5 sessions judged on their own so a provider rule change shows as a drop (ST-1). The minimum cacheable prefix is bounded from usage: the largest uncached prompt lies below it, the smallest cached prefix at or above it.

| Model | Sessions | Match rate | Recent sessions | Recent match rate | Verdict |
|---|---:|---:|---:|---:|---|
| claude-fable-5-1 | 10 | 99.0% | 5 | 98.9% | calibrated |
| claude-haiku-4-5-20251001 | 1 | 100.0% | 1 | 100.0% | calibrated |

- claude-fable-5-1: minimum cacheable prefix: at most 40563 tokens, no uncached prompt seen (rules say 512)
- claude-haiku-4-5-20251001: minimum cacheable prefix: at most 14873 tokens, no uncached prompt seen (rules say 4096)

## Break causes

| Cause | Count |
|---|---:|
| prefix diverged inside the message history at an unknown block | 4 |

## Sessions not analyzed

None.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
