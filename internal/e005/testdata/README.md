# E005 public fixtures

The minimum needed to re-derive the frozen E005 result, and nothing else.

## Provenance

Derived 2026-09-26 from the preserved E005 artifacts, which are **not** in this
repository:

- `~/Development/replay-e005-corpus-2026-09-26/oracle2.json`, the frozen oracle
- `~/Development/replay-e005-corpus-2026-09-26/replay_out.txt`, the detector output

Neither is published. `replay_out.txt` quotes message and tool-result content in
its `where:` lines. `oracle2.json` carries no message text, but it carries
session-derived identifiers the scorer never reads.

## What was removed, and why

`internal/e005` reads eight values per event. The original record carried
eighteen. The ten it never read are not published:

| dropped | what it was |
|---|---|
| `uuid` | the assistant message's identifier |
| `cur.uuid`, `prev.uuid` | the same, for the two requests |
| `cur.req`, `prev.req` | provider request identifiers |
| `cur.ts`, `prev.ts` | full per-request timestamps |
| `cur.inp`, `prev.inp` | input-token counts, unused by every computation |
| `model` | the model name |

The break index lost more. The scorer asks only whether a boundary carries any
break, and how many; it never reads the cause strings, so the index is a count.
That also drops Replay's five cause phrases.

## What remains, and why it must

| field | why it cannot be removed |
|---|---|
| `f` | groups events and breaks into the same boundary. **Pseudonymised** to `f0001`-`f0710`; the real transcript names are not needed for grouping and are not published |
| `s` | alignment is by whole second. **Date stripped**: the recovered procedure extracted `T(HH:MM:SS)` and discarded the date already, so no calendar is published and no behaviour changes |
| `c` | the provider's divergence class. The partition being measured |
| `m` | `cache_missed_input_tokens`. Its median is a published column, and E005's point is that it is *not* a materiality proxy. Absent on the 303 indeterminate events, as in the original |
| `cw`, `pw` | provider-reported cache-creation tokens. **The dependent variable**: their difference is the +5,005 |
| `cr`, `pr` | provider-reported cache-read tokens, for the median Δread column |

All six are integers or fixed vocabulary. None identifies a session, a project,
a machine, a model or a date.

## Schema

`events.json`, 2,064 objects:

```json
{"f":"f0001","s":"20:57:55","c":"tools_changed","m":13482,"cw":5005,"cr":28000,"pw":0,"pr":185000}
```

`breaks.json`, 866 entries, `"<file>\t<second>": <break count>`:

```json
{"f0001\t20:57:55":1}
```

## Verified

The audit over both files finds zero UUIDs, long hex identifiers, ISO dates,
URLs, filesystem paths, model names, request identifiers or email addresses.
Every `f` matches `f\d{4}`, every `s` matches `HH:MM:SS`, and every other value
is an integer or one of the six provider class labels.

`score_test.go` re-derives every frozen quantity from these two files. **The
frozen result was not altered to make sanitizing easier**, and nothing was
rounded, re-binned or dropped to fit.
