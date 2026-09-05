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
| Case variance, out-of-project | `UNSAFE/shadow.env` in a different case | refused on every platform |
| Case variance, in-project | `SRC/app.js` against a root of `src` | platform-dependent; see below |
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

## The case-variance finding, which the harness produced on its first CI run

The first version of this suite asserted that case variance must fail closed. It passed on
darwin and **failed on windows**, which is the harness doing its job on a platform nobody
had checked by hand.

```text
darwin   filepath.Rel is case-sensitive    Rel(".../src", ".../SRC/app.js") = "../SRC/app.js"  refused
windows  filepath.Rel is case-insensitive  Rel(`..\src`,  `..\SRC\app.js`)  = "app.js"        accepted
```

**Windows is the correct one, and the assertion was wrong.** Both filesystems are
case-insensitive, so `SRC/app.js` and `src/app.js` are the same file, and that file is
inside the project. Darwin over-refuses. The cost of over-refusing is an unrestored
placeholder in a file; nothing escapes on either platform.

So case-sensitivity is not the property worth asserting, and asserting it would have pinned
darwin's accident as a requirement. The test now asserts what actually matters on every
platform, that a path resolving **outside** the project is refused whatever its case, and
logs which comparison the platform performed rather than demanding one.

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
