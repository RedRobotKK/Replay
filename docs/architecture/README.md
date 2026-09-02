# Architecture

System design as it exists today. Decisions that led here are in [`../adr/`](../adr/).

## Current state

Nothing proxies traffic yet. The repository contains the command skeleton, build pipeline, and documentation. This page will grow with v0.1; until then, the target shape is:

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
