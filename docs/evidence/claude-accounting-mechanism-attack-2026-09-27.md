# Attack on the accounting mechanism, 2026-09-27

```text
interesting first observation
+ clean negative control
+ second replication NOT_MEASURED
= do not implement yet
```

No reader was written. Claude's branch was not modified.

The note under audit is
[The session grok usage will not report](grok-largest-no-usage-forensic-2026-09-27.md).
This pass recomputed the sums from
`01a01136-57f8-7672-a9c6-b24e02d1e4c9`. They match the note: no
`usage.json`, 653 usage objects, input sum 581,144,282, 245 decreases in
652 steps, 58 `subagent` prompt ids summing to 62,012,873. All 33
`meta.json` files still name only `01a09207-8168-7802-b465-0bedbf66f83a`.

## 1. Negative control

```text
CLEAN NEGATIVE CONTROL
```

The hypothesis under test is that an interesting accounting relationship
requires reconstructing a parent report from events plus child ledgers.
This session cannot be that reconstruction. It has events and no ledger.
It names 58 child ids and none of those sessions exist. The command's
refusal is the missing `usage.json`. Relating the files that do exist does
not produce a number the log does not already contain. A case with no
second artifact cannot support a cross-artifact mechanism. It can only
show the mechanism's precondition failing, which is what a negative
control is for.

It is not evidence that the first parent's equation is false.

## 2. Two phenomena

**Phenomenon A. Ledger file appears.** Sessions created on or before
2026-09-06 have no `usage.json`. Sessions created on or after 2026-09-11
have one. The current `grok usage` reports `No usage recorded` when the
file is absent. The August DoorKik session is this phenomenon. Its
581,144,282 is a sum of per-turn event fields, not a CLI report.
581,882,542 is eleven such directories, not this session.

**Phenomenon B. One parent ledger is not the sum of its own events.**
Parent `01a09207`, measured earlier and rechecked this pass on input and
output: usage input 222,856,819, event input 193,337,607, difference
29,519,212. Nine turns equal the event at that second plus the child
ledgers completed in the window. Those children contribute 29,519,212
input and 274,819 output. Twenty-four other children of the same parent
are not in that difference. Adding all 33 children would include
19,055,196 input tokens the parent turn does not contain.

Do not explain A with B. Do not explain B with A.

## 3. What still supports the candidate

The candidate is: some facts are recoverable only by relating parent
events, parent `usage.json`, and child ledgers through time and lineage.

Supported, on one parent only:

- 120 usage turns match an event once each event is joined by time,
  including two pairs one second apart. Row index does not.
- Nine turns match event plus the children in that window, on input and
  on output. The sums are the entire input difference and the output
  difference of 274,819.
- Neither file states that split. The child ids and the two numbers have
  to be added.

Not supported:

- A rule, stated in advance, that picks those nine children and not the
  other twenty-four. The window rule fits the nine. The same window rule
  also gathers the twenty-four, and adding those breaks the turns. The
  selection was read off the residual.
- Any second parent. See below.
- The DoorKik session. It has nothing to relate.

One turn on the first parent is still outside the nine-window rule. The
usage turn at 2026-09-13T23:37:59 equals two events,
`965,575 + 498,596` input and `3,516 + 935` output. A check that only
adds children fails that turn. A usage turn of zeros at 01:01:47 has no
event. A one-second join that is allowed to reuse an already matched
event will attach the 01:01:46 event to that zero turn and report a
false failure.

## 4. Replication

```text
REPLICATION:
NOT_MEASURED
```

One parent id. Thirty-three children of that parent. No other
`subagents/` directory. Children of `01a09207` are not independent
parents. Generalization is `NOT_MEASURED`.

## 5. Strongest competing explanation

The number `grok usage` prints is the `usage.json` session object, and
that object is the sum of its own turns. That explains every CLI result
in this corpus, including the DoorKik refusal. It does not explain why
nine parent turns are larger than their events by exactly the child
ledgers.

The competing explanation of those nine turns is that the equality was
found by testing the residual, not by a qualification rule that could
have been applied with the parent ledger hidden. Until a field or a
second parent predicts which children enter the turn, the equality is
an observation about one session. It is not a procedure.

## 6. Next experiment

One only. Do not run it until a second parent exists.

When a `meta.json` names a `parent_session_id` other than
`01a09207-8168-7802-b465-0bedbf66f83a`, join that parent's usage turns
to `turn_completed` events by timestamp, assign children by
`completed_at` to the window since the previous turn, and test input
and output separately:

```text
usage == attached events
or usage == attached events + children in the window
```

Pass only if every turn does one of those, at least one turn needs
children, and at least one child of that parent is not used. Do not
refit the window because a turn fails. A fail is the result.

## 7. NOT_MEASURED

A second parent. Why the twenty-four children are outside the parent
turns. Whether a child's own input already contains the parent's prompt,
which would mean the 29,519,212 is partly context counted inside the
child before it is added. Which binary wrote the August session. The
contents of the fifty-eight child ids named in the DoorKik log and
absent now. Dollars. A production reader.
