# Project Replay

**Replay shows you the turn your coding agent's prompt cache broke, what it cost, and what a better context layout would have saved, on sessions you have already paid for.**

It reads the transcripts your agent already writes, reproduces the provider's caching turn by turn, and only then scores alternatives. Later, as a local proxy, it applies the better layout live using only mechanisms the provider itself sanctions, and keeps improving from your own history. Everything runs on your machine. No API calls are spent on analysis. Nothing leaves.

> **Status: v0.1 and v0.2 in development, no release tagged yet.** `replay`, `blame`, `diff`, and `redact` work offline on Claude Code transcripts and have been calibrated against 11 real sessions so far, all from this repository's own development; the 20-session corpus the roadmap requires is still pending. `serve` is a byte-for-byte passthrough proxy that records a derived-data ledger; it has been exercised against a fake provider, not yet against the real one. Follow [`docs/ROADMAP.md`](docs/ROADMAP.md).

[![CI](https://github.com/RedRobotKK/Replay/actions/workflows/ci.yml/badge.svg)](https://github.com/RedRobotKK/Replay/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/RedRobotKK/Replay)](https://goreportcard.com/report/github.com/RedRobotKK/Replay)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

## Why

Coding agents resend the whole conversation on every turn. Three things go wrong quietly:

- **You cannot see what a task cost** until the invoice arrives.
- **Prompt caches break silently.** A timestamp in a system prompt or a reordered tool list can double the bill with no error anywhere.
- **Secrets leave the machine.** API keys and tokens in tool output ride along in the prompt.

Replay addresses each one locally, without changing how the agent works.

## What it will do

| Command | What you get |
|---------|--------------|
| `replay <path>` | Reproduces your sessions' caching, prints how well it matched the provider's own numbers, then scores alternative layouts in tokens saved |
| `replay blame` | Ranks which files and blocks are eating your prompt tokens, per session |
| `replay diff` | Points at the exact turn where the cached prefix diverged and classifies the cause |
| `replay doctor` | What Replay can see on this machine (transcripts, proxy variable, a running proxy, ledger) and the next command to run |
| `replay corpus` | Calibration summary across every session in a directory, as Markdown with no paths or content, for reporting how well Replay understands your sessions |
| `replay serve` | Local proxy: byte-for-byte passthrough that records what the provider charged, so the three commands above run on measured data (policies and guards are implemented; see below) |

Every number is labeled *estimated* (from transcripts) or *measured* (from the wire), with the calibration that justifies it. Release sequence, gates, and what is deliberately deferred: [`docs/ROADMAP.md`](docs/ROADMAP.md). Full requirements: [`docs/requirements.md`](docs/requirements.md).

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/RedRobotKK/Replay/main/install.sh | sh
```

Downloads the released binary for your platform and verifies it against the release checksums.
**An unverifiable download aborts**; pass `--no-verify` to accept one deliberately. The checksums are
fetched from the same origin as the archive, so this defends against corruption, not against a
compromised release: verify the Sigstore signature below if that is your threat model. Falls back to building from source when no release is tagged. Set `REPLAY_BIN_DIR` to
choose where it lands. Read the script first if you would rather not pipe to a shell; it is short.

With Go already installed:

```sh
go install github.com/RedRobotKK/Replay/cmd/replay@latest
```

Or from a clone:

```sh
make build && ./bin/replay doctor
```

## Quick start

No proxy, no configuration, no trust required.

```sh
replay doctor                                        # what is on this machine, and what to run next
replay ~/.claude/projects/<your-project>/
```

Real output from one Claude Code session, the session in which Replay itself was written. It is a
paste from a single run, not a summary: the aggregate across every session measured so far is in
[`docs/evidence/`](docs/evidence/calibration-corpus-2026-09-03.md), which reports 398 of 402 turns
reproduced across 11 sessions and is candid that those sessions are this repository's own.

```text
Tier: estimated (transcripts only)
Calibration: reproduced provider cache reads on 77/78 turns (7 read more than predicted: a sibling request extended the prefix); 1 cache breaks
Assumption: replayed savings assume the agent would have behaved identically under the alternative layout
Rules: anthropic-2026-09-01; user-content fit 0.469 tokens/byte ±64% from 34 turns; system prefix 39k (measured from the first request's cache read)

  policy                                    prompt tokens  cached share  vs as-run  misses  guardrail
  as-run                                           22.87M           97%          -       0
  ttl-5m0s                                         22.87M           91%       +35%       4  none
  ttl-1h0m0s                                       22.87M           97%        +0%       0  none
  context-edit(keep=6,trigger=239k) *              20.98M           79%     +206%       0  re-read rate and failed edits: unknown until a live trial

  turn 32 at 22:08:56 (+2m30s): read 39k of 228k expected, 189k re-billed
    cause: client re-rendered history after the system prefix (no edit visible in transcript)
    where: message 0 (user text)
    evidence: read 38987 tokens, about the size of the system prefix (38547); the message history was re-billed from the first message
```

Add `--dollars` for a list-price column computed from a dated first-party price table; the output names the table date because prices change and other platforms differ.

The calibration line is the point: Replay first proves it can reproduce what the provider charged, and only then says anything about alternatives. Every figure is labeled estimated or measured. Here the 1-hour TTL the client chose reproduces as-run to the token, the 5-minute TTL would have cost 35% more across four idle gaps, and context editing would have cost more, not less, for this session shape. How it works: [`docs/architecture/replay-engine.md`](docs/architecture/replay-engine.md).

### Contributing a calibration corpus

The roadmap gate for the first release is calibration on twenty real sessions. If you have Claude Code transcripts, this produces a report that contains no paths, project names, or content:

```sh
make build
./bin/replay corpus ~/.claude/projects > docs/evidence/calibration-corpus-$(date +%F).md
```

Open it, check that nothing in it identifies your projects, and commit it on a branch. The report also judges calibration per model with the newest sessions on their own, so a provider rule change shows up as "provider behavior changed" rather than as a silent drift in the numbers, and it bounds the minimum cacheable prefix from your usage next to what the rules file says. `replay learn` scores no alternatives for a model reported that way.

### Measured numbers with the proxy

Transcripts do not contain the system prompt, tool definitions, or cache markers, so figures from them are labeled estimated. The proxy sees all of it:

```sh
./bin/replay serve                                  # listens on 127.0.0.1:4000
export ANTHROPIC_BASE_URL=http://127.0.0.1:4000    # in the shell that runs your agent
./bin/replay ~/.replay/ledger/                       # same commands, measured tier
```

What the proxy does and does not do:

- Forwards every request and response byte for byte, including streaming, cache markers, and the `anthropic-beta` and `anthropic-version` headers. By default it never rewrites a body or removes a header. Two opt-in features do, and say so: `--mask` rewrites secrets in the body, and a context-edit policy adds the provider's parameter. Rehydration also drops `Accept-Encoding` so the response can be read.
- Forwards your credential and never stores or logs it. A Claude subscription stays the active credential when no API key is set, as the client's gateway documentation describes.
- Writes one ledger file per session with block kinds, sizes, labels, timings, and usage. No message text, ever. Ledger files live under `~/.replay/ledger` with owner-only permissions.
- Binds loopback only, refuses browser-originated requests, and accepts an optional shared token (`--token` or `REPLAY_TOKEN`, sent as `x-replay-token`).
- Fails open: if anything inside Replay's own bookkeeping fails, the bytes still flow. If the provider is unreachable you get a 502 that says how to bypass Replay.
- Off switch: `REPLAY_DISABLED=1` refuses to start; unsetting `ANTHROPIC_BASE_URL` bypasses it entirely.

Added latency, measured on 2026-09-03 with a 46KB request against a local fake provider on a 4-core Xeon, 300 requests after warm-up: p50 48µs, p99 98µs on top of the round trip. Provider latency is three orders of magnitude larger. The method is in [`docs/evidence/proxy-latency-2026-09-03.md`](docs/evidence/proxy-latency-2026-09-03.md).

While it runs, every response's cache read is checked against the expectation from the previous request, and a break is logged the moment it happens with the tokens re-billed and the likely cause. Because the proxy hashes the tool definitions and system prompt of every request, a break caused by a changed prefix is named with certainty, which a transcript can only infer.

After every turn the proxy also re-scores the session's candidate layouts (5-minute and 1-hour TTLs, context editing at two triggers) with the same simulator `replay replay` uses, from measured usage. This is a dry run: nothing changes on the wire, and the live figures are exactly what `replay replay` prints for the same ledger. `GET /replay/status` returns per-session totals (requests, prompt tokens, cached share, breaks, prefix changes, list cost) and the `what_if` rows, each with a `vs_as_run` delta and how a user would turn it on, as JSON. `GET /replay/metrics` exposes the totals as Prometheus text. Both honor the token when one is set and refuse browser origins. Every ten requests a session gets one log line naming its best candidate or saying none beats what ran.

Guards, all off unless you set them (see `replay serve -h`):

- `--max-session-tokens` and `--max-day-tokens` refuse the *next* request once a cap is reached, never a response in flight. `--max-session-usd` and `--max-day-usd` do the same at list price from the dated price table (a model not in the table counts as free, and the status endpoint shows the same figure). The refusal is a provider-shaped error the agent shows you; send `x-replay-override: <reason>` to proceed once.
- `--error-budget 0.3` refuses a session's next request once that share of its prompt tokens carried error content: failed tools, failed edits, repeated identical calls, overflow notices. It trips before the spend cap because an agent stuck on failures burns money on nothing; sessions under ten thousand prompt tokens are never judged. The same figure is `error_share` on `/replay/status`, and `replay replay` on the ledger names what failed.
- `--loop-warn` and `--loop-block` count how many times in a row the agent has just made the same tool call with the same input, and add a warning header or refuse the request. A repeated command earlier in the session never counts; only the current run does. `x-replay-override` passes a block once.
- `--breaker-failures` opens a circuit after consecutive provider failures and answers locally with `Retry-After` until the cooldown passes, so the agent stops burning retries against a provider that is already saying no.
- `--retries` resends a request up to that many times on rate limit, overload, server error, or connection failure, with doubling jittered backoff from `--retry-base` capped at `--retry-max`, and the provider's `Retry-After` in place of the backoff when it fits under the cap. A retry can only happen before any byte of a response has reached the client, never on a client error, never once a stream has started, and never after the request was sent: a connection that drops after sending may already have been billed, so only a failure to connect is resent. The ledger records the count per request; the log names each attempt and its reason.

One live policy, experimental and off by default:

- `--context-edit-trigger <tokens>` asks the provider to clear old tool results server-side once the prompt passes the trigger, keeping the last `--context-edit-keep` (default 6). Replay adds the provider's `context_management` parameter after the client's own bytes, which stay byte-identical, and only on requests whose client already enabled the context-management beta and set no such parameter itself. The decision is made at a session's first request and pinned. Each edit is logged with body hashes before and after, never content; the ledger records which requests carried it and the provider's applied edits and cleared tokens, so `replay replay` on the ledger shows what it did. `--policy-file ~/.replay/policy.json` applies the context-edit candidate `replay learn` selected instead, read at each session's first request; an explicit trigger flag wins over the file. Either way the decision is pinned on disk, so a session keeps its first decision through a rewritten file or a restarted proxy. A learned policy runs as a bounded trial: `--trial-share 0.5` applies it to half of new sessions by a stable hash of the session id and holds the rest out as controls, and `--guardrail-reread 0.5` reverts it for new sessions once `--revert-after` treated sessions (default two) show a re-read rate after the provider's clears at or above that share. The revert is persisted and survives restarts; a newer `replay learn` result lifts it. An explicit trigger flag is your own decision and is never split or reverted. `REPLAY_NO_POLICY=1` forces every policy off. The parameter shape follows the provider's documentation and has not yet been exercised against the real provider (roadmap spike 4); the `what_if` rows tell you what it should save before you turn it on, and the `re_reads` figures (file reads that repeated a path already in context, before and after the provider's first clear) tell you whether the agent is paying the savings back by re-reading.
- `--hold-siblings 10s` is the `hold-parallel-siblings` policy. The provider's cache entry becomes readable only once the first response that writes it begins streaming, so sub-agents started together with the same tools and system prompt all pay the write price. With the flag on, a request whose prefix is already in flight and not yet cached waits for that first response to begin, then goes out and reads the entry; the value bounds the wait, a request with another prefix never waits, and a prefix that had a response begin within the short cache lifetime holds nobody. The wait is on the ledger record as `held_ms`, in the log line, and on status and metrics, so the cost of the policy is visible next to its saving in the following record's cache read. Off by default.

### Learn from your own sessions

```sh
replay learn ~/.claude/projects/* ~/.replay/ledger
```

Re-scores the policy catalog (both cache TTLs and context editing at four triggers) over every session with the replay simulator, then selects one with rules built for a corpus of tens of sessions: a minimum number of sessions that actually carry evidence, a margin above noise, a repeat on held-out sessions chosen by a stable hash, and ties to the simpler policy judged on the paired per-session difference (ADR-0006). The verdicts and the selection go to `~/.replay/policy.json` in a documented format ([`docs/architecture/policy-file.md`](docs/architecture/policy-file.md)). On a small corpus the honest answer is "none", and that is what it says. Reads files only; never the network.

### Get told what to change

```sh
replay advise ~/.claude/projects/* ~/.replay/ledger
```

Turns the largest token sources across every session into suggestions with a predicted saving: tool inputs that dominate prompts (long heredocs), tool results that should be truncated, files read again and again, first-turn instruction files that every request re-carries, tool definitions a session never calls (visible in the ledger), and cache breaks to look at with `replay diff`. Each prediction assumes the target is halved and is stated as a share of prompt tokens first, the scale-free metric, then as tokens across the corpus. Suggestions are tracked to closure: pending until the newest sessions show the target shrinking, then applied, then verified or not verified against the prediction. Written to `~/.replay/advice.json`.

### Secret masking (experimental)

```sh
replay serve --mask [--project .] [--mask-patterns ~/.replay/patterns.txt] [--rehydrate-scope name=dest,dest]
```

Replaces secrets in outbound request bodies with placeholders before anything leaves the machine, and restores them in responses where the scope allows. Only the matched bytes inside JSON string values change; every other byte is forwarded as sent, and thinking blocks and signatures are never read or changed in either direction. The same secret always maps to the same placeholder, an HMAC under the vault key, so the cached prefix stays stable across turns and sessions and the secret the model writes back is masked again on the next request. The mapping lives in `~/.replay/vault`, encrypted with AES-256-GCM under a key file that sits beside it with owner-only permissions. **That file is the whole boundary**: anyone who can read the directory can decrypt the vault. Masking turns secrets that were transient in flight into secrets at rest on your machine, and nothing is evicted, so leave `--mask` off unless you want that trade, and survives restarts.

The pattern set is named, not complete: Anthropic, OpenAI, AWS access key ids, GitHub, GitLab, Slack, Google API keys, Stripe, private key blocks, JWTs, bearer tokens, and credentials embedded in URLs, plus your own patterns from a file with one `name<TAB>regexp` per line. On the repository's labeled corpus the set scores precision 1.00 and recall 1.00; that is a statement about the corpus, not about your secrets. `--mask-entropy` adds a heuristic for credentials no pattern names: a run of 32 or more base64 characters that mixes cases and digits, changes character class often enough not to be an identifier, has no path-shaped segment, and has high entropy. It is reported as pattern `entropy` and scored on its own corpus (precision 1.00, recall 1.00 on 10 positives and 15 negatives covering hashes, uuids, identifiers, paths, URLs, and timestamps). Off by default, because a guess by shape can mask a value that was not a secret, and that value then reaches the model as a placeholder.

Rehydration is scoped (ADR-0004), because content the agent reads can tell the model to write a placeholder into a command. By default a placeholder is restored in assistant text and in the input of a file-edit tool (`Edit`, `Write`, `MultiEdit`, `NotebookEdit`, and the common editor tool names) whose path is under `--project` (default: the directory `replay serve` runs in). Shell tools, network tools, tools Replay does not recognize, edits outside the project, and edits without a path keep the placeholder. `--rehydrate-scope` changes that per pattern: `--rehydrate-scope github-token=none` never restores GitHub tokens, `--rehydrate-scope url-credential=text,edit,tool:Bash` lets URL credentials into shell commands, and `*` sets the default. `--rehydrate=false` masks without restoring, to evaluate coverage. Every response's restored and denied placeholders are counted by destination in the log, the ledger record, `/replay/status`, and `/replay/metrics`, never with a value or a path.

What rehydration costs and where it stops: a tool call's input is delivered when its block ends rather than as it streams, because the path that decides the scope can arrive after the placeholder; a text delta ending in bytes that could begin a placeholder waits for the next delta. Responses are requested uncompressed, and a response the provider compresses anyway, or one over the proxy's size limit, is forwarded untouched with a log line. A placeholder the model spells with JSON escapes is not recognized and stays a placeholder. A file edit receives a secret only when the tool's own path field is present and every path-like field in its input is under the project, so a decoy in-project path beside the real target does not open the scope; symbolic links are resolved along the part of a path that exists, and a link the edit is about to create cannot be. `REPLAY_NO_POLICY=1` turns masking and rehydration off with the policies.

One client caveat from the gateway docs: with a non-first-party base URL, Claude Code disables MCP tool search unless `ENABLE_TOOL_SEARCH=true` is set. Replay forwards `tool_reference` blocks unchanged, so setting it is safe. Details: [`docs/architecture/proxy-protocol.md`](docs/architecture/proxy-protocol.md).

## Install

No release is tagged yet. When one is, every release carries `checksums.txt` signed with Sigstore keyless signing, built only by CI from the tag. Verify before running:

```sh
cosign verify-blob --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/RedRobotKK/Replay/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Until then, build from source as below.

## Development

Requires Go 1.24+, [golangci-lint](https://golangci-lint.run/) v2, and Node 22+ (Markdown lint only).

```sh
make ci       # lint, test, build, docs-lint: exactly what CI runs
make build    # ./bin/replay
make help     # all targets
```

## Documentation

- [`docs/ROADMAP.md`](docs/ROADMAP.md): what ships when, and why
- [`docs/adr/`](docs/adr/): architecture decision records
- [`docs/architecture/`](docs/architecture/): system design
- [`docs/guide/`](docs/guide/): getting started, every command, and troubleshooting
- [`docs/evidence/`](docs/evidence/): the measurements behind the claims above
- [`docs/requirements.md`](docs/requirements.md): current requirements, with build status
- [`docs/maintainers.md`](docs/maintainers.md): how this repository is run

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) first. Bugs and features go through the issue templates. Security reports go through [`SECURITY.md`](SECURITY.md), never a public issue.

## Who maintains this

Replay is built and maintained by **Daniel Saito** at [Red Robot K.K.](https://redrobot.jp), a
studio in Tokyo working on production AI, data platforms and security.

The quickest way to reach me about Replay is an [issue](https://github.com/RedRobotKK/Replay/issues)
or a [discussion](https://github.com/RedRobotKK/Replay/discussions). For anything else,
[redrobot.jp](https://redrobot.jp) or [LinkedIn](https://www.linkedin.com/in/danielsaito/).

## License

Open source under the [Apache License 2.0](LICENSE). See [`NOTICE`](NOTICE) for attribution. The decision is recorded in [ADR-0005](docs/adr/0005-apache-2-license.md).

## What masking does not catch

`--mask` is useful and it is not a guarantee. Three limits, stated plainly because the failure mode
is silent and the consequence is a leaked credential.

**Hex and lowercase secrets are not caught by shape.** A 32-character Twilio token and a
40-character git SHA are the same string to any entropy test, and the SHA actually scores *lower*
(3.63 bits against 3.91). Masking every hex run would corrupt the diffs, checksums and tool output a
coding agent reads all day, so Replay does not. It catches these by **context** instead: a name that
says the value is a credential, sitting next to the value, as in `TWILIO_AUTH_TOKEN=…` or
`"apiKey": "…"`. **A bare hex secret with no name beside it passes through.**

**If the vault cannot be written, the request is forwarded unmasked.** Replay fails open everywhere,
and masking is not an exception: a full disk or an unwritable vault directory means secrets Replay
had already matched go to the provider in cleartext. It logs `MASKING FAILED … forwarded UNMASKED`,
but in a backgrounded `serve` nobody reads that. **If masking must not fail silently for you, do not
background the proxy.**

**The vault is only as private as its directory.** The key file sits beside the ciphertext. Anyone
who can read that directory can decrypt it, and nothing is ever evicted, so turning masking on
converts secrets that were transient in flight into secrets at rest on your machine.

The published precision and recall figures are a statement about the corpus in
`internal/masking/testdata/`, not about your secrets. **Treat masking as a second layer under
not-sending-secrets, never as the first.**
