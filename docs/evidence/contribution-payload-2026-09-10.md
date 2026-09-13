# What a corpus contribution contains, 2026-09-10

**What this measures:** the exact bytes `replay cost --contribute` produces, and
the surface `replay mcp` exposes. Both were read from the types that build them
rather than from prose about them, and the payload was serialized the way
`internal/observation/corpus.go:217` serializes it.

Written because [`SURFACES.md`](../SURFACES.md) records that these artifacts
exist and not what is in them. A reader deciding whether to contribute needs the
field list and the size, not an assurance.

> **Amended later the same day.** The payload described below was 13 scalars.
> Three optional waste fields were added afterwards — `cacheBreaks`, `reReads`
> and `errorShare`, the distribution ADR-0009 asks the corpus to carry — taking
> a populated submission to **16 fields and 502 bytes**. A real one, generated
> against this machine's corpus in an isolated home:
>
> ```json
>   "cacheBreaks": 768,
>   "reReads": 428,
>   "errorShare": 0.07984523390784383,
> ```
>
> They are pointers, absent when nil, for two reasons that happen to coincide.
> ADR-0018: a build that did not measure an error share and a corpus whose share
> is genuinely zero are different states. And `Pool.Add` recomputes the digest
> rather than trusting it, so a field that serialised when absent would have
> invalidated every submission written before it existed. A test pins that: an
> old-shape payload re-digests to the value already in its file.
>
> Everything below about what is NOT in the payload still holds — the three new
> fields are a ratio and two counts, with no path from any of them back to code,
> prompts or a project.

> **Amended 2026-09-12, and this one changes a claim rather than extending it.**
> Three more optional fields were added for defect #284: `binaryVersion`,
> `commit` and `pricingDigest`. A populated submission is now **19 fields**, and
> the bound is **under 600 bytes**: 328 at its smallest, 572 for a submission
> with every optional field set and every float at full round-trip width. All
> three figures were measured by serialising the type, not estimated, and
> `internal/observation/corpussize_test.go` fails if they move.
>
> ```json
>   "binaryVersion": "v0.6.0",
>   "commit": "87a9b2a",
>   "pricingDigest": "p02eb9163145c",
> ```
>
> **WHY THE SIZE CLAIM MOVED, AND WHY THE "NO TEXT" ONE NEEDS RESTATING.**
>
> On 2026-09-12 two builds read the same transcript directory on the same
> machine and reported $4,088.49 and $11,969.37. Both stamped
> `rulesVersion: anthropic-2026-09-01`, and the label was honest: the provider's
> document had not changed, the code applying it had. Six models one build
> declined to price became priced, and the unknown-model read multiple became a
> named rule and moved. A pool adding those two submissions was summing
> different arithmetic under one name, and `replay.doctor/pool/` was saying so
> by hand, in prose, because the file could not say it itself.
>
> `pricingDigest` is computed from the price table, the caching floors, the
> unknown-model fallback and any loaded rules document, so it moves when any of
> them does. `binaryVersion` and `commit` say which build, so a reader can go and
> look at it.
>
> **These are the first three strings in this payload that are not a date, a
> schema, or a tag the contributor chose, so the sentence "counts and ratios, no
> text" stops being exactly true and is restated here rather than quietly
> dropped.** What they carry: a semantic version, a short hex SHA of a public
> commit, and a hex digest of numbers compiled into every copy of the binary.
> Anyone can compute the third from a release they downloaded. Two contributors
> running the same release send the same three strings, which is the test that
> matters: they describe the binary, not its operator.
>
> Everything below about what is NOT in the payload still holds. No paths, no
> project names, no session ids, no model ids, no tool names, no message text,
> no timings, no per-task rows, no hostname, username or hardware identifier.
>
> All three are optional and the schema string did not move, for the reason the
> previous amendment gives: `Pool.Add` recomputes the digest, so a field that
> serialised when absent would invalidate every submission already written.
> Absent means a build from before they existed, which is a fact a pool can show
> rather than a gap it has to guess at.

## The payload: 13 scalars, 394-432 bytes

`observation.Corpus` (`internal/observation/corpus.go:81-119`), serialized with
`json.MarshalIndent(c, "", "  ")` and written `0600`:

```json
{
  "schema": "replay.corpus.v1",
  "takenAt": "2026-09-10T04:00:00Z",
  "tasks": 116,
  "totalUsd": 3424.33,
  "avoidableUsd": 162.82,
  "avoidableShare": 0.0475,
  "medianTaskUsd": 1.87,
  "pricedAt": "2026-09-07",
  "rulesVersion": "anthropic-2026-09-01",
  "unpriced": 6,
  "sourceTag": "a3f19c02b7e4d581",
  "tagBasis": "local",
  "digest": "9f2c1e77a4b0d3e6f8129ab45cd7e0f31b6a8d92c4e5f70a1b3c6d8e9f024a5b"
}
```

