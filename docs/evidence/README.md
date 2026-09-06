# Evidence

Measurements behind the claims in the README. Each file records the method as well as the result, so
a reader can decide whether the number means what it says. Files are dated and never edited after the
fact; a new measurement gets a new file.

| Document | What it measures | Headline |
|---|---|---|
| [Calibration corpus, 2026-09-03](calibration-corpus-2026-09-03.md) | How well the replay engine reproduces the provider's own cache reads | 398 of 402 turns across 11 sessions, all from this repository's own development on one machine |
| [Calibration corpus, 2026-09-05](calibration-corpus-2026-09-05.md) | How well the engine reproduces the provider's own numbers, across 1363 transcripts from many unrelated projects rather than this repository's own development work. **It called those transcripts "sessions"; see the 2026-09-06 correction** | Supersedes the 2026-09-03 corpus, which covered 11 self-referential sessions |
| [Calibration corpus, 2026-09-06](calibration-corpus-2026-09-06.md) | The same engine, re-read, and a correction: the previous file counted transcript files and called them sessions | **1450 transcripts from 78 sessions**, 97.46%. The 1363 published as "sessions" was a file count; one session supplied 1020 of them |
| [The fan-out premium, 2026-09-06](fan-out-premium-2026-09-06.md) | What parallel subagent lanes cost, against a baseline where siblings share the cache write. **Corrected the same day: most of the number is arithmetic** | 1.68x / 2.56x / 3.34x, sitting at 88-99% of a ceiling fixed by the group size and the provider's own multipliers. No corpus can produce a premium below 1, so the rise with width is the shape of the estimator. Only the dispersion ratio (~0.9, flat) is empirical |
| [Compaction and the index, 2026-09-06](compaction-and-index-2026-09-06.md) | What context compaction discards, and what indexing transcripts saves | **39 compactions keep a median 2.55%**, discarding 30.5M tokens over 73.5 minutes of wall clock; the index takes `replay cost` from 6.474s to 0.046s |
| [Wire families, 2026-09-06](wire-families-2026-09-06.md) | Which request shapes a terminal GenAI client actually sends, captured off the wire from a live session | **Three families, not two.** The Grok CLI speaks OpenAI *Responses* (`/responses`), which Replay does not parse, and returns the four `x-ratelimit-*` headers this project had never captured. `quota.go` already allowlists them and has never fired |
| [Lane isolation, 2026-09-06](lane-isolation-2026-09-06.md) | How much of a fan-out session's re-billing comes from a changed prefix, and what the answer was while the instrument compared each sub-agent against a different one | **4.2%**, published the same morning as **98.8%**. Both the 98.8% and the 11.0% intermediate are retracted in the file rather than removed; 31 of the 34 events had never happened |
| [Break causes, 2026-09-06](break-causes-2026-09-06.md) | Which causes actually re-bill tokens, across the whole corpus rather than a sample | **735 breaks, 31.26M tokens.** Client re-render 50.8%, TTL expiry 33.9%. The same measurement on the 40 largest sessions said TTL 75.2% - sorting by size selected for the cause |
| [Quota titration, 2026-09-06](quota-titration-2026-09-06.md) | Whether a cache break burns a flat-seat subscription quota the way it burns a metered bill | **Null, and stated as null.** 3.09M tokens across matched cold-write and warm-read arms moved the 5h counter by zero steps; the instrument is too coarse to answer at this budget |
| [Proxy added latency, 2026-09-03](proxy-latency-2026-09-03.md) | What `replay serve` adds to a request, and why the first attempt could not measure it | **~1.7ms p50** (corrected 2026-09-05; the 48µs originally published was noise) |
| [Rehydration boundary under attack, 2026-09-05](rehydration-boundary-2026-09-05.md) | Whether a poisoned agent can write a real credential outside the project | Eight vectors refused; the harness was checked by weakening the boundary, which showed `filepath.Clean` alone defeats dot-dot and only `EvalSymlinks` stops the symlink escape |
| [Spike: OpenAI-compatible path, 2026-09-05](spike-openai-compatible-2026-09-05.md) | What the proxy does with a request it was not built for | It forwarded it correctly and applied nothing: a one-token spend cap refused none of three requests. Led to the warnings, then to the path being built |
| [Spike: Cursor, 2026-09-05](spike-cursor-2026-09-05.md) | Whether Cursor is a transcript problem or a provider problem | A provider problem. Cursor stores 29,665 message rows and **zero** cache fields, so the transcript path could never produce cache forensics |
| [Spike 4: the real provider, 2026-09-05](spike-4-real-provider-2026-09-05.md) | Whether the proxy works against the real provider at all | Ten turns, all 200, 1,816,417 prompt tokens measured, zero credentials and zero message content in the ledger |
| [Adversarial security review, 2026-09-04](security-review-2026-09-04.md) | An external reviewer reading the code and running the proxy end to end | Five findings open, one fixed, and what was verified to hold |

> [!IMPORTANT]
> The calibration corpus is 1450 transcripts from **78 sessions** on one machine, not the independent sessions the
> [roadmap](../ROADMAP.md) gate asks for. It is published because an under-powered measurement stated
> plainly is worth more than a claim with nothing behind it.

Contributing your own corpus takes one command and shares no paths, project names or content. See
[Contributing a calibration corpus](../../README.md#contributing-a-calibration-corpus).

---

[Documentation index](../README.md) · [Repository README](../../README.md)
