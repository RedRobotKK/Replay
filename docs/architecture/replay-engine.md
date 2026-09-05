# Replay Engine

How Replay reproduces the provider's prompt caching from a transcript, decides whether it may score alternatives, and keeps estimated figures apart from measured ones. Read this before changing anything under `internal/analysis` or `internal/cachemodel`.

## The invariant that makes calibration possible

On every request the provider reports four numbers: uncached input tokens, cache-creation tokens, cache-read tokens, and output tokens. The prompt size is the sum of the first three. The client places its last cache breakpoint just before a small tail, which is the uncached input on that request.

If nothing in the prefix changed and the cache is still warm, the next request's cache read equals the previous request's prompt size minus its uncached tail. That is the whole calibration: for every turn, compare the reported read with that expectation.

| Outcome | Meaning |
|---------|---------|
| reproduced | The read equals the expectation. The model of the session is right for this turn. |
| exceeded | The read is larger than expected. Another request sharing the prefix (a sub-agent, a background classifier) wrote more of it. Counts as a match. |
| broken | The read is smaller than expected. The prefix diverged or the cache expired. |

No tokenizer is involved. The calibration line on every report is the share of turns that were reproduced or exceeded, and alternatives are scored only above 95 percent.

## Reconstructing requests from a transcript

Claude Code writes one JSONL line per content block. A request produces several assistant lines sharing a request id, each carrying the same usage. The request's input is the parent chain from its first output line back to the root. The parser rebuilds that chain, merges assistant runs into one message, resolves tool results to the tool call that produced them, and groups requests into lanes (the main loop, and one lane per sub-agent conversation). Decoded messages are memoized because every request's context is a prefix of the next one's.

Lines that are not conversation content (hook summaries, queue operations, attachments) are skipped and counted; a format change shows up as a skipped count, never as a silent gap.

## Measured tier: ledger sessions

When the session comes from the proxy's ledger, every request's context begins with a synthetic system message holding the system prompt size and the tool definitions size, so nothing ahead of the first message is unseen and a change in that prefix shows up as a divergence at position zero. Message identity in the ledger is positional shape (role, block count, total bytes), which is exactly the history-edit signal the diff looks for. Assistant output structure and usage are recorded from the response itself.

## What the transcript does not contain

Three things are on the wire and not in the file: the system prompt, the tool definitions, and content the client injects (attachments, reminders). Two measurements bound them:

- The first request's cache read, when non-zero, is exactly the shared prefix that was already cached from an earlier session: the system prompt and tools. It is reported as measured.
- The first request's prompt size minus that read minus the visible first message, if positive, is client-injected content. It is reported as estimated.

## Breaks: where and why

For each broken turn the classifier applies causes in order of specificity: gap longer than the write TTL; model changed; effort changed; an earlier message differs between the two requests' histories; nothing was read (the divergence is ahead of the first message); the read is about the size of the system prefix (the client re-rendered history after the system prefix, which is what a client restart looks like); otherwise the divergence is inside the history and its position is estimated from the byte-to-token fit.

The deficit, expected minus actual, is what was re-written instead of read.

## Estimated figures and the fit

Assistant output never needs estimating: the provider reports output tokens, and thinking tokens separately, and replays exactly those when the message is sent back. Only user-side content (tool results, user text) needs a byte-to-token conversion.

The fit pools every reproduced turn whose new user content is at least 512 bytes, so fixed per-message overhead does not dominate, and reports a byte-weighted relative spread as its uncertainty. Attribution distributes each turn's reported new-content tokens across that turn's blocks in proportion to bytes, so per-turn sums match provider usage exactly; only the split within a turn is estimated. Figures carry the uncertainty of their estimated part only.

## Replaying a policy

The simulator keeps one cached prefix with a last-touch time and a TTL. It is seeded with whatever the first request found already cached, so a policy is never charged for a cold start the real session did not pay. On each turn it decides the read from the TTL and the cached prefix, then follows the client's observed behavior where the invariant did not hold: on broken turns within the TTL and on exceeded turns, the simulated read is the observed read. That way replaying the as-run TTL reproduces as-run exactly, and the only difference between as-run and a policy is the policy.

| Policy | What changes | Reachable live |
|--------|--------------|----------------|
| ttl-5m, ttl-1h | Expiry and write multiplier | Yes, as a client setting: Claude Code exposes `promptCacheTtl`; the proxy never changes client markers |
| context-edit | Old tool results are cleared in bulk when the prompt passes a trigger; the cache is invalidated from the earliest cleared block and the prompt shrinks afterwards | Yes: a request parameter the proxy can set |

Context-editing triggers are chosen relative to the session's largest prompt so a policy is never scored at a threshold the session could not reach. Effective tokens price uncached input at 1x, writes at the TTL multiplier, and reads at the read multiplier; they are a relative measure for comparing layouts, not a bill.

Every replay table states the assumption that the agent would have behaved identically under the alternative layout. Behavior effects, such as re-reads after a clear, are only measurable in a live trial and are listed in the guardrail column as unknown until then.

## Redaction and fixtures

`replay redact` rewrites a transcript so that structure, block kinds, byte lengths, usage, timestamps, ids, and tool names survive and nothing readable does: text becomes filler of the same length, paths become hashes that keep their extension, and machine-specific fields are replaced. Redacting a redacted file is a fixed point, and the analysis of a redacted session is identical to the original. The repository's test fixture is a redacted real session.

## Staleness: when the rules stop fitting

The rules are a dated description of the provider, and the provider changes. Calibration is therefore also judged per model across sessions, with the newest five sessions on their own (`analysis.ModelCalibrations`). When a model's earlier sessions calibrated and three of its newest five each fall below the threshold, `replay corpus` reports that the provider's behavior changed and `replay learn` scores no alternatives for that model until the rules are updated. One bad session is not a rule change, and a model whose sessions never calibrated is left to the per-session gate.

Of the rules, one can be bounded from usage alone: the minimum cacheable prefix lies above the largest prompt that saw no cache activity and at or below the smallest cached prefix, and both reports print those bounds next to the rules file's value. The lookback window and the TTLs cannot be told from usage and are not refit.

## Updating the rules

The rules live in `internal/cachemodel/anthropic.go` as named constants and one model table, with `RulesVersion` and `PriceTableVersion` printed on every report. To update them: change the constant or table row, move the version date, cite the provider document the value comes from in the commit message, and run `replay corpus` on a recent corpus to show calibration back above the threshold. A rules change is a user-visible change and gets a changelog entry.

---

[Architecture](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
