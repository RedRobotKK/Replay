# 0004. Secret masking with persistent vault and scoped rehydration

**Status:** Accepted (implemented 2026-09-03 with the amendments below)
**Date:** 2026-09-02

## Context

Masking replaces secrets in outbound prompts with placeholders and restores them in responses. Three attacks shape the design. Placeholder forgery: untrusted content the agent reads can instruct the model to emit a placeholder inside a shell command, and naive rehydration would put the real secret into that command. Non-deterministic placeholders break caching and history binding. A RAM-only vault orphans every placeholder in the client's transcript when the daemon restarts.

## Decision

Placeholders are derived by HMAC of the secret under a per-project key, so the same secret always maps to the same placeholder. The vault persists encrypted at rest with a key held in the operating system keychain. Rehydration is scoped: by default placeholders are restored only in assistant text and in file-edit tool inputs targeting paths inside the project, never in shell or network tool inputs; scope is configurable per pattern; every rehydration is logged with its destination. Thinking blocks are never modified in either direction. Detection uses a named pattern set plus user patterns and an optional entropy heuristic, and the README names the patterns rather than claiming completeness. Masking ships last in the release sequence because it depends on the deterministic rendering and history-binding test harness the earlier releases establish.

## Consequences

- The provider does not see the secret, and content the model reads cannot route the secret to the network through Replay.
- The same secret produces the same placeholder across sessions of a project, which lets the provider correlate them. Documented as a known limitation.
- Some legitimate workflows (a secret needed in a shell command) require the user to widen scope for that pattern. That is a deliberate friction.

## Alternatives considered

**Random placeholders per request.** Rejected: breaks caching and history binding.

**Rehydrate everywhere.** Rejected: turns masking into an exfiltration primitive.

**RAM-only vault.** Rejected: data loss on restart.

## Amendment 2026-09-03

Masking shipped ahead of rehydration, opt-in and documented as an evaluation of coverage. Two deviations from the decision above are in force until their prerequisites exist: the placeholder key is per Replay install rather than per project, because the proxy has no project identity it can trust at a request's first byte, which widens the correlation the consequences section already accepts from one project to one machine; and the vault key lives in an owner-only file under the Replay directory rather than the operating system keychain. Both are recorded in the PRD status column and the roadmap.

Rehydration followed the same day with three refinements the decision left open. Tools the proxy does not recognize are denied like shell and network tools, because a default that guesses a new tool is harmless is the exfiltration path the decision exists to close; the file-edit tools are a named list and `--rehydrate-scope tool:NAME` admits others. A streamed tool call's input is held until its block ends, because the target path that decides scope can arrive after the placeholder; text deltas are rewritten as they arrive, holding at most one delta whose tail could begin a placeholder. A file edit is inside the project only when the tool's own path field is present and every path-like field in its input resolves under the project, with symbolic links followed along the part of the path that exists; a link the edit itself would create cannot be checked, which is documented. Responses are requested uncompressed when rehydration is on, and a compressed response is forwarded untouched with a log line.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
