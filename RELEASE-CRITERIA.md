# Release criteria

What has to be true before a version claims what it claims. Written 2026-09-06,
because until then there was a roadmap and no bar, and "production-ready" was a
phrase rather than a test.

## Where this stands today

**Updated 2026-09-13. Every gate below is closed, and this is still not a
1.0.**

That sentence is the one worth reading carefully, because the checkboxes now
say otherwise and a reader who counts them will reach the wrong conclusion.
Five gates are ticked: the vault key boundary, the OpenAI-compatible path,
Windows, and the two measurement rules, which turned out not to be gates at all
and are recorded below as standing rules instead.

**What is left is not in this file.** [`docs/ROADMAP.md`](docs/ROADMAP.md) has
carried a one-line 1.0 definition since before this document existed, and it
names three things none of which are ticked anywhere:

| | State, 2026-09-13 |
|---|---|
| An external security review, published | **Not commissioned.** Needs a third party and has weeks of lead time |
| Signed, reproducible releases | Signed, yes: Sigstore keyless bound to the workflow and tag, an SBOM per ARCHIVE (the ten Linux packages get none; `artifacts: archive` in .goreleaser.yaml). **Reproducible: verified once, on 2026-09-13, not yet enforced.** All four published platforms of v0.5.4 rebuild to identical bytes, by a script anybody can run ([evidence](docs/evidence/reproducible-build-2026-09-13.md), [script](scripts/reproduce-release.sh)). Nothing runs it on a new tag, so this is a fact about v0.5.4 rather than a property of releases. **Corrected 2026-10-08, not deleted: verified a second time**, against v0.8.0 on all four platforms ([evidence](docs/evidence/reproducible-build-2026-10-08.md)), confirming the procedure still works and the 2026-09-13 result was not a one-off. Still not wired into CI, and still not a fact about any v1.0 tag, because none exists yet. **Corrected 2026-10-13, not deleted: wired into the release workflow, not yet run on a tag.** `release.yml`'s `reproduce` job rebuilds the binary in each of the four published archives after publication and compares it, byte for byte, with the bytes `replay upgrade` would install. It runs after publication: it detects a release that does not reproduce and does not prevent one. What it checks, what it does not, and the states below are in "The reproduce gate" under the v1.0 bar. State, kept separate: implemented and reviewed locally 2026-10-13 (one reviewer, the same model throughout); CI-verified: no; verified against a real tag: v0.9.0 on all four platforms by hand, 2026-10-13, and no tag has run the job; production Sigstore trust: not part of this gate |
| Caching rules for a second provider | **Not started** (as of 2026-09-13). **Corrected 2026-10-08, not deleted:** typed, mutation-tested `CacheRules` for OpenAI's published Astra-tier pricing landed 2026-09-15 as `AstraRules()` (`internal/cachemodel/openai.go`, see [`docs/ROADMAP.md`](docs/ROADMAP.md)'s v0.8 section). Confirmed again today by a repository-wide search: no non-test caller selects it. "Not started" undersold the code that exists; "calibrated" would oversell it. The gate is published rules **and** a calibration corpus against real traffic, and only the first half exists. Second-provider production calibration remains outstanding. **Corrected again 2026-10-08, same day, not deleted: the first half moved too.** `cachemodel.RulesForModel` and `ClassifyBreakForModel` now exist, and all three production call sites that classify a cache break (`internal/proxy/state.go`, the live proxy; `internal/analysis/diff.go`, the offline diff; `cmd/replay/costusage.go`, the usage-export cost path) call the new function instead of the Anthropic-pinned `ClassifyBreak`. `AstraRules()` is reachable in production for any model id containing `gpt-6-astra`, proven by RED/GREEN at both the unit level (`internal/cachemodel`) and by calling the live proxy's own `(*stats).breakCause` directly (`internal/proxy/breakcauseprovider_test.go`): reverting the wiring reproduces the exact defect (a 20-minute Astra gap reported as TTL expiry under Anthropic's 5-minute rule) and the fix removes it, with every pre-existing test in both packages staying green throughout. Two things this does NOT close, stated so neither is mistaken for done: (1) Astra's MinPrefix floor (1024 tokens) is not reachable from any of the three call sites, because none of them knows the current request's visible prefix size at the point of classification, and `ClassifyBreakWith`'s own contract is to skip the floor test rather than guess — this is an existing gap, unchanged by this pass; (2) **no calibration against real OpenAI traffic has been performed.** `AstraRules()`'s numbers are still exactly what they were: read from OpenAI's published documentation on 2026-09-15, never replayed against a live response. Checked explicitly on 2026-10-08: no `OPENAI_API_KEY`, no billing-linked account, and no other mechanism for real, billable OpenAI traffic exists anywhere in this environment. The gate is published rules, production dispatch, **and** a calibration corpus against real traffic; the first two now exist and the third does not, and does not become closer to existing by the first two landing. |

