# Grok surface release review, 2026-09-27

Independent audit. No reader was added, and no production Go file was edited.

| | |
| --- | --- |
| Review date | 2026-09-27 |
| PR reviewed | [329](https://github.com/RedRobotKK/Replay/pull/329) |
| Evidence revision reviewed | `90a3c89` |
| Historical reader read, not executed | `6074075` (`cmd/replay/grok.go`, `internal/transcript/grok.go`) |
| Tests commit read | `912dd8` adds `internal/transcript/grok_test.go` only |
| `origin/main` | `2974d19`, `git describe` `v0.6.2-1-g2974d19` |
| Official CLI | `grok 1.0.41` |

Artifacts inspected: `origin/main` tree, `docs/SURFACES.md` at `origin/main`, dirty working-tree `docs/guide/commands.md`, the three `grok usage` JSON files on this branch, live `~/.grok/sessions` on this machine, and the two historical commits above. `/usage` was not opened. No invoice was opened.

## Repository state

| State | What is true |
| --- | --- |
| RELEASED | `v0.6.2` and `origin/main` contain no path whose name includes `grok`. There is no released Grok reader. |
| ON MAIN | Same. `git cat-file origin/main:cmd/replay/grok.go` fails because the path is not in that tree. |
| COMMITTED BUT NOT RELEASED | Nothing. The max reader is not in any commit on `origin/main`. |
| LOCAL / UNTRACKED | `cmd/replay/grok.go` and `cmd/replay/grok_test.go` are untracked. A dirty `docs/guide/commands.md` describes that command. Neither is the product. |
| DOCUMENTED | `docs/SURFACES.md` on main says Grok needs a reader and that the dollar total is not settled. |
| OBSERVED | The untracked binary was run once. It printed a max-based total. That run is not a release. |
| TESTED | The historical parser has tests at `912dd8`. They were not run in this review. The untracked tests were not run. |

## VERIFIED

`DIRECTLY_OBSERVED` on the frozen JSON in
[Official grok usage, 2026-09-27](grok-official-usage-2026-09-27.md), recomputed in this review:

| File | Turns | Sum of `inputTokens` | Session | Max | Last | Sum equals session | Max equals session | Last equals session |
| --- | ---: | ---: | ---: | ---: | ---: | --- | --- | --- |
| one turn | 1 | 1,764,978 | 1,764,978 | 1,764,978 | 1,764,978 | yes | yes | yes |
| `01a0e417` | 6 | 7,142,396 | 7,142,396 | 3,716,413 | 2,363,388 | yes | no | no |
| `01a09207` | 131 | 222,856,819 | 222,856,819 | 21,452,883 | 0 | yes | no | no |

The same equality holds for `outputTokens`, `cachedReadTokens`, `totalTokens`, and `modelCalls`. `inputTokens + outputTokens == totalTokens` on every turn of all three files. Adding `cachedReadTokens` on top of that sum matches 0 of 6 turns in `01a0e417`, 0 of 1 in the one-turn file, and 2 of 131 in `01a09207`. `cacheCreationTokens` is never non-zero. One turn in `01a09207` (turn 133) has no `costUsdTicks`; the session cost equals the sum of the 130 turns that have the field.

`DIRECTLY_OBSERVED` on a later pass over the live session directory, which is not the frozen fixture. The disk moved after `90a3c89`:

| Quantity | At the earlier write-up | This pass |
| --- | ---: | ---: |
| `updates.jsonl` files with a usage record | 46 | 46 |
| Usage records | 832 | 839 |
| Sum of per-file maxima | 78,413,855 | 78,809,418 |
| Sum of every record | 828,573,565 | 840,417,238 |
| Sum of 35 `usage.json` session inputs | 276,210,235 | 288,053,908 |

The earlier figures are a snapshot. They must not be cited as the current disk. The relationship did not flip: max, record-sum, and `usage.json` sum are still three different numbers.

On this pass, consecutive `inputTokens` in those 839 records fall 310 times and rise 483 times. `inputTokens + outputTokens == totalTokens` holds on 839/839. `cachedReadTokens <= inputTokens` holds on 839/839. Adding the cache to input plus output matches 1/839. `cacheCreationTokens` is present on 839/839 and non-zero on 0/839. That zero is `OBSERVED ZERO`. It is not proof that cache writes were free.

`usage.json` turn sums equal `usage.json` session input on 35/35 files. Comparing each directory's `updates.jsonl` usage records with that directory's `usage.json`: equal on 34/35, `updates` sum lower on 1/35, `updates` sum higher on 0/35. Thirteen `updates.jsonl` directories have no `usage.json`. No directory under `~/.grok/sessions` is a symlink.

Session `01a09207` is the one disagreement, and the excess is unchanged: `usage.json` input 222,856,819, `updates.jsonl` usage-record sum 193,337,607, difference 29,519,212. Both sides have 131 records. They are not the same multiset: 11 values are only in `usage.json` (sum 35,633,999) and 11 values are only in `updates.jsonl` (sum 6,114,787). The difference of those sums is the excess. None of the 11 `usage.json`-only values appears as an `inputTokens` field in any other session's `updates.jsonl`.

## PARTIALLY VERIFIED

The historical comment at `6074075` says this same pair of totals, 222,856,819 against 193,337,607, is sub-agent work also stored as its own sessions, and that the turns which differ have `prompt_id` `subagent-completed-...`. Today's `usage.json` for that session has no `prompt_id`. Lines in its `updates.jsonl` whose `promptId` starts with `subagent-completed` and which carry a usage object sum to 3,813,209 input tokens, not 29,519,212. The double-count those comments warn about was not found as a second copy of these tokens in another `updates.jsonl`. The excess itself is verified. The explanation is not.

`docs/SURFACES.md` on main says two aggregations differ by exactly 2×. That sentence was not re-derived in this review. It stays a dated document claim.

The Grok 1.0.41 guide says `costUsdTicks` is 10^10 ticks per USD. The sentence was read. It was not compared with a movement in `/usage`, and no invoice was opened.

## UNVERIFIED

- Whether the historical parser accepts this corpus. It was read and not executed.
- The 842-record negative regression cited by the untracked reader. Not rerun. The live usage-record count is 839, not 842.
- Maximum line length against that parser's 16 MiB scanner cap.
- Any behavior of the untracked tests.
- Dollar movement of the account allowance.

## Correction to `90a3c89`

[Do not ship the max Grok reader](grok-reader-do-not-ship-2026-09-27.md) is right that the max reader must not ship, and right about the frozen session arithmetic. Two statements in it are ahead of the evidence.

Original claim: port the `6074075` rule that records a larger `usage.json` and does not add it, because summing `usage.json` double-counts.

Why that is not yet shown: on this disk the 29,519,212 is not present in any other session's `updates.jsonl`. Dropping it makes Replay disagree with `grok usage` by that amount. Adding it does not, on the files that exist now, count a second on-disk copy. The comment's sub-agent mechanism was not confirmed.

Original claim: `docs/guide/commands.md` states that `costUsdTicks` has no documented scale.

Why that overreaches: that sentence is in the dirty working tree only. `origin/main:docs/guide/commands.md` has no Grok command section. Main does not currently publish the false sentence. The dirty guide must not be committed as written.

## Aggregation

| Hypothesis | Result | Evidence |
| --- | --- | --- |
| H1 maximum turn | REFUTED as the session total | Frozen `01a0e417` and `01a09207`: max is not the session object. The one-turn file cannot refute it, because max, last, and sum coincide. |
| H2 last turn | REFUTED as the session total | Same two files. `01a09207` last input is 0. |
| H3 sum of turns | SUPPORTED for `grok usage` and for `usage.json` | 3/3 frozen files and 35/35 live `usage.json` files. |
| H4 cumulative running total | REFUTED | Decreases in the frozen turns and in 310 of the live consecutive steps. Summing did not multiply the `usage.json` session total. |
| H5 `usage.json` session object as the machine-wide sum | UNRESOLVED as a fleet total, SUPPORTED as what `grok usage` prints per session | It matches the CLI output. Summing every `usage.json` is a different operation and was not shown to be free of inherited history. |

What survives for one session that has a `usage.json`: the official number is that file's session object, which equals the sum of its turns. The `updates.jsonl` sum is the same number on 34 of 35 directories and not on `01a09207`.

## When the sources disagree

| Case | Status | What the files show | Policy that is supported | Open |
| --- | --- | --- | --- | --- |
| Turn sum equals `usage.json` | OBSERVED, 34 directories plus all 35 `usage.json` internal sums | The two files agree | Report that number | None for the arithmetic |
| Turn sum lower than `usage.json` | OBSERVED, `01a09207` only, by 29,519,212 | Not a second copy in another `updates.jsonl` | Do not print one unlabeled total | Which source a user should be told is "the" total |
| Turn sum higher than `usage.json` | Not observed | 0/35 | None | Needs a fixture before a rule is claimed |
| `usage.json` absent | OBSERVED, 13 directories | Updates file exists alone | A total can only come from `updates.jsonl`, and must be labeled as such | Not run through a reader |
| No usage records | OBSERVED as files with no extracted usage | Absent is not zero | Report absent | None |
| Malformed JSON | Not observed in the outer scan (0 bad lines) | Historical `check()` refuses negatives, cache above input, and a broken input+output identity | Refuse the record and count the refusal. Do not add zero | Parser not executed on this corpus |
| Duplicated records | NOT_MEASURED | | | |
| Partial session | NOT_MEASURED as a named state | | | |
| Sub-agent directory with `updates.jsonl` | Not observed under `01a09207/subagents` (33 entries, 0 such files) | | Do not assume a child log exists | Where the 11 unmatched turns are from |
| Schema change that breaks input+output==total | Not observed (839/839) | Historical code refuses the record | Keep the refusal | A future record that breaks it |

## Cache and cost

Adding `cachedReadTokens` to `inputTokens` double-counts on this corpus. Input already contains the cache: adding it again matches the total on 1 of 839 live records. A price path copied from a provider where cache is extra will overstate Grok.

`cacheCreationTokens` is `OBSERVED ZERO` wherever it was read. It is not `PROVEN FREE`.

`COST = NOT_MEASURED`. The guide states a conversion. Nothing in this review reconciled `costUsdTicks / 1e10` to a change in `/usage` or to an invoice. The minimum evidence to leave that state is one session, the allowance before and after, and the tick delta, published as those three numbers. Until then a release prints no dollars.

## Golden tests

The three published JSON files are necessary and not sufficient.

They catch a max bug and a last-turn bug only if the code under test is given the multi-turn files. The one-turn file passes all three hypotheses and cannot be the only fixture. They catch a cumulative-record assumption only as an assertion on these turns, which already decrease. They do not catch an `updates.jsonl` walker: the fixture is the CLI's session envelope, not the log the walker reads. They do not catch `usage.json` double-counting or the `01a09207` disagreement, because inside that JSON the turn sum equals the session. They can catch cache double-counting only if a test asserts `input + output == total` and forbids adding `cachedReadTokens`.

Smallest additional fixtures, with no message text:

1. An `updates.jsonl` of three usage objects whose `inputTokens` are 10, then 3, then 4, beside a `usage.json` session input of 17. Pass: reported total 17. Fail: 10, or 4, or 17×3.
2. A pair where the `updates.jsonl` sum is 17 and the `usage.json` session input is 25. Pass: the output shows both numbers and does not emit one unlabeled total. Fail: printing 17 alone, or 25 alone, as if the other file agreed.
3. One non-JSON line and one usage object whose `inputTokens + outputTokens` is not `totalTokens`. Pass: both refused, neither added as zero.
4. A session with `updates.jsonl` and no `usage.json`. Pass: the total is labeled as coming from the log alone.

## Production threat model

Concrete paths only.

The untracked reader and the historical reader both walk every session directory under `~/.grok/sessions`. That tree holds conversation logs for every project on the machine. A release that prints a raw line on parse failure exposes that text. Totals do not. The failure path is the error formatter, not the happy path.

A summed total and a max total are far enough apart (about 78 million versus about 840 million on this pass) that a wrong choice still looks like a real bill. That is the release risk. It is not hypothetical. Both numbers were produced from this disk.

`01a09207`'s two files disagree by 29,519,212 input tokens. Shipping either figure as the sole total ships a number the other official file contradicts.

No symlink was present under the session root. That does not test a walker that follows one. It means this machine did not contain the case.

The historical scanner caps a line at 16 MiB. This review did not measure the longest line. A line over that cap is an untested refusal.

Output that depends on Go map iteration would be unstable across runs. That was not observed in the CLI JSON, which is keyed. It is a requirement on whatever prints model names, not a bug found in a released binary.

## Documentation

| Statement | Class |
| --- | --- |
| Main's `docs/SURFACES.md`: Grok needs a reader | DOCUMENTED and still true. No reader is on main. |
| Main's `docs/SURFACES.md`: dollar total not settled, scale unverified against an invoice | DOCUMENTED. Consistent with this review. `INVOICE-RECONCILED` is false. |
| Main's `docs/SURFACES.md`: two aggregations differ by 2× | DOCUMENTED. Not re-derived here. |
| Main's `docs/SURFACES.md`: Grok 1.0.5, 74 files | DOCUMENTED as of 2026-09-09. The client inspected now is 1.0.41. The file count is not the current disk. |
| Dirty `docs/guide/commands.md`: records are cumulative and the scale is undocumented | LOCAL text. Not on main. Both clauses contradict the frozen JSON and the user guide. Do not commit that section as written. |
| User guide: 10^10 ticks per USD | DOCUMENTED. Not invoice-reconciled. |

## RELEASE NOW

Nothing that prints a Grok token total. `replay doctor` saying the surface is present and unread matches main, and is safer than the untracked command.

The evidence pages on PR 329 can be published. They do not install a reader.

## BLOCKERS

1. Do not release the untracked max reader. Its rule is refuted by the frozen multi-turn files.
2. Do not release a single total for a session whose `updates.jsonl` sum and `usage.json` session input disagree, until the output shows both. `01a09207` is the real case.
3. Do not release dollars.
4. Do not add `cachedReadTokens` to `inputTokens`.
5. The golden fixtures above have to fail on the wrong aggregations before the command exists in a tag. The three JSON files alone do not do that for an `updates.jsonl` walker or for the disagreement case.

## REQUIRED TDD CASES

| Case | Pass | Fail |
| --- | --- | --- |
| Frozen multi-turn `grok usage` JSON | Total equals sum of turns | Total equals max, or last, or the one-turn file is the only fixture |
| Non-monotonic `updates.jsonl` of 10, 3, 4 with `usage.json` 17 | Report 17 | Report 10, 4, or 51 |
| Disagreement fixture, log sum 17, `usage.json` 25 | Print both, no unlabeled single total | Print either number alone |
| Broken identity and a non-JSON line | Refused, not added | Added as zero, or the command aborts the whole machine total with no count of refusals |
| No `usage.json` | Total labeled as log-only | A blank `usage.json` zero mixed into the sum |
| Cache | `input + output` is the total; cache reported as a share of input | Cache added on top |
| Money | No dollar line | Any division of ticks into dollars |

## POST-RELEASE

Re-run the historical parser on a frozen copy of this corpus and publish its total next to `grok usage`. Identify the 11 unmatched turns in `01a09207`. One allowance-versus-ticks comparison. A `grok -p` pair for prefix size. None of these block the decision above, and none of them repair a wrong total after it has shipped.

## NOT JUSTIFIED

A memory or work-state subsystem. Printing dollars from the guide's scale. A cache-write savings claim while `cacheCreationTokens` is zero. Treating `OBSERVED ZERO` as free. Copying a hook that trims tool output after the model chose the command. Lowering reasoning effort. Summing every `usage.json` into a fleet bill. Committing the dirty `replay grok` section of `docs/guide/commands.md`.

## Production recommendation

The smallest Grok implementation I would approve is a command that, per session directory, prints the `usage.json` session total when the log sum equals it, prints both numbers when they do not, labels a log-only directory as log-only, refuses records that break input-plus-output, does not add cache on top of input, and prints no dollars. I would not approve a command that prints one number for `01a09207`.

The single missing piece of evidence that would make me refuse even that is a second on-disk copy of those 29,519,212 tokens. This review looked for one in every other `updates.jsonl` and did not find it. If a later pass finds it, matching `grok usage` by printing the `usage.json` total would double-count, and the command should not ship until that copy is in the fixture.

## Later the same day

[What usage.json and updates.jsonl are](grok-usage-vs-updates-2026-09-27.md)
found the copy. It is not another `updates.jsonl`. The parent's
`usage.json` turns are larger than its `turn_completed` lines by exactly the
child sessions' own `inputTokens`, and those children have their own
`usage.json`. Summing every `usage.json` counts that 29,519,212 twice.
Summing `turn_completed` records does not. `grok usage` of the parent is
still the parent's `usage.json`, not the log sum. The "print both when they
disagree" rule is unchanged. The sentence above that the explanation was
missing is superseded by that file.
