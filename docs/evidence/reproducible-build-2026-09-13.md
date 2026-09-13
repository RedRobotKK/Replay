# v0.5.4 reproduces byte for byte, on all four platforms

**Read 2026-09-13.** `RELEASE-CRITERIA.md`'s second 1.0 gate said:

> Signed, yes. **Reproducible is unverified.** Nobody has rebuilt a published
> tag and compared the bytes, and a reproducibility claim nobody has tried to
> falsify is the class of claim this project refuses elsewhere.

Somebody has now, and it was tried as a falsification rather than a
demonstration.

| Platform | Published sha256 | Rebuilt | |
|---|---|---|---|
| darwin/arm64 | `85ebb258…4762f` | identical | **reproduces** |
| darwin/amd64 | | identical | **reproduces** |
| linux/amd64 | | identical | **reproduces** |
| linux/arm64 | | identical | **reproduces** |

Repeat it with `./scripts/reproduce-release.sh v0.5.4 darwin arm64`. It exits 0
only on a byte match, and on a mismatch it prints both binaries' build info
rather than a summary.

## The three things that decide it, and two of them are not obvious

**The toolchain is read out of the binary, never assumed.** The published
artifact records `go1.24.13`. Building the same source with go1.27.1 produces a
different, equally valid binary. The script parses the version from
`go version -m` for this reason: guessing it is the difference between a
reproduction and a disagreement nobody can explain.

**A clone, not a worktree.** This is the finding that cost the most time. Under
go1.24, building in a linked `git worktree` stamps the main module as `(devel)`,
and the bytes differ. The identical command in a fresh clone stamps `v0.5.4` and
reproduces exactly. The compiled code is the same either way: both binaries were
**8,364,258 bytes** and only the embedded build-info blob differed.

A reproduction attempt run in a worktree therefore fails for a reason that has
nothing to do with the code. That is the sort of false negative that gets a true
reproducibility claim quietly abandoned, and it is the reason this file exists
rather than a sentence in a README.

**The ldflags must carry the same three values**, taken from the binary's own
vcs stamps rather than from the current clock or the current checkout.

## A correction, because the earlier belief was wrong

An earlier attempt in this repository concluded that v0.5.4 does **not**
byte-reproduce, and that the delta was embedded provenance: `(devel)` instead of
`v0.5.4`, and missing vcs stamps. That conclusion was never written down, which
is the only reason it did no damage.

It was half right and wrong where it mattered. Those really are the fields that
differed, and they differed **because the build ran in a worktree**, not because
the release is irreproducible. The observation was correct and the inference
from it was not: a difference in build metadata was read as a property of the
release when it was a property of the checkout.

## What this does and does not close

It closes the falsification: the claim has been tested against the published
bytes, on every platform shipped, by a script anybody can run.

It does not make reproducibility a standing property. **Nothing runs this on a
new tag.** Until something does, this is a fact about v0.5.4 on 2026-09-13 and
not a fact about the next release, which is the same distinction every other
figure in this directory carries.

The honest state of the 1.0 gate is therefore: **verified once, with a repeatable
procedure, not yet enforced.**

---

[Evidence index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md) ·
[The script](../../scripts/reproduce-release.sh)
