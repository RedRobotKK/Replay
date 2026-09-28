---
agent: claude
task_id: replay-agent-optimization
experiment_id: verifier-coverage
run_id: 2026-09-28-verifier-coverage-baseline
turn_id: 2026-09-28-verifier-coverage-baseline
started_at: 2026-09-28T00:00:00Z
completed_at: 2026-09-28T00:00:00Z
status: COMPLETE
base_commit: 71e6fe1cdd08f595f4bc208ec991fefdb16c892c
head_commit: ed4b35f
branch: feat/perf-contract
provider: anthropic
surface: claude-code-transcript

files_changed:
  - experiment/claude-code/verifier-coverage/baseline-2026-09-28.md
  - research/agents/claude/2026-09-28-verifier-coverage-baseline.md

tests_run:
  - "replay cost --json : exit 0, 721 tasks priced, 0 unpriced, 20 unreadable"
  - "replay advise -out <scratch> ~/.claude/projects : exit 0, 256 suggestions"
  - "no unit tests were added or run this turn; this is a measurement turn, not a code change"

delegation:
  - "none. No sub-agents were launched this turn, so there are no child runs to account for."

usage_accounting:
  - "DIRECT: this turn's own token usage is NOT_MEASURED. The session transcript is still open and Replay reads closed transcripts."
  - "CHILD: none launched."
  - "NOT_MEASURED: latency and throughput for this turn. No boundary was captured."

findings:
  - "OBSERVED: verifier coverage on this corpus is 7.8%. 20 of 256 suggestions are verifiable; 236 are advice only."
  - "OBSERVED: one kind, hot-file, is 235 of 256 suggestions (91.8%), and advisor.track() returns AdviceOnly for it by construction. Coverage is bounded by the advisor's kind distribution, not by the verifier."
  - "OBSERVED: all 20 verifiable suggestions are pending. Zero applied, zero verified, zero not-verified. The loop has never recorded an intervention on this corpus."
  - "DERIVED: corpus spend $18,353.73 across 721 tasks, of which $710.15 (3.87%) is re-billed, over 122,349,718 re-billed tokens."
  - "ESTIMATED: the $1,064.13 predicted total is a counterfactual. 252 of 256 suggestions carry estimated:true, meaning the token figure came from the byte-to-token fit rather than provider-reported usage."
  - "NOT_MEASURED: whether any suggestion would help if applied. Nothing was applied."
  - "NOT_MEASURED: whether hot-file advice is useful despite being unverifiable. Unverifiable is not the same as wrong."

decisions:
  - "Wrote the advice file to a scratch path rather than ~/.replay/advice.json, so the operator's existing applied and dismissed state was not overwritten."
  - "Committed aggregate measurements only. Suggestion targets are filenames from the operator's machine and are not reproduced in the artifact."
  - "Did not assert a relationship between verifiable predicted spend ($710.70) and corpus re-billed spend ($710.15). The two are computed by different paths; the $0.55 gap is recorded for a later run to test, not claimed."

unresolved:
  - "Can hot-file be made verifiable, or should the advisor's mix shift toward kinds that can be? Both are product decisions and now have a number attached."
  - "Is the advice correct? Coverage measures eligibility for verification, not quality."
  - "This turn's own token usage is unmeasured because the session transcript is open. Whether a live session can be measured is an open engineering question."

next_action:
  - "Pick one verifiable suggestion, apply it, and run the loop end to end. That would be the first recorded intervention on this corpus."
  - "Decide whether hot-file's unverifiability is a defect to fix or a property to document."
---

# Baseline: verifier coverage of Replay's own advice

## What was measured

Replay was run against the operator's Claude Code transcript tree, read-only,
using a binary built from the branch under development. The question was
deliberately narrow and asked before any optimization: of the advice Replay
produces, how much is eligible for its own verification loop?

## Result

**7.8%.** Twenty of 256 suggestions can be verified. The other 236 are
`advice only`, meaning their effect is not something Replay can observe.

The cause is not the verifier. One kind, `hot-file`, accounts for 235 of the
256 suggestions, and `advisor.track()` returns `AdviceOnly` for that kind by
construction. The loop's reach is set by what the advisor finds, not by how the
verifier decides.

## Second result

All twenty verifiable suggestions are `pending`. On this corpus, Replay has
produced findings and recommendations and has never recorded an intervention.
The second half of the loop is implemented and unexercised.

## What this does not show

Nothing was applied, so nothing is claimed about whether the advice works. The
predicted $1,064.13 is a counterfactual computed from a byte-to-token fit on 252
of 256 suggestions, and is not a measured saving. Whether `hot-file` advice is
useful despite being unverifiable is unmeasured; being unobservable is not the
same as being wrong.

## Scope note

Figures come from one operator's corpus on one date and are not a claim about
any other workload.
