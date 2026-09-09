# Quota data census, 2026-09-09

**What this is:** a count of the quota data that actually exists on this
machine, run so that a `replay quota` design and its critique argue about the
same facts. Every number below is the output of a command that is reproduced
with it. No interpretation of product implications — that is another agent's
job.

**Headline:** the Anthropic quota capture path has produced **zero**
observations. Not few. Zero.

---

## 1. How many ledger records exist

```text
$ cd ~/.replay/ledger && for f in *.jsonl; do echo "$f: $(jq -c . "$f" | wc -l)"; done
351d4e3d-9908-4577-aa11-fdd093c0b21f.jsonl:        2
7022b9f2-59f7-4356-9a44-f13b211512b7.jsonl:       10
f11756ad-8ad7-43f2-b88a-ce8076218921.jsonl:        2
f79cae6d-0d11-4033-ac6f-15f25a5503d5.jsonl:        2
session-91d4ea1705704971.jsonl:        1

$ jq -c . *.jsonl | wc -l
      17
```

**17 records across 5 session files.**

Date range:

```text
$ jq -r '.ts' *.jsonl | sort | sed -n '1p;$p'
2026-09-04T22:45:08.671689-07:00
2026-09-06T13:42:27.566512-07:00
```

Two bursts, not a range: 16 records inside a 100-second window on 2026-09-04
(22:45:08 to 22:46:54), and **one** record on 2026-09-06 at 13:42:27.

Every record is `schema: 2`, every one is `path: /v1/messages`:

```text
$ jq -r '.schema' *.jsonl | sort | uniq -c
  17 2
$ jq -r '.path' *.jsonl | sort | uniq -c
  17 /v1/messages
$ jq -r '.status' *.jsonl | sort | uniq -c
  16 200
   1 401
```

Other ledger locations checked and empty of records:

```text
$ find ~ -maxdepth 4 -type d -name "ledger*"
/Users/daniel/.replay/ledger-grok
/Users/daniel/.replay/ledger
...
$ find ~/.replay/ledger-grok -type f
/Users/daniel/.replay/ledger-grok/spend-day.json      # {"day":"2026-09-06","tokens":0,"usd":0}
/Users/daniel/.replay/ledger-grok/.label-key
```

`~/.replay/archive` holds one git bundle, no records.

## 2. How many carry a non-empty `quota` object

**0 records. 0.00% of 17.**

```text
$ jq -s '[.[] | select((.quota // {}) | length > 0)] | length' *.jsonl
0
$ jq -s '[.[] | select(has("quota"))] | length' *.jsonl
0
```

The key is not merely empty, it is absent. `Quota` is
`json:"quota,omitempty"` (`internal/ledger/record.go:95`), so an absent key
means `quotaFrom` returned nil.

Confirmed independently of `jq`, by raw byte search:

```text
$ grep -ric -e 'ratelimit' -e 'retry-after' -e 'quota' *.jsonl
351d4e3d-9908-4577-aa11-fdd093c0b21f.jsonl:0
f79cae6d-0d11-4033-ac6f-15f25a5503d5.jsonl:0
session-91d4ea1705704971.jsonl:0
7022b9f2-59f7-4356-9a44-f13b211512b7.jsonl:0
f11756ad-8ad7-43f2-b88a-ce8076218921.jsonl:0
```

And at any nesting depth, not just the top level:

```text
$ jq -c '[paths | select(.[-1]|type=="string")
         | select(.[-1]|test("quota";"i")) | join(".")]
         | select(length>0)' *.jsonl
(no output)
```

The union of top-level keys across all 17 records contains no `quota`:

```text
$ jq -s 'map(keys) | add | unique' *.jsonl
["cache","effort","latency_ms","model","path","policy","prefix_hash",
 "prompt","request_id","response","schema","session_id","status","stream","ts"]
```

### Why zero — the timing

The capture code is younger than almost all of the data:

```text
$ git log --diff-filter=A --format='%H %ad %s' --date=iso -- internal/proxy/quota.go
b76390e5cecaeaf9de394142fac7a823b76ec3ff 2026-09-05 23:18:33 -0700 proxy: capture what a flat-seat user actually spends
```

`quotaFrom` was wired in at `internal/proxy/server.go:595` on **2026-09-05
23:18**. Sixteen of the seventeen records were written on **2026-09-04**, a day
and a half earlier. They *could not* have carried quota headers.

The single record written after the capture path existed is the 2026-09-06 one,
and it is the 401:

