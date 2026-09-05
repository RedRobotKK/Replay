# The scoped-rehydration boundary, under attack

**2026-09-05.** An adversarial matrix against the check that decides whether a real
credential may be written to a path an agent chose. Harness:
[`internal/masking/sandbox_test.go`](../../internal/masking/sandbox_test.go).

## The threat

Not a local attacker, who has already won. A **poisoned agent**: content the model read
from a web page, a dependency, or a tool result persuades it to write a secret somewhere it
should not. Every vector below is something such an agent can express in an ordinary tool
call, with no special access.

The boundary fails closed. A wrong rejection leaves a placeholder in a file, which is
visible and someone notices. A wrong acceptance writes a live credential outside the
project, which is neither.

## Results

Seven vectors, run on darwin/arm64.

| Vector | What the agent submits | Result |
|---|---|---|
| Dot-dot traversal | `src/../unsafe/shadow.env`, and three other spellings | refused |
| Symlink out of the project | a link it created inside the project, written through | refused |
| Escape through a path that does not exist yet | `src/ghost/../../unsafe/x` | refused |
| A new in-project file | `src/new/file.go` | **allowed**, or the boundary would be unusable |
| A placeholder in the path itself | `src/REPLAY_SECRET_…` | refused |
| Project root reached through a link | the ordinary macOS `/var` → `/private/var` case | **allowed**, both directions |
| Case variance on a case-insensitive filesystem | `SRC/app.js` against a root of `src` | refused, fails closed |
| Decoy path field beside the real one | `{"file_path": in, "path": out}` | refused |

## What each defence is actually doing

Recorded because the harness answered a question the code's comments did not. The check was
weakened to the naive version a reasonable person would write, a `strings.HasPrefix` after
`filepath.Clean`, and the suite was re-run:

```text
--- FAIL: TestBoundaryRefusesASymlinkOutOfTheProject
--- FAIL: TestBoundaryRefusesAPlaceholderInThePath
      (dot-dot traversal still passed)
```

**`filepath.Clean` alone defeats lexical traversal.** `src/../unsafe/x` collapses to
`unsafe/x` before any comparison, so `..` is not what the symlink resolution is for.

**`EvalSymlinks` is what stops the real escape.** A link the agent created inside the
project is textually in-project and stays in-project through `Clean`. Only resolving it
reveals where the write lands. That is the vector a prefix check misses, and it is the one
an agent can set up with a single earlier tool call.

**The placeholder check is separate again**, and neither of the other two would catch it.

`filepath.Rel` with a `..` test, rather than a string prefix, is what stops
`/project-evil` matching a root of `/project`. That case is structural rather than
adversarial and is covered by the same `Rel` result.

## The case-variance decision

On macOS, writing to `SRC/x` writes to `src/x`, so a case-insensitive comparison would be
**more permissive** than a case-sensitive one. The boundary refuses, and the test pins that
it refuses.

This is a deliberate choice against convenience. Widening the comparison to match the
filesystem would restore a placeholder for a path the operator did not name, in exchange
for avoiding an unrestored placeholder in a file. Those costs are not symmetric.

## Limits of this harness

**A path whose tail does not exist cannot be resolved**, only its existing prefix can. An
agent that submits a path through a link it has not created yet, then creates the link
before the write, is not caught here. Closing that needs the write to happen under a handle
the check opened, which the proxy does not do: it never touches the filesystem, it only
decides what bytes to forward.

**This tests the boundary, not the whole pipeline.** That a path is out of scope is one of
seven denial reasons, each covered separately in
[`reasons_test.go`](../../internal/masking/reasons_test.go).

**One platform.** Run on darwin/arm64; the case-variance case is skipped on case-sensitive
filesystems, and CI runs the rest on Linux and Windows too.

---

[Evidence](README.md) · [Documentation index](../README.md)
