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
| `~/.replay/ledger/<session>.jsonl` | write | Block kinds, sizes, timings, usage. No message text. `0600` in a `0700` directory | **Verified** by inspecting a record produced from a request stuffed with secrets |
| `~/.replay/ledger/.label-key` | write | HMAC key for path labels | Read |
| `~/.replay/vault/`, `.vault-key` | write | Only with `--mask`. AES-256-GCM, key file alongside | **Verified**: a reviewer decrypted it in five lines of Python. The key file is the whole boundary |
| `~/.replay/.pins`, `.revert` | write | Per-session policy decisions, persisted so a restart cannot change a session's mind | Read |
| `~/.replay/policy.json`, advice file | write | `learn` and `advise` output | Read |
| `${XDG_CONFIG_HOME:-~/.config}/replay/corpus-consent.toml` | write | Only from `install.sh --corpus-opt-in`. Sends nothing | **Verified** |
| `/usr/local/bin/replay` or `~/.local/bin/replay` | write | The binary, at install | **Verified** end to end |

**Adjacent directories that exist on a normal machine and are NOT Replay's business.** Both were
found by scanning a real install, and confusing them is the likely support question:

- `~/.local/share/claude` — the Claude Code binary and `versions/`. Not transcripts.
- `~/Library/Application Support/Claude` — the desktop app's Electron profile, 15GB on the machine
  scanned. **A different product.** Replay never reads it.
- `~/.claude/history.jsonl` — prompt history, a `{"display":…}` record, **not a session**. The glob
  stays scoped to `projects/<project>` so the parser is never handed it.

## 2. Network

**Outbound, in normal use: exactly one host — the provider you configured.** Default
`https://api.anthropic.com`, overridable with `REPLAY_UPSTREAM`.

| Direction | Endpoint | When | Status |
|---|---|---|---|
| out | the configured provider | every proxied request, byte for byte | **Verified** against a fake upstream |
| out | `api.github.com`, `github.com` | **installer only**, never the binary | **Verified** |
| out | corpus endpoint | **only** `replay corpus --submit`, after showing the payload | Not built yet |
| in | `127.0.0.1:4000` `/` | the proxy itself. Loopback enforced at construction | **Verified**: a non-loopback `-listen` refuses to start |
| in | `/replay/status` | JSON per-session totals. `Origin` and `Sec-Fetch-Mode` refused | **Verified** |
| in | `/replay/metrics` | Prometheus text, aggregate only | Read |
| in | `/replay/healthz` | **no origin check, no token check** | **Verified as a gap** |

**Known gaps here, all recorded in the security review:** `/replay/status` and `/replay/metrics` are
**unauthenticated unless `--token` is set**, so any local process can read model names, token counts
and per-session list-price dollars. There is **no `Host` header validation**; the anti-rebinding
defence rests on `Origin` being present. And `/replay/healthz` answers cross-origin, so a web page
can fingerprint that Replay is running.

## 3. Environment

Read: `ANTHROPIC_BASE_URL`, `REPLAY_UPSTREAM`, `REPLAY_DISABLED`, `REPLAY_TOKEN`,
`REPLAY_NO_POLICY`, `CLAUDE_CONFIG_DIR`, `XDG_CONFIG_HOME`, `NO_COLOR`, `REPLAY_VERSION`,
`REPLAY_BIN_DIR`.

**Never read: any credential variable.** There is no reference to `Authorization`, `x-api-key`, or
any auth environment variable anywhere in the source. **Verified by grep and by sending a real-shaped
key through and inspecting the log, the ledger, the metrics labels and the 502 body.** It cannot log
what it does not read.

## 4. What a stranger sees

| Surface | Status |
|---|---|
| Repository, README, docs, ADRs, evidence | **Verified** by an adversarial claims audit; 6 false and 9 partial claims found and fixed |
| `install.sh`, piped to a shell | **Verified** end to end: happy path installs, tampered archive refuses, runs under dash/sh/ksh/zsh |
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

## What is genuinely unknown

Listed because a surface map that only contains what we checked is a marketing document.

**No release has ever been built or installed from.** The signing, checksums, SBOM and the six-target
matrix are all configured and none has run. The installer's release path was proven against a **fake**
release served locally, not a real one.

**The proxy has never run against the real provider.** Roadmap spike 4 says so. Every measured-tier
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
