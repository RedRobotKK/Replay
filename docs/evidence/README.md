# Evidence

Measurements behind the claims in the README. Each file records the method as well as the result, so
a reader can decide whether the number means what it says. Files are dated and never edited after the
fact; a new measurement gets a new file.

| Document | What it measures | Headline |
|---|---|---|
| [Calibration corpus, 2026-09-03](calibration-corpus-2026-09-03.md) | How well the replay engine reproduces the provider's own cache reads | 398 of 402 turns across 11 sessions, all from this repository's own development on one machine |
| [Proxy added latency, 2026-09-03](proxy-latency-2026-09-03.md) | What `replay serve` adds to a request, against a local fake provider | p50 48µs, p99 98µs |
| [Adversarial security review, 2026-09-04](security-review-2026-09-04.md) | An external reviewer reading the code and running the proxy end to end | Five findings open, one fixed, and what was verified to hold |

> [!IMPORTANT]
> The calibration corpus is 11 sessions from one machine, not the twenty independent sessions the
> [roadmap](../ROADMAP.md) gate asks for. It is published because an under-powered measurement stated
> plainly is worth more than a claim with nothing behind it.

Contributing your own corpus takes one command and shares no paths, project names or content. See
[Contributing a calibration corpus](../../README.md#contributing-a-calibration-corpus).

---

[Documentation index](../README.md) · [Repository README](../../README.md)
