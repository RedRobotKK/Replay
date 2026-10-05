# Control boundary of the cache-TTL setting, 2026-10-05

**Question.** What does `promptCacheTtl` actually control, which requests
can it reach, and how would Replay know the state changed?

## Evidence sources, in the order consulted

1. Repository code and docs: Replay reads the setting from the Claude Code
   settings file, writes it with `advise --apply --yes`, and reads each
   request's TTL from the usage breakdown the client writes
   (`cache_creation.ephemeral_5m_input_tokens` and
   `ephemeral_1h_input_tokens`). `docs/architecture/proxy-protocol.md`
   already stated that the main conversation and the helpers have separate
   settings from client 2.1.242.
2. The installed client, outside the repository: Claude Code 2.1.278 at
   `~/.local/share/claude/versions/2.1.278`, read as strings, read-only.
   Its TTL resolver is deterministic code, not a distribution.
3. This machine's transcripts, for the observable state only.

## The resolver, as read from the client (OBSERVED, external)

For each request the client resolves a TTL from a query source:

1. `FORCE_PROMPT_CACHING_5M` set: 5m.
2. If the query source is on the main-thread list (`repl_main_thread*`,
   `sdk`, `auto_mode`, `memdir_relevance`): the environment variable
   `CLAUDE_CODE_PROMPT_CACHE_TTL`, then the setting `promptCacheTtl`.
   Otherwise: `CLAUDE_CODE_SUBAGENT_PROMPT_CACHE_TTL`, then the setting
   `subagentPromptCacheTtl`.
3. An agent definition's own `cacheTtl` (ignored for 1h while in overage).
4. `ENABLE_PROMPT_CACHING_1H`: 1h.
5. Otherwise: 5m unless the account is a subscription not in overage and
   the query source is on the allowlist, in which case 1h.

The setting's own description in the client: "Prompt cache TTL for the
main conversation (interactive, -p and SDK turns, plus the helpers that
run inline with it) ... Unset = automatic: 1 hour on a Claude subscription
within its usage limits, 5 minutes on an API key, Bedrock, Vertex or
Foundry."

## Control-boundary record

| Item | Finding | Status |
|---|---|---|
| Controlled surface | the `cache_control.ttl` the client sends on main-thread requests | OBSERVED, external code |
| Control input | `promptCacheTtl` in the settings file; the environment variable overrides it | OBSERVED, external code |
| Controlled request class | query sources `repl_main_thread*`, `sdk`, `auto_mode`, `memdir_relevance` | OBSERVED, external code |
| Not controlled | every sub-agent request, governed by `subagentPromptCacheTtl` and agent frontmatter | OBSERVED, external code |
| Observable state | per request, the TTL the write was billed at, in the transcript's usage breakdown | OBSERVED, transcripts |
| In-record marker of the class | sub-agent transcripts live under `<session>/subagents/` and carry `isSidechain` on every assistant record: 2,222 of 2,223 files here, the other has no assistant record; 0 of 994 main files carry a sidechain record | OBSERVED, transcripts |
| Replay's marker | `Lane.Sidechain`, set by the parser from `isSidechain` | OBSERVED, repository |
| Verification mechanism | after a write, the settings file is read back (shipped today); the request-level effect is visible in later transcripts' usage breakdown | OBSERVED |
| Outcome signal | cost per new input token from usage, list prices | CALCULATED; task outcome UNAVAILABLE |
| Unknown scope | whether the subscription or overage state changed during a session, which flips the automatic default; whether the allowlist is remotely configured (`tengu_prompt_cache_1h_config`) on other accounts | UNAVAILABLE |

## Causality

The split in this corpus (main sessions 1h, sub-agents 5m) is explained by
the resolver, not inferred from the split: with both settings unset, a
subscriber's allowlisted sources resolve to 1h and every other source to
5m. The observation and the code agree, and the code is the cause.
CONTROL_BOUNDARY = main-thread query sources, ESTABLISHED from external
deterministic code.

What remains UNAVAILABLE: the effect of the setting on any account that
is not a subscription, and on an account in overage, where the automatic
default is already 5m and a 5m write is a no-op.

## ELIGIBLE_POPULATION

Lanes with `Sidechain == false`. Both predictors now refuse the rest: the
TTL family is not scored on a sidechain lane in `learn`, and a sidechain
lane is not an input to `advise --apply` before any aggregation.

## Predictor semantics on the eligible population

Every figure here is CALCULATED by the simulator over this machine's
records, 2026-10-05, after the two corrections below. Nothing is a
measurement of the setting's effect.

| | `learn`, ttl family | `advise --apply`, `ttlPlan` |
|---|---|---|
| Population | main-thread lanes that calibrate; 3,217 files found, 2,570 calibrated, 1,038 held out | main-thread lanes at or above 95% match rate with both TTLs priced |
| Baseline | as-run: the session's own usage, priced | none: the two simulated TTLs against each other |
| Candidate | simulated ttl-5m, simulated ttl-1h | simulated ttl-5m against simulated ttl-1h |
| Weighting | one session, one vote | one token, one vote |
| Numerator | (as-run effective tokens minus candidate's) per session | dearer TTL's total minus cheaper TTL's total |
| Denominator | the session's as-run effective tokens | the dearer TTL's total |
| Tie handling | a session whose share is under one part per million is not evidence | identical totals are refused before any margin |
| Coverage | not applied | scored as-run spend over trusted as-run spend, at least 50% |
| Veto | none | the costliest tenth of sessions disagreeing with the total |
| Margin | mean share with a two-standard-error interval and a held-out mean | the share stated, at least 1% |
| Units | share of a session's as-run effective tokens | share of the dearer TTL's simulated total |
| Result today | ttl-5m 26.3% (25.6 to 27.0), held-out 25.9%, 897 sessions, selected; ttl-1h minus 4.4% over 9 sessions | 1h, margin stated by the evidence line; would write `(unset) -> 1h` |

Two corrections, each with its RED test and frozen mutant:

- The tie floor compared a share to a token count (M142). Before it,
  every session over a million effective tokens was excluded; ttl-5m was
  scored on 755 sessions and is now scored on 897.
- Sub-agent lanes were scored for the TTL family (M143). Before it, learn
  reported ttl-1h at minus 35.1% over 1,087 sessions; those were 2,223
  sub-agent transcripts that ran 5m, scored under 1h. On main-thread lanes
  only 9 sessions differ from as-run under 1h at all, at minus 4.4%: the
  simulator reproduces the policy the client ran.

The disagreement that remains is real and explained. The typical
main-thread session prefers 5m by about a quarter of its own cost; the
bill prefers 1h, because the sessions that carry it are long with idle
gaps. The automatic default on this subscription is already 1h, so the
bill-weighted recommendation is a no-op here and the session-weighted one
is a change whose bill-weighted simulated effect is negative. Neither
predictor is authoritative; the experiment names which number it tests.

## Reconciliation with the earlier bill-weighted probe

The earlier probe over the recursive population put simulated 5m at 0.2%
below as-run and 1h at 11.8% above; both figures were dominated by
sub-agent sessions that ran 5m. On main-thread lanes, the apply
predictor's own aggregate prefers 1h, and that is the figure that stands.
