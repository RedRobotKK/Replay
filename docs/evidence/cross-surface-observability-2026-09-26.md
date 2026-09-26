# Cross-surface observability and measurement contracts

**Audit, 2026-09-26. No experiment run. E005 evidence not reopened.**

Every third-party AI surface on this machine was measured directly against the
two observables E005 used. Remote surfaces were checked against their published
contracts. The rule applied throughout: **where an observable is absent, that is
recorded as a measurement boundary, not filled with a proxy.**

This audit corrects two claims in `docs/RESEARCH-ARCHITECTURE-2026-09-26.md`.
Both were wrong, and both were wrong in the direction of understating what is
reachable. The corrections are in section 5.

## 1. What is being looked for

E005 used two observables:

- **Economic**: change in cache-creation (write) tokens across a boundary.
  Requires a write counter that is separately reported and actually populated.
- **Structural oracle**: a provider-declared cause class for a cache miss.

A surface qualifies for comparable replication only if it carries the observable
as a **billed quantity the provider reports**, not as something derivable.

## 2. Measured on this machine

Raw files parsed directly. Replay's own readers were not used as the instrument,
so a defect in a reader cannot produce these numbers.

| surface | records | write counter | populated | read counter | structural oracle |
|---|---:|---|---|---|---|
| **Claude Code** | 622,462 | `cache_creation_input_tokens` | **352,303 / 352,648 (99.9%)** | yes, 99.0% | **`cache_miss_reason`, 6 classes** |
| **Codex** | 22,827 | `cache_write_input_tokens` | **0 / 11,776** | yes, 99.9% | none |
| **Grok** | 579,339 | `cacheCreationTokens` | **0 / 1,144** | `cachedReadTokens`, 1,133 / 1,144 | none |

**Units, corrected by the reproducibility run.** The counts above are not all in
the same unit, which was invisible until the probe reported both. A surface may
state the same counter more than once in a record: Grok writes it at `usage` and
again under `modelUsage/<model>`, Codex at `total_token_usage` and
`last_token_usage`. **Codex's 11,776 and Claude Code's 352,648 are field
occurrences; Grok's 1,144 is records.** In matched units the readings are Codex
5,830 records / 11,776 occurrences, Grok 1,144 records / 2,288 occurrences,
Claude Code 210,321 records / 353,088 occurrences. **No conclusion moves**, since
zero occurrences non-zero and zero records non-zero are the same fact, but a
reader re-running the probe would otherwise see 2,288 where this table says
1,144 and reasonably think the measurement had failed.
| **Cursor** | 129,080 values + 4,838 records | none found | n/a | none | none |
| **Ollama** | 38 files | none at rest | n/a | none | none |
| **JEV** | 0 captures here | none in contract | n/a | none | none |

Claude Code's live oracle counts (1,009 `tools_changed`, 582 `messages_changed`,
255 `previous_message_not_found`, 119 `system_changed`, 48 `unavailable`,
45 `model_changed`) differ slightly from E005's frozen corpus. E005 froze against
a snapshot of 709 transcripts; the live tree has moved since. **E005 stays frozen
and was not recomputed.**

Three surfaces (Codex, Grok, and OpenClaw as already recorded in
`cmd/replay/othersurfaces.go`) share one signature: **a write counter that exists
in the schema, sits beside a read counter that is populated, and is itself zero
on every single record.** That pattern is the audit's main finding and section 3
is about what causes it.

Cursor and Ollama are different. Cursor has no cache field anywhere across
129,080 scanned values in its real store, and JEV's reader contract records the
same in its own words: Jev reports no cache figures, so there are none to report
as zero. **Absent and zero are different failures and are kept apart.**

**Correction, same day, from the reproducibility run.** The Cursor scan above
read the sqlite state store only. Cursor also keeps agent transcripts as JSON
Lines under `~/.cursor/projects/<project>/agent-transcripts/`, 118 files and
4,838 records, and that boundary was not scanned. It has since been probed by
`internal/surface` and carries no cache-write field either, so **the conclusion
is unchanged and now rests on two independent boundaries.** The original row
understated which boundary it had read, which is the kind of thing a throwaway
script hides and a committed probe does not.

## 3. Why the zeros happen, which is not what it looked like

The obvious reading is that these providers do not charge for cache writes, so
there is nothing to report. That reading is **false for Codex**, and the
measurement that kills it is specific.

