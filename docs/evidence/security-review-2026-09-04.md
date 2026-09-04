# Adversarial security review, 2026-09-04

An external reviewer read the code and then **ran the proxy end-to-end against a fake upstream**,
confirming each finding empirically rather than inferring it from documentation. This file records
what was verified to hold and what was found, before the repository was made public.

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
| 1 | **HIGH** | `--mask` fails open on a vault write error and forwards secrets it had already positively identified | `masking/mask.go:96`, `proxy/server.go:521-526` |
| 2 | **HIGH** | The entropy heuristic requires lowercase **and** uppercase **and** digits, so it is structurally blind to hex and lowercase credentials | `masking/entropy.go:86` |
| 3 | MEDIUM | The vault key file sits beside the ciphertext, so the vault is plaintext-equivalent to anyone who can read the directory. Masking converts transient secrets into secrets at rest, with no eviction | `masking/vault.go:29-38` |
| 4 | MEDIUM-LOW | Response-side `call_key` is an unkeyed SHA-256 of the full tool input, so a ledger holder can confirm guessed tool calls offline. The request side is HMAC'd; the response side is not | `transcript/wire.go:171-177` |
| 5 | LOW-MEDIUM | `install.sh` failed open on checksum verification, and its `grep` was unanchored so it accepted the hash of any file listed | `install.sh` |
| 6 | LOW | `/replay/status` and `/replay/metrics` are unauthenticated by default; `/replay/healthz` has no guard; there is no `Host` validation | `proxy/server.go:332-366` |
| 7 | LOW | Directory permissions are set at creation and never verified, so a pre-existing world-writable directory stays world-writable | `ledger/store.go:52`, `masking/vault.go:62` |

### On finding 1, which is the one that matters

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

**Findings 5 was fixed on 2026-09-04.** Verification is now fatal unless `--no-verify` is passed, the
digest is bound to the archive filename, and the README no longer claims more than the script does.
The rest are open and recorded here rather than quietly carried.
