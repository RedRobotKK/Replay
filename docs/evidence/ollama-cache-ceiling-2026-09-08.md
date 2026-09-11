# A local model cannot report a 100% cache hit, 2026-09-08

**What this measures:** whether "100% of the prompt served from cache" is a
claim any Ollama request can make, read off 40 MB of one machine's own server
logs rather than reasoned about.

## Summary

**It cannot, and the refusal is deliberate.** When llama.cpp finds the entire
prompt already resident it does not serve the whole thing from cache. It backs
off by exactly one token so it has something to evaluate, and says so on the
line above:

```text
slot   operator(): id  0 | task 128 | new prompt, n_ctx_slot = 4096, n_keep = 4, task.n_tokens = 125
slot   operator(): id  0 | task 128 | need to evaluate at least 1 token for each active slot (n_past = 125, task.n_tokens() = 125)
slot   operator(): id  0 | task 128 | n_past was set to 124
slot   operator(): id  0 | task 128 | cached n_tokens = 124, memory_seq_rm [124, end)
slot print_timing: id  0 | task 128 | prompt eval time =      39.10 ms /     1 tokens
```

`n_past = 125` equals `task.n_tokens = 125`: every token of the prompt was
already there. The server then sets `n_past` to 124 and evaluates one token.

So the ceiling on a per-request cache hit rate is `(n-1)/n`, and it is a ceiling
the server enforces rather than an asymptote the workload approaches.

| | |
|---|---|
| Back-off lines in the corpus | **586** |
| Of those, `n_past == task.n_tokens` | **586 (all of them)** |
| Requests with a `prompt eval` of exactly 1 token | **586 (all of the measured set)** |
| Highest hit rate observed | **0.998516** |
| `(maxContext-1)/maxContext`, maxContext 674 | **0.998516** |

The observed maximum equals the arithmetic ceiling to six decimal places,
because the server always gives back exactly one token.

## Finding 1: the measured population is not a sample

This is the part that changes what may be reported.

`n_past` appears in the log **only** when the whole prompt was already cached.
586 requests carry it; all 586 are back-off cases. The other 2,575 requests in
the corpus log no reuse figure at all.

So a cached share computed over the requests that carry `n_past` is not an
estimate of cache performance across the workload. It is a measurement of a
population defined by having been fully cached, and its value is pinned near
`(n-1)/n` by construction. Averaging it produced **99.7318%**, which is not
wrong so much as it is a restatement of the selection rule.

The honest count is two numbers and no average: **586 of 3,161 requests were
served entirely from cache apart from the one token the server re-evaluates by
rule. For the remaining 2,575 the log does not say.**

## Finding 2: what "100%" was, before this

`replay burn` printed **"100% of the prompt served from cache"** for this
corpus. The share was 99.7318% and `%.0f` rounded it.

Those are different statements. 100% says nothing was ever recomputed; 586
tokens were. Shares now only print as 100% when they are 100%, and only as 0%
when they are 0%; anything that merely rounds to an extreme gains a decimal
place. Fixed the same day this file was written, by the measurement in it.

## Finding 3: the unmeasured majority is where the work is

The corpus holds prompt evals of 270, 283 and 2,046 tokens. None of them is in
the measured set, because a request that actually processes a prompt does not
produce a back-off line. Whatever this machine's real cache behaviour is, it
is in the 2,575 requests the log declines to describe.

That bounds what any advice built on this surface can say, and it is worth
saying before building one: on these logs, Ollama reports reuse only for the
requests where reuse was total.

## Method

`replay burn` over `~/.ollama/logs/*.log`, 40 MB, 3,161 parsed requests, one
machine, ollama 0.33.2. Back-off lines counted directly out of the raw logs
with a regex over `need to evaluate at least 1 token for each active slot
(n_past = N, task.n_tokens() = M)`, comparing N and M per line.

**Scope.** One machine, one ollama version, one workload. It establishes that
the back-off rule exists and fires, and that on this corpus it accounts for
every request carrying a reuse figure. It does not establish the ratio of
back-off requests to prompt-processing requests on anybody else's machine, and
the 18.5% here is a fact about this log, not about the tool.

An earlier count using `awk -F'[=,)]'` reported 0 of 561 lines with
`n_past == task.n_tokens`, which is the opposite of the truth. `task.n_tokens()`
contains a closing parenthesis, so the field split put the second number two
fields further along than the script read. Recounted with an explicit regex:
586 of 586.

## The suppression this file justified did not hold (2026-09-11)

`burnOllama` withheld the cached share, citing this document. It withheld it on
the wrong condition: `unmeasured > 0`, meaning some *other* request in the same
log lacked an `n_past` line. The reason written directly above that check is a
property of the field itself — `n_past` appears only when the prompt was already
resident, so any share over the requests carrying one measures a population
selected for having been cached.

The two came apart on a log where every block carries `n_past`. That is not an
exotic input; it is what a handful of turns on one slot produces. On such a log
`burn` reported a cached share of **99.98%** — the disqualified quantity, at its
most flattering, presented as a measurement.

On this machine's corpus the condition happened to hold (2,682 of 3,294 requests
lack the field), which is why the log this file was written from never showed
it. A guard that passes because of the corpus in front of it is not a guard.

Fixed by never computing the share on this surface. `TestBO1` builds the
fully-labelled log and fails if a share comes back.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
