# Every AI surface on one machine, opened, 2026-09-08

**What this measures:** what each agent actually writes to this disk, checked by
reading the directories rather than by inferring from the wire or from a
vendor's documentation.

## Why it was done

`wire-families-2026-09-06.md` stated "There is no local transcript" for Grok.
`~/.grok/sessions` holds 3.8 GB. The claim was inferred from a capture proxy,
which is an instrument for the network and says nothing about a disk, and
nobody opened the directory.

One wrong surface claim is a mistake. The question this file answers is whether
it was the only one.

## The census

| Surface | On disk | Read by Replay | Carries usage | Prior claim |
|---|---|---|---|---|
| Claude Code `~/.claude/projects` | 1.5 GB, **1,681** `.jsonl` | **yes, all 1,681** | yes | correct |
| Codex `~/.codex/{sessions,archived_sessions}` | 42 MB, **150** rollouts | **yes, both roots, 150** | yes | correct |
| Ollama `~/.ollama/logs/server*.log` | 42 MB, **3,161** requests | **yes** | 586 of 3,161 | corrected 2026-09-08 |
| Grok `~/.grok/sessions` | **3.8 GB, 6,787 files** | no | **no** | "no local transcript" — **wrong** |
| Cursor `~/.cursor` | 11 MB, **118** agent transcripts | no | **no** | "unreadable by design" — **wrong** |

Everything the tool claims to read, it reads completely. The two surfaces it
does not read were both documented with a false reason and a correct
conclusion.

## Finding 1: the three read surfaces are exact

`~/.claude/projects` holds 3,471 files, of which **1,681 are `.jsonl`**, and
`replay doctor` reports 1,681. The other 1,790 are `.json` sidecars, `.txt`,
`.jpg`, `.pdf` and `.md`. No transcript is missed and nothing else is counted.

Codex reads two roots. `sessions` holds 29 rollouts and `archived_sessions`
holds 121; `replay burn` reports **150**, which is both. Archiving a Codex
session does not hide it.

Ollama's glob is `server*.log`, and the `app*.log` files beside them contain
**zero** `prompt eval time` or `n_past` lines: they are application lifecycle
logs, "starting Ollama" and "signaling Ollama app process". The narrower glob
is right.

## Finding 2: two surfaces were called absent and are present

Both were documented as unavailable for reasons that turn out to be false.

**Grok.** `~/.grok/sessions`, 3.8 GB across 6,787 files, organised by
URL-encoded working directory, one directory per project, each session carrying
`chat_history.jsonl`, `events.jsonl`, `prompt_history.jsonl`,
`hunk_records.jsonl`, `rewind_points.jsonl` and `prompt_context.json`, beside a
`session_search.sqlite` full-text index.

**Cursor.** `~/.cursor/projects/*/agent-transcripts/*.jsonl`, **118 files**,
plain JSONL carrying `{message, role}`, plus a 6.7 MB
`ai-tracking/ai-code-tracking.db` with tables `conversation_summaries`,
`scored_commits`, `ai_code_hashes` and `tracked_file_content`. Nothing about it
is unreadable.

## Finding 3: neither carries usage, so the conclusion survived the reasoning

This is the part that matters for what can be built.

Searched across all 4.2 GB of `~/.grok`: `inputTokens`, `outputTokens`,
`cachedReadTokens`, `cacheCreationTokens` and `costUsdTicks` appear in **zero
files**. One session's `events.jsonl` was walked field by field over **13,444
events** with no token, cost, cached or usd key at any depth; the events are
lifecycle, `phase_changed`, `tool_started`, `turn_started`, `first_token`.

Searched across every `.jsonl` under `~/.cursor`: `usage`, `tokens`,
`inputTokens`, `outputTokens`, `promptTokens`, `cost`, `cache_read`,
`cachedTokens` and even `model` appear in **zero files**.

Neither database carries a usage column either. `~/.codex/logs_1.sqlite` is
166,237 rows of diagnostic logging with columns `level`, `target`,
`module_path`, `file`, `line`; Cursor's tracking database has no token, cost,
usd, cache or usage column in any table.

So both are conversation surfaces and neither is a spend surface. A Grok or
Cursor integration would read prompts, tool calls and edits. It would not read
cost, and anything claiming otherwise is claiming something these files do not
contain.

## Finding 4: a figure withdrawn

An analysis on 2026-09-07 reported that Grok's records carry per-turn token
counts and `costUsdTicks` over 1,411 turns, giving **$406.07** as "the only
measured dollar figure on the machine". It was relayed into a business memo and
sent off this machine before anyone opened a file.

The fields are absent and the string `406.07` appears nowhere under `~/.grok`.
Withdrawn, and corrected to the recipient.

The general form is worth keeping, because it is the same error as the Grok
claim one level up: **a figure from a summary is not a figure from a
measurement.** The first mistake was trusting a capture proxy about a disk. The
second was trusting a summary about a file. Both were fixed by opening the
thing.

## Finding 5: 20 GB that is not a surface at all

`~/.codex/worktrees` is **20 GB**, which is 95% of everything under `~/.codex`
and larger than every agent transcript store on this machine combined. It is
checked-out source, not agent data, and Replay correctly ignores it.

Recorded because a reader comparing `du -sh ~/.codex` against what `replay
burn` reports would otherwise conclude the tool was missing almost everything.
It is missing nothing; the directory is mostly not the thing.

## Method

`du`, `find` and `grep` over each store, plus `sqlite3` schema reads on the two
databases. No file content is reproduced here and none was needed: the
questions were how much is there, what shape is it, and does it carry usage.

**Scope.** One machine, one operator, one point in time, and the versions of
each client installed on it. It establishes what these clients write here. It
does not establish what they write on anyone else's machine, and a client that
adds usage to its local records tomorrow would make Finding 3 stale without
making it wrong today.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
