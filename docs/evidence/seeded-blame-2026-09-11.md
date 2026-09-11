# Does Replay name the event that broke the prefix?

**Measured 2026-09-11.** Ten seeded events, ten correct answers, one limit
found and one confirmed safe degradation.

## Why this exists

Twenty-nine dated files sit in this directory and every one measures a
property of the tool — cache ratios, token counts, kill matrices, price
tables. Not one measures whether the tool's central claim is true.

The claim is on the front page: Replay names the event that broke your prefix.
Until this ran, nothing had checked it against a break whose cause was known
in advance. The evidence base was falsifiable in every file and closed as a
population: every reading could only ever land on the instrument.

## Method, and the trap it avoids

**Seed the event, not the symptom.** Setting `cache_read` to zero and then
asserting the classifier says "prefix changed" tests that a equals a — it
restates the classifier's own rule back to it.

So each case describes something that happened to a session — a model swap, an
MCP connector finishing its handshake, a lunch break — and lets the usage and
the prefix follow from it. `cache_read` is zero in every case, because that is
the consequence of all of them, not an input chosen to steer the answer. The
expected cause is written into each case before the run.

**Refuted if** blame names a different event than the one seeded.

The trial is `internal/proxy/blametrial_test.go`, so it runs on every commit
rather than being a number in a file that stops being true.

## Result: 10 of 10

| Seeded event | Named | |
|---|---|---|
| A lunch break (20 min gap) | cache expired | ✓ |
| Model switched mid-session | model changed | ✓ |
| MCP connector finished its handshake | tool definitions changed | ✓ |
| A connector dropped out | tool definitions changed | ✓ |
| A tool's schema grew (same name, new size) | tool definitions changed | ✓ |
| CLAUDE.md gained a paragraph | system prompt changed | ✓ |
| An edit that preserved its length | system prompt **or** tool definitions changed | ✓ (see below) |
| Model changed after a long gap | cache expired | ✓ |
| Model and tools both changed | model changed | ✓ |
| System and tools both moved | system prompt or tool definitions changed | ✓ |

## The one limit, and why the answer is still right

`prefixDelta.systemChanged()` compares **byte counts**, not content. Swapping
`2026-09-10` for `2026-09-11` in a project instruction file moves the prefix
and moves no count. Reordering a sentence does the same.

So that break reaches the default branch, and the tool reports the wider cause
that covers both halves rather than naming one. **That is correct behaviour,
not a defect.** The code says so where it happens: *"the prefix carries
something this build does not summarise. Saying so beats naming a half at
random."*

A reader is told less than the truth, never something false. Recorded here so
the limit is known rather than discovered by someone whose bill it explains.

## Precedence, where two things are true at once

Two cases exist because more than one event can be true and only one is the
cause.

**Model changed after a long gap** blames the gap. The cache was already gone
when the model changed, so the swap cost nothing — blaming it would send a
reader to pin a model that was never the problem.

**Model and tools both changed** blames the model. A model swap invalidates on
its own, so the tool change is downstream of a break that had already happened.

## The trial can fail

A passing test that cannot fail is not evidence (ADR-0014), so both halves of
the chain were mutated.

| Mutation | Result |
|---|---|
| Swap the TTL/model precedence in `ClassifyBreak` | red on *the model changed after a long gap*, and only that case |
| Return `CauseToolsChanged` where the system changed | red on *the project instructions were edited*, and only that case |

Each mutation lands on exactly the case written for it, which is what
distinguishes a trial from a suite that happens to be green.

## What this does not establish

It does not show the blame is right on a **real** corpus. These are
constructed sessions with one event each, and a real break can have a cause
outside this vocabulary entirely.

It does not measure whether naming the event **helps** anyone. That is still
unmeasured, and it is the larger gap: this trial moves the flagship claim from
untested to tested, and says nothing about whether a person who reads the
answer ends up better off.
