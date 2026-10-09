# Second-provider calibration: what exists, what was validated today, and the runbook, 2026-10-09

**What this answers.** Exactly what condition C-2 of the v1.0 closure
ledger requires, which parts of it exist on candidate `37c0d0a`, what this
environment could verify without a provider, and the precise procedure
and evidence that would close it. It is labelled throughout: the work run
today is **fixture validation of the analysis pipeline**, not
second-provider calibration, and nothing here changes RELEASE-CRITERIA.md's
"production calibration remains outstanding".

**State: BLOCKED on authorized external execution.** No OpenAI credential
exists in this environment (zero matching environment variables; checked
again today, names only), no spend is authorized, and the 1.0 definition
("checked by somebody other than the person who wrote them") means a corpus
produced on the maintainer's own machine would not close the gate alone.

## 1. The requirement, from the documents that set it

| Source | Requirement |
|---|---|
| RELEASE-CRITERIA.md, second-provider row (corrected 2026-10-08) | published rules, production dispatch, **and** a calibration corpus against real traffic; the first two exist, the third does not |
| docs/ROADMAP.md, "Caching rules for a second provider" | "needs published provider rules and a calibration corpus for them, so it starts here and lands when it calibrates" |
| docs/architecture/multi-provider.md, sequencing | "Add the second provider only when there is real traffic to calibrate against" |
| closure-council-1.0-2026-10-09.md, C-2 | "A calibration record from real OpenAI traffic, or a dated amendment by Daniel" |
| closure-phase2-2026-10-09.md, section 6 | recommendation B (dated amendment) for 1.0, A (funded corpus) as the first 1.x gate; minimum evidence for A stated |

## 2. The baseline: Anthropic

| Item | Value |
|---|---|
| Record | [calibration-corpus-2026-09-10.md](calibration-corpus-2026-09-10.md), a later reading of the 2026-09-06 record |
| Corpus | 1,751 transcripts from 116 distinct sessions on the maintainer's own machine |
| Metric | compared turns 34,750; matched 33,983; overall match rate 97.79%, with the exact and inexact split recorded since #209 |
| Method | the engine's predicted cache behaviour per turn against the provider's own usage fields in the transcript, per model, with break causes attributed |
| Independence | none: one machine, one operator, the author of the rules; this is why the 1.0 definition does not treat even this as externally checked |

## 3. The second provider: OpenAI, as it stands on `37c0d0a`

| Part | State | Where |
|---|---|---|
| Published rules | `AstraRules()` typed and mutation-tested, read from OpenAI's documentation on 2026-09-15, never replayed against a live response | `internal/cachemodel/openai.go`, `docs/rules/openai-2026-09-15.json` |
| Production dispatch | `RulesForModel` and `ClassifyBreakForModel` are called by the live proxy, the offline diff and the usage-export cost path for any model id containing `gpt-6-astra` | `internal/proxy/state.go`, `internal/analysis/diff.go`, `cmd/replay/costusage.go`, `internal/proxy/breakcauseprovider_test.go` |
| Known gap in dispatch | Astra's 1,024-token MinPrefix floor is unreachable from all three call sites, because none knows the request's visible prefix size at classification time | RELEASE-CRITERIA.md second-provider row |
| Chat completions path | read, guarded and ledgered; labelled EXPERIMENTAL, UNMASKED; verified against a stub (2026-09-05) and a local Ollama endpoint; the inclusive-counting trap (`prompt_tokens` contains `cached_tokens`) is handled by `usage.FromInclusive` | [spike-openai-compatible-2026-09-05.md](spike-openai-compatible-2026-09-05.md), docs/architecture/multi-provider.md |
| Responses API path (Codex) | wire to ledger to economics chain built 2026-10-04 against fixtures **built, not captured**, to Codex CLI's own parser and its own test fake; no live Responses endpoint was called | [responses-r1-2026-10-04.md](responses-r1-2026-10-04.md), `internal/ledger/testdata/openai-responses/README.md` |
| Prior live evidence from a non-OpenAI OpenAI-shaped endpoint | the DeepSeek run of 2026-09-05 answered the inclusive-counting question with real numbers; it is a different provider and is not OpenAI calibration | docs/architecture/multi-provider.md, "The DeepSeek run" |
| Calibration corpus | **none** | |
| Guard against overclaiming | `TestC039_NoOpenAICalibrationAgainstLiveTrafficClaimIsAsserted` scans README.md, RELEASE-CRITERIA.md, CHANGELOG.md and docs/ROADMAP.md for a sentence asserting the calibration happened | `cmd/replay/openaicalibrationclaim_test.go` |

## 4. Fixture validation run today, labelled as such