Two of those three cannot be done alone. The review needs a reviewer, and the
independence problem underneath the second provider needs a corpus from a
machine that is not this one. The third was done alone, on 2026-09-13, and is
the only one of the three that moved. **Corrected 2026-10-08:** the second-provider
row also moved since this sentence was written, with the typed rules landing
2026-09-15, but the gate itself (rules **and** a calibration corpus) has not,
so the sentence's conclusion stands even though its premise is now dated.
**A 1.0 tagged before all three land would be a version
number asserting something nobody checked**, which is the exact defect this file
was written to prevent.

The older assessment, kept because it is still the honest description of the
code:

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

- [x] **Finding 3, the vault key boundary.** Half done, 2026-09-10, and the
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
      **Closed 2026-09-13 by the second route, and the key has not moved.** The
      keychain is still unreachable for the reason above, and trading the
      `os/exec` ban for one finding is a worse deal than the finding is worth.
      The README has carried the sentence since 2026-09-10, and that was the
      letter of this gate and not the spirit: **the binary never said it.**
      Somebody who turns on `--mask` because they want secrets protected is
      exactly the person who has not read the footprint section of a six
      hundred line README, and a disclosure that lives only in documentation is
      aimed at the reader who already agrees with it.
      It is now said twice, in `cmd/replay/vaultdisclosure.go`: on the `-mask`
      flag, which is the last moment the user can decline, and under the running
      `masking: on` banner, because that line is what an operator reads as the
      answer to "are my secrets safe" and a qualification belongs beside the
      claim it qualifies. It stays silent when masking is off, because warning
      about a vault the run did not create is how a real disclosure stops being
      read. Three tests, each mutated red.
      **This closes the gate and does not close the finding.** The key still
      sits beside the ciphertext. What changed is that nobody can now turn
      masking on without being told.
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

### Measurement, which is a standing rule rather than a gate

**Restructured 2026-09-13.** These two were checkboxes and could never be
ticked, because they do not describe work that finishes. They bind every figure
this project publishes from now on, including figures that do not exist yet, so
a box beside them would either stay open forever and block 1.0 on nothing, or
be ticked dishonestly the first time somebody wanted to ship.

They are the reason the gates above exist rather than items among them. A
release cannot satisfy them; a release can only be shipped by somebody
currently obeying them.

- **No headline figure without the instrument that produced it being
      checked first.** This release exists partly because a 98.8% claim was
      shipped from a classifier that compared each agent lane against a
      different one. The rule is not "measure more", it is: before a number
      goes in a README, a commit message or a card, something must have tried
      to falsify the instrument.
- **The same rule binds commercial figures, and until 2026-09-13 it did
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

### The reproduce gate: what it checks and what it does not

Added 2026-10-13 with `scripts/reproduce-release.sh` as release.yml runs it.
It is a check after publication. A release that fails it is already public and
must be pulled or superseded; nothing here stops a bad release from going out.

**What it compares.** For each of linux/amd64, linux/arm64, darwin/amd64 and
darwin/arm64:

- The tag is peeled to its commit in a fresh clone of github.com/RedRobotKK/Replay.
  The commit time and the go.mod `toolchain` line come from that commit. A tag
  whose go.mod has no toolchain line, which is every tag before v0.6.0, is
  refused as ambiguous.
- The archive must match its own `checksums.txt` entry, listed exactly once.
- The archive must be in the plain form goreleaser writes: one gzip member
  holding ustar regular files with plain relative names, no PAX, GNU or global
  headers, no duplicates, and nothing but padding after end-of-archive. Every
  release archive is in that form (v0.9.0, all four platforms, read
  2026-10-14). An archive outside it can mean different things to different tar
  readers, and the gate can only check agreement with the tar on its own
  machine, so it refuses the form rather than trusting the agreement. tar and
  Go must also list exactly the same members.
  **Corrected 2026-10-14, not deleted: plain is a property of the bytes, not
  of the values a reader returns.** The check above read each header with Go's
  reader and judged what it returned, and Go's reader normalizes. A size field
  written with a leading NUL read as the full size to Go and as 0 to bsdtar, so
  one archive listed the same names to both while bsdtar extracted a planted
  replay hidden inside another member's content (found by the v3.5 review).
  Every header must now be, byte for byte, the block Go's writer, which
  goreleaser uses, produces for the values Go read; each member's content must
  sit where Go read it, with zero padding; and the archive must end in two zero
  blocks followed only by zeros. 23 single-field re-encodings that Go reads as
  the canonical values are each refused (scripts/archive-member/main_test.go).
