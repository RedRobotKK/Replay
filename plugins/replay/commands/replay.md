---
description: Read this machine's agent transcripts with Replay Doctor and report what the prompt cache re-billed, with the turn that did it.
---

Run Replay Doctor on this machine and report, in this order:

1. `replay doctor`, and say what it can see. If `replay` is missing, tell me the install line (`curl -fsSL https://replay.doctor/replay.sh | less`, then `| sh`) and stop.
2. `replay cost ~/.claude/projects/`. Quote the Calibration line first. If the match rate is low, say so and stop. Then the total, the median task, the p90, and the avoidable line with its date.
3. Find the most recently modified session transcript under `~/.claude/projects/` (or `$CLAUDE_CONFIG_DIR/projects/` if set) and run `replay diff` on it. Quote every break line: turn, cause, tokens.
4. Say whether the dollars apply to me (metered API) or are list price for someone else (subscription seat), and that the tokens are mine either way.

Keep every figure's tier, population and date. Do not forecast and do not use the word "save". If the tool refuses something, quote the refusal.

$ARGUMENTS
