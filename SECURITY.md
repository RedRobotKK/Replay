# Security Policy

Replay will sit between developers and their model provider and will hold API keys, session tokens, and plaintext of masked secrets in memory. We treat every report seriously.

## Reporting a vulnerability

**Do not open a public issue.** Use one of:

1. GitHub private vulnerability reporting: <https://github.com/RedRobotKK/Replay/security/advisories/new>
2. Email: <security@redrobot.jp>

Include the version (`replay version`), platform, a reproduction, and the impact as you understand it. You will get an acknowledgement within 3 working days and a status update at least every 7 days until resolution.

## Scope

In scope: anything in this repository, including the daemon, its build and release pipeline, and the documentation where it would lead a user into an unsafe configuration.

Out of scope: vulnerabilities in the model providers, IDEs, or agents Replay talks to. Report those upstream.

## What Replay keeps, and for how long

Replay runs on your machine and sends nothing anywhere. Everything it keeps lives under
`~/.replay`, and `replay privacy` lists all of it — every store, its size, and what it holds in
plain terms.

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
