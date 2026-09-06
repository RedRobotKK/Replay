# Release criteria

What has to be true before a version claims what it claims. Written 2026-09-06,
because until then there was a roadmap and no bar, and "production-ready" was a
phrase rather than a test.

## Where this stands today

**v0.4.0 is a working tool with a documented threat model. It is not a v1.0.**

The distinction is not code quality. The test posture is strong: 675 test
functions across 120 files against 101 source files, `go vet` plus
`go test -race -count=1` on every push, and every fix in this release was
reproduced red before it was fixed and then mutation tested. What is missing is
that four security findings are open by choice, and one provider path has never
touched a live provider.

## The v1.0 bar

Each line is a gate. A release cannot claim 1.0 with any of them unmet, and
"nearly" does not count.

### Security

- [ ] **Finding 3, the vault key boundary.** The key file sits beside the
      ciphertext, so masking converts transient secrets into secrets at rest
      with no eviction. Either the key moves to the OS keychain, or vault
      entries expire, or the README stops implying masking is durable
      protection. One of the three, chosen deliberately.
- [ ] **Finding 4, response-side `call_key`.** Currently unkeyed SHA-256
      (`transcript/wire.go:181`), so a ledger holder can confirm guessed tool
      calls offline. The request side is already HMAC'd. The two halves of one
      ledger must not have different properties.
- [ ] **Findings 6 and 7** either fixed or restated in the README as the
      supported operating model, in the README itself and not only in the
      security review. An operator who never opens `docs/` should still know.

### Provider coverage

- [ ] **The OpenAI-compatible path** is either exercised against a live
      provider, or labelled **EXPERIMENTAL, UNMASKED** wherever it is offered.
      It has only ever run against a test stub, and secret masking does not
      cover it at all. Shipping it unlabelled implies a parity that does not
      exist.

### Platform

- [ ] **Windows is either supported or the claim is removed from CI.** As of
      2026-09-06 it is declared unsupported in the README, and the job remains in
      the matrix as a non-blocking signal of how far away support is. Fourteen
      tests fail, and the ones that matter assert Unix file-mode semantics that
      guard the ledger and the masking vault. A Windows binary that runs while
      not keeping those promises is worse than no Windows binary.

### Measurement

- [ ] **No headline figure without the instrument that produced it being
      checked first.** This release exists partly because a 98.8% claim was
      shipped from a classifier that compared each agent lane against a
      different one. The rule is not "measure more", it is: before a number
      goes in a README, a commit message or a card, something must have tried
      to falsify the instrument.

## Deliberately NOT gates

Recorded so they are not smuggled in later as blockers.

- **The `rescore` quadratic.** 17.8 ms of proxy CPU for a 200-request session,
  measured and bounded by a growth-ratio test. It is a throughput concern on a
  busy host, not a correctness or safety one, and it is invisible to the client
  because it runs after the response has streamed.
- **Feature completeness.** `replay recall` is designed and unbuilt. That is a
  roadmap item, not a release gate.
- **Corpus size.** Model routing is blocked on evidence, not on code. A 1.0 can
  ship saying so.

## Cadence

There was none, and 67 changelog entries accumulated in a day after v0.3.0 was
cut. That is engineering inventory decoupled from the people running it.

**Cut a release when the changelog has something a user would act on, and do not
let `Unreleased` run past roughly twenty entries.** A release is how work
reaches the 26 installs that exist; holding it is not caution.

---

[Changelog](CHANGELOG.md) · [Security review](docs/evidence/security-review-2026-09-04.md)
