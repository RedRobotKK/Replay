# Every surface Replay touches

**Built 2026-09-04 by enumerating the code, then checking the claims against a running proxy.**
This exists so nobody has to take the README's word for what a tool in their request path does.

Each row says how it was established. **Verified** means someone ran it and looked. **Read** means it
was traced in source but not exercised. **Unknown** means exactly that, and those rows are the point
of the document.

---

## 1. Filesystem

| Path | Direction | What | Status |
|---|---|---|---|
| `$CLAUDE_CONFIG_DIR/projects/*/*.jsonl`, else `~/.claude/projects/…` | read | Agent transcripts. Never modified | **Verified** |
| `~/.replay/ledger/<session>.jsonl` | write | **Message text is genuinely never written** and that was verified against a request stuffed with secrets. But "block kinds, sizes, timings, usage" understated it: records also carry the request `path`, `session_id` and `agent_id` from client headers, the provider's `request_id`, `model`, `effort`, **and tool names verbatim** — `SanitizeLabel` runs on read, not on write. `0600` **at creation only** | **Verified**, and the description corrected |
| `~/.replay/ledger/.label-key` | write | HMAC key for path labels | Read |
| `~/.replay/vault/`, `.vault-key` | write | Only with `--mask`. AES-256-GCM, key file alongside | **Verified**: a reviewer decrypted it in five lines of Python. The key file is the whole boundary |
| `~/.replay/.pins`, `.revert` | write | Per-session policy decisions, persisted so a restart cannot change a session's mind | Read |
| `~/.replay/policy.json` | **read and write** | `learn` writes it; **the proxy reads it at each new session's first request** (`server.go:763`), so anything that can write this file changes the parameters sent to the provider. Listed as write-only in the first version | Read |
| `$CLAUDE_CONFIG_DIR/settings.json`, else `~/.claude/settings.json` | **read, and write with `advise --apply --yes`** | The one configuration file Replay touches, and only when you pass the flag. It reads the current `promptCacheTtl`, and `--yes` writes that single key back after copying the file to a timestamped `.bak-…` sibling; a file that is not valid JSON is refused untouched (`cmd/replay/apply.go`). **Missing from the first version of this table**, which is why [`WHAT-YOU-GET.md`](WHAT-YOU-GET.md) claimed a boundary wider than the code holds | Read |
| `~/.replay/advice.json` | write | `advise` output. **Contains raw tool names and file base names** taken from your transcripts | Read |
| `~/.replay/vault/vault.tmp` | write | Fixed-path temp file, rewritten per newly-seen secret. The §3b claim of "no predictable-path temp file" was true of `os.TempDir` and wrong as a conclusion | Read |
| `$GOMODCACHE`, `$GOCACHE` | write | **Only via the installer's `go install` fallback**, which is the only path available today. Hundreds of MB | Read |
| `${XDG_CONFIG_HOME:-~/.config}/replay/corpus-consent.toml` | write | Only from `install.sh --corpus-opt-in`. Sends nothing | **Verified** |
| `/usr/local/bin/replay` or `~/.local/bin/replay` | write | The binary, at install. Since 2026-09-05 the installer runs `replay version` before reporting success, so a binary that lands but cannot execute fails the install instead of being announced as one | **Verified** end to end |

**Adjacent directories that exist on a normal machine and are NOT Replay's business.** Both were
found by scanning a real install, and confusing them is the likely support question:

- `~/.local/share/claude` — the Claude Code binary and `versions/`. Not transcripts.
- `~/Library/Application Support/Claude` — the desktop app's Electron profile, 15GB on the machine
  scanned. **A different product.** Replay never reads it.
- `~/.claude/history.jsonl` — prompt history, a `{"display":…}` record, **not a session**. The glob
  stays scoped to `projects/<project>` so the parser is never handed it.

> **Owner-only permissions are POSIX-only.** Everything above created `0600` is created `0600` on
> macOS and Linux. Windows has no POSIX permission model, and Go's `Chmod` there toggles only the
> read-only bit, so the same files report `0666`. The README says "owner-only" without qualifying
> the platform; on Windows that protection is not enforced by the filesystem and you should treat
> `~/.replay` as readable by any process running as you.

## 2. Network

> **This page has now been wrong twice, in the same section, about the same claim.** Both corrections
> came from re-checking rather than re-reading. Treat it as a working document, not an assurance.

**Outbound: the provider you configured, unless the environment says otherwise — and, behind one
explicit flag, a price database.**

