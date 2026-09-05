# Claude Code conformance fixtures

**Captured 2026-09-05** by pointing a real `claude -p` session at `replay serve` via
`ANTHROPIC_BASE_URL`. No API key: the CLI authenticates from the keychain and the proxy
simply sits in front. Cost was ordinary usage.

Two kinds of file, and the difference is the point:

| Kind | Files | Parsed by the test? |
|---|---|---|
| **Wire bodies** | `stream-*.sse` | **Yes** — through `StreamParser`, so a parser regression fails the test |
| Ledger records | `*.json` | No — already parsed; these check the recorded shape only |

**The `.json` records cannot catch a parser bug**, and that was found the hard way:
the first version of this suite read only ledger records, and mutating
`ParseResponse` to drop `raw_usage` entirely left every test green. A check that
cannot fail is not evidence — the same vacuity the OpenAI suite's counting invariant
turned out to have. The `.sse` files exist because of it, recorded by a small proxy
sitting between Replay and the provider.

## What the capture found

**`raw_usage` was null on this path.** The OpenAI path keeps the provider's usage
object verbatim; this one — the primary path, the source of the 1,363-session corpus
— never did.

**It paid for itself on the first request after the fix.** The retained object carries
fields this build does not declare:

```json
"output_tokens_details": {"thinking_tokens": 0},
"iterations": [{"input_tokens": 1172, "output_tokens": 13,
                "cache_creation": {"ephemeral_5m_input_tokens": 0,
                                   "ephemeral_1h_input_tokens": 0}, "type": "message"}]
```

`iterations` is per-iteration accounting nobody here knew existed, discarded silently
on every request until it was kept. That is verbatim the case
`docs/architecture/multi-provider.md` makes for keeping the raw payload.

**Claude Code writes at the 1-hour TTL and never the 5-minute one.** Every observed
write sets `ephemeral_1h_input_tokens` and leaves the 5m figure at zero, so the TTL
comparison advice is reasoning about a choice the client is not making.

**A single turn both reads and writes.** `stream-cache-read-and-write.sse` reads
170,642 tokens while writing 11,383. Treating the two as alternatives would misprice
the most common shape in a long session.

## Pass/fail conditions

`A1` the 5m and 1h figures sum to `cache_creation_input_tokens` ·
`A2` a usage-reporting response keeps the provider's object verbatim ·
`A3` an aborted response records no usage at all, not zeros ·
`A4` no field is negative · `A5` the ledger holds no message text.

A1 and A2 are verified falsifiable by mutation against the wire fixtures.