- The member compared is the one `replay upgrade` installs.
  `scripts/archive-member` calls `selfupdate.Unpack`, the function upgrade
  runs, so the gate does not have its own reading of the archive. tar must also
  list exactly one member whose base name is `replay`, named exactly `replay`,
  and extract the same bytes, because install.sh uses tar. Two readers agreeing
  on the bytes is the condition; either one alone is not.
- That binary's build stamps (commit, commit time, toolchain, unmodified tree)
  must agree with the tag before anything is built.
- The rebuild uses goreleaser's flags, under a stated Go environment and a new
  build cache, with the tag's toolchain. REPRODUCES means the two binaries are
  byte-identical.

**Exit codes.** 0 means the bytes match. 1 means they differ, and the
difference is printed. 2 means nothing was compared, because an input was
missing, malformed, ambiguous or disagreed with the tag. 2 is never a pass, and
any other non-zero exit is a failure.
**Corrected 2026-10-14, not deleted.** A pass is exit 0 together with the
`REPRODUCES:` line, not exit 0 alone: under bash as sh, a caller's
`SHELLOPTS=noexec` or `onecmd` makes the shell exit 0 before any check runs
(below). Exit 1
also comes from a missing argument and from a command that fails under `set -e`;
neither is ever a pass.

**The environment.** The script runs itself again under an allow-list if its
environment holds anything else. What is kept, and why:

- `PATH` finds sh, git, gh, tar and go. It is not narrowed: whoever controls
  PATH controls every tool the gate runs. In CI the runner sets it.
- `HOME` holds gh's stored login when the script is run by hand. git reads no
  user or system configuration (`GIT_CONFIG_GLOBAL=/dev/null`,
  `GIT_CONFIG_NOSYSTEM=1`), and go runs under its own allow-list.
- `GH_TOKEN` is the read-only token release.yml passes for the download.
- `LC_ALL=C` makes grep and awk match bytes, whatever the caller's locale.

Everything else is dropped, including the `GIT_*`, `GH_*`, `TAR_OPTIONS`,
`GREP_OPTIONS` and Go variables that were each shown on 2026-10-13 to change
what the gate fetched, read or built. gh is given an explicit `github.com` host.

**Corrected 2026-10-14, not deleted: exported shell functions.** Where sh is
bash (macOS `/bin/sh` is bash 3.2), bash imports the functions a caller
exports, and a function shadows any command or regular builtin of its name. A
function named `env` answered the allow-list check with a clean listing, and
`read` or `[` could end it early, so the re-run was skipped and a hostile
`GIT_CONFIG_*` reached gh and git (found by the v3.5 review). Until the re-run
the script now uses only reserved words, special builtins and `/usr/bin/env` by
path: bash cannot import a function whose name holds a slash, and when it runs
as sh a function does not replace a special builtin. The re-run starts under
`env -i`, with no functions. dash imports none.

**The caller's shell options.** bash as sh also enables what a caller's
`SHELLOPTS` lists before the first line runs, and with `xtrace` it expanded
the caller's `PS4` before every command: `PS4='$((clean=1,path_seen=1))'`
rewrote the allow-list verdict and the run printed REPRODUCES with a hostile
`GIT_CONFIG_*` reaching gh and git, and the trace printed `GH_TOKEN` (found by
the v3.6 review). The first line now turns tracing off with `set +vx`, and a
`SHELLOPTS` or `BASHOPTS` in the environment is refused with exit 2 instead of
re-run, because `PS4` is still expanded once, for that first line, and a
refusal is the only outcome that one expansion cannot turn into a pass. What
remains, stated rather than implied:

- `SHELLOPTS=noexec` and `SHELLOPTS=onecmd` make bash as sh exit 0 having run
  nothing: noexec runs no line, and onecmd reads one line, the `#!` line, which
  is a comment. Both act before any line can refuse (dash ignores both; the
  other 25 of the 27 options bash 3.2 accepts there are refused). That is why a pass is exit 0 together
  with the REPRODUCES line. release.yml's step still decides on the exit status
  alone; it runs on Ubuntu, whose sh is dash, so neither option reaches it.
- The one `PS4` expansion runs as the caller, before the gate starts. It cannot
  make this run pass, but anything it starts in the background runs with the
  caller's own rights, as anything on PATH does.
- Run as `bash scripts/reproduce-release.sh`, outside posix mode, a function
  can replace even `set` and `exit`, and no script can defend itself from that.
  It is run as sh: by release.yml, by its own re-run, and in these documents.
- When run by hand, gh reads its configuration under `HOME`.