**`replay rules --check-prices` fetches `raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json`.**
Added 2026-09-05. It is the only outbound request the binary makes that is not the operator's own
traffic to the operator's own provider, it happens only when that flag is typed, and it sends
nothing: a plain GET whose response is compared against the compiled price table. Nothing is
installed from it.

Recorded here on the day it shipped, because this section has been wrong twice about exactly this
claim and the third time would have been the author's own commit twenty minutes after making the
claim stronger.

**`HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY` silently redirect every upstream request.** The transport
is built with `Proxy: http.ProxyFromEnvironment` (`internal/proxy/server.go:170`), which reads all six
spellings of those variables. **No Replay flag mentions this and nothing in the code or docs did until
now.** An independent reviewer demonstrated it: with `HTTPS_PROXY` set to an unreachable host, a
request to `api.anthropic.com` failed at `proxyconnect tcp` and never reached the provider.

**Two consequences worth stating plainly.** For an `https://` upstream the intermediary sees CONNECT
metadata only. But **if `REPLAY_UPSTREAM` is `http://`, the full request and its credential headers go
in plaintext to whatever `HTTP_PROXY` names.** `doctor` inherits the same behaviour through
`http.DefaultClient`.

This is standard Go behaviour and arguably correct — a corporate proxy is exactly how many people
reach a provider at all. **It is listed here because "the outbound host is the one you configured"
was not true**, and a document whose purpose is completeness has to say so.

**A correction to an earlier version of this page**, found by re-checking rather than by reading it
back: the claim "exactly one host" was wrong. `replay doctor` issues a `GET` to
`$ANTHROPIC_BASE_URL/replay/healthz` to find out whether a Replay proxy is already running
(`cmd/replay/doctor.go:114`). That is normally loopback, and it sends no credential, reads at most
64 bytes and times out. **But the host comes from an environment variable**, so if you have pointed
`ANTHROPIC_BASE_URL` at a remote gateway, `doctor` will probe that remote host. Small, and it was
not on the map.

| Direction | Endpoint | When | Status |
|---|---|---|---|
| out | the configured provider | every proxied request, byte for byte | **Verified** against a fake upstream |
| out | `api.github.com`, `github.com` | **installer only**, never the binary | **Verified** |
| out | corpus endpoint | **None. There is no such request, and no flag that could make one.** `corpus` takes directories and defines zero flags, so `replay corpus --submit` exits with `flag provided but not defined: -submit`. The submission path in ADR-0007 and ADR-0008 is a design, not a build | **Verified absent** |
| in | `127.0.0.1:4000` `/` | the proxy itself. Loopback enforced at construction | **Verified**: a non-loopback `-listen` refuses to start |
| in | `/replay/status` | JSON per-session totals. `Origin` and `Sec-Fetch-Mode` refused | **Verified** |
| in | `/replay/metrics` | Prometheus text, aggregate only | Read |
| in | `/replay/healthz` | **no origin check, no token check** | **Verified as a gap** |
| out | `$ANTHROPIC_BASE_URL/replay/healthz` | `doctor` only, probing for a running proxy. No credential, 64-byte read, timeout | **Verified**, and missing from the first version of this page |
| out | the configured provider, `/v1/messages` and `/v1/messages/count_tokens` | **`probe --execute` only.** The one command that ORIGINATES traffic rather than forwarding it: synthetic, cache-defeating, billable, on your own key. It refuses to run without `--execute`, prints the plan first, and asks for confirmation unless `--yes`. `count_tokens` is unbilled; `/v1/messages` is not | **Verified**, and missing until 2026-09-06 — see the note below |

**Known gaps here, all recorded in the security review:** `/replay/status` and `/replay/metrics` are
**unauthenticated unless `--token` is set**, so any local process can read model names, token counts
and per-session list-price dollars. There is **no `Host` header validation**; the anti-rebinding
defence rests on `Origin` being present. And `/replay/healthz` answers cross-origin, so a web page
can fingerprint that Replay is running.

### Provider surface, and what this build does not read

