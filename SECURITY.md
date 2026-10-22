# Security Policy

Replay will sit between developers and their model provider and will hold API keys, session tokens, and plaintext of masked secrets in memory. We treat every report seriously.

## Reporting a vulnerability

**Do not open a public issue.** Use one of:

1. GitHub private vulnerability reporting: <https://github.com/RedRobotKK/Replay/security/advisories/new>
2. Email: <security@redrobot.jp>

Include the version (`replay version`), platform, a reproduction, and the impact as you understand it. You will get an acknowledgement within 3 working days and a status update at least every 7 days until resolution.

**If 3 working days pass with no acknowledgement, escalate.** Email `hello@redrobot.jp` with `SECURITY ESCALATION` in the subject, and if that also goes unanswered, treat the 90-day coordinated-disclosure clock below as having started on the date you first wrote. This project is maintained by one person and is launching on 2026-09-14; the clock a reporter is held to should not depend on one inbox being read. Saying that here costs nothing and removes the only thing that makes a private report worse than a public one.

## Scope

In scope: anything in this repository, including the daemon, its build and release pipeline, and the documentation where it would lead a user into an unsafe configuration.

Out of scope: vulnerabilities in the model providers, IDEs, or agents Replay talks to. Report those upstream.

## What leaves the machine

**Correction, 2026-10-20.** This section used to open "Replay runs on your machine and sends
nothing anywhere." That was false for as long as it was here: `replay serve` forwards your traffic
to a provider by design, and four typed commands reach other hosts. The list below is derived from
the code and held to it by `internal/regression/security_outbound_test.go` (SEC1 to SEC3), which
fails when a new sender appears in the binary without a line here.

**Nothing is sent automatically.** The binary has no telemetry, no account, no background or
startup update check, and no request that starts without a command you typed. Reading transcripts
(`cost`, `diff`, `blame`, `context`, `corpus` and the other offline readers) opens no
connection to another host; `burn` is the one reader that touches loopback, below. The `--contribute` flags write a file for you to submit yourself; they send nothing.

**The proxy forwards what your client sends it.** Once you run `replay serve`, every request your
agent sends to the loopback listener goes on to the upstream without a per-request prompt: by
default `https://api.anthropic.com`, or whatever `--upstream` / `REPLAY_UPSTREAM` names (an
`http://` upstream is accepted and is then cleartext). What is forwarded is your client's request:
its headers, including your provider API key, and its body. Replay removes its own
`x-replay-token` and `x-replay-tenant-id` headers first. With `--mask`, detected secrets in
`/v1/messages` and `/v1/responses` bodies are replaced by placeholders before egress
(`maskable`, internal/proxy/passthrough.go); `/v1/chat/completions` is forwarded UNMASKED, and
other POST paths are forwarded unread. `HTTPS_PROXY`, `HTTP_PROXY` and
`NO_PROXY` in the environment redirect this leg. The proxy itself originates nothing: no ping and
no health check of the upstream.

**Four typed commands reach a host off this machine:**

| Command | Host | What is sent |
|---|---|---|
| `replay rules --check-prices` | `raw.githubusercontent.com` (the LiteLLM price table) | one GET, no identifier; nothing is installed from it |
| `replay rules --update <url>` | the host you name; `https` only, a redirect to `http` is refused | one GET; a `402` payment demand (x402) is reported and never paid, and this binary holds no wallet and cannot sign |
| `replay probe --execute` | `$ANTHROPIC_BASE_URL` or `api.anthropic.com` | synthetic billable requests on `ANTHROPIC_API_KEY`, after printing the plan and asking unless `--yes` |
| `replay upgrade` | `github.com/RedRobotKK/Replay/releases` | GETs for the latest-tag redirect, the archive, `checksums.txt` and its signature; no identifier. The archive is checked against `checksums.txt`, and `checksums.txt` against its Sigstore signature when `cosign` is installed (`cosign` is a separate program and, by its own defaults, consults the Sigstore transparency log; that traffic is cosign's, and it has not been measured here). The staged binary is run once before it replaces this one |

**Two typed commands and one automatic step touch loopback only.** `replay doctor` and `replay tui`
GET `/replay/healthz` or `/replay/status` at `$ANTHROPIC_BASE_URL`, and refuse to send when that
address is not loopback. `replay burn` asks `127.0.0.1:11434` for a local Ollama version on every
run. None of these leaves the machine.

**Compiled in, not reachable.** `internal/feed` can fetch a signed rules bundle and its signature;
no command calls it today.

Outside the binary: `install.sh` fetches from `api.github.com` and `github.com` over https only, and
the npm launcher fetches a release archive only when its platform package is missing and
`REPLAY_DOCTOR_ALLOW_FETCH=1`. The full table, with every surface and its tests, is
[`docs/SURFACES.md`](docs/SURFACES.md).

## What Replay keeps, and for how long

Everything Replay keeps lives under `~/.replay`, and `replay privacy` lists all of it: every
store, its size, and what it holds in plain terms.

**The ledger holds counts and timings, never message content.** Per request: a timestamp, a session
id, the request path, the status, token counts and cache outcomes. No prompts, no responses, no
file contents. Paths in derived findings are HMAC'd.

**One store is different.** The masking vault (`~/.replay/vault`) holds the real values behind
placeholders sent to a provider. It is the only store here that keeps your secrets rather than
counts about them, and `replay privacy` marks it.

**It is the one store with an automatic expiry, and the reason is the sentence below it.** Entries
are evicted 24 hours after they are vaulted (`serve --mask-ttl`; `0` restores the old behaviour of
keeping them indefinitely). This file used to say the vault "is never removed by a retention
window". That was true and it was the wrong default: masking turns a transient credential into one
at rest, and the vault key file sits next to the ciphertext it decrypts, so an unbounded vault meant
a host compromised on day 200 gave up 200 days of credentials. Expiry costs almost nothing — the
placeholder is derived from the secret, so re-sending a secret whose entry lapsed restores it
unchanged — and what it does not do is make the vault safe: within the window, anyone who can read
that directory can read the secrets. Treat masking as a control on what leaves the machine, not as
storage.

**Retention: 90 days is the recommended default, and nothing is deleted unless you ask — with the
one exception above, the masking vault.** For everything else Replay sets no automatic expiry, because silently deleting a reader's own measurements is not a decision a
tool should make for them. What it gives you instead is the means to enforce whatever period you
choose:

```sh
replay privacy                                        # what is held, and where
replay purge ~/.replay/ledger --older-than 90d        # what would go
replay purge ~/.replay/ledger --older-than 90d --yes  # remove it
```

**Erasure by subject.** `replay purge ~/.replay --session <id> --yes` removes every record for one
session from inside the files that hold it, leaving other sessions in those files intact. Add
`--export <file>` to write a copy of what is about to be removed — the portability half of an
erasure request, and worth having before any irreversible command.

Both default to reporting and change nothing without `--yes`.

## Supported versions

Until 1.0, only the latest minor release receives security fixes.

## Disclosure

We follow coordinated disclosure. We ask for 90 days from acknowledgement before public disclosure, and we will credit reporters in the release notes unless they prefer otherwise.