| values | bytes |
|---|---:|
| all-minimal | 394 |
| this machine, 2026-09-10 | **415** |
| a corpus 8600x larger | 432 |

The size is fixed by the key names, not the data. A contributor with a million
tasks and a seven-figure bill sends 432 bytes. **It is smaller than one line of
one transcript.**

**The numbers above are a representative submission, not a captured one.** The
field set, order and serialization are read from the type; the values are this
machine's real figures placed into it. No submission has been made from here.

## What is not in it

No paths. No project names. No session ids. No model ids. No tool names. No
message text. No timings. No per-task rows. No hostname, username or hardware
identifier.

That is enforced rather than intended: `TestO7_ThisPackageCannotSend`
(`internal/observation/observation_test.go:275`) walks the package's imports and
fails if `net`, `net/http`, `os/exec` or `net/url` appears, and asserts the
banned list is genuinely absent from the allowlist rather than passing
vacuously. **The package cannot transmit, so the file is the whole disclosure.**

## Why each field is there

- `tasks` travels with the money because a total without an n cannot be weighted
  into a pooled figure.
- `pricedAt` and `rulesVersion` travel because an aggregate of totals computed
  against different price tables is not an aggregate of anything, and a reader
  of the pooled number has no way to notice unless each submission says.
- `unpriced` is how many transcripts were read and excluded for having no model
  in the table — excluded rather than counted as free, so a pooled figure can
  state what it does not cover.
- `digest` is computed over the payload with the digest field empty, so it can
  be recomputed from the published file. It is what makes "$X across N
  contributed corpora" a claim someone can walk back to its parts.

## The tag, and the residual risk its own source names

`sourceTag = HMAC(key = campaign, msg = identity)`, truncated to 16 hex
characters (`internal/observation/observation.go:125-137`). The campaign is the
key, which scopes a tag to one campaign so two campaigns' tags for one identity
cannot be linked — that is what stops it being a persistent installation id.

The campaign salt is public, so **someone holding a candidate list of identities
can test membership against a tag.** For `tagBasis: "local"` the identity is 32
bytes from `crypto/rand`, which makes the oracle useless. For an account-derived
basis it is an oracle for someone who already knows their target rather than a
way to learn who contributed. The source says this itself and says it belongs in
the docs rather than in one maintainer's head; this file is where it now lives.

## Where the disclosure actually is, and it is not the field list

**1. `totalUsd` is a near-unique fingerprint, and the same binary encourages
publishing it.** The submission is built from the same summary that `--share`
and `--png` render into a social card. A contributor who posts a cost card and
separately submits a corpus links their tag to their public identity by exact
float match, with no adversarial effort required.

**2. The transport binds an identity the payload carefully omits.** The
submission is attached to a pull request. A GitHub account and a git identity
are bound to it at the moment of submission, so the HMAC protects nothing
against the only party that receives it.

**3. Repeated submissions are a time series.** The tag is stable per machine and
superseded submissions are published, so a public pool exposes per machine: a
spend trajectory, task-count growth, and activity gaps — holidays, project ends,
a contract ending. `takenAt` at hour resolution is right for one submission and
becomes a working-hours and timezone envelope when repeated. For a company
contributor, total agent spend is commercially sensitive on its own.

None of these are defects in the payload. They are properties of publishing a
small honest number under a stable name, and a contributor should be told them
before they decide rather than after.

## The MCP surface, for comparison

`replay mcp` is a JSON-RPC server on **stdio**: Replay is the server, the agent
is the client, and there is no transport beyond the pipe the agent already owns.
`cmd/replay/mcp.go` imports no network package. Seven tools:

| tool | answers from |
|---|---|
| `replay_surfaces` | which agents leave readable state on this machine |
| `replay_price_check` | the compiled table, one model |
| `replay_rules_free` | the compiled table, all models |
| `replay_mcp_overhead` | the size of tool definitions the client is carrying |
| `replay_quota` | a stored rate-limit reading, or "no reading" |
| `replay_rules_latest` | **nothing — it refuses** |
| `replay_installer_release` | **nothing — it refuses** |

The last two are facts about remote state. Rather than fetch them or answer from
a compiled-in value that would go stale, they return a sentence saying so:
*"This server does not hold the maintained feed and will not invent it"*
(`mcp.go:259`). That is ADR-0018's absence/zero/unknown rule implemented in a
tool, and it is why this server needs no network import.

`replay_surfaces` is the one whose output is about the operator rather than
about prices. It stays on the machine, and it is the shape any future
`replay_cost` tool would take with money attached.

## Limits

**No submission was captured.** Every field name, its order and the
serialization are read from the type and the writer; the values are this
machine's figures placed into that shape. A real submission has never been made
from here, so nothing in this document is a captured artifact.

**The tag's residual risk is reasoned, not tested.** No attempt was made to
build a candidate list and run the membership oracle described above.

**The MCP tool list is this build's.** It is read from `cmd/replay/mcp.go` on
2026-09-10 and will not follow the code on its own.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
