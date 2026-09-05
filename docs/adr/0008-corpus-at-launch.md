# ADR-0008: Collecting a corpus from a public launch without shipping telemetry

- **Status:** Proposed. **Not implemented** — see the implementation note of 2026-09-05 at the foot of this record
- **Date:** 2026-09-04
- **Amends:** ADR-0007 (federated calibration corpus)
- **Related:** [`architecture/multi-provider.md`](../architecture/multi-provider.md)

## Context

ADR-0007 says no aggregation before the twenty-session gate is met honestly. **A public release is the
only realistic way to reach twenty independent sessions.** The gate blocks the thing that satisfies
the gate, and that circularity has a deadline attached: a first release gets one wave of curious
users, and if there is no way for them to contribute, that wave produces nothing but downloads.

The opposing risk is sharper than it looks. **Replay is positioned on not phoning home.** It sits in
the request path, its README says nothing leaves the machine, and its whole pitch is that you can
read the 218-line installer before running it. A tool with that posture that ships anything
resembling telemetry does not get a debate on launch day. It gets one comment near the top of the
thread and the positioning never recovers.

So the question is not whether to collect. It is how to collect in a way that is still true to the
sentence "nothing leaves your machine".

## Decision

**Ship the contribution mechanism at v0.1. Do not ship an aggregate, and do not publish one until the
gate is met.** Those are separate things and ADR-0007 conflated them.

Four properties, and the design fails if any one is missing.

### 1. It is a command a person runs, never a default and never a prompt

**Daniel proposed asking at install, since we control the installer. Three reasons it lands one step
away, and one reason he is right anyway.**

`curl | sh` has no terminal. Stdin is the pipe from curl, so a prompt cannot read at all without
grabbing `/dev/tty`, which breaks CI and unattended installs and reads as a dark pattern in the one
file people audit line by line on launch day.

**Consent at install is consent to data that does not exist yet.** The user has not run the tool, has
no corpus, and has never seen the report. Agreeing to an unseen payload is precisely what property 3
exists to prevent.

Homebrew is the worked example: analytics on by default, years of argument, eventual reversal.

**But the underlying point stands and it is the strongest objection to this ADR.** A command nobody
is prompted to run is a command most people never run, and "read the docs" is not a distribution
plan. So the installer takes the half of the job it can do honestly: **discovery, not consent.** Its
last lines name the command, state that Replay makes no network request except to the configured
provider, and stop. Deciding happens later, with the report on screen.

`install.sh --corpus-opt-in` exists for the other case: someone who has already read this and wants
to say yes once, non-interactively, across a fleet. It writes a config file and sends nothing;
`replay corpus --submit` is still the only thing that transmits.

No first-run question, no opt-out, no config flag that quietly enables it, no "anonymous usage
statistics" checkbox during install. **The only way a byte leaves is that someone typed
`replay corpus --submit` and confirmed.**

This is stricter than opt-in. Opt-in still means the software raised the subject at a moment when the
user wanted something else. Here the software never raises it; the docs do.

### 2. The local report has to be worth running for its own sake

`replay corpus` already tells the operator things they want to know about their own machine: the
match rate, which models are calibrated, whether any session fell below the threshold, and the bounds
observed against what the rules claim. **That report is useful with no network involved**, and it is
what makes this a shared local tool rather than a data-collection funnel with a report bolted on.

The submission is a second, deliberate step on a report they already have on screen.

### 3. The payload is shown, not described

`--submit` prints the exact bytes and waits. Not a summary, not a link to a privacy policy: the
document. A user who reads the 218-line installer will read a 40-line report, and if they cannot see
what leaves, the earlier promise was worthless.

Session id prefixes are stripped at this step, per ADR-0007.

### 4. The aggregate is published back, publicly, or this is extraction

**The contributor gets the corpus.** Their fit parameter against the corpus median. Whether their
prefix bound agrees with everyone else's. Whether their client version breaks caches more than
others do.

And the aggregate is **public**: committed to `docs/evidence/` where anyone can read it, including
people who never contributed and the provider whose behaviour it describes. **A calibration corpus
held privately is a commercial asset built from other people's traffic. Published, it is a commons
and the incentive to fabricate submissions mostly evaporates**, because there is nothing to capture.

This is the model Go's telemetry settled on after a long public argument: collect locally, show the
user, upload only on an explicit act, and publish the result so the data is a public good rather than
a vendor's private advantage. **The argument was won by publishing, not by promising.**

## What ships at v0.1, concretely

- `replay corpus <dir>` unchanged: a local report, no network.
- `replay corpus <dir> --submit`: prints the payload, asks, sends on yes.
- `replay corpus --show-aggregate`: fetches the published aggregate and prints the contributor's
  own figures beside it. Read-only, sends nothing.
- The README says in plain words that Replay makes no network request except to the provider you
  configured, unless you run `--submit`.

## What does not ship at v0.1

- **No aggregate figures anywhere**, in docs or in the tool, until k independent contributors have
  reported a model. Until then `--show-aggregate` says "not enough contributors yet" and names the
  number. **Publishing a median over three machines would be the same dishonesty as claiming twenty
  sessions we do not have.**
- No learned rules file derived from the corpus. That stays a pull request with evidence, per
  ADR-0007.

## Consequences

**Good.** The launch wave becomes the corpus instead of being wasted on it. The gate can be met
honestly and in public, with the count visible the whole time. And the tool's central promise stays
literally true.

**Costs.** Take-up will be low, because a command nobody is prompted to run is a command most people
never run. **That is the price of the positioning and it is worth paying**: a 2% submission rate on a
tool people trust beats a 60% rate on one they resent, and the second number is not available to us
anyway once the launch thread has happened.

**The honest failure mode.** Nobody submits, the corpus stays at eleven sessions from one laptop, and
every report keeps saying so. That is a worse product and an intact reputation, which is the correct
way round for a tool whose entire value is that its numbers can be believed.

## Open

The value of k. Whether `--show-aggregate` is worth building before there is an aggregate to show.
And whether the endpoint should exist at all at v0.1, or whether submission should be a pull request
against `docs/evidence/` for the first cohort, which is slower, harder to game, and requires no
server to run or defend.

## Implementation note 2026-09-05

**None of "what ships at v0.1" shipped.** `replay corpus <dir>` shipped exactly as described — a
local Markdown report over transcript directories, no network — and that half of the argument holds.
The rest did not: `corpus` defines no flags, so neither `--submit` nor `--show-aggregate` exists,
and `replay corpus --submit` exits with `flag provided but not defined: -submit`. There is no
endpoint to send to and no published aggregate to fetch.

`install.sh --corpus-opt-in` does exist and still writes
`${XDG_CONFIG_HOME:-~/.config}/replay/corpus-consent.toml`. It records an intention and nothing
reads it, which makes it, for now, a file that consents to a mechanism that was never built.

The installer's closing lines and the README have been corrected accordingly. They no longer name a
submit command, and they say the thing this ADR was reaching for in a stronger form than the ADR
could: **the released binary makes no network request at all**, except the proxy forwarding your own
traffic to the provider you configured. "Nothing is sent until you run `--submit`" understated it,
and any claim that undersells a privacy property is still an inaccurate claim.

The reasoning above stands as the standard a future build has to meet. It is a decision record, not
a feature list.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