OpenAI's published contract charges **1.25x the uncached input rate for cache
writes on GPT-5.6 and later**, and reports them as
`input_tokens_details.cache_write_tokens`. Earlier generations carry no
cache-write charge.

The local Codex corpus contains 117 turns on `gpt-5.6-terra`, a 5.6-generation
model. On those turns:

- **117 of 117** carry uncached input, so there were new tokens to cache.
- median uncached remainder is **1,278 tokens**, and **79 of 117 exceed 1,024**,
  which is OpenAI's cache minimum. They are not all sub-minimum remainders.
- **0 of 117** report a non-zero write.

A generation that is billed 1.25x for writes, processing above-minimum uncached
input on every turn, recording zero writes every time. **That is the Codex
client failing to persist a field the provider returns, not a provider that
charges nothing.** The observable exists at the API boundary and is destroyed at
the durable-artifact boundary.

Mapping the name `gpt-5.6-terra` onto the documented "GPT-5.6 and later" pricing
tier is an inference from the model name, not a measurement, and is labelled as
one.

A note in `internal/transcript/codex.go` attributes Codex zero-writes to Replay's
own reader not carrying the field. **That is a separate defect and fixing it
changes nothing here**, because these numbers come from the raw rollout files and
the raw value is zero.

### Codex observability is regressing, not improving

| month | session files with a reply | of those, carrying `last_token_usage` |
|---|---:|---:|
| 2026-03 | 26 | **26 (100%)** |
| 2026-09 | 235 | **20 (9%)** |

215 of 235 real September sessions record no usage at all. The newest rollouts
carry only `multi_agent_usage_hint` and `usage_hint_hash` where token accounting
used to be. **A replication plan that assumes Codex transcripts will carry usage
is planning against a surface that is losing the field.**

## 4. Published contracts, remote surfaces

Checked against provider documentation. Not measured here, and labelled so.

| provider | write counter | write premium | structural oracle |
|---|---|---|---|
| **Anthropic** | `cache_creation_input_tokens` | **1.25x** | **`cache_miss_reason`** |
| **OpenAI GPT-5.6+** | `input_tokens_details.cache_write_tokens` | **1.25x** | **none documented** |
| OpenAI GPT-5.5 and earlier | none | none | none |
| DeepSeek | none | writes bill at standard input | none |
| Gemini | none | writes bill at standard input | none |
| xAI / Grok | schema field present, never populated | unknown | none |

The structural column is the harder constraint and it did not move. **No provider
other than Anthropic publishes a cause class for a cache miss.** OpenAI's own
caching guide documents no miss-reason field. E005's design, which compares a
detector against a provider oracle, **remains unrepeatable off Anthropic**, and
that is a property of what providers publish rather than a gap in effort.

The economic column did move. **OpenAI GPT-5.6+ is a second prefix-cache contract
with a separately priced, separately reported cache write.** That is one of the
two things E005 needs, and the architecture document said it did not exist.

DeepSeek and Gemini are a third category worth naming. Both report cached reads
and bill a cache miss at ordinary input price. There is no write premium to
observe, so there is no Δwrite, but the cost of a miss is directly visible as
full-price input. That is a real economic observable under a **different cache
contract**, not a proxy for Anthropic's. It answers a related question, not
E005's, and merging the two would be a scale-identity error of the kind the GTM
pre-registration already prohibits.

## 5. Corrections to the research architecture document

**Correction 1.** The matrix row `Codex | absent, reads reported, write field
absent entirely` is wrong. The field is present, appears 11,776 times across
5,830 records, and is uniformly zero. Absent and zero are different, and the difference decides whether
a proxy is even tempting.

**Correction 2.** The conclusion "Claude Code is currently the only surface where
either half of E005 can be measured" is too strong. Correct statement:

> Claude Code is the only surface where **both** halves can be measured, and the
> only surface where either can be measured **from durable artifacts on this
> machine**. A second priced cache-write contract exists on OpenAI GPT-5.6+, and
> is reachable at the API seam rather than from transcripts.

**Correction 3.** Items 5, 10 and 12 of the fifteen questions treat the second
contract as non-existent and the generality question as blocked on instruments
that cannot be built. That is no longer accurate. The second contract exists and
the instrument that reaches it, a recording proxy, **already ships as
`replay serve`**. The blocker is running traffic through it, not building it.

Nothing in E005 changes. These corrections concern what can be done next.

## 6. What qualifies for comparable replication

**Full E005 replication, both arms: Claude Code only.** No second surface
qualifies, and none can be made to, because the structural oracle is one
vendor's field.

