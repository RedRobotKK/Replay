# Troubleshooting

Start with `replay doctor`. It reports what Replay can actually see on this machine and names the
next command worth running, which resolves most of what follows.

## Replay found no sessions

`replay <path>` needs a directory of transcripts, or a ledger directory. On Claude Code the
transcripts usually sit under `~/.claude/projects/`, one directory per project, so point it at the
project rather than at the parent.

If `doctor` reports no transcript directory at all, your agent may write them somewhere else, or may
not write them. Replay reads what already exists; it does not ask the agent for anything.

## `doctor` and `cost` report different transcript counts

They are counting different things, and both are right. Claude Code writes one transcript per session
at `<project>/<sessionId>.jsonl`, and one more per sub-agent lane under
`<project>/<sessionId>/subagents/`. `doctor` reports sessions; `cost` reads every file. On the machine
this was found on, 91 sessions and 1494 files. `doctor` now prints both with the reason, and prints
the second only when it differs from the first.

The same fan-out has a second consequence: a lane re-renders its parent's requests, so a few requests
appear in more than one file and are priced once per file. `cost` says how many — 430 of 30,716
requests, 1.4%, on the corpus this was measured over — and which way that pushes the total. It is
disclosed rather than silently deduplicated, because the overlap is a fact about how the client writes
transcripts and a reader who does not know it exists cannot judge any per-file figure.

## The report shows dollars and I am on a subscription

Then those dollars are not your money, and the report says so under the figures. A flat seat — Claude
Pro or Max, Copilot, Cursor — is not billed per token, so a broken cache costs the subscriber nothing
and the dollar column is list price for someone who is metered.

Read the token figure on the line beneath instead. Those tokens are yours: they are context the work
did not get, on a rate-limit window you are measured against either way. `replay advise` ranks what to
cut.

Whether a broken cache also burns a subscription's rate-limit budget the way it burns a bill is an
open question here, not a settled one. It was measured — matched cold-write and warm-read arms, 3.09M
tokens — and the utilisation counter moved zero steps, so the answer is published as null rather than
guessed in either direction.

## The first `replay cost` run is slow, or an old figure came back

The first run over a large corpus parses every transcript; over 1,483 files that was 6.3s wall and
19.6s CPU. Later runs read `~/.replay/cost-index.json` and reparse only what changed.

An entry is reused only when a file's size and modification time both still match, and the whole index
is discarded when the price table or rules version changes, so a figure computed under old prices is
never served. If you suspect the index anyway, delete it — a missing or corrupt index is a cache miss,
not an error, and the next run rebuilds it.

## The numbers say "estimated" and I want "measured"

That is working as intended. Transcripts do not contain the system prompt, the tool definitions or
the cache markers, so figures derived from them are partly inferred, and Replay says so rather than
quietly presenting them as fact.

To get measured numbers, run `replay serve`, set `ANTHROPIC_BASE_URL=http://127.0.0.1:4000` in the
shell that runs your agent, work normally for a while, then run `replay ~/.replay/ledger/`.

## My agent cannot reach the provider now that the proxy is running

Check the shell. `ANTHROPIC_BASE_URL` has to be set in the same shell that launches the agent, and
some editors and launchers do not inherit it.

If the provider itself is unreachable, Replay returns a `502` that tells you how to bypass it. To
bypass immediately, unset `ANTHROPIC_BASE_URL`. `REPLAY_DISABLED=1` stops the proxy starting at all.

Replay is built to fail open: if its own bookkeeping breaks, the bytes still flow. If you are seeing
traffic blocked, it is far more likely to be a guard you turned on than the proxy failing.

## A request was refused and I did not expect it

Every guard is off unless you enabled it, so check the flags you passed to `serve`. The likely ones:

- **`--error-budget`** refuses the next request once too much of the session's spend went to failed
  tools, failed edits or repeated identical calls. It deliberately trips before any spend cap.
- **`--loop-block`** refuses when the same tool call with the same input has repeated consecutively.
- **`--max-session-*` or `--max-day-*`** refuse once a cap is reached.
- **`--breaker-failures`** answers locally after consecutive provider failures, until the cooldown
  passes.

Refusals arrive as a provider-shaped error your agent will surface. To proceed once, send
`x-replay-override: <reason>`.

A **daily** spend-cap refusal also names the session that spent the budget and its share, attributed
on today's spend rather than the lifetime total. When the guard's session table has evicted enough
that the largest survivor cannot be the largest spender, it says the attribution is partial instead of
naming a session it is not sure about.

## `replay context` says the attribution is overstated

Because it is, and the alternative was to say nothing. Content leaves a context as well as entering
it: the provider clears old tool results under a context-edit policy, and Claude Code compacts the
history. A ranking of everything that ever entered describes a context that no longer exists, so the
closing note gives the number of compactions, the tokens the client recorded as dropped, and the share
by which the table above overstates.

If the dropped share is larger than what is attributed, that is not a bug either — it means most of
what passed through this session is gone, and the table describes what remains. Where a compaction
recorded no size, the note says the overstatement cannot be measured rather than treating it as zero.

## The browser cannot reach `/replay/status`

It cannot, by design. The proxy refuses browser-originated requests and binds loopback only. Use
`curl`, and include `x-replay-token` if you set a token.

## Replay reports cache breaks I do not understand

Run `replay diff`. It points at the exact turn where the cached prefix diverged and classifies the
cause. Common ones are a client re-rendering history, a timestamp inside a system prompt, and a
reordered tool list. Any of those changes the prefix, and a changed prefix cannot be read from cache.

Working from the ledger rather than transcripts names the cause with more certainty, because the
proxy hashes the tool definitions and system prompt of every request, where a transcript can only
infer them.

## Calibration is poor and `learn` will not recommend anything

That is the design. `replay learn` refuses to score alternatives for a model whose calibration looks
unreliable, because advice built on a session the engine does not understand is worse than no advice.

`replay corpus` judges calibration per model with the newest sessions separated, so a provider
changing its behaviour appears as a provider change rather than as drift. If a model reports that
way, the honest answer is to wait for the rules to catch up.

## I want to report a bug without handing over my work

Two commands exist for this.

`replay redact session.jsonl > redacted.jsonl` writes a redacted copy of a transcript.

`replay corpus <dir>` produces a calibration report containing no paths, no project names and no
message content. Open it and confirm that before sharing, whichever you use.

Security problems do not go in a public issue. Read [`SECURITY.md`](../../SECURITY.md).

## Something else

Open an issue with the output of `replay doctor` and `replay version`. Those two answer most of the
questions a maintainer would otherwise have to ask.

---

[Guide](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