**The workflow guard.** `internal/regression` pins the `reproduce` job line for
line (RR3), requires one comparison per platform goreleaser builds (RR2), and
holds release.yml to a canonical form it can read completely (RR6). RR6 is not
a YAML parser: this module has no YAML library. Outside `run:` script bodies it
refuses tabs, explicit keys, merge keys, anchors and aliases, flow mappings,
quoted keys and keys with a space before the colon, and it requires exactly
the top-level keys `name`, `on`, `permissions` and `jobs` and exactly the jobs
`release` and `reproduce`, each once. It does not check what GitHub's own
parser would accept, which was not run.

**Required CI.** A pull request that changes the gate is accepted only when
every required check finished successfully on the exact head commit, including
the blackbox job that runs the crafted-archive corpus and the hostile
environment tests under the runner's GNU tar. A skipped, cancelled or neutral
check is not a pass. A cancelled release run shows grey, not red, and nothing
here turns that into a failure.

**Not covered, stated rather than implied.**

- GNU tar has not run the archive corpus: no machine this was written on had
  it. By construction a disagreement with Go can only refuse, because tar's
  bytes and member list must equal Go's, but that has not been observed on GNU
  tar. The gate's shell has: the corpus, gate and hostile-environment tests
  pass with dash as sh, as on the Ubuntu runner (2026-10-14).
  **Corrected 2026-10-14, not deleted.** "Can only refuse" held only for the
  tar on the gate's own machine. release.yml checks all four platforms on
  Ubuntu with GNU tar, while install.sh on macOS extracts with bsdtar, so an
  archive the two tars read differently could pass there and install other
  bytes here. What closes that is the byte form above, not the agreement
  checks: an archive whose every header is the canonical ustar block leaves a
  conforming reader nothing to resolve its own way. That GNU tar and bsdtar
  read every such archive alike is reasoned from the ustar format, not
  observed; bsdtar was run, GNU tar was not.
- The corpus is 36 crafted archives, plus a seeded differential fuzz
  (TestPDF_ in internal/blackbox) that generates archives from the same
  constructs. On 2026-10-13 it ran 1,500 archives on seed 20261013 and 8,000 on
  seed 7 with no violation, and found the two reintroduced defects it was
  checked against. It is evidence about the archives those seeds produce on
  bsdtar, not a proof that the readers agree on every archive.
- The script that checks a tag is the one in that tag's checkout.
- Release assets can still be replaced after the check runs.
- The archives' other bytes, the Linux packages, which carry the same binary,
  and the SBOMs are not compared. The signature on `checksums.txt` is the
  separate signing gate and is not checked here.

### Named in the preamble, not yet met

Added 2026-10-13 so that counting the boxes in this file gives the right
answer. These are the three items the 1.0 definition in docs/ROADMAP.md names,
whose state is the table at the top of this file.

- [ ] **An external security review, published.** Not commissioned.
- [ ] **Signed, reproducible releases.** Signed: yes. Reproducible: checked by
      the release workflow on every tag from 2026-10-13, not yet run on a tag.
- [ ] **Caching rules for a second provider, calibrated against real traffic.**
      Rules and dispatch exist; no calibration corpus.

### Dated evidence this supersedes, left as recorded

Added 2026-10-13. Two dated files describe `scripts/reproduce-release.sh` as it
was before the release workflow ran it, and are not edited:

- `docs/evidence/reproducible-build-2026-09-13.md` says to repeat its result
  with `./scripts/reproduce-release.sh v0.5.4 darwin arm64`. The script now
  takes the toolchain from go.mod at the tag and refuses a tag with no
  toolchain line, v0.5.4 among them, so that command exits 2. The result stands
  as recorded; repeating it needs the script at its 2026-09-13 version.
- `docs/evidence/release-runbook-1.0-2026-10-09.md` says the script reads the
  toolchain from the published binary. It now reads the toolchain from go.mod
  and the commit from the tag, requires the published binary to agree, and runs
  in `release.yml`'s `reproduce` job after publication. The runbook's step 4.5
  loop is unchanged.
- Added 2026-10-15. The same runbook's step 4.5 verifies `checksums.txt` with
  `--certificate-identity-regexp 'https://github.com/RedRobotKK/Replay/.*'`.
  cosign matches that pattern unanchored, and it accepts a certificate from the
  repository's `ci.yml` as readily as one from `release.yml` (PR #339, which
  replaces it in `install.sh` and `replay upgrade`). Verify with the exact
  identity the release footer gives instead:
  `--certificate-identity 'https://github.com/RedRobotKK/Replay/.github/workflows/release.yml@refs/tags/<tag>'`
  and the same `--certificate-oidc-issuer`. The rest of step 4.5 is unchanged.

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

[Changelog](CHANGELOG.md) · [Security review](docs/evidence/security-review-2026-09-04.md) ·
[External review scope and provenance correction](docs/evidence/security-review-scope-2026-10-08.md)
