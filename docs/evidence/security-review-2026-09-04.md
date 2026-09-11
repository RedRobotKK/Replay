# Adversarial security review, 2026-09-04

An external reviewer read the code and then **ran the proxy end-to-end against a fake upstream**,
confirming each finding empirically rather than inferring it from documentation. This file records
what was verified to hold and what was found, before the repository was made public.

**Updated 2026-09-06, superseded by the note below.** Findings 1, 2 and 5 are resolved; 3, 4, 6 and
7 were open as of that date and are described below in terms of what an operator inherits by
installing this. The file kept saying "the rest are
open" after two of them had been fixed, which overstated the risk in one direction while leaving the
real ones undescribed in the other. A stale security document is worse than none, because it is
read as current.

**Updated 2026-09-10.** Findings 4, 6 and 7 are now resolved, and finding 3 is resolved in one of
the three ways `RELEASE-CRITERIA.md` offers and NOT in the other two. One sentence of the summary
below therefore changes and the rest does not: the ledger is now safe to hand to someone you would
hand the derived data to, because both halves of it are keyed. Everything else about the shortest
honest summary stands. See [What changed on 2026-09-10](#what-changed-on-2026-09-10).

## Verified to hold

**Credential handling is better than the README claims.** The proxy never reads the credential at
all: there is no reference to `Authorization`, `x-api-key`, or any credential environment variable
anywhere in `internal/` or `cmd/`. **It cannot log what it does not read.** Confirmed by sending a
real-shaped key and inspecting the upstream request, the log line, the 502 body on an unreachable
provider, the retry transport's logging, and every metrics label.

**"No message text, ever" holds.** A request stuffed with an AWS secret key, a Twilio token, a
`postgres://` URL with credentials, and a `.env` tool result produced a ledger record containing
block kinds, byte counts, HMAC'd labels, tool names and usage. **Nothing recoverable.** Directory
`drwx------`, files `-rw-------` at creation.

**`panic(aborted)` at `server.go:511` is the correct idiom, not a crash path.** Five mid-stream
client aborts against a streaming upstream: process stayed up, `healthz` still answered, no stack
trace. No finding.

**Loopback enforcement and the browser guard hold.** A non-loopback `-listen` refuses to start.
`Origin` and `Sec-Fetch-Mode` both produce 403.

**Log injection via headers is not exploitable.** Go's `net/http` rejects control characters in
header values before Replay sees them.

## Findings

| # | Severity | Finding | Location |
|---|---|---|---|
| 1 | ~~HIGH~~ **RESOLVED 2026-09-06** | `--mask` failed open on a vault write error and forwarded secrets it had already positively identified | fixed at `masking/mask.go:126` |
| 2 | ~~HIGH~~ **RESOLVED 2026-09-06** | The entropy heuristic required lowercase **and** uppercase **and** digits, so it was structurally blind to hex and lowercase credentials | fixed at `masking/entropy.go:177` |
| 3 | MEDIUM **PARTLY RESOLVED 2026-09-10** | The vault key file sits beside the ciphertext, so the vault is plaintext-equivalent to anyone who can read the directory. Masking converts transient secrets into secrets at rest, with no eviction | `masking/vault.go:29-38`. Eviction added; the key boundary is unchanged and stays open |
| 4 | ~~MEDIUM-LOW~~ **RESOLVED 2026-09-10** | Response-side `call_key` is an unkeyed SHA-256 of the full tool input, so a ledger holder can confirm guessed tool calls offline. The request side is HMAC'd; the response side is not | fixed at `ledger/store.go` (`Append`) and `ledger/summarize.go` (`KeyResponseCalls`) |
| 5 | ~~LOW-MEDIUM~~ **RESOLVED 2026-09-04** | `install.sh` failed open on checksum verification, and its `grep` was unanchored so it accepted the hash of any file listed | `install.sh` |
| 6 | ~~LOW~~ **RESOLVED 2026-09-10** | `/replay/status` and `/replay/metrics` are unauthenticated by default; `/replay/healthz` has no guard; there is no `Host` validation | fixed at `proxy/hostguard.go` |
| 7 | ~~LOW~~ **RESOLVED 2026-09-10** | Directory permissions are set at creation and never verified, so a pre-existing world-writable directory stays world-writable | fixed at `internal/ownerdir`, called from `ledger/store.go` and `masking/vault.go` |

## Resolved, 2026-09-06

Two of the three highest findings are closed. They are kept in the table above
rather than deleted, because a review that quietly loses its own findings cannot
be audited against what it once said.

**Finding 1, masking failing open.** `Mask` now fails SECURE. When the vault
cannot store a match the secret is replaced with `BlindPlaceholder`
(`[REDACTED_BY_PROXY_ERROR]`) rather than handed back verbatim: the stream
survives, the credential does not, and the caller still receives the error.
Rehydration of that one value is lost, which is the deliberate cost. This is the
first of the three options below, taken in the form that does not refuse
traffic. Guarded by `masking/failclosed_test.go`, which also asserts the blind
placeholder is textually distinct from a vault one, so the two can never be
confused downstream.

**Finding 2, the entropy heuristic.** `LooksLikeHexSecret` covers the hex and
lowercase case the character-class rule could not see. It requires at least 32
characters AND a credential cue within 40 characters before the run, because
the blast radius of a looser rule was measured first and was large: hex runs are
common in ordinary source. Guarded by the same file, including negative cases
for benign hex.

**Neither fix widens what is masked without evidence.** The measurement that
sized finding 2's rule came before the rule, not after.

### On finding 1, which is the one that matters, as it stood on 2026-09-04

`Mask` returns the **original body** when the vault cannot store a match, abandoning secrets it had
already located. Demonstrated with a read-only vault directory: two dead-centre pattern matches
reached the provider verbatim, with the only signal a single stderr line that scrolls past in a
backgrounded `serve`.

**This is where the fail-open principle and the masking promise point in opposite directions**, and
the resolution is a deliberate choice rather than a bug fix:

- **Fail closed on masking only.** Refuse the request when a positively identified secret cannot be
  masked. Safest for confidentiality, and the one place where refusing traffic is arguably right.
- **Redact without vaulting.** Replace the match with a placeholder that has no vault entry. The
  secret does not leave; rehydration of that value is lost.
- **Keep current behaviour, document it.** Cheapest, and defensible only if the README stops
  implying masking is reliable.

**Finding 5 was fixed on 2026-09-04.** Verification is now fatal unless `--no-verify` is passed, the
digest is bound to the archive filename, and the README no longer claims more than the script does.

**Findings 1 and 2 were fixed on 2026-09-06**, above.

## What changed on 2026-09-10

Each item below is closed by a test that fails when the protection is removed.
That qualifier is the whole claim: every guard here was neutralised in the
source and the suite watched to go red, because a security guard nothing
exercises is a promise nobody has checked. The neutralisations and the tests
that caught them are named in the test files, next to the code.

**Finding 4, the response-side `call_key`, is closed.** `Store.Append` now
re-keys the response half under the same ledger secret the request half
already used, so both halves of a ledger file have one property.
`sha256("Bash\x00" + guess)[:16]` no longer confirms anything. The re-key is
an HMAC over the digest rather than over the input, because the input is gone
by the time a record exists — `ParseResponse` strips block text and the stream
parser never accumulates `input_json_delta`. That is enough for what the
finding names: a guesser can compute the digest and cannot compute the HMAC
of it. Guarded by `internal/ledger/responsecallkey_test.go`, which asserts on
the bytes written to the file rather than on the in-memory record, because a
ledger holder holds the file. The streaming path is asserted separately: it is
the path most traffic takes and it is parsed by different code.

**Finding 6 is closed, in two of its three parts, and the third is a stated
decision rather than an omission.**

- `Host` validation now exists on every TCP-served route, including the
  passthrough. Before this, a page at `evil.example` whose DNS answered
  `127.0.0.1` reached `/v1/messages` and the request was forwarded — the test
  that first ran against the unfixed code recorded five such requests arriving
  at the provider. `Origin` was the only defence and a `<form>` post does not
  send one, so the guard depended on the attacker's cooperation.
- `/replay/healthz` now carries the browser guard, so a page cannot use it to
  fingerprint that Replay is running.
- **The token stays optional on the read endpoints, and healthz answers
  without one.** `replay doctor` probes healthz with no token to explain why
  an agent is failing, and it cannot learn a token it did not set. Requiring
  one would break the one command whose job is to diagnose a broken setup, to
  deny a local process a fact it could get from `connect(2)`.

The guard is confined to TCP. A Unix socket has no DNS to rebind and its
clients address it through a placeholder authority, so the same check there
would refuse the more isolated transport. Guarded by
`internal/proxy/hostguard_test.go`, including the metrics listener, which is a
separate mux by design and is exactly the surface a guard added to the main
one silently misses.

**Finding 7 is closed.** `internal/ownerdir` verifies the ledger and vault
directories on open, and the key files by name, tightening them when they are
not owner-only and refusing when they cannot be. Both halves of that matter:
`os.MkdirAll` applies its mode only when it creates, and `os.WriteFile` does
not change the mode of a file that already exists, so a `~/.replay` that
arrived from an archive or a `mkdir -m 777` kept whatever it had while the
code read as though it had set `0700`. The refusal is not decoration: a chmod
that reports success while the bits do not change is what a network mount or
an exFAT volume does, and continuing after one would mean writing the vault
key into a directory this process has just discovered it does not control.

**Finding 3 is closed on eviction and open on the key boundary.**
`RELEASE-CRITERIA.md` offers three ways to discharge it and asks for one. The
second is taken: vault entries now expire, 24 hours by default, settable with
`--mask-ttl`, and `0` restores the old unbounded behaviour for anyone who
needs it.

Why that one. The keychain is not reachable from this binary — macOS's needs
`security`, and `TestX402_ExecIsConfinedToTheMutationHarness` confines every
`os/exec` importer to the `mutation` build tag or a named exemption, on the
grounds that `os/exec` can call anything. Trading a capability guard over the
whole binary for a boundary on one file is the wrong trade. The third option
is a documentation change and leaves the retention itself unbounded.

What expiry buys, narrowly: the window. `~/.replay/vault` used to accumulate
every secret the proxy had ever masked for the life of the machine, so a host
compromised on day 200 handed over 200 days of credentials; now it hands over
a day. It costs almost nothing, because the placeholder is the HMAC of the
secret and is therefore stable — a client re-sending a secret whose entry
lapsed gets the same placeholder back and the entry returns. The only loss is
rehydrating a placeholder whose secret has not been seen for a day.

**What it does not buy, and this is the part that stays open: the key file
still sits beside the ciphertext.** Within the TTL the vault is still
plaintext-equivalent to anyone who can read the directory. Finding 7's check
narrows who that is on a shared machine. It does nothing against a compromised
host.

## Still open, and what threat model you are adopting by installing this

One finding is open, in part.

| # | Open finding | What it means for you |
|---|---|---|
| 3 (part) | The vault key file sits beside the ciphertext | **Masking still converts transient secrets into secrets at rest, for as long as the TTL.** Anyone who can read `~/.replay/vault` can read the key next to it. If the host is compromised, the vault adds nothing beyond bounding the window. The code says so: *"until then the key file is the boundary."* Moving it to the OS keychain needs `os/exec`, which this binary keeps out on purpose |

**The shortest honest summary: Replay is safe to run on a machine you trust, on
a loopback interface, by one operator. It is not hardened against a compromised
host.** The two clauses that used to follow have changed and are worth stating
rather than deleting: a hostile local process now meets a `Host` check, a
browser guard on every route, and directories verified rather than assumed —
which is a real narrowing, not hardening against a process running as you. And
the ledger's two halves are now keyed alike, so it is safe to hand to someone
you would hand derived data to. The vault is not, and was never claimed to be.

One finding gates a v1.0 claim, and that is recorded in `RELEASE-CRITERIA.md`
rather than left as an intention here.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
