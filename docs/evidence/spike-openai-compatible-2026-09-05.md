# Spike: what happens when OpenAI-shaped traffic reaches the proxy

**2026-09-05.** Run against a stub upstream speaking OpenAI's
`/v1/chat/completions` shape. No credentials, no live provider.

## The question

`architecture/multi-provider.md` sequences a second provider third, after
normalising usage and dating the rules. Before designing that work, the spike
asked what the proxy does **today** with a request it was never built for.
Cursor's own config on this machine uses `provider: "openai-compatible"`, so
this is a real path a user can take now.

## Result: it works, and that is the problem

```
POST /v1/chat/completions  ->  HTTP 200, correct body forwarded
log: status=200 ms=1 session= model= usage=none
ledger: no record written
```

The request is proxied byte for byte and the client sees a correct answer. What
does not happen is everything else.

`isMessages(r.URL.Path)` gates the masker (`server.go:412`), the request summary
and every guard (`:418`), rehydration (`:435`) and the ledger record (`:440`).
On any other path all of them are skipped.

**Tested directly, with a spend cap of one token:**

| | |
|---|---|
| `--max-session-tokens 1`, three requests | **all three returned 200** |
| ledger records written | **none** |
| `--loop-block 2` | never fired |
| secret masking | never ran |

A cap of one token did not refuse a second request. Nothing was refused, nothing
was recorded, and the proxy reported itself healthy throughout.

## Why this is a finding and not a gap

A user who points Cursor at `replay serve` gets a working proxy and no
protection, with no way to discover the difference. They configured a spend cap;
they believe it is on. If they enabled `--mask`, they believe secrets are being
redacted and none are.

This is the same shape as two defects already fixed here. The day cap that reset
on restart was "the protection silently disappearing for the exact threat it
exists to stop". `CapNotEnforced` exists because a cap that cannot be applied to
some traffic is worse than no cap, "because the user believes it". Silent
pass-through is that failure with a wider blast radius, because it covers the
masker too.

## What shipped from the spike

Not the OpenAI request path. That is still step three and still needs design.
What shipped is the honesty fix, which was small:

- `noteUnparsed` warns once per path, naming the path and every protection that
  is inert for it. Once per path rather than per request, because a line on
  every request is noise an operator learns to scroll past.
- `replay_unparsed_requests_total` on `/replay/metrics`, so the condition is
  watchable rather than only greppable.

```
NOT PARSED /v1/chat/completions: Replay forwards this path unchanged and cannot
read it. No ledger record, no spend cap, no error budget, no loop detection and
no secret masking apply to it. Only /v1/messages is understood by this build.
```

## What a real OpenAI path would still have to answer

The counting convention is already handled: `internal/usage.FromInclusive` does
the subtraction, because `prompt_tokens` includes `cached_tokens` where
Anthropic's `input_tokens` excludes the cache. The open questions are the ones
this spike cannot answer without live traffic:

1. Whether the response carries enough to reach the measured tier at all.
   `prompt_tokens_details.cached_tokens` gives reads; nothing observed here
   distinguishes a cache write, which the explicit-breakpoint model assumes.
2. What the cache economics actually are. Reported figures say automatic
   caching, no separate write charge, reads at about half price. If the write
   penalty is genuinely zero the break-even trim inequality goes negative and
   the advice inverts. **That is a claim to check, not a number to hardcode**,
   which is what the rules document's `documented` versus `observed` split is
   for.
3. Whether streaming responses report usage the same way.

## Status

**Unverified against any live OpenAI-compatible provider.** Every figure above
about the proxy's behaviour is measured against a stub; every figure about
provider pricing is quoted from documentation and has not been checked by
replaying real requests. See `SURFACES.md`.

---

[Evidence](README.md) · [Architecture](../architecture/multi-provider.md)
