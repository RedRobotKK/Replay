---
name: replay-doctor
description: Find the turn a prompt cache broke on, its cause, and the tokens re-billed, from the transcripts already on this machine. Use when the user asks why agent spend went up, what a session cost, whether a cache broke, what filled the context, or whether switching models would be cheaper.
---

# Replay Doctor

Replay reads the transcripts Claude Code and Codex already keep on disk and replays each turn against the provider's caching rules. Everything runs locally. The binary makes no network request except four commands the user types (`rules --check-prices`, `probe --execute`, `upgrade`, `rules --update`).

## First, check what is here

```sh
replay doctor
```

If `replay` is not installed, install it and read the script before running it:

```sh
curl -fsSL https://replay.doctor/replay.sh | less
curl -fsSL https://replay.doctor/replay.sh | sh
```

Or `go install github.com/RedRobotKK/Replay/cmd/replay@latest`. macOS and Linux only; say so if the user is on Windows.

## The four commands that answer the usual questions

| The user asks | Run | What to quote |
|---|---|---|
| "What did my sessions cost?" | `replay cost ~/.claude/projects/` | total, median task, p90, and the avoidable line with its date |
| "Why did it get expensive?" / "Did the cache break?" | `replay diff <session.jsonl>` | one line per break: the turn, the cause, the tokens |
| "What is filling my context?" | `replay context <session.jsonl>` | the ranked list, and whether the session was compacted |
| "Would a cheaper model be cheaper?" | `replay route --to <model> <session.jsonl>` | the crossover turn, or the refusal if the pair was never measured |

## Rules for quoting figures

- Quote the **Calibration** line before any dollar figure; it says how many turns Replay reproduced against what the provider charged. If the match rate is low, say so and stop.
- Every figure carries a tier (measured / estimated / structural), a population and a date. Keep them when you quote it.
- On a subscription seat the dollars are list price for someone else; the tokens are still the user's context. Say which applies.
- Never say "save" or forecast a monthly figure. Replay reports what was already spent twice.
- If the tool refuses (a model pair never measured, a compacted session), quote the refusal. It is the answer.

## Contributing and watching, only if asked

`replay cost --contribute <campaign>` writes a file of about 19 counts and no text; the user reads it and posts it themselves with the printed curl line. `replay watch emit` does the same per session for a repository the user named. The binary never sends either; explain that if the user asks what leaves the machine.
