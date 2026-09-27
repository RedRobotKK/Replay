# Second-parent gate, 2026-09-27

```text
REPLICATION:
NOT_MEASURED
```

No session besides `01a09207-8168-7802-b465-0bedbf66f83a` has the artifact
layout the frozen test requires. The numerical procedure was not run,
because running it would have meant relaxing the requirements. No reader
was written.

## What was searched

Every `meta.json` under `~/.grok/sessions`. There are 33. Every one has
`parent_session_id` `01a09207-8168-7802-b465-0bedbf66f83a`.

Every `subagent_spawned` event in a top-level `updates.jsonl`. Two session
ids appear as `parent_session_id`:

| Parent id | Children named | Parent `usage.json` | Parent `updates.jsonl` | Child ledgers on disk | `meta.json` naming this parent |
| --- | ---: | --- | --- | ---: | --- |
| `01a09207-8168-7802-b465-0bedbf66f83a` | 33 | present | present | 33 | 33 |
| `01a01136-57f8-7672-a9c6-b24e02d1e4c9` | 58 | absent | present | 0 | 0 |

`DIRECTLY_OBSERVED`. No token sums were computed for this gate. The second
row is the August DoorKik session already used as a negative control. It
does not meet the frozen requirements: no parent `usage.json`, no child
`usage.json`, no `meta.json`. It was not used as a replication and its
event totals were not re-added.

## Why the corpus cannot run the experiment

The test needs one parent that is not `01a09207`, with `usage.json`,
`updates.jsonl`, at least one child session that itself has `usage.json`,
and timestamps on both. The only other id that the logs call a parent is
missing the ledger, the child files, and the meta records. There is no
third id.

```text
SECOND-PARENT REPLICATION: NOT_MEASURED
GENERALIZATION: NOT_MEASURED
```

The nine-turn equality on `01a09207` stays one parent. DoorKik stays a
case with nothing to relate. Neither result is a reason to implement a
reader.
