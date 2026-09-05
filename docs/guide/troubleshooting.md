# Troubleshooting

Start with `replay doctor`. It reports what Replay can actually see on this machine and names the
next command worth running, which resolves most of what follows.

## Replay found no sessions

`replay <path>` needs a directory of transcripts, or a ledger directory. On Claude Code the
transcripts usually sit under `~/.claude/projects/`, one directory per project, so point it at the
project rather than at the parent.

If `doctor` reports no transcript directory at all, your agent may write them somewhere else, or may
not write them. Replay reads what already exists; it does not ask the agent for anything.

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
