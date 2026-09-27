# Second-parent replication, 2026-09-27

```text
RESULT:
NOT_MEASURED
```

```text
CANDIDATE MECHANISM:
A parent usage turn equals the turn_completed event at the same second,
or that event plus the child ledgers completed since the previous turn.
```

```text
INFORMATION GAIN:
Not measured on a second parent. On the first parent the equation is
not written in any one file. Whether a second parent produces an equation
that no one file states is unknown.
```

```text
STRONGEST FALSIFIER:
No independent parent exists in this corpus, so the mechanism has not
been given a chance to fail.
```

```text
NEXT EXPERIMENT:
When a second session directory carries subagent meta.json files whose
parent_session_id is not 01a09207, apply the procedure below once,
without refitting it.
```

No reader was written. Claude's branch was not modified. No synthetic
parent was built.

## Frozen question

Can the parent-level accounting reported by the runtime be reconstructed
from parent event evidence plus a principled subset of
temporally and lineage-qualified child-session accounting?

## Frozen procedure, written before the search

Parent identity: a session directory that is not inside `subagents/`, and
that is named as `parent_session_id` by at least one `meta.json`.

Parent usage: each object in `usage.json` `turns`. Fields `inputTokens`,
`outputTokens`, `totalTokens`. Time is `endedAt` truncated to a UTC second.

Parent events: `updates.jsonl` lines whose `sessionUpdate` is
`turn_completed` and whose `usage` is an object with integer `inputTokens`.
Time is the line's `timestamp`, as UTC seconds, truncated to a second.
Events with `usage: null` are counted and not added.

Child identity: `meta.json` `child_session_id`, included only when
`parent_session_id` equals the parent. The child's totals are that session's
`usage.json` `session` object. A child of parent 1 is not a second parent.

Time identity: truncate to one second. A usage turn and an event are the
same instant if their seconds are equal or differ by one. Each event is
assigned to at most one usage turn.

Temporal window: children whose `completed_at` second is greater than the
previous usage turn's second and less than or equal to this turn's second.

Aggregation: if several events attach to one usage turn, add them.

Child qualification: every child in that window. Not "the children that
make the sum work." Not "all children of the parent, ignoring time."

Zero-event turns: a usage turn with no attached event is recorded as zero
events. It is reconciled only if its input, output, and total are all 0,
or if it equals the sum of the children in its window alone.

Offset cases: a one-second gap is the same instant. A larger gap is not.

Equality: for every usage turn, input, output, and total each satisfy

```text
usage == attached events
or usage == attached events + children in the window
```

A parent replicates only if every turn passes, at least one turn needs the
children, and at least one child of that parent is not in any passing
child-window. The last clause is required because parent 1 already showed
that adding every child is the wrong rule.

This procedure was not changed after the search. The search found nothing
to apply it to.

## Reference case, not re-fit

Parent `01a09207-8168-7802-b465-0bedbf66f83a`, as already measured:

```text
parent usage.json input       = 222,856,819
parent event input            = 193,337,607
difference                    = 29,519,212
qualifying child contribution = 29,519,212
```

The same addition held for output on those nine turns. Twenty-four other
children of this parent did not enter the difference. Row order produced an
apparent lag. A timestamp join removed it: 118 pairs at the same second, 2
pairs one second apart. One usage turn was two events,
`965,575 + 498,596 = 1,464,171`. One usage turn had no event. Row position
is not identity.

## Search

Every `meta.json` under `~/.grok/sessions` was opened. There are 33. All 33
name parent `01a09207-8168-7802-b465-0bedbf66f83a`. There is one directory
named `subagents/`, and it is that parent's. No other session is named as a
parent.

`DIRECTLY_OBSERVED`.

```text
SECOND-PARENT REPLICATION: NOT_MEASURED
GENERALIZATION: NOT_MEASURED
```

## What this does not say

It does not say the first parent's equation is false. It does not say the
equation is how Grok works. It does not say a second parent would fail.
Those are `NOT_MEASURED`. Stopping here is the result.
