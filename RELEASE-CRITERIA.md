# Release criteria

What has to be true before a version claims what it claims. Written 2026-09-06,
because until then there was a roadmap and no bar, and "production-ready" was a
phrase rather than a test.

## Where this stands today

**v0.5.0 is a working tool with a documented threat model. It is not a v1.0.**

The distinction is not code quality. The test posture is strong: every package
carries tests, `go vet` plus `go test -race -count=1` runs on every push, and
every fix in this release was reproduced red before it was fixed and then
mutation tested. The counts that stood in this paragraph until 2026-09-11 —
"816 test functions across 152 files against 127 source files" — are deleted
rather than refreshed. They were written when this file was, on 2026-09-06, and
nothing recomputed them; counted again on 2026-09-11 the tree held 1811 test
functions across 358 test files against 202 source files, so two of the three
were wrong by more than double. Fresher counts would rot the same way.
`go test ./... -count=1` runs what exists, and
`find . -name '*_test.go' | wc -l` counts the files. What is missing is
that one security finding is open in part by choice, and one provider path is
labelled rather than covered.

That second clause read "one provider path has never touched a live provider"
until 2026-09-12, and it was wrong: see the correction under the provider
coverage gate below. The path has touched two live providers. What it has never
touched is the masker.

**Updated 2026-09-10.** Findings 4, 6 and 7 are closed and finding 3 is closed
on eviction. Each closure is a test that fails when the guard is removed; the
guards were neutralised in the source and watched to go red rather than
declared. What is left of finding 3 — the vault key file sitting beside the
ciphertext — is unchanged and still gates 1.0.

## The v1.0 bar

Each line is a gate. A release cannot claim 1.0 with any of them unmet, and
"nearly" does not count.

### Security

- [ ] **Finding 3, the vault key boundary.** Half done, 2026-09-10, and the
      remaining half is the one this line is about. Vault entries now expire
      — 24 hours by default, `--mask-ttl` to change it, `0` for the old
      unbounded behaviour — which is the second of the three options below,
      taken deliberately because the first is not reachable: the OS keychain
      needs `os/exec`, and `TestX402_ExecIsConfinedToTheMutationHarness` keeps
      `os/exec` out of every ordinary build on the grounds that it can call
      anything. **Still open: the key file sits beside the ciphertext**, so
      within the TTL the vault is plaintext-equivalent to anyone who can read
      the directory. A 1.0 needs the key somewhere else, or a README that says
      plainly that masking is a transit control and not storage.
- [x] **Finding 4, response-side `call_key`.** Closed 2026-09-10.
      `Store.Append` re-keys the response half under the ledger secret, so both
      halves of one ledger have one property. Guarded by
      `internal/ledger/responsecallkey_test.go`, which asserts on the bytes on
      disk and covers the streaming parser separately.
- [x] **Findings 6 and 7** closed 2026-09-10, and restated in the README's
      Footprint section rather than only here — an operator who never opens
      `docs/` should still know. Finding 6: `Host` validation on every
      TCP-served route and the browser guard on `/replay/healthz`
      (`internal/proxy/hostguard.go`); the token stays optional on the read
      endpoints and absent from healthz, which is a decision the README now
      states. Finding 7: `internal/ownerdir` verifies and tightens the ledger
      and vault directories and their key files on open, and refuses when it
      cannot.

### Provider coverage