| Surface | Status |
|---|---|
| `POST …/v1/messages` | **Verified** end to end against the real provider (spike 4). Parsed, guarded, masked, ledgered, policy applied |
| `POST …/v1/chat/completions` | **Read, guarded and ledgered since 2026-09-05.** Usage is converted out of inclusive counting, the raw payload is kept, and the spend cap, error budget and loop detector all apply. **NOT masked**: `--mask` walks the Messages body shape only, and the proxy warns once per path (`NOT MASKED`) and counts `replay_unmasked_requests_total`. **No policy applied**, deliberately: this family caches automatically, so there is no breakpoint to place and no TTL to choose, and ADR-0003 admits only a parameter the client left unset. **Streaming works**: OpenAI SSE has its own parser, and because this family sends no usage on a stream unless `stream_options.include_usage` is set, Replay sets it when the client did not (ADR-0003 kind one, a parameter the client left unset; a client that set it keeps its own value). **Verified against a stub, not against any live OpenAI-compatible provider** |
| **Any other POST path**, e.g. `/v1/responses` | **Forwarded unchanged and NOT read.** No ledger record, no guard, no masking. The proxy warns once per path (`NOT PARSED`) and counts `replay_unparsed_requests_total` |

**This surface is expected to change.** The chat/completions row is verified
against a stub only. What a real provider reports, whether a cache write is even
distinguishable in that shape, and whether streaming carries usage the same way
are all open. Until each is measured, the warnings are the contract: Replay says
what it cannot see rather than letting a running proxy imply protection.

**Correction, 2026-09-06.** This page and the README both said the binary makes no network request
except the proxy and `rules --check-prices`. That stopped being true when `replay probe` was added:
`internal/probe/run.go` POSTs to `/v1/messages/count_tokens` and `/v1/messages` on the operator's own
credential, and spends their money. The command was built deliberately and its own tests say so in
plain words — but neither this inventory nor the README was updated, so the claim drifted from true
to false without anyone editing it. The row above is the fix.

It is still an opt-in surface in the same sense `rules --check-prices` is: nothing happens without a
typed `--execute`, the plan is printed before anything is sent, and a confirmation is required unless
`--yes` is passed. That is the honest version of the claim, and it is what the README says now.

## 3. Environment

**A second correction: the first version of this section conflated the binary and the installer.**

**Read by the binary:** `ANTHROPIC_BASE_URL`, `REPLAY_UPSTREAM`, `REPLAY_DISABLED`, `REPLAY_TOKEN`,
`REPLAY_NO_POLICY`, `CLAUDE_CONFIG_DIR`, **`HOME`** (via `os.UserHomeDir`, and it decides where the
ledger, the vault and both key files land), and **`HTTPS_PROXY` / `HTTP_PROXY` / `NO_PROXY`** through
`http.ProxyFromEnvironment`.

**Read by `install.sh` only, and by no Go file:** `NO_COLOR`, `XDG_CONFIG_HOME`, `REPLAY_VERSION`,
`REPLAY_BIN_DIR`. Verified: zero occurrences of any of the four in `cmd/` or `internal/`.

**Never read: any credential variable.** There is no reference to `Authorization`, `x-api-key`, or
any auth environment variable anywhere in the source. **Verified by grep and by sending a real-shaped
key through and inspecting the log, the ledger, the metrics labels and the 502 body.** It cannot log
what it does not read.

## 3b. Process

| Surface | What | Status |
|---|---|---|
| `SIGINT`, `SIGTERM` | `serve` only, for graceful shutdown (`cmd/replay/serve.go:148`) | Read |
| stdout, stderr | Reports and log lines. The log is where the `MASKING FAILED` warning goes | **Verified** |
| exit codes | Non-zero on refusal or error | Read |

**Absences worth stating, because each is checkable in one command and each removes a whole class of
question.** In all non-test code there is **no `exec.Command` anywhere**: Replay never shells out, so
there is no command-injection surface. There is no `os.Setenv`, so it never mutates the environment
of anything it starts. No `os.Symlink`. No `os.TempDir` in the binary, so no predictable-path temp
file and no symlink-attack surface there. No `filepath.Walk`, so it cannot wander outside the
directory it was handed. Every write in the tool resolves under `~/.replay` or a directory the user
named.

## 4. What a stranger sees

