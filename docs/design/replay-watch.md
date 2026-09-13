# Replay Watch

**Status:** step 1 of 3 built, 2026-09-13. Nothing in a release emits a record
yet, and no command produces one.

## What it is

`replay` reads one machine once. Watch is the standing version: one record per
session per repository, produced where the session happened and carried to
whoever owns the invoice.

It exists for one question, and the question is the whole product: **spend on
this repository went up and nobody can say which week or which change.** A
person who can answer that has something a dashboard of totals does not.

## The boundary, which is a test rather than a paragraph

Counts, ratios, cause classes and hour-resolution timestamps travel. Content
does not: no prompt, no path, no tool name, no file name, no session id.

`TestWA1_NoFieldCanCarryContent` checks the **wire form** rather than the
struct, because a json tag can differ from its field name and the wire form is
what leaves. `TestO7_ThisPackageCannotSend` walks the imports of
`internal/observation` and there is no transport among them. A record is a value
and a file; something outside the package posts it. That separation is what lets
the binary keep saying it makes no network call while a plugin the operator
configured makes one.

The bridge between a number and its cause is one local command named at the end
of a report, and that command runs on the operator's machine against data that
never left it.

## Opt-in is a third grant, not a flag on an existing one

`internal/consent` now holds three files:

| File | What it permits |
|---|---|
| the update check | a version lookup |
| `corpus-consent.toml` | BUILDING a submission that a human then moves by hand |
| `watch-consent.toml` | a machine sending, on a schedule, with nobody present for the individual send |

The third is a different promise and gets its own answer. Somebody who opted
into the corpus because a human carries each file has not agreed to a hook that
posts one at the end of every session, and a design that read one file for both
would have quietly upgraded their answer.

**Unset is refused exactly as Declined is.** Silence is not a grant anywhere in
this repository, and this is the path where that rule earns its keep.

The gate is at construction. `BuildWatch` is the only supported way to get a
record, and it returns nothing at all without a grant. A check at the sending
edge is a check somebody adds a second code path around, and the second code
path is always written by someone who did not read the comment above it.

## The session tag

`WatchSessionTag(repoKey, sessionID)` is keyed by the repository key rather than
being a bare hash, and that is the entire design. A bare hash is stable wherever
it is computed, so two organisations holding records derived from the same
session would hold a value that joins their data sets. Keying it means one
session under two repository keys is two unrelated values, and neither side can
discover that by looking.

It de-duplicates re-reads, so a session read twice does not count twice.

## What is refused

A record with nothing measured. Nothing measured is not a pass (ADR-0018): a
week of empty records draws a flat line, and a flat line reads as a quiet week
rather than as an absent one. That distinction matters most to exactly the
customer this is for, the one asking why a number moved.

A refusal for want of consent is a **distinct error** from a refusal for want of
a measurement. Both arrive at the far end as no record, and the operator at the
terminal has to be told which, because "watch is off" is fixed by a command and
"nothing measured" is not fixed by anything.

## The six cause classes, by mechanism

These are the labels the classifier already produces for `replay diff`, restated as the mechanism each one names so a reviewer who knows how prefix caches work can check the definition rather than the name. A provider cache is a byte-stable prefix of the request as assembled on the wire; a break is the first byte that differs, and everything after it is written again.

| Class | Mechanism | How the classifier decides |
|---|---|---|
| `rerender` | the client re-serialised history (block merge, reminder stripped in place, a newline at a boundary) so the prefix bytes changed with no edit visible in the persisted transcript | cache-read tokens fall on a turn whose persisted messages are byte-identical to the previous turn's prefix; inferred, not observed, and the record says so by keeping this class separate |
| `ttlExpiry` | the gap since the previous request exceeded the TTL in force (5 minutes, or 1 hour when requested); the entry aged out | inter-request gap greater than the TTL, and cache-creation tokens on the next turn approximately equal to the previous prefix length |
| `toolChange` | the tool list or a tool schema changed; tools precede system and messages in the cached prefix on Anthropic's order, so every byte after them is rewritten | tool block set or bytes differ between consecutive requests |
| `systemChange` | the system prompt bytes changed (a clock, a cwd, a memory file inserted before the breakpoint) | system block differs between consecutive requests |
| `modelSwitch` | the cache is per model; a different model starts cold | model id differs between consecutive requests |
| `unknown` | the prefix diverged inside message history and the block could not be named | none of the above matched and cache reads fell |

Counts, never shares: the ranking of these classes flipped between two readings of the same corpus on 2026-09-06 and 2026-09-11 (`docs/evidence/rerender-band-sensitivity-2026-09-11.md`), so a share is a snapshot and a count is a fact.

## What a local-server record would need, not decided here

vLLM and SGLang implement prefix caching at block granularity (a hash over token blocks, evicted under memory pressure), not as a TTL over a byte prefix. A `provider: "vllm"` record would need the block size and the eviction policy in its provenance to make `ttlExpiry` and `rerender` meaningful, and `idleGapsOverTtl` would have no TTL to compare against. This ADR leaves `provider` open and refuses nothing, but the classifier must not label a local-server session with the Anthropic classes until an evidence file has measured what a break looks like there; `docs/evidence/ollama-cache-observable-2026-09-09.md` is the starting point.

## Build identity is mandatory here

`binaryVersion`, `pricingDigest` and `rulesVersion` are required on a Watch
record and optional on a Corpus one. The difference is the point: corpus
submissions exist in the wild from before those fields did, and making them
mandatory would orphan every one. No Watch record has ever been written, so the
stricter rule costs nothing **and can only be set now** — which is ADR-0024's
argument applied to a schema instead of a verb.

The reason is measured rather than theoretical. Two builds priced one corpus at
$4,088.49 and $11,969.37 under one rules label. A weekly timeline that silently
mixes them reports a step change in spend that was a step change in arithmetic,
which is the worst failure available to a product whose job is explaining why a
number moved.

## The size, pinned

A fully populated record is **625 bytes** of a 1,024-byte ceiling, across 20
wire fields, and `TestWA15` fails if either moves. It is posted at the end of
every session, so its size is a standing cost on somebody else's network and a
claim in this file; changing it means changing both in one commit.

## Not built

2. The receiving endpoint, and the aggregation that turns records into a week.
3. The emitter: a hook the operator installs, which is the only piece that makes
   a network call and the only piece the consent file above governs.

Nothing here is wired to a command. `replay` today still sends nothing.

---

[Design index](README.md) · [ADR index](../adr/README.md) ·
[The money path](../MONEY-PATH.md)
