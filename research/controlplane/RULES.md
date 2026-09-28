# Control-plane rules

These rules apply to files under this directory and to notes that point at them. They do not change Replay's measurement code.

1. `queue/<task>.md` means work is waiting. It starts no process and names no owner.
2. `claims/<task>` is created only if absent. The second caller loses. The first file is left as written.
3. The author of a change does not mark it verified. A different owner does.
4. If a surface does not expose usage, write `NOT_MEASURED`. Do not write zero, and do not add a new provenance type. Replay already uses that distinction in `cmd/replay/costgate.go` and `internal/transcript/perf.go`.
5. Public Git may contain task ids, owner tokens, status, commit SHAs, and conclusions. It must not contain prompts, transcripts, credentials, customer data, or raw session logs.
6. Owner is one token (`claude`, `grok`, `openai`, `deepseek`). A new token needs no new code. A new required seat needs a repeated job the existing seats do not do.
7. This lock is one filesystem. Two clones can still both commit. Do not describe it as a distributed lock.