```text
$ jq . ~/.replay/ledger/session-91d4ea1705704971.jsonl
{
  "schema": 2,
  "ts": "2026-09-06T13:42:27.566512-07:00",
  "session_id": "session-91d4ea1705704971",
  "request_id": "req_011CenqQTy2oUZRGuFhUzqTw",
  "path": "/v1/messages",
  "model": "claude-haiku-4-5-20251001",
  "stream": false,
  "prompt": { ... "bytes": 2 ... },
  "prefix_hash": "prefix-6e340b9cffb37a98",
  "status": 401,
  "latency_ms": 132,
  "response": {}
}
```

A 2-byte prompt rejected for auth. Anthropic does not attach rate-limit headers
to a 401. So the capture path has had exactly **one** opportunity to fire, on a
request that could never have supplied it.

This is a coverage gap, not a demonstrated absence of headers upstream. The
ledger cannot tell us whether Anthropic sends these headers to this account,
because no successful request has passed through the proxy since the code
landed.

## 3. Which header keys actually appear

**None. Zero header keys have ever been recorded.**

There is no frequency table to give, because the population is empty.

For contrast, here are the keys the *code* names — these are what the
implementation expects, not what has been observed:

```text
$ grep -ohE '"(anthropic|x)-ratelimit-[a-z0-9-]*"|"retry-after"' \
      internal/quota/*.go internal/proxy/*.go cmd/replay/*.go | sort | uniq -c | sort -rn
   7 "anthropic-ratelimit-tokens-remaining"
   6 "anthropic-ratelimit-unified-5h-utilization"
   4 "anthropic-ratelimit-tokens-limit"
   3 "retry-after"
   3 "anthropic-ratelimit-tokens-reset"
   2 "x-ratelimit-remaining-tokens"
   2 "anthropic-ratelimit-requests-remaining"
   2 "anthropic-ratelimit-requests-limit"
   1 "x-ratelimit-reset-tokens"
   1 "x-ratelimit-remaining-requests"
   1 "x-ratelimit-limit-requests"
   1 "x-ratelimit-"
   1 "anthropic-ratelimit-unified-7d-utilization"
   1 "anthropic-ratelimit-"
```

Every one of these is a string literal in source or a test fixture
(`internal/proxy/quota_test.go:33-37` sets them by hand). None is a value read
back from a file on this machine.

No repo fixture carries one either:

```text
$ grep -rl '"quota"' --include="*.jsonl" .
(no output)
```

## 4. `anthropic-ratelimit-unified-*` specifically

**Zero occurrences in any data file anywhere on this machine.**

```text
$ grep -ril 'ratelimit-unified' ~/.claude ~/.replay ~/.codex
/Users/daniel/.claude/projects/-Users-daniel-Development/eec05948-.../subagents/agent-a952074feb4660810.jsonl
... (5 files)
```

Those hits are **not** captured headers. They are prior agents' own prose,
quoting the `predictor.go` comment back at themselves. Verified by searching for
the JSON key form rather than the bare substring:

```text
$ grep -rhoE '"anthropic-ratelimit-[a-z-]*"\s*:' ~/.claude/projects | sort | uniq -c
(no output)
```

```text
$ grep -rho '.\{60\}anthropic-ratelimit-unified.\{90\}' ~/.claude/projects | head -3
 randomized trial from 2026-09-06: **142 responses carried `anthropic-ratelimit-unified-*`; 3,778,706 re-billed tokens moved 5h utilization from 0.10 to 0.15** — five steps at t
cross a 30-lane randomized trial: 142 responses carried\n// anthropic-ratelimit-unified-*, and 3,778,706 re-billed tokens moved the 5h\n// utilization figure from 0.10 to 0.15.
cross a 30-lane randomized trial: 142 responses carried\n// anthropic-ratelimit-unified-*, and 3,778,706 re-billed tokens moved the 5h\n// utilization figure from 0.10 to 0.15.
```

All three are the same sentence: an agent reading `internal/analysis/predictor.go`
and repeating its header comment.

### The only claim about real unified values, and where it lives

`internal/analysis/predictor.go:5-11`:

> Measured 2026-09-06 across a 30-lane randomized trial: 142 responses carried
> `anthropic-ratelimit-unified-*`, and 3,778,706 re-billed tokens moved the 5h
> utilization figure from 0.10 to 0.15. That is five steps at the header's
> published 0.01 resolution, each step spanning ~28 requests across both arms.

Restated as a table in `docs/evidence/subscription-allowance-2026-09-09.md:33-39`,
which cites the comment as its source — not a data file.

**The raw data behind that claim does not exist on disk.** It is not in the
ledger (all 17 records checked above), not in any repo fixture, and not in git
history:

```text
$ git log --all --diff-filter=D --name-only --format="" | sort -u \
    | grep -iE 'trial|quota|ratelimit|lane.*\.json'
(no output)
```

