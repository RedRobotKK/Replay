# 24. Deprecation is a promise that has to be made before 1.0

**Status:** Proposed
**Date:** 2026-09-13

## Context

There are **31 dispatch verbs** in `cmd/replay/main.go` and no stated procedure
for retiring one. `docs/ROADMAP.md` names the gap and says nothing about how to
close it.

The timing is the whole decision. A version number is a promise about
compatibility, and tagging 1.0 converts all 31 verbs into a promise held by one
person, indefinitely, with no agreed way to hand any of it back. **After 1.0 the
policy cannot be written, because writing it then is itself a breaking change:
whatever it says, somebody's script is already relying on the absence of it.**

Two facts make this concrete rather than theoretical.

`cmd/replay/command_table_test.go` requires every verb to carry a section in the
command guide. So each verb is not one promise but two, and the documentation
obligation grows with the surface exactly as fast as the code does.

The exit codes were frozen on 2026-09-13 and published, which makes them the
first compatibility surface in this project written down before it had users.
This ADR is the second, and the two are the same argument: a contract stated
before anyone depends on it is a decision, and the same contract stated
afterwards is a concession.

## Decision

**A published surface is removed only after a full minor release in which it
still works and says it is going away.**

The surfaces this covers, which are the ones the version number covers:

- dispatch verbs and their flags
- the exit codes
- `--json` output shapes
- the corpus submission schema, the pool document and the budget artefact
- the ledger format and the policy file

### The procedure

1. **Announce in a release.** The verb keeps working. It prints one line to
   **stderr**, never stdout, naming the release in which it will be removed and
   what to use instead. Stderr because stdout is somebody's pipe, and a
   deprecation notice that corrupts a JSON parse is a breaking change wearing a
   warning's clothes.
2. **Wait a full minor release.** Not a patch. A user who upgrades within a
   patch series did not choose to read a changelog.
3. **Remove it, and say so in the changelog entry for that release.**
4. **A removed verb exits 1 with a message naming its replacement**, for one
   further minor release. An unknown-command error is the right answer
   eventually and the wrong answer immediately, because it tells a reader they
   made a typo when they did not.

`--json` shapes follow the same clock with one addition: **a field may be added
at any time and may not be removed or repurposed without it.** That asymmetry is
already how the corpus schema behaved through two changes, and it is why those
changes cost nothing.

### What this does not cover

Anything that has never been published: unreleased flags, internal packages,
anything behind a build tag, and output explicitly marked experimental. The
`EXPERIMENTAL, UNMASKED` label on the OpenAI-compatible path means exactly this,
and the point of saying it loudly is to keep that path outside this promise.

### The rule that makes it affordable

**Adding a verb is the expensive decision, not removing one.** Thirty-one verbs
each carrying a guide section, a compatibility promise and a removal cost of two
releases is a standing obligation for one maintainer, and the honest way to
manage it is to add fewer rather than to deprecate faster.

So: a new verb needs an argument for why it is not a flag on an existing one.
That argument belongs in the pull request that adds it.

## Consequences

The obligation is now bounded and dated instead of unbounded and implicit. A
user can read what a deprecation will look like before they build a script on a
verb, which is the thing they cannot do today.

It also costs something real, and pretending otherwise would undercut the point.
A verb that turns out to be a mistake now takes two releases and a stderr line
to remove rather than one commit. That is the price of the promise, it is paid
by the maintainer rather than the user, and it is the correct direction for that
cost to run.

**The surface should be reduced before 1.0 rather than after**, because until
1.0 this procedure does not apply and removal is still free. Thirty-one verbs is
a decision nobody made; it is the sum of thirty-one decisions each of which was
individually reasonable.

---

[ADR index](README.md) · [Roadmap](../ROADMAP.md) ·
[Release criteria](../../RELEASE-CRITERIA.md)
