# Adversarial security review, 2026-09-04

An external reviewer read the code and then **ran the proxy end-to-end against a fake upstream**,
confirming each finding empirically rather than inferring it from documentation. This file records
what was verified to hold and what was found, before the repository was made public.

**Updated 2026-09-06.** Findings 1, 2 and 5 are resolved; 3, 4, 6 and 7 are open and are described
below in terms of what an operator inherits by installing this. The file kept saying "the rest are
open" after two of them had been fixed, which overstated the risk in one direction while leaving the
real ones undescribed in the other. A stale security document is worse than none, because it is
read as current.

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
| 3 | MEDIUM | The vault key file sits beside the ciphertext, so the vault is plaintext-equivalent to anyone who can read the directory. Masking converts transient secrets into secrets at rest, with no eviction | `masking/vault.go:29-38` |
| 4 | MEDIUM-LOW | Response-side `call_key` is an unkeyed SHA-256 of the full tool input, so a ledger holder can confirm guessed tool calls offline. The request side is HMAC'd; the response side is not | `transcript/wire.go:171-177` |
| 5 | LOW-MEDIUM | `install.sh` failed open on checksum verification, and its `grep` was unanchored so it accepted the hash of any file listed | `install.sh` |
| 6 | LOW | `/replay/status` and `/replay/metrics` are unauthenticated by default; `/replay/healthz` has no guard; there is no `Host` validation | `proxy/server.go:332-366` |
| 7 | LOW | Directory permissions are set at creation and never verified, so a pre-existing world-writable directory stays world-writable | `ledger/store.go:52`, `masking/vault.go:62` |

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

## Still open, and what threat model you are adopting by installing this

Findings **3, 4, 6 and 7** are open. They are not latent unknowns; they are
choices, and an operator should know which ones they are inheriting.

| # | Open finding | What it means for you |
|---|---|---|
| 3 | The vault key file sits beside the ciphertext | **Masking converts transient secrets into secrets at rest.** Anyone who can read `~/.replay/vault` can read the key next to it. There is no eviction. If the host is compromised, the vault adds nothing. The code says so: *"until then the key file is the boundary."* |
| 4 | Response-side `call_key` is unkeyed SHA-256 (`transcript/wire.go:181`) | A ledger holder can confirm **guessed** tool calls offline. The request side is HMAC'd; the response side is not, so the two halves of the same ledger have different properties |
| 6 | `/replay/status` and `/replay/metrics` are unauthenticated unless `--token` is set | Any local process reads model names, token counts and per-session list-price dollars. No `Host` validation; the anti-rebinding defence rests on `Origin` being present. `/replay/healthz` answers cross-origin, so a page can fingerprint that Replay is running |
| 7 | Directory permissions are set at creation and never verified | A pre-existing world-writable `~/.replay` stays world-writable |

**The shortest honest summary: Replay is safe to run on a machine you trust, on
a loopback interface, by one operator. It is not hardened against a hostile
local process or a compromised host, and the ledger is not safe to hand to
someone you would not hand the transcripts to.**

Two of these gate a v1.0 claim, and that is recorded in `RELEASE-CRITERIA.md`
rather than left as an intention here.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
