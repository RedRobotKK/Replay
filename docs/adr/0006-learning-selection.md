# 0006. Learning selects a policy by paired, held-out, relative saving

**Status:** Proposed
**Date:** 2026-09-03

## Context

A developer's corpus is tens of sessions. Scoring several candidate layouts over them and taking the best mean elects the lucky candidate as often as the good one (red/blue review, M1). The PRD asks for held-out sessions, a minimum sample, a margin above noise, a win that repeats, and ties to the simpler policy (LN-2), with confidence reported rather than a point (LN-4). It also names cached share per turn as the primary metric (LN-6).

Cached share is blind to a policy that shrinks the prompt: context editing can lower the cached share while lowering cost more, and a policy that grows the prompt can raise it while costing more. The learner needs a metric that a smaller, cheaper prompt improves.

## Decision

`replay learn` scores every catalog candidate over every calibrated session with the replay simulator and selects on the **relative effective-token saving**: the share of as-run effective tokens (input plus writes and reads at the provider multipliers) the candidate would have avoided. It is scale-free like cached share and sees prompt shrinking. Cached share is reported next to it, not used to select.

Selection rules, in order:

1. **Evidence.** A session counts for a candidate only when the candidate's score differs from as-run. A trigger the session never reached says nothing about that trigger. A candidate needs `--min-sessions` (default five) such sessions.
2. **Held-out split.** Each session is assigned to the held-out set by a stable hash of its id (about 30%), so the split does not move with file order or reruns. Selection uses training sessions only.
3. **Margin above noise.** The candidate's mean saving on training sessions must have a two-standard-error band that excludes zero.
4. **Repeat.** The candidate's mean saving on held-out sessions must be positive.
5. **Ties to the simpler.** Among qualifying candidates the largest mean wins, unless a simpler candidate (client setting before proxy parameter) is within noise of it. Noise is judged on the **paired** per-session difference between the two, because sessions vary far more than candidates do on one session; the unpaired bands of two candidates overlap or not for the wrong reasons.

Candidate parameters form a coarse, bounded, absolute grid (context-edit triggers of 50k, 100k, 200k, and 400k tokens with the default keep count) because the proxy must know the parameter at a session's first request and every extra candidate is another chance to elect noise.

The result is a versioned JSON policy file, `~/.replay/policy.json`, that lists every candidate's verdict with its numbers and the selected candidate or the reason none qualified. Learning reads files and writes one file; it never touches request bytes, and the proxy will read the file only at a session's first request (PX-8, not yet built).

## Consequences

- The learner says "none" on small corpora, which is the correct answer for them; the what-if rows on `/replay/status` remain the per-session view.
- Estimated candidates (context editing, which depends on the byte-to-token fit) carry the estimated mark through to the file. The proxy can apply only the context-edit family; TTL candidates are advice for a client setting.
- Session types (LN-3) are not part of this decision; one policy is selected for all sessions until the corpus is large enough to split.

## Alternatives considered

**Cached share as the selection metric.** Rejected for the reason above; it is reported.

**Random or chronological held-out split.** Rejected: a random split changes the answer between runs, and a chronological one confounds the split with drift in the developer's work.

**Unpaired comparison for ties.** Rejected: with session-level variance an order of magnitude above candidate differences, unpaired bands either always overlap or never do, depending on corpus size, not on the candidates.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