On `37c0d0a`, this host, go1.27.2, 19:46 UTC:

```sh
go test -count=1 -v -run 'OpenAI|Openai|Responses|Astra|R1_|C039|OAE|Codex' \
  ./internal/cachemodel ./internal/ledger ./internal/proxy \
  ./internal/transcript ./cmd/replay ./internal/probe
```

Exit 0. 136 top-level tests passed, 0 failed, 0 skipped, across six
packages (`cachemodel` 0.305s, `ledger` 0.514s, `proxy` 0.850s,
`transcript` 1.941s, `cmd/replay` 1.304s, `probe` 1.485s). The black-box
Responses tests against the built binary
(`go test -tags mutation -run Responses ./internal/blackbox/`) also passed.

What this establishes: the parsing (chat completions and Responses, stream
and non-stream), the attribution (inclusive to exclusive usage, cache write
tokens, reasoning tokens), the absence semantics (no usage and empty usage
are absence, not zero; an error behind a 200 is unparsed), the ledger
record shape, the Astra rule dispatch and the overclaim guard all behave
as specified **on fixtures**. What it does not establish: anything about
how OpenAI's production endpoint actually counts, prices or expires
cached prefixes. The provider has not been observed.

## 5. The runbook for option A, when it is authorized

Preconditions, all outside this environment's authority:

1. Daniel chooses A (phase 2 section 6) and authorizes a written spend
   cap. Spend is incurred by the provider's billing, not by this tool.
2. A machine that is not the maintainer's, operated by someone who is not
   the author of the rules, with its own OpenAI account and key. The key
   is never committed, printed, or copied into any record.
3. An agent client that speaks one of the two paths: Codex CLI for
   `/v1/responses` (preferred; the transport probe of 2026-10-04 showed
   it reaches an ordinary HTTP proxy over SSE), or any
   OpenAI-compatible client for `/v1/chat/completions`.

Procedure:

```sh
# on the independent machine, with the release binary (never a local build)
replay version                                 # record the stamped version and commit
replay serve -upstream https://api.openai.com -mask   # loopback proxy; masking on
# point the client at the proxy (Codex: the base URL for /v1/responses)
# run at least 20 real sessions of ordinary work, across at least two days,
# so that TTL expiry and prefix growth both occur naturally
replay corpus ~/.replay/ledger --json > corpus-openai-<date>.json
replay cost --json ~/.replay/ledger > cost-openai-<date>.json
replay rules --check-prices                     # the price table date in force
```

Evidence that closes the gate (phase 2 section 6, made concrete):

| Item | Requirement |
|---|---|
| Sample | at least 20 distinct sessions, stated as sessions and lanes, never transcripts |
| Reconciliation | the provider's own usage fields (`prompt_tokens` and `cached_tokens`, or `input_tokens` and `input_tokens_details`) reconciled turn by turn against the engine's prediction under `AstraRules()` |
| Metrics | match rate with its denominator, exact and inexact split, per-model rows, break-cause distribution, and the MinPrefix-floor cases counted separately because dispatch cannot classify them |
| Thresholds | none pre-registered beyond the Anthropic baseline's shape; the record must report whatever rate it finds and the rules move if the observed field disagrees with the documented one, as `cachemodel.Claim` is built to do |
| Controls | the same ledger replayed through `replay simulate` to show the policy path agrees with the live one; one session with masking off as a contrast, on synthetic content only |
| Provenance | the record names the operator, the machine, the dates, the stamped binary version and commit, the price-table date, and the spend; it is signed by the operator, not the maintainer |
| Where it lands | `docs/evidence/calibration-corpus-openai-<date>.md`; RELEASE-CRITERIA.md's row corrected by appending, not rewriting; `docs/rules/openai-<date>.json` if any field's `observed` differs from `documented` |

What the maintainer's own machine may do beforehand, and did today: the
fixture validation in section 4. Nothing else without authorization.

## 6. Option B, which this environment did not write

A dated amendment by Daniel in `docs/evidence/`, one row correction in
RELEASE-CRITERIA.md, and one sentence on each surface that promises a
second provider (README.md, docs/ROADMAP.md, the replay.doctor docs after
the re-pin), stating that 1.0 ships with one provider calibrated end to
end and a second with rules, dispatch and a labelled experimental path,
and that calibration is the first 1.x gate. The runtime already states
this limitation on the labelled path. The choice between A and B is
Daniel's; no amendment was written and no claim was narrowed in this pass.

---

[Evidence index](README.md) · [Closure ledger](closure-ledger-1.0-2026-10-09.md) ·
[Release criteria](../../RELEASE-CRITERIA.md)
