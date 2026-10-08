# v0.8.0 reproduces byte for byte, on all four platforms

**Read 2026-10-08.** RELEASE-CRITERIA.md's reproducibility gate was verified
once, on 2026-09-13, against v0.5.4, and explicitly recorded as **not yet
enforced on a new tag**: nothing in CI runs `scripts/reproduce-release.sh`,
so that earlier result was a fact about one release rather than a property
of releases.

This is a second, independent falsification attempt, run on a different
machine session, against the most recent tag actually published at the time
(`v0.8.0`, commit `5c87562`), on all four platforms the project ships:

| Platform | Published sha256 | Rebuilt sha256 | |
|---|---|---|---|
| darwin/arm64 | `87a67566234154c73314d6be778bb93e45e2d490062727a8826db43dff6f1f1d` | identical | **reproduces** |
| darwin/amd64 | `a723ab670bf74173e322e2bc8f1ee599122686098290a0b5435d4b606a154a95` | identical | **reproduces** |
| linux/amd64 | `549ba46fb6b10668c80916a9d3ade40b9ae741c9d84e3ea410fc35742bab2cd2` | identical | **reproduces** |
| linux/arm64 | `37c70543dc4e44ee8862563dc0d51e7900b20de21fd1ae82fe7f4304e2c046aa` | identical | **reproduces** |

All four built by `go1.25.13`, commit `5c87562`, commit time
`2026-10-05T06:20:34Z`, exactly as stamped on the published artifact. Produced
with:

```sh
./scripts/reproduce-release.sh v0.8.0 darwin arm64
./scripts/reproduce-release.sh v0.8.0 darwin amd64
./scripts/reproduce-release.sh v0.8.0 linux amd64
./scripts/reproduce-release.sh v0.8.0 linux arm64
```

Each invocation exited 0 and printed `REPRODUCES: ... is byte-identical.`

## What this adds, and what it does not

**It adds a second data point on a different release**, closer to today,
confirming the 2026-09-13 result was not a one-off: the machinery still works,
unmodified, against a tag it was never specifically run against before today.

**It does not close the gate.** `scripts/reproduce-release.sh` is still not
wired into any CI workflow (`grep -rl reproduce-release .github/workflows/`
finds nothing), so this remains a fact about v0.8.0 on 2026-10-08 and a fact
about v0.5.4 on 2026-09-13, not a standing property of every release. Most
importantly, **this is not v1.0.** No v1.0 tag exists yet at the time this
file is written. When the actual v1.0 tag is created and the real release
build runs, the reproduction has to be performed again, against that tag
specifically, and its own evidence recorded separately. A reproduction of
v0.8.0 is evidence that the procedure works, not a substitute for running it
against v1.0.

The honest state of the gate is therefore: **verified twice, on two different
releases, with a repeatable procedure, still not enforced in CI, and not yet
run against any v1.0 tag because none exists.**

---

[Evidence index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md) ·
[The script](../../scripts/reproduce-release.sh) ·
[The first verification, v0.5.4](reproducible-build-2026-09-13.md)