| Surface | Status |
|---|---|
| Repository, README, docs, ADRs, evidence | **Verified** by an adversarial claims audit; 6 false and 9 partial claims found and fixed |
| `install.sh`, piped to a shell | **Verified against a FAKE release.** The download-and-verify path works and a tampered archive is refused, but **no real release exists, so today every user takes the `go install` fallback instead** — which contacts `proxy.golang.org` and `sum.golang.org`, writes hundreds of MB to `$GOMODCACHE` and `$GOCACHE`, and performs **none** of the checksum or signature verification. The verified path is not the reachable one |
| GitHub Actions | **Verified**: all 15 were floating tags, now SHA-pinned |
| Release artefacts, checksums, Sigstore signatures | Read. **Never exercised** — no release is tagged |
| Issue templates, SECURITY.md, Discussions | **Verified**: Discussions was linked and disabled; now enabled |
| **git history** | **KNOWN PROBLEM.** Deleted PRDs, both adversarial reviews and the former project name `Buffy` are all still reachable |
| `internal/transcript/testdata/session-redacted.jsonl` | **KNOWN PROBLEM.** Paths and bodies hashed, **tool names are not**, including a connector UUID |

## 5. Provider surface

**One, and only one.** Replay models Anthropic's explicit-breakpoint caching and reads Claude Code
transcripts. `architecture/multi-provider.md` sets out why the other two families, implicit prefix
and rented cache, are different products rather than variants, and why the rented family breaks the
engine's assumption that more caching is better.

**~6,800 of 10,310 non-test lines are provider-neutral already.** The coupling is concentrated in
`internal/transcript` (1,100), `internal/ledger` (957) and `internal/cachemodel` (267).

---

## Weaknesses found by the independent pass, not yet fixed

- **`replay_rehydrated_total{destination=…}` and the denied counter carry model-supplied tool names**
  on `/replay/metrics`, which is unauthenticated unless `--token` is set. **Same tool-name disclosure
  class as the redacted-fixture problem**, on an endpoint any local process can read. Unbounded
  cardinality too.
- **No `O_EXCL` anywhere, and no write re-checks permissions.** Every key and state file is written
  through a pre-existing symlink if one is there, and an existing file keeps its existing mode.
- **`loadOrCreateKey` silently overwrites on any read failure.** A permission-denied read rotates the
  label key, or replaces the vault key and orphans the vault.
- **No file locking at all.** Two `replay serve` instances on one ledger directory rely entirely on
  `O_APPEND` atomicity; the mutex in `store.go` is per-process.
- **`isLoopback` accepts the literal string `localhost` without resolving it**, so that one input
  trusts `/etc/hosts`.
- **Log injection into stderr** via `r.URL.Path`, already percent-decoded by `net/url`, at
  `server.go:500`. Neighbouring sites sanitise; this one does not.
- **Client-visible egress not previously listed:** the loop-guard refusal body and the
  `x-replay-warning` response header carry the raw tool name; the 502 body carries the full upstream
  URL.

## What is genuinely unknown

Listed because a surface map that only contains what we checked is a marketing document.

**No release has ever been built or installed from.** The signing, checksums, SBOM and the six-target
matrix are all configured and none has run. The installer's release path was proven against a **fake**
release served locally, not a real one.

**The proxy has now run against the real provider, once.** [Spike 4](evidence/spike-4-real-provider-2026-09-05.md), 2026-09-05: a ten-turn session completed intact with the context-editing parameter applied, and the ledger carried no credential and no message content. The provider applied zero context edits on that session, so the parameter is accepted and not yet shown to do anything. The guards, retries and provider-error handling were still not exercised, because nothing failed. Roadmap spike 4 says so. Every measured-tier
figure and every guard has been exercised against a fake upstream. `--context-edit-trigger` in
particular sends a provider parameter that has never been sent to that provider.

**Windows is built and never tested.** goreleaser produces the target; nobody has run it there, and
`install.sh` refuses Windows outright.

**Linux is built and barely tested.** The musl and glibc split is handled in the installer and both
the tool and the tests have been exercised almost entirely on macOS arm64.

**Terms.** Spike 3 is marked passed on a citation that establishes routing behaviour and says nothing
about permission. **No terms document is cited anywhere in the repository.** This one needs counsel,
not another audit.

**Calibration breadth.** Eleven sessions, one machine, one project, two models. The corpus says this
about itself. What Replay does on a codebase that is not Go, a session that is not agentic, or a
client that is not Claude Code is **unmeasured**.

**Masking recall against real secrets.** Precision and recall of 1.00 are statements about a 15+15
and 10+15 corpus in `testdata/`. Against the shapes a real developer machine produces, **unknown**.

**Concurrency at scale.** Tests are race-clean, and the highest concurrency ever exercised is a
handful of parallel sub-agents. Behaviour under a large fleet sharing one proxy is untested.

---

[Documentation index](README.md) · [Repository README](../README.md)
