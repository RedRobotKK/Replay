# Architecture

System design as it exists today. Decisions that led here are in [`../adr/`](../adr/).

## Current state

The offline analysis (`replay`, `blame`, `diff`, `redact`) is implemented; see [`replay-engine.md`](replay-engine.md). Nothing proxies traffic yet. The target shape once the proxy lands:

```text
 agent (Claude Code, Aider, custom)
   │  ANTHROPIC_BASE_URL / OPENAI_BASE_URL -> http://127.0.0.1:4000
   ▼
 buffy serve
   ├─ listener        loopback TCP, header-token auth
   ├─ passthrough     bytes in, bytes out; SSE preserved
   ├─ usage capture   provider usage fields per request
   ├─ cache detector  prefix diff between adjacent requests
   └─ dashboard       local page: cost, tokens, cache ratio
   ▼
 provider (api.anthropic.com, api.openai.com, ...)
```

Invariants every component must hold are listed in the repository `CLAUDE.md` under "Non-negotiables".