No data file matching those patterns was ever deleted. It appears never to have
been committed.

So, answering the question as asked:

- **Sub-fields that appear in data:** none.
- **Sub-fields the code names:** `unified-5h-utilization`, `unified-7d-utilization`.
  Not `limit`, not `remaining`, not `reset`, not `status` — the unified family as
  this codebase models it is a utilization scalar only.
- **Values:** the only quoted values anywhere are `0.10` and `0.15`, from prose.
- **Unit:** a fraction of the window's allowance, resolution `0.01`, per that
  same prose. Unverified against any retained response.

## 5. Does `retry-after` ever appear?

**No. Never. Zero occurrences.**

Included in the `grep -ric` in section 2 above — count `0` in all five ledger
files — and absent from the recursive `jq paths` walk.

Also absent from `~/.claude/projects` and `~/.codex`.

**No lockout has ever been observed locally.** There is no local example of what
the provider sends when the budget runs out: not the header, not the status
code, not the wait it names.

## 6. Observed range of utilisation values

### In the ledger: no such field, no observations

### On this machine, in another client: a real corpus that moves

The Codex rollout logs under `~/.codex` carry a rate-limit record that the
Anthropic ledger does not. This is OpenAI's data, not Anthropic's, and it
reaches disk without any proxy.

```text
$ cd ~/.codex
$ grep -rl '"rate_limits":{' sessions archived_sessions | wc -l
     147
$ grep -roh '"rate_limits":{' sessions archived_sessions | wc -l
    6871
$ find sessions archived_sessions -name "rollout-*.jsonl" \
    | sed 's/.*rollout-\([0-9T-]*\)-.*/\1/' | sort | sed -n '1p;$p'
2026-03-19T18-03-11
2026-09-06T23-33-49
```

**6,871 rate-limit events across 147 files, spanning 2026-03-19 to 2026-09-06.**

Shape of one, verbatim:

```json
"rate_limits":{"limit_id":"codex","limit_name":null,
  "primary":{"used_percent":0.0,"window_minutes":300,"resets_at":1774548575},
  "secondary":{"used_percent":40.0,"window_minutes":10080,"resets_at":1774901177},
  "credits":null,"plan_type":"plus"}
```

Sub-fields: `limit_id`, `limit_name`, `plan_type`, `credits`, and two windows
each carrying `used_percent`, `window_minutes`, `resets_at`.

**Units.** `used_percent` is a percent (0–100), float-typed but observed only at
integer values. `window_minutes` is minutes: `300` = 5 hours, `10080` = 7 days.
`resets_at` is a Unix epoch second. Note this differs from the Anthropic unified
header the code models, which is a 0–1 fraction at 0.01 resolution — a factor of
100 apart, and a different reset encoding (epoch int vs RFC 3339 string).

**Observed ranges** (full distributions computed, endpoints quoted here):

```text
$ grep -roh '"primary":{"used_percent":[0-9.]*' sessions archived_sessions \
    | sed 's/.*://' | sort -n | uniq -c
 534 0.0
 618 1.0
 ...
  10 51.0
   2 52.0

$ grep -roh '"secondary":{"used_percent":[0-9.]*' sessions archived_sessions \
    | sed 's/.*://' | sort -n | uniq -c
  33 0.0
  39 1.0
 ...
  15 89.0
```

| Window | Field | Min | Max | Distinct values |
|---|---|---:|---:|---:|
| 300 min (5h) | `primary.used_percent` | 0.0 | **52.0** | 48 |
| 10080 min (7d) | `secondary.used_percent` | 0.0 | **89.0** | 89 |

**Correction to an existing claim.** `internal/transcript/codex.go:19-20` states
that "measured across 6,871 events in a local corpus, `used_percent` ranged 0 to
52 over two windows." The event count (6,871) reproduces exactly. The range does
not: `0 to 52` is the *primary* window only. The secondary window reaches
**89.0**. The docstring understates the observed maximum by 37 points.

**Does it move within a session?** Yes.

```text
$ for f in $(grep -rl '"rate_limits":{' sessions archived_sessions | head -60); do
    n=$(grep -oh '"primary":{"used_percent":[0-9.]*' "$f" | sed 's/.*://' | sort -u | wc -l)
    echo "$n $f"
  done | sort -rn | head -3
      46 sessions/2026/03/19/rollout-2026-03-19T18-04-54-019d08c6-....jsonl
      18 sessions/2026/03/21/rollout-2026-03-21T00-36-01-019d0f52-....jsonl
       1 sessions/2026/03/26/rollout-2026-03-26T21-12-24-019d2d7e-....jsonl
```

