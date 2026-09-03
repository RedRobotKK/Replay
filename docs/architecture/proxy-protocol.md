# Proxy Protocol Facts

What the client sends through a local gateway and what the proxy must preserve. Sourced from the Claude Code documentation on 2026-09-02 (environment variables, LLM gateway, gateway protocol, prompt caching, sessions pages); re-verify against those pages before changing `buffy serve`.

## Routing

- `ANTHROPIC_BASE_URL` redirects every request Claude Code makes, including `/v1/messages` and `/v1/messages/count_tokens`.
- With `ANTHROPIC_BASE_URL` set and no gateway credential (`ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, or an API key helper), a saved claude.ai subscription login stays the active credential and its limits apply. This is the documented configuration Buffy relies on: the proxy sees subscription traffic without replacing the subscription.
- With a gateway credential set, the credential replaces the subscription for that session. Buffy must never set one on the user's behalf.
- Pointing the base URL at a non-first-party host disables MCP tool search by default; `ENABLE_TOOL_SEARCH=true` restores it when the gateway forwards `tool_reference` blocks unchanged. Buffy's passthrough does, and the README will say so.

## Headers the proxy forwards unchanged

| Header | Why |
|--------|-----|
| `anthropic-version` | Always `2023-06-01`; changing it breaks features |
| `anthropic-beta` | Comma-separated capabilities; forwarded verbatim, never allowlisted |
| `Authorization`, `x-api-key` | The client's credential; forwarded, never stored or logged |
| `x-claude-code-session-id` | Session identity for the ledger and policy pinning |
| `x-claude-code-agent-id`, `x-claude-code-parent-agent-id` | Distinguish sub-agent lanes |
| `anthropic-workspace-id` | Required when the upstream is Claude Platform on AWS |
| Custom headers from `ANTHROPIC_CUSTOM_HEADERS` | User-supplied; opaque to Buffy |

## Headers Buffy removes or adds

- `x-buffy-token`, when the listener token is enabled, is Buffy's own secret and is removed before forwarding.
- `x-buffy-override` is read by the guards and forwarded unchanged; it is harmless to the provider.
- `x-buffy-warning` is the only header Buffy adds to a response, and only when a guard warns without blocking.
- `X-Forwarded-For` is not added.

## Body rules

- The request body is the same the client sends to the provider, `cache_control` markers included. Removing or converting markers bills the conversation uncached on every turn.
- Block-form `system` content is never converted to a string and the `system` array is never reordered.
- Nothing in the body is re-serialized in passthrough mode; bytes in are bytes out.
- With a live policy enabled, the only change is one top-level member spliced before the closing brace of the request object, on requests whose client enabled the provider feature and did not set the member itself. Every byte the client sent stays in place. The transformation is a pure function of the flags and the body, so the same client body always renders to the same provider bytes, and it is logged with the body hashes before and after (PX-10). `BUFFY_NO_POLICY=1` turns every policy off (PX-6).

## Cache TTL

Claude Code requests the one-hour TTL on a subscription within plan usage, and the five-minute TTL on API keys, credits, cloud providers, or when over the plan limit. Both are user-configurable: `promptCacheTtl` (main conversation) and `subagentPromptCacheTtl` (helpers), with environment-variable overrides, from client version 2.1.242. Buffy's TTL replay policies are therefore reachable as a client setting; the proxy itself never touches markers.

## Transcripts

Claude Code writes one JSONL file per session under `~/.claude/projects/<project>/`. The entry format is documented as internal and subject to change between versions; the parser therefore counts every line it cannot interpret and never fails silently. The per-message usage fields the parser reads are observed on client 2.1.258 and are not documented as stable.