- [x] **The OpenAI-compatible path** is either exercised against a live
      provider, or labelled **EXPERIMENTAL, UNMASKED** wherever it is offered.
      **Closed 2026-09-12 by the labelling route, not the verification route.**
      The path is labelled, not newly proven: `/v1/chat/completions` still
      applies no masking, and no Cursor or other generic OpenAI-compatible CLI
      has ever been pointed at `replay serve`. What changed is that a reader
      now meets the label before their first request instead of after it. The
      label is on the `-upstream` and `-mask` flag help (which is where the
      path is offered, and where a user who never opens `docs/` will be), on
      the `masking: on` startup banner, on the `s` screen in the TUI, in
      `docs/SURFACES.md`, `docs/CLI.md` and the command guide, and on stderr
      from the proxy itself once per path. Each says what UNMASKED costs in the
      same breath as the word, because "unmasked" alone does not tell a reader
      their API keys are in flight. Guarded by
      `internal/proxy/unmasked_test.go` and `cmd/replay/openailabel_test.go`;
      three mutations of the runtime disclosure and two of the flag help were
      run and watched to go red.

      **Correction to this gate's own premise, 2026-09-12.** The sentence that
      stood here said the path "has only ever run against a test stub". That
      was false when it was written, and the repository held its own refutation
      the whole time: `docs/architecture/multi-provider.md` records a live
      DeepSeek run on 2026-09-05 across four surfaces which caught a defect a
      stub structurally cannot produce (`RawUsage` was silently dropping
      `prompt_cache_hit_tokens`), `docs/evidence/ollama-cache-observable-2026-09-09.md`
      records the same endpoint driven against a local Ollama, and
      `internal/ledger/testdata/deepseek` holds the captured bytes. The
      surface registry had it right in `internal/ledger/surface_registry_test.go`,
      splitting the live DeepSeek row from the stub-only Cursor row, and
      nothing reconciled the two. What is genuinely unproven is narrower and
      is what the label now says: never OpenAI itself, never a third
      implementation of the API, no cache write ever observed in that response
      shape, and no OpenAI-compatible CLI ever pointed at the proxy. **A gate
      that overstates its own gap is the same defect as one that hides it**,
      and it is worse here than elsewhere, because this file is the document
      that decides what the release is allowed to claim.

### Platform

- [x] **Windows is either supported or the claim is removed from CI.** Closed
      2026-09-13, and every factual claim in the line this replaces was stale.
      The fourteen failures were fixed on 2026-09-10 and the job has been green
      and blocking since, which turned out to be the defect rather than the
      progress: Windows passes because the promise is switched off there.
      `internal/ownerdir` reports 100% statement coverage on ubuntu and 40% on
      windows, `modeIsChecked()` returns false so `tighten` never chmods or
      re-stats, and twenty-two tests across the ledger, the vault, the consent
      gate and the contributor secret skip with "Unix permission bits". A
      blocking green check whose greenness comes from disabling what it checks
      is what ADR-0014 exists to forbid.
      **The binary now refuses on Windows**, so "unsupported" is something the
      program does rather than a line in a README, and two tests hold it: the
      refusal fires, and `run()` still consults it. The route that made it
      reachable is closed too, because install.sh refused Windows while offering
      a release archive that does not exist and a `go install` that works.
      The port is not refused on difficulty. An owner-only DACL needs no new
      dependency, since `syscall` and `unsafe` are already on the import
      allowlist. It is refused on evidence: `guard reachability` and
      `frozen mutants` both run on ubuntu only, so every refusal in an ACL layer
      would ship unmutated. **A Windows leg on those two jobs is the condition
      that reopens this**, and it is the only thing that should.

### Measurement

- [ ] **No headline figure without the instrument that produced it being
      checked first.** This release exists partly because a 98.8% claim was
      shipped from a classifier that compared each agent lane against a
      different one. The rule is not "measure more", it is: before a number
      goes in a README, a commit message or a card, something must have tried
      to falsify the instrument.
- [ ] **The same rule binds commercial figures, and until 2026-09-13 it did
      not.** A price is a headline figure. `docs/MONEY-PATH.md` carried a $199
      per repository list price derived by applying a 1 to 3 percent comparable
      to $13,000 a month of *unreachable capacity*, which is not spend the
      customer makes, and no instrument had been pointed at it. A twelve-person
      review found the term change between two paragraphs. The list price is now
      marked provisional and is published nowhere a launch reader sees it, and
      the falsifier is pre-registered in that document: if a Van Westendorp
      series over about ten people who have actually run the tool clusters "too
      expensive" below $99, the section is corrected rather than defended.
      **A number nobody has tried to falsify does not become exempt by being
      about money.**

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
let `Unreleased` run past roughly twenty entries.** Holding a release is not
caution.

There is no defensible install count, and the figure this file carried until
2026-09-07 was one. It said "the 26 installs that exist", which was a fetch
count wearing the word "installs". Measured on 2026-09-07: **53 fetches of the
install script over its lifetime, from 2 distinct IP addresses**, and 152
release-asset downloads that include scanners and CI. A fetch is not an install,
an install is not a person, and a download is not either. The honest statement
is that no external user has been observed. Wrong units in a count is the defect
this project spends most of its time finding in its own output, and it had one
in the file that tells it when to ship.

---

[Changelog](CHANGELOG.md) · [Security review](docs/evidence/security-review-2026-09-04.md)
