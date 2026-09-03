# Architecture

System design as it exists today. Decisions that led here are in [`../adr/`](../adr/).

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

Invariants every component must hold are listed in the repository `CLAUDE.md` under "Non-negotiables".