One session traverses **46 distinct** primary values. But movement is
lumpy, not per-turn: the same file's leading run holds flat for dozens of
consecutive events before stepping.

```text
$ grep -oh '"primary":{"used_percent":[0-9.]*,"window_minutes":[0-9]*' "$best" | head -30
3.0,"window_minutes":300
3.0,"window_minutes":300
   ... (30 identical) ...
```

And the third-place file has **1** distinct value across its whole session — the
counter never budged. So "moves within a session" is true of some sessions and
false of others.

## 7. Do the transcripts carry any rate-limit field?

**No. The ledger — currently holding zero quota observations — plus the Codex
logs are the only sources. `~/.claude/projects` contributes nothing.**

Corpus size:

```text
$ find ~/.claude/projects -name "*.jsonl" | wc -l
    1717
$ du -sh ~/.claude/projects
1.6G    /Users/daniel/.claude/projects
```

Searched across all 1.6 GB:

```text
$ grep -rhoiE '"[a-zA-Z_]*(ratelimit|rate_limit|quota|retry.?after|usage_limit|five_hour|weekly_limit)[a-zA-Z_]*"' \
      ~/.claude/projects --include="*.jsonl" | sort | uniq -c | sort -rn
   1 "ghRateLimitHint"
```

**One hit, in the entire corpus.** It is GitHub's, and it is a system-reminder
string rather than a quota reading:

```text
$ grep -rho '.\{80\}ghRateLimitHint.\{160\}' ~/.claude/projects | head -1
niel/Development","interrupted":false,"isImage":false,"noOutputExpected":false,
"ghRateLimitHint":"<system-reminder>GitHub API rate limit exceeded (5,000/hr
shared across all tools and agents). Run `gh api rate_limit --jq .resources`
and sleep until reset
```

Broadened to any key containing "limit", in case of an unguessed name:

```text
$ grep -rhoE '"[a-zA-Z_]{0,24}[Ll]imit[a-zA-Z_]{0,24}"\s*:' ~/.claude/projects \
      --include="*.jsonl" | sort | uniq -c | sort -rn
1358 "limit":
  37 "messageLimit":
   1 "head_limit":
   1 "ghRateLimitHint":
```

`limit` and `head_limit` are tool parameters (Read, Grep). `messageLimit` is a
conversation setting. None is a provider rate limit.

**This independently confirms the earlier finding.** The Claude transcripts carry
token usage but no allowance, no utilization, no reset, and no retry-after.

## Adjacent files checked, for completeness

`~/.replay/measurements.jsonl` (918 bytes, 4 records) holds output-token ceiling
probes, not quota:

```json
{"takenAt":"2026-09-06T01:57:51Z","method":"2026-09-06.1","model":"claude-haiku-4-5-20251001","serviceTier":"standard","geo":"not_available","above":4094,"atMost":4097,"documented":4096,"probes":9,"confirm":3}
```

`~/.replay/quota.json` — the path `cmd/replay/quotastore.go:94` writes a stored
reading to — **does not exist**:

```text
$ find ~/.replay -name "*quota*"
(no output)
```

So `loadQuota` at `cmd/replay/mcp.go:269` has never had anything to load.

## Summary table

| Question | Answer |
|---|---|
| Ledger records | **17**, across 5 files |
| Date range | 2026-09-04 22:45 to 2026-09-06 13:42 (two bursts) |
| Records with non-empty `quota` | **0 (0.00%)** |
| Distinct header keys observed | **0** |
| `anthropic-ratelimit-unified-*` observations | **0** |
| `retry-after` observations | **0** — no lockout ever seen locally |
| Ledger utilisation range | no field, no observations |
| Codex `rate_limits` events | **6,871** across 147 files, 2026-03-19 to 2026-09-06 |
| Codex `primary.used_percent` | 0.0 to **52.0**, window 300 min |
| Codex `secondary.used_percent` | 0.0 to **89.0**, window 10080 min |
| `~/.claude/projects` quota fields | **none** (1 × `ghRateLimitHint`, GitHub's) |
| `~/.replay/quota.json` | does not exist |

## Method

Read-only throughout. The only file written is this one. Counts come from `jq`
over `~/.replay/ledger/*.jsonl` and `grep -c` over `~/.codex` and
`~/.claude/projects`; `git log` was run against
`/Users/daniel/Development/Replay-clean` at HEAD `c48a8fa`, branch
`docs/narrow-the-two-promises`. Ledger record counts were taken with `jq -c |
wc -l` rather than `wc -l` alone so a missing trailing newline could not
undercount; both agreed at 17. The zero-quota result was obtained three
independent ways — `jq` on the top-level key, a recursive `jq paths` walk, and a
raw `grep` for the substrings — which agreed.
