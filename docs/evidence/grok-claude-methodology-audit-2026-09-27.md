# Audit of the Grok source-contract reasoning, 2026-09-27

Independent review of Claude's page
[The Grok source contract](https://github.com/RedRobotKK/Replay/blob/docs/grok-source-contract/docs/evidence/grok-source-contract-2026-09-27.md),
commit `9529434` on
[pull request 330](https://github.com/RedRobotKK/Replay/pull/330).
No reader was written. Claude's branch was not modified.

Also checked against this branch: the frozen `grok usage` fixtures, and
[usage.json against updates.jsonl](grok-usage-vs-updates-2026-09-27.md)
at `d58b273`. That page is corrected below. It over-unified two different
disagreements into one child-session story.

## What was rechecked

| Claim in `9529434` | This pass |
| --- | --- |
| `origin/main` has no `cmd/replay/grok.go` | Stands. No `grok` path on `origin/main`. |
| Max reader is untracked | Stands. Not on main, not in `v0.6.2`. |
| `grok usage` fails with no `usage.json` | Reproduced. `01a01136-57f8-7672-a9c6-b24e02d1e4c9` and `01a0788f-7560-7060-9238-d70fd98be287` both exit 1 with `Error: No usage recorded for session`. |
| 653-record session sums to 581,144,282, and the 13 no-ledger sessions sum to 581,882,542 | Arithmetic matches the same census. 581,882,542 is 66.9% of all `turn_completed` input on this machine. |
| `sum(turns) == session.inputTokens` on 35/35 `usage.json` | Stands. |
| 34 equal, 1 higher, 0 lower | Stands. |
| 18 of 131 turns in `01a09207` disagree | Stands, as 18 of 131 positions. 113 positions are equal. |
| A one-turn lag | Stands for 7 positions in the tail, not for the block at turns 103-110. |
| Subagents are not the explanation, because there is no `subagents/**/updates.jsonl` | The absence is real. The conclusion is not licensed. |

## CLAUDE METHODOLOGY AUDIT

### VERIFIED REASONING

Stopping before writing a reader was the right stop. The max rule is refuted
by the frozen multi-turn fixtures. `usage.json` is internally a sum of its own
turns on every file that exists. `grok usage` reproduces that file when it
exists and refuses when it does not. That refusal was run, not inferred from
the filename.

The warning that a live `usage.json` grows is correct and important. The
published `01a0e417` fixture is 6 turns and 7,142,396 input tokens. Claude
later read 12 turns and 16,622,681. A still later read of the same file was
14 turns and 26,799,498. A test pointed at the live file is not a test. The
committed snapshot is the pin.

The token-weighted trap is real. Counting sessions, 35 of 48 have a ledger.
Counting `turn_completed` input, 66.9% of the tokens sit in directories the
CLI will not report. A corpus that is "mostly in agreement" by session is not
mostly in agreement by token. Claude did not hide that.

Marking cost as `NOT_MEASURED`, and an observed zero `cacheCreationTokens` as
not a free write, follows the evidence. So does refusing the historical
reader as a specification.

### UNSUPPORTED INFERENCES

**"Nothing is missing, because both sides have 131 records."** Same count is
not the same multiset. Eighteen positions differ. Eleven values are only in
`usage.json` and eleven are only in the log.

**"The disagreement is per-record drift, structured as a one-turn lag."**
The lag is real and narrow. From the position of turn 126 onward, with one
exception, the log's `inputTokens` equals the previous `usage.json` turn.
That shift's net contribution to the 29,519,212 is not the 29,519,212. The
eight positions at turns 103-110 differ by 29,290,876 in total. One later
position differs by 228,336. Those two nets are the entire difference. The
lag rearranges values. It does not explain the sum.

**"Subagents do not explain the gap."** What was observed is that
`subagents/` contains no `updates.jsonl`. Each of the 33 entries has a
`meta.json` whose `child_session_id` is a normal session with its own
`usage.json`. For the eight early positions and the 228,336 position,
`usage.json` input equals the log line at that second plus the child-session
`inputTokens` completed in that window. Those child sums are 29,519,212.
Calling the nested-log absence a refutation looked in the wrong directory.

**"`grok usage` reads `usage.json`."** The CLI's output matches the file, and
the CLI errors when the file is absent. That is reproduced behavior. The
binary was not traced. "Reads that file" is the best behavioral account, not
a source observation. A third writer that both the command and the file
follow was not found, and was not ruled out by a trace.

**"Grok does not count any of" the 581,144,282.** The CLI declines to
report it. The `turn_completed` objects are on disk and contain
`inputTokens`. "No report" and "no usage occurred" are different sentences.
Claude's own later sentence, that a reader which sums the log invents tokens
the tool does not report, is the accurate one. The earlier sentence is not.

**Ship the source contract while the cause is `NOT_MEASURED`.** For the claim
"print the number `grok usage` prints," the contract is supported without
knowing the cause. For the claim "this is the usage," it is not. The page
ships one contract under both readings.

### CATEGORY ERRORS

The page collapses these into one ledger:

| Claim | What would decide it | What was used |
| --- | --- | --- |
| What `grok usage` prints | The command and `usage.json` | Used, correctly |
| What a `turn_completed` event recorded | The log | Treated as a defective copy of the ledger |
| What a parent and its children cost if added | Both files, kept apart | Not in the contract |
| What the account was charged | `/usage` or an invoice | Correctly left `NOT_MEASURED`, then not kept separate in the word "ledger" |
| Context-window size after compact | The compact event (401,088 to 8,080) | Not the same number as either file |

"Usage.json excess" and "updates.jsonl shortfall" are the same subtraction
with a direction chosen in the noun. The evidence establishes a difference of
29,519,212. It establishes that the parent ledger is larger than the parent
log by the child-session totals. It does not establish that the log failed
to record a turn, or that the ledger invented one. The child sessions have
their own ledgers. The tokens are recorded twice if every `usage.json` is
summed, and recorded once in `turn_completed` lines if every log is summed.
Neither file is deficient. They answer different questions.

### MISSING EXPERIMENTS

Claude had already counted the corpus. More directories of the same kind will
not promote 34/35 into a semantic law. The missing measurements are:

1. A trace or a documented read path showing `grok usage` opens `usage.json`,
   so the behavioral agreement is not the proof.
2. Write order for the lag: whether `usage.json` is rewritten in place after
   the log line, or the log line is the previous projection. Seven positions
   show the lag. Nothing shows which write happened last.
3. The 24 child sessions from 2026-09-11, whose totals are not inside the
   29,519,212. Whether those parent turns already contain the same tokens is
   still open. Corpus-wide equality cannot see inside an equal turn.
4. One allowance reading against `costUsdTicks`. Still open. Correctly not
   filled in.

### TDD CORRECTIONS

A test `sum(turns) == session.inputTokens` proves this snapshot of
`usage.json` is a sum. It does not prove summing is the definition of usage,
and it does not prove the log should be summed. The frozen multi-turn files
do refute max and last for those session objects. The one-turn file cannot.

| Kind | What it may claim | What it must not claim |
| --- | --- | --- |
| Unit | Given these turn records, the sum is this, and the max is not | That the records are the right source |
| Golden | The committed `grok usage` stdout equals `session.inputTokens` | A number read from today's live file. `01a0e417` has already moved twice |
| Differential | `grok usage <id>` matches that session's `usage.json`, and errors when the file is absent | That the error means the log is empty |
| Negative | Parent `usage.json` plus child `usage.json` is not one bill. The 29,519,212 is the fixture | "No nested `updates.jsonl`, therefore subagents are irrelevant" |
| Safety | A directory with only a log is reported as log-only or omitted under an explicit label | A single total that silently drops 581,144,282 or silently adds it |

### PRODUCTION CONTRACT CORRECTIONS

The contract that may ship is narrow:

`grok usage` for a session that has `usage.json` is that file's session
object. It is the sum of that file's turns. It is not the max and not the
last. When the log sum differs, print both and name them. Do not call the log
sum `grok usage`. Do not sum a parent ledger with its children. Do not print
dollars. A directory with no `usage.json` is not a zero, and it is not a
number the CLI reported.

The contract that must not ship is: `usage.json` is the only usage that
occurred, and `updates.jsonl` is a drift-prone copy of it.

### MOST IMPORTANT BLIND SPOT

The unit of the claim. Claude asked which file is authoritative. The files
disagree because they are not trying to be the same quantity. `usage.json` is
the report `grok usage` will give, and on this parent it includes child
session totals. `updates.jsonl` is one `turn_completed` usage object per
event, and on this parent those objects do not include the child totals. A
one-turn lag in the tail is a third fact about write timing, and it does not
move the sum. Choosing a winner throws away the distinction the discrepancy
was built out of.

The same blind spot is in `d58b273` on this branch. That page showed the
child-session addition and treated it as the whole disagreement. The lag is
real, and it is not that addition. Both accounts were half of the positional
evidence.

### FINAL ASSESSMENT

`METHODOLOGY SOUND WITH CORRECTIONS`

The stop, the fixture pin, the session-versus-token trap, and the refusal to
invent dollars are sound. The production sentence is sound only if it means
"this is what `grok usage` prints." It is not sound if it means "this is the
usage."
