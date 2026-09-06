# Audit outreach

What to say to someone who has not asked for anything yet, and what not to.

The offer is a measurement, not a claim about their systems. We have not seen
their traffic. Anything that sounds like we have is both untrue and easy for a
technical buyer to catch, and getting caught once costs more than the meeting
was worth.

## Rules

- Do not tell a prospect they have the defect. Offer to measure whether they do.
- Every number carries where it came from and how big the sample was.
- If we measured one account, say one account.
- A finding that closes a line of work is worth as much as one that opens it.
  "There is nothing to poll here" saves a quarter.
- Nothing sends without Daniel naming it in that turn.

## Vendor case study: what a wire capture found on Grok

Useful because it is checkable. A prospect can reproduce it in twenty minutes
and see whether their account behaves the same way. That is the point: the
first thing we say should be something they can verify before deciding whether
to believe the second thing.

### The rate-limit headers did not move

Grok's responses carry four headers that look like live quota tracking:

```text
x-ratelimit-limit-requests       x-ratelimit-remaining-requests
x-ratelimit-limit-tokens         x-ratelimit-remaining-tokens
```

On the one account we measured, across eight model calls and about 940KB of
responses, `remaining` never moved. It sat at the plan ceiling every time:

```text
53000000 / 53000000 tokens        8300 / 8300 requests
```

Eight samples on one account cannot prove the counter never moves. They are
enough to say it did not move under ordinary work, which is the case a budget
guard would be built for.

We also checked every other endpoint the client talks to. `/settings` is polled
repeatedly and was the obvious candidate for a quota heartbeat. It holds 40
keys and no quota state at all. Across `/responses`, `/settings`, `/models`,
`/feedback/config`, `/traces` and `/sessions/*`, those four headers are the only
quota-shaped thing on the wire.

**What that means for a guard.** If you were planning to poll the vendor for
remaining budget, on this surface there is nothing live to poll. The counting
has to happen locally. That is a design conclusion, not a product pitch, and it
holds whether or not you ever talk to us again.

**What we do not claim.** We have not looked at your account. Your tier may
behave differently, and if your headers do decrement that is worth knowing too.
Checking takes about twenty minutes of capture against your own traffic, and
you can do it without us.

### The context strategy comes from the server

Most prompt-cost advice assumes your client decides what sits in the context
window: where the cache breakpoints go, how much tool output to keep, what
order the prefix is in.

Grok's client polls a `/settings` block, and 18 of its 40 keys are context
policy pushed down from the vendor:

```text
pruning_enabled          pruning_keep_last_n_turns    pruning_soft_trim_threshold
flush_enabled            flush_soft_threshold_tokens  flush_idle_timeout_secs
memory_enabled           memory_embedding_model       memory_search_min_score
memory_mmr_enabled       memory_mmr_lambda            memory_temporal_decay_enabled
```

Pruning rules, flush thresholds, and a retrieval stack with an embedding model,
MMR reranking and temporal decay. The vendor can change any of it without
shipping a new client.

**What that means.** Advice like "reorder your prefix" or "cap your tool
output" is aimed at a decision your client does not own on this surface. The
work that still pays is measuring what each turn actually cost you and where
the cache missed, because the vendor does not show you either.

### Where this came from

`docs/evidence/wire-families-2026-09-06.md` has the method, the full scope, and
the parts we got wrong on the way. Worth reading before quoting any of it: an
earlier version of that file described these headers as a high-fidelity live
counter, which was written from the header names before anyone had read a
value. It is retracted in place rather than deleted, because a prospect who
finds the retraction learns more about how we work than the finding does.

---

[Documentation index](README.md) · [Repository README](../README.md)
