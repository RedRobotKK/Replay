# Architecture

System design as it exists today. Decisions that led here are in [`../adr/`](../adr/README.md).

| Document | What it covers |
|---|---|
| [Replay engine](replay-engine.md) | How caching is reproduced from a transcript, when alternatives may be scored, and how estimated stays apart from measured |
| [Proxy protocol](proxy-protocol.md) | What the client sends through a local gateway and what `serve` must preserve, with the source for each fact |
| [Policy file](policy-file.md) | The format `replay learn` writes and the proxy reads |
| [Multi-provider caching](multi-provider.md) | How caching differs across providers, and what that forces in the engine |
| [Predictive design](predictive-design.md) | What the ADR-0009 constraints remove and narrow in the schema |
| [Rewriting prompts](rewriting-prompts.md) | Where rewriting what the client sends is safe, and where it dismantles the tool |
| [Vectorising session data](vectorising.md) | Three questions inside "vectorise", measured before deciding |

The last four are dated working notes rather than descriptions of shipped behaviour. The first three
describe code that exists.

## Current state

The offline analysis (`replay`, `blame`, `diff`, `redact`) is implemented; see [`replay-engine.md`](replay-engine.md). The passthrough proxy (`serve`) is implemented for the Anthropic Messages API and records a derived-data ledger; the client-side facts it honors are in [`proxy-protocol.md`](proxy-protocol.md). Guards (spend caps, loop detection, circuit breaker, retries, error budget), dry-run scoring of candidate layouts, one opt-in live policy (server-side context editing as a request parameter, ADR-0003), and the offline learning job (`learn`, ADR-0006, [`policy-file.md`](policy-file.md)) are built. The proxy reads the policy file at each session's first request when asked to. The shape today:

```text
 agent (Claude Code, Aider, custom)
   │  ANTHROPIC_BASE_URL / OPENAI_BASE_URL -> http://127.0.0.1:4000
   ▼
 replay serve
   ├─ listener        loopback TCP, header-token auth
   ├─ passthrough     bytes in, bytes out; SSE preserved
   ├─ response tap    usage and output structure, parsed after forwarding
   ├─ guards          spend caps, loop detection, circuit breaker, retries (off by default)
   ├─ policy          adds one request parameter the client left unset (off by default)
   ├─ live analysis   break classification and dry-run scoring, after the response
   └─ ledger          ~/.replay/ledger/<session>.jsonl, derived data only
   │
   └─ replay replay | blame | diff  read the ledger at the measured tier
   └─ replay learn                   scores the catalog over all sessions, writes ~/.replay/policy.json
   ▼
 provider (api.anthropic.com, api.openai.com, ...)
```

Invariants every component must hold are listed in the repository `AGENTS.md` under "Non-negotiables".

---

[Documentation index](../README.md) · [Repository README](../../README.md)
