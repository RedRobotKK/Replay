# Spike 4: the proxy against the real provider

**2026-09-05.** First run of `replay serve` against `api.anthropic.com`. Until
this, every claim about the proxy, the guards, the ledger and the measured tier
rested on a fake upstream, and `docs/SURFACES.md` said so.

## The criterion, and the result

> Does adding the context-editing parameter from a proxy leave Claude Code's
> behavior intact? A ten-turn session completes with the parameter present.

**Passed.** Ten turns recorded, all HTTP 200, the session completed and produced
correct output. `replay serve -context-edit-trigger 20000 -context-edit-keep 6`,
driving Claude Code with `ANTHROPIC_BASE_URL=http://127.0.0.1:4000`.

| | |
|---|---|
| Turns recorded | 10 |
| All completed 200 | yes |
| Policy pinned and applied | `context-edit`, on the admissible request |
| Prompt tokens measured off the wire | 1,816,417 |
| Largest prompt | 204,536 (trigger was 20,000) |
| Credential strings in the ledger | 0 |
| Message content in the ledger | 0 |

## The finding inside the pass

**The provider applied zero context edits and cleared zero tokens.**

The parameter was sent, the session was intact, and the criterion is about
behaviour remaining intact, so the spike passes on its own terms. But the thing
the parameter exists to do was not observed happening. `applied_edits` is 0 and
`cleared_input_tokens` is 0 across all ten turns, on a session whose largest
prompt was ten times the configured trigger.

So the honest status of context editing after this spike is: **accepted without
breaking anything, and not yet shown to do anything.** Those are different
claims and the second one is the one a user would care about. It stays marked
experimental, and the reason is now this measurement rather than an absence of
one.

Possible explanations, none of them established here: the client sets its own
`context_management` and the proxy correctly declines to override it; the beta
header the policy requires was not present on those requests; or the provider's
own conditions for clearing were not met. Distinguishing these needs the
request-side hashes, which the ledger already records, read against a session
where the client's own parameters are known.

## What this settles, and what it does not

**Settled.** Byte-for-byte passthrough works against the real provider on a
real client. The ledger carries no credential and no message content on a
policy-applied session, which is the case that most needed proving. The measured
tier is real: 1.8M prompt tokens captured from provider usage, including an
`ephemeral_1h_input_tokens` cache write of 106,449 tokens on an earlier run.

**Not settled.** Whether context editing does anything. The guards were not
exercised: no cap was configured, nothing was refused, and the circuit breaker
never opened. Retries and provider-error handling were not exercised because the
provider did not fail. One machine, one client, one model.

---

[Evidence](README.md) · [Roadmap](../ROADMAP.md) · [Repository README](../../README.md)
