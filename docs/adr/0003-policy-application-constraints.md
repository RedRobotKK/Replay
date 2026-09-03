# 0003. Live policies use only provider-sanctioned mechanisms

**Status:** Proposed
**Date:** 2026-09-02

## Context

A context layout policy that edits an earlier turn client-side (for example, clearing a tool result from turn 3 at turn 9) is a history edit. On the newest models it returns a 400 for organizations created on or after 2026-08-31, and on every model it invalidates the cache from the edited point, which defeats the purpose. The provider excludes its own server-side context editing, compaction, and cache-marker changes from that check. Server-side compaction, however, returns blocks the client must echo back, which a client unaware of the proxy will not do.

## Decision

A policy is admissible only if it is one of three kinds: a request parameter the client left unset (context editing, breakpoint placement, TTL), an append-only addition after the cached prefix (mid-conversation system messages, turn-scoped reminders), or a deterministic per-message transformation applied identically every time the message is seen (secret masking with HMAC-derived placeholders). The proxy never enables server-side compaction, never overrides a parameter or marker the client set, pins the chosen policy at a session's first request for the life of that session, and forwards original bytes on any internal failure except a tripped spend cap. Learning writes a policy file; the proxy reads it only at session start.

## Consequences

- No policy can trip the history-binding check or corrupt a session. The catalog is small and each entry is auditable.
- Some savings visible in replay (aggressive client-side pruning) are unreachable live by design. Replay reports them as unreachable rather than hiding them.
- Session identity must be reliable enough to pin policies; when uncertain, no policy is applied.

## Alternatives considered

**Client-side rewriting with a deterministic render cache.** Rejected: deterministic within a session is not enough when the policy's own inputs (which files are "hot") evolve across turns.

**Enable compaction from the proxy and rewrite responses to hide it.** Rejected: it requires Buffy to hold provider state the client does not know about, which is exactly the fragility the transparency principle forbids.
