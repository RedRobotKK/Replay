# Rewriting what the client sends

**2026-09-04.** "Act as a proxy for my prompts and form them in a Claude-efficient manner." There is
a strong version of this and a version that dismantles the tool. They are separated by one question:
**can the change be proven not to alter what the model sees?**

## The line

**Semantics-preserving.** The model receives the same content, arranged so the provider's caching
works. Provably equivalent, and the benefit is measurable in cache reads.

**Semantics-altering.** The model receives different words. Might be better. **Replay cannot see
outcomes, so it can never know.**

Replay already does one semantics-preserving rewrite: `--context-edit-trigger` adds the provider's
own `context_management` parameter, off by default, pinned per session, logged with body hashes
before and after. **That is the precedent and it is the right shape.**

## The strong version, and it is already named in your own roadmap

**`breakpoint-on-stable-block`. Listed in `docs/requirements.md:235`, and `docs/ROADMAP.md` ends with
"`breakpoint-on-stable-block` is not built."**

The provider allows a small number of `cache_control` breakpoints. Most clients place them naively,
or place them once and never reconsider. **Replay is the only thing that knows where the stable and
unstable boundary actually falls in your sessions**, because it has replayed them turn by turn and
knows exactly which block changed on the turn the cache broke.

Placing a breakpoint is the ideal intervention:

- **The model sees byte-identical content.** A cache marker is metadata; it changes nothing about
  what is read.
- **The benefit is directly measurable** in the next response's `cache_read_input_tokens`. Not
  inferred, not modelled: the provider tells you.
- **It fails safe.** A badly placed breakpoint costs a cache write. It cannot produce a wrong answer.
- **The counterfactual is computable offline first**, which is what `replay` already does for TTL and
  context-edit candidates.

**This is the feature. It is the one place where Replay can act rather than advise, keep every
guarantee it makes, and be checked by the provider's own numbers on the very next turn.**

Two things worth ordering the same way as the existing rewrite. Score it offline against real
sessions before it ever runs live. Then ship it exactly like `context-edit`: off by default, pinned
per session, logged with hashes, and reverted automatically by a guardrail.

## The version that dismantles the tool

Rewording, restructuring or compressing the prompt itself. Four objections, and the fourth is fatal.

**It breaks the sentence the whole product rests on.** "It forwards every request and response byte
for byte" is on the README, the landing page and in the security review a stranger can read. **The
opt-in features that modify bodies are already the hardest thing to explain**, and each is
semantics-preserving. A reworder is not.

**Something has to do the rewriting, and that something is a model.** An LLM call inside the request
path, on a proxy whose measured overhead is **p50 48 microseconds**. That number is a headline claim
and it would become meaningless.

**There is no verification path.** `replay learn` refuses to recommend a policy whose calibration is
weak. **That refusal is the credibility of this project.** A prompt rewriter cannot be calibrated,
because the thing it would need to measure is answer quality, and Replay cannot see outcomes. It
would be the one feature in the tool with no evidence behind it.

**And it cannot fail open.** Every other feature degrades to passthrough. A rewriter that fails has
two choices: send the original, which helped nothing, or send the rewrite, which may be wrong.
**"Fails open" and "rewrites your prompt" cannot both be true**, and fail-open is the property blue
team called the strongest thing in the repository.

## The middle, which is probably what you actually want

**Advise on prompt structure without touching it.** Replay can already see which shapes cache and
which do not, so it can say:

> Your instructions block changes every session because it contains a date. That breaks the cache on
> turn one, every time, and re-bills the whole prefix. Move the date into the first user message and
> your system prefix becomes cacheable.

**That is prompt optimisation delivered as insight rather than intervention.** It needs no model in
the request path, keeps byte-for-byte passthrough intact, and the user makes the change once in their
own configuration where they can see it. It also happens to be the same shape as the tool-server
advice: **name the cause, price it, and leave the decision alone.**

## What to do

1. **Build `breakpoint-on-stable-block`**, offline scoring first. It is on the roadmap, it is not
   built, and it is the highest-value thing in this document.
2. **Extend the advisor to prompt structure**, using cache-break causes already classified.
3. **Do not put a model in the request path.** If prompt rewriting is genuinely wanted later, it
   belongs in a separate tool that is not also making a byte-for-byte promise.