**Economic arm only, Δ cache-write on a second contract: OpenAI GPT-5.6+
qualifies, via `replay serve`, not via Codex transcripts.** This is the one
genuinely new option the audit surfaces. It tests whether class-dependent
materiality is an Anthropic artifact or a property of prefix caching, which is
alternative explanation (1) in the architecture document and is currently
uneliminated. It cannot use the provider's cause classes, because there are none,
so it tests materiality against Replay's own detector rather than against an
oracle. **That is a weaker design than E005 and must be labelled as such, not
reported as an E005 replication.**

**Measurement boundary, no replication possible: Codex transcripts, Grok, Cursor,
Ollama, JEV, AnythingLLM, OpenClaw, Oracle.** For Codex, Grok and OpenClaw the
boundary is an unpopulated field; for the rest the field does not exist. No proxy
is constructed for any of them.

**Related but not comparable: DeepSeek, Gemini.** Different cache contract, real
economic observable, different question. Kept separate.

### Ranking, cheapest first

1. **A second operator's Claude Code corpus.** No new instrument, no spend, tests
   habit against property. This was already the architecture document's first
   choice and the audit does not displace it.
2. **`replay serve` against OpenAI GPT-5.6+.** Needs an API key and spend, which
   is task #30's standing blocker. Delivers a second contract on the economic arm
   only.
3. Everything else on the list. Not worth queuing.

## 7. Constraint carried into any protocol

Restated because it survives every option above:

> E005's 2,064 events across 14 sessions are clustered observations, not 2,064
> independent replications. **Any replication protocol must specify the
> session-level and operator-level analysis before the data are examined.**

A second contract does not relax this. It makes it sharper, because a
two-contract comparison with one operator on each side confounds contract with
operator completely.

## 8. Claim taxonomy

### ESTABLISHED BY THIS AUDIT

- The measured table in section 2, from raw files, readers not used as instrument.
- Codex, Grok and OpenClaw share the populated-read, zero-write signature.
- Codex zero-writes on `gpt-5.6-terra` are a client logging gap: 117/117 turns
  with uncached input, 79/117 above the 1,024-token minimum, 0/117 non-zero.
- Codex durable usage logging fell from 26/26 sessions to 20/235.
- Cursor carries no cache field across 129,080 scanned values.

### ESTABLISHED FROM PUBLISHED CONTRACTS, NOT MEASURED HERE

- OpenAI prices GPT-5.6+ cache writes at 1.25x and reports
  `input_tokens_details.cache_write_tokens`.
- OpenAI documents no cache-miss reason field.
- DeepSeek and Gemini bill cache writes at standard input price.

### NOT ESTABLISHED

- That `gpt-5.6-terra` is priced under the documented 5.6 tier. Inferred from the
  model name.
- That a `replay serve` capture on OpenAI would reproduce class-dependent
  materiality. Untested, and it cannot use provider cause classes.
- Anything about xAI's cache pricing. The field is unpopulated and the contract
  was not found.
- That fixing Codex's client logging is feasible or will happen.

## 9. Reproducing this

The readings above were first produced by throwaway scripts. They are now
re-derivable from the repository, which is the only reason to trust them without
re-running the originals:

```sh
REPLAY_PROBE_REAL=1 go test ./internal/surface/ -run RealLocal -v
```

`internal/surface` separates the two inputs that a classification needs. The
probe reports what a corpus exposes; `contracts.go` carries the provider pricing
facts with the document each was read from. **Neither can stand in for the
other**, and a corpus of zeros with no contract returns `undetermined` rather
than a finding. Seven mutations that would upgrade a class on weaker evidence
are killed by the suite.

Re-run on 2026-09-26, hours after the original scan. Codex reproduced exactly at
0 of 11,776 occurrences across 5,830 records, Grok at 0 of 2,288 occurrences
across 1,144 records. Claude Code's totals ran ahead of the
table above because the live tree grows while this machine works; a Python and a
Go implementation read the same tree at the same instant and agreed to the
record, so **the difference is corpus growth and not disagreement between
instruments.**

The package is deliberately not part of the shipped binary. `docs/design/
UNWIRED-LOG.md` #10 says why, and what would change it.

## 10. Standing

The research universe is **two contracts on the economic arm and one on the
structural arm**, not eight surfaces. Seven of the surfaces named in earlier
planning carry neither observable, and adding them would grow a sample while
answering nothing.
