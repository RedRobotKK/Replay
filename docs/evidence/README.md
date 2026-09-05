# Evidence

Measurements behind the claims in the README. Each file records the method as well as the result, so
a reader can decide whether the number means what it says. Files are dated and never edited after the
fact; a new measurement gets a new file.

| Document | What it measures | Headline |
|---|---|---|
| [Calibration corpus, 2026-09-03](calibration-corpus-2026-09-03.md) | How well the replay engine reproduces the provider's own cache reads | 398 of 402 turns across 11 sessions, all from this repository's own development on one machine |
| [Calibration corpus, 2026-09-05](calibration-corpus-2026-09-05.md) | How well the engine reproduces the provider's own numbers, across 1363 sessions from many unrelated projects rather than this repository's own development work | Supersedes the 2026-09-03 corpus, which covered 11 self-referential sessions |
| [Proxy added latency, 2026-09-03](proxy-latency-2026-09-03.md) | What `replay serve` adds to a request, and why the first attempt could not measure it | **~1.7ms p50** (corrected 2026-09-05; the 48µs originally published was noise) |
| [Rehydration boundary under attack, 2026-09-05](rehydration-boundary-2026-09-05.md) | Whether a poisoned agent can write a real credential outside the project | Eight vectors refused; the harness was checked by weakening the boundary, which showed `filepath.Clean` alone defeats dot-dot and only `EvalSymlinks` stops the symlink escape |
| [Spike: OpenAI-compatible path, 2026-09-05](spike-openai-compatible-2026-09-05.md) | What the proxy does with a request it was not built for | It forwarded it correctly and applied nothing: a one-token spend cap refused none of three requests. Led to the warnings, then to the path being built |
| [Spike: Cursor, 2026-09-05](spike-cursor-2026-09-05.md) | Whether Cursor is a transcript problem or a provider problem | A provider problem. Cursor stores 29,665 message rows and **zero** cache fields, so the transcript path could never produce cache forensics |
| [Spike 4: the real provider, 2026-09-05](spike-4-real-provider-2026-09-05.md) | Whether the proxy works against the real provider at all | Ten turns, all 200, 1,816,417 prompt tokens measured, zero credentials and zero message content in the ledger |
| [Adversarial security review, 2026-09-04](security-review-2026-09-04.md) | An external reviewer reading the code and running the proxy end to end | Five findings open, one fixed, and what was verified to hold |

> [!IMPORTANT]
> The calibration corpus is 1363 sessions from one machine, not the independent sessions the
> [roadmap](../ROADMAP.md) gate asks for. It is published because an under-powered measurement stated
> plainly is worth more than a claim with nothing behind it.

Contributing your own corpus takes one command and shares no paths, project names or content. See
[Contributing a calibration corpus](../../README.md#contributing-a-calibration-corpus).

---

[Documentation index](../README.md) · [Repository README](../../README.md)
