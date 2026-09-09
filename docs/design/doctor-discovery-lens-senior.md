# Senior lens: `replay doctor` agent discovery

Reviewer stance: staff engineer reviewing a colleague's PR, intending to be
useful rather than agreeable. Branch `feat/doctor-agent-discovery`, reviewed
2026-09-09 against `cmd/replay/discover.go`, `cmd/replay/discover_test.go`, and
the `agents` block at `cmd/replay/doctor.go:67-89`.

Everything below was run. Experiments were performed on a copy of the tree at
`/tmp/rlens`; no file in the repository was modified by this review except this
one. The feature was reviewed twice, because `discover.go` and
`discover_test.go` changed on disk mid-review — the multi-root Codex fix and
`TestDS6` landed while the first pass was running. **All measurements and all
mutation results below are against the current code**, not the first draft.

Verified green before anything else: `go build ./cmd/replay` clean, `go vet
./cmd/replay` clean, `gofmt -l` clean on all three files, `go test ./...` clean.

---

## Verdict

| | |
|---|---|
| Block | B1 symlinked store is silently invisible; B2 `500+` is not honest at the boundary or for multi-root stores; B3 the suite does not test the rule it says it exists to hold |
| Merge with a comment | `const cap` shadows a builtin (likely CI lint failure); Cursor's root is wider than its own claim; discovery has no timeout where the rest of `doctor` does; three test hygiene items |
| Good, no notes | the `SkipAll` cap logic itself, symlink-loop safety, the `n == 0 → no line` rule, the `Priceable` distinction, `TestDS6` |

The design instinct is right and the writing is unusually good. The defects are
all in the same place: the feature's stated standard is honesty about what was
measured, and three of its claims are not yet measurements.

---

## 1. Correctness and failure modes

### 1.1 The cap boundary — the arithmetic is right, the label is not

`countStoreFiles` (`discover.go:147-169`) is correct in isolation. Measured:

```text
files on disk=499  counted=499
files on disk=500  counted=500
files on disk=501  counted=500
files on disk=600  counted=500
```

`n >= cap` is evaluated on entry to the next callback, so `SkipAll` fires one
entry after the cap is reached and `n` never exceeds 500. No off-by-one.

The dishonesty is one layer up. `doctor.go:79-84` labels on the *value*:

```go
if f.Files >= 500 {
    count = "500+ files"
}
```

but since the multi-root change the cap is applied **per root**
(`discover.go:120-126` calls `countStoreFiles` once per `rel` and sums), while
the label is applied **per finding**. Measured, with a two-root Codex store:

```text
on disk= 500 (300 live + 200 archived)  Files= 500  doctor prints "500+ files"
on disk= 700 (400 live + 300 archived)  Files= 700  doctor prints "500+ files"
on disk=1200 (600 live + 600 archived)  Files=1000  doctor prints "500+ files"
on disk= 600 (600 live +   0 archived)  Files= 500  doctor prints "500+ files"
```

Three separate wrongs in four rows:

- **700 exact.** Neither root was capped. The count is complete and known, and
  the tool refuses to say it. `500+` where the honest answer is `700`.
- **500 exact.** A `+` that has not been earned. Nothing was truncated.
- **1200 on disk.** Both roots capped, so the ceiling is 1000, not 500. `500+`
  is true but useless, and the number it is derived from (`1000`) is an
  artefact of how many directories the store happens to be split across.

The comment at `doctor.go:81-82` — *"Reporting the cap as a total would be a
measurement that is really a limit"* — is exactly the right principle, and the
code implements the inverse: it reports a limit that is really a measurement.

Fix, five lines: `countStoreFiles` returns `(n int, capped bool)`; OR-fold
`capped` across the roots into the finding; label on the flag, not on the value.
Then 700 prints `700 files` and 1200 prints `1000+ files`.

### 1.2 A symlinked store is reported as absent — **the worst defect here**

`filepath.WalkDir` uses `Lstat` on its root and does not follow symlinks. It
resolves *intermediate* path components, so a symlinked parent is fine, but if
the final component of the store path is itself a link, the walk sees one
non-directory entry whose name matches no glob and returns 0. Measured:

```text
~/.grok symlinked to a real store (1 updates.jsonl)  -> discoverAgents = []
~/.codex/sessions symlinked to 3 rollouts            -> discoverAgents = []
a symlinked subdirectory inside a real store         -> counted 0 of 3
```

This is `TestDS2` inverted, and inverted is not milder. DS2 protects against
naming a tool the reader does not have; this names *nothing* for a tool the
reader has and uses heavily. The reader's conclusion is the same one DS2's
comment predicts — "this report is decorative" — and it is reached faster,
because a user who symlinked `~/.grok` onto an external volume knows perfectly
well that `~/.grok` has 4 GB behind it.

This is not an exotic layout. Dotfile managers (`stow`, `chezmoi`), iCloud/
Dropbox-synced config, and "my laptop SSD is full, the sessions live on the
external disk" all produce exactly this shape, and the last is *more* likely on
precisely the large stores this feature is for.

Fix: `if r, err := filepath.EvalSymlinks(root); err == nil { root = r }` before
the walk. Resolving only the root keeps the loop safety in 1.3 intact.

### 1.3 Symlink loops — safe, correctly, and by accident

A loop inside a store (`sessions/loop -> sessions`) terminates and counts
correctly (measured: `terminated, counted 1`), because `WalkDir` does not
descend links. Good — but it is the same property that causes 1.2, so whoever
fixes 1.2 must resolve *only the root* and not switch to a following walk. Worth
a comment in the code so the next person does not "fix" it into a hang.

### 1.4 Permission-denied subtree — defensible, but the store vanishes

Measured: a store whose only matching files sit under a `0o000` directory
returns `count=0`, and the finding is dropped entirely (`discover.go:127`). The
`err != nil → return nil` at `discover.go:151-153` is the right call for a
*partial* failure. For a *total* one it means an agent the reader has is absent
from the report with no signal at all, which is 1.2 again by a different route.
Not a blocker — silence is at least not a false claim — but the comment
"unreadable is not found" is doing more work than it admits: unreadable is not
*not* found either.

### 1.5 Network mounts: no timeout, in a command that times out everything else

`doctor` bounds its network probe at `doctorTimeout = 2 * time.Second`
(`doctor.go:22`). Its disk walk is unbounded in time. A store on a stale NFS or
SMB mount — `~/.grok` symlinked onto a NAS is the same user as 1.2 — makes
`replay doctor` hang with no output and no way to skip, before the proxy section
that would have told the user what is broken. This is the command people run
*because* something is wrong.

I did not simulate a hung mount, so this is structural reasoning rather than a
measurement, and I flag it as such. The inconsistency is real regardless: the
same function bounds the cheap remote call and not the expensive local one. A
`context.WithTimeout` around discovery, printing `agents  discovery timed out`,
costs ten lines and removes the whole class.

### 1.6 `home` is consistent; the *roots* are not

`discoverAgents(home)` is fed from `os.UserHomeDir()` at `doctor.go:38`, the
same value that feeds `claudeConfigDir(home)` at `doctor.go:50`. That is
consistent, and taking `home` as a parameter is what makes the tests honest.
Nothing to fix.

The inconsistency is one level down, and the recent commit fixed the biggest
instance of it. Before the fix, discovery scanned `~/.codex/sessions` only while
`codexRoots` (`codex.go:23-32`) — the function behind the command discovery
prints on the very next line — also reads `archived_sessions`. Measured on this
machine: doctor said **29 files**, `replay codex` says **150 Codex session(s)**.
`TestDS6` now pins it and `doctor` now prints 150. Good catch, correctly tested,
and the right invariant in the test's comment: discovery must not undercount a
surface relative to the reader it points at.

What remains is that the invariant is asserted for Codex and unenforced for
everyone else. `Ollama → replay burn` has the same shape (`burn.go:64-67`
derives its own roots) and nothing checks that the two agree. That is a comment,
not a blocker — but the general form of DS6 is the test worth having.

### 1.7 The Cursor globs are wider than Cursor's own claim

`rels: [".cursor"]` with `patterns: ["*.db", "*.jsonl", "*.sqlite"]` matches
anywhere in the tree. Measured on this machine:

```text
matched under ~/.cursor            119
of those, under ~/.cursor/projects 118   (the transcripts)
the other one                      ~/.cursor/ai-tracking/ai-code-tracking.db
```

So the shipped feature prints `Cursor 119 files` while `discover.go:27-28`
states, twice, that the machine has **118** transcripts. The extra file is a
telemetry database, not a conversation, on a line that reads "conversation
transcripts only". It is one file and nobody will die, but this is a feature
whose entire doc comment is about not letting a checkable number be wrong.

The structural risk is larger than the current miss. `~/.cursor` also holds
`extensions/`, `plugins/`, `browser-logs/`, `agents/`, `skills/` — an
extension tree, where third-party extensions ship their own SQLite files. Today
that contributes zero. It is unbounded in principle and moves without warning on
someone else's release schedule.

Anchoring to `.cursor/projects` gives exactly the documented 118, removes the
telemetry DB, immunises the count against the extension tree, and cuts the walk
from 2500 entries to 926. Codex (`.codex/sessions`, `.codex/archived_sessions`)
and Ollama (`.ollama/logs`) are already anchored this way. Cursor is the outlier.

---

## 2. The tests

Six now exist. DS1–DS5 test behaviour, not implementation: they call
`discoverAgents(home)` against a fixture home and assert on the returned
findings, so they survive any refactor that keeps that signature. The failure
messages are the best I have read in this repo — each one names the consequence
rather than the assertion. DS6 goes further and asserts against `codexRoots`
rather than a literal, so it cannot drift from the reader it guards.

That is the good news. The bad news is what survives.

### 2.1 Mutation results

Each mutation was applied to a copy of `discover.go`, compiled, and run against
`go test -run 'TestDS[0-9]'`. Mutations that did not compile or did not apply
are excluded as invalid.

| Mutation | Result |
|---|---|
| **M1 — delete the glob match; count every file under the root** | **ESCAPES** |
| M14 — widen Cursor's patterns to `*` | ESCAPES |
| M10 — widen Codex's root to `~/.codex` | ESCAPES |
| M15 — add a bogus extra root to Grok | ESCAPES |
| M7 — count directories as files | ESCAPES |
| M4 — cap 500 → 3 (silent 99% undercount) | ESCAPES |
| M5 — remove the cap entirely (unbounded walk) | ESCAPES |
| M2 — mark Codex `priceable: false` | ESCAPES |
| M8 — rewrite Cursor's `reads` to claim per-turn usage | ESCAPES |
| M6 — report `Path` as `home` rather than the store dirs | ESCAPES |
| M3 — reverse the sort order | ESCAPES |
| M9 — report every known store unconditionally | caught (DS2, DS3, DS5) |
| M11 — discard the second root's count | caught (DS6) |
| M13 — inflate every count 100× | caught (DS6) |

Eleven of fourteen escape. Two of the three catches arrived with DS6, which is
also the only test that pins `Files` to an exact expected value — that is not a
coincidence, and it is the pattern the other five should copy.

### 2.2 The mutation that matters

**M1: delete the pattern loop at `discover.go:160-165` and increment `n` for
every file. All six tests pass.**

That is not a subtle mutation. It removes the entire notion of *evidence* — the
thing that distinguishes "an agent wrote data here" from "a directory with this
name exists" — and the suite does not notice, because every fixture home is
built so that the only files present are matching files. DS3 covers the *empty*
directory; nothing covers the **populated-but-irrelevant** directory.

In production, M1 means: `~/.cursor` containing only `argv.json` and
`cli-config.json` — which is what it holds for anyone who opened Cursor once,
looked at it, and closed it — prints `Cursor 2 files`, followed by two lines
about transcripts that do not exist. That is `discover.go:21-25` verbatim: *"a
dotfile outlives the tool that made it… names a tool the reader does not have."*
The file's first rule is unguarded, and it is unguarded in the exact direction
the file says destroys trust.

Current behaviour is correct — I checked: a store containing only `notes.txt`
returns no finding. It is simply held up by nothing.

### 2.3 The test that should exist and does not

```go
// DS7: files that are not this agent's evidence do not make it a finding.
//
// The rule DS2 and DS3 hold is "report a store only when its files were
// counted". DS3 covers the empty directory. The commoner case is the
// directory that is not empty and holds nothing of ours: ~/.cursor exists
// for anyone who opened Cursor once, and holds argv.json whether or not a
// transcript was ever written.
func TestDS7_NonEvidenceFilesAreNotAFinding(t *testing.T) {
    home := fakeHome(t, map[string]string{
        ".cursor/argv.json":            "{}\n",
        ".cursor/extensions/readme.md": "x\n",
        ".codex/sessions/x/notes.txt":  "x\n",
        ".ollama/logs/README":          "x\n",
    })
    if found := discoverAgents(home); len(found) != 0 {
        t.Errorf("a home whose agent directories hold no agent data reported %v; "+
            "the directory existing is not the evidence, the files are", names(found))
    }
}
```

One test, four lines of fixture, and it kills M1, M7, M10, M14 and M15 — five of
the eleven escapes, including the one that matters. It should have been written
before `countStoreFiles` was, and its absence is the reason the glob patterns
can be wrong (§1.7) without anything going red.

### 2.4 Three smaller test defects

- **DS5 is satisfied by the wrong field.** `discover_test.go:147` concatenates
  `Reads + " " + Next` before searching, so `Next`'s "not a spend surface"
  satisfies the assertion on its own. Measured: rewriting `Reads` to
  *"transcripts with per-turn usage"* — the exact overclaim DS5 exists to
  prevent, in the exact field DS5 names — passes. Assert on `Reads` separately.
- **DS4 passes vacuously.** `discover_test.go:117` ranges over the findings with
  no prior assertion that there are any. Zero findings is zero iterations is a
  pass. Add `if len(found) != 3`.
- **DS6 re-runs discovery inside its own loop.** `discover_test.go:189-193`:
  `codex = &discoverAgents(home)[i]` takes the address of an element of a
  *different* slice than the one being ranged, indexed by the first slice's
  position. It is correct only because discovery is deterministic. Hoist the
  call to a variable; it is a footgun sitting inside the best test in the file.

---

## 3. Performance

Measured on the author's machine, warm cache, 20 sequential runs of
`replay doctor > /dev/null`, against two binaries built from identical trees
except that one has the `agents` block short-circuited:

| build | 20 runs | per run |
|---|---|---|
| with discovery | 1.77s, 2.18s, 1.73s | **~88 ms** |
| without discovery | 0.46s, 0.60s, 0.50s | **~26 ms** |

Discovery is ~62 ms of an 88 ms command: it roughly **triples** `doctor`'s wall
time. Independently, `discoverAgents(realHome)` best-of-5 in-process: **49 ms**.

At 88 ms this is fine. Nobody will feel it. The problem is that the comment
explaining why it is fine describes a bound the code does not have.

`discover.go:143-146`: *"Bounded rather than exhaustive… The count stops at the
cap… without walking 3.8 GB to do it."*

Per-root measurement:

```text
Codex   .codex/sessions           entries=  41  matched= 29  capTripped=false
Codex   .codex/archived_sessions  entries= 122  matched=121  capTripped=false
Grok    .grok                     entries=8840  matched= 74  capTripped=false
Ollama  .ollama/logs              entries=  13  matched= 12  capTripped=false
Cursor  .cursor                   entries=2500  matched=119  capTripped=false
                                  ------
TOTAL entries stat'd per run:     11516        cap tripped: never
```

The cap bounds **matches**, not **traversal**. On the machine the comment was
written on it never fires once, and every one of the five roots is walked to
completion — 11,516 directory entries stat'd on every `replay doctor`. `~/.grok`
is walked entirely, all 8,840 entries, to find 74 files. The 3.8 GB is not read,
which is true and is not what "walking" means; the cost of a walk is entries,
not bytes, and the 4.2 GB in `~/.grok` is irrelevant to the 37 ms it takes.

So the comment is not describing this code. It describes a store with >500
matching files, which none of the four are. The pathological case the cap is
supposed to defend against — a store with 200,000 non-matching files — is
defended against not at all: it would be walked in full, with the cap never
reaching 1.

Two honest options, either acceptable:

1. Cap **entries visited**, not matches (`if entries++ > 20000 { return
   filepath.SkipAll }`), which bounds the thing that actually costs, and rewrite
   the comment to say so.
2. Keep the match cap, and change the comment to say what it does: *"the count
   stops at 500 matches; the walk itself is proportional to the number of
   entries in the store, which on a normal machine is ten thousand and takes
   50 ms."*

Anchoring Cursor to `.cursor/projects` (§1.7) removes 1,574 entries — 14% of the
total — as a side effect of a correctness fix.

---

## 4. The design decision: no `$PATH`, no `brew list`, no exec

The doc comment at `discover.go:33-40` makes two claims. They deserve different
verdicts.

**The invasiveness claim is right, and I would defend it against a reviewer who
wanted `brew list`.** *"Enumerating installed software is a different and more
invasive act than counting files an agent already wrote"* is correct, and the
distinction is one most tools get wrong. Executing a package manager to find out
what a user has installed is a different consent question from reading files in
directories the user's own agents created. Shelling out also imports the
failure modes of whatever it shells out to. Keep this. It is the good half.

**The sufficiency claim is a rationalisation, and it is defended with a
strawman.** *"nothing here needs it: the store is what carries the data, and a
tool installed but never run has nothing to measure anyway."*

The premise is true and the conclusion does not follow, because `$PATH` is not
the alternative to a store scan. It is the alternative to a **fixed list of
guessed default store locations**. The case the fixed list misses is not
"installed but never run" — that case is correctly handled and the argument
disposes of it neatly. The case it misses is **installed, used heavily, store
not where we guessed**, and the doc comment never mentions it:

- **`$CODEX_HOME`.** Codex relocates its entire store with an environment
  variable. Set it and `replay doctor` reports no Codex. So does `replay codex`
  — `codexRoots` (`codex.go:23-32`) hardcodes `~/.codex` too — so this is a
  repo-wide gap rather than one this PR introduced. But `doctor` is the one
  command whose entire job is to say what Replay can see on this machine, and it
  will confidently say "nothing" while 150 sessions sit in `$CODEX_HOME`.
- **Ollama outside the default.** `OLLAMA_MODELS` and a systemd install both
  move or eliminate `~/.ollama/logs`.
- **A symlinked store** (§1.2), measured invisible today.
- **XDG.** Replay itself honours `XDG_CONFIG_HOME` in two places
  (`defaultroot.go:63`, `contribute.go:101`). The codebase has already conceded
  that home-relative guessing is insufficient for its *own* files; this file
  does not concede it for anyone else's.

Note what these have in common: none of them is solved by `$PATH`, and none
requires executing anything. The cheap, non-invasive, non-enumerating fix is to
read the environment variables the agents themselves document — `$CODEX_HOME`
before `~/.codex`, exactly as `claudeConfigDir` (`doctor.go:262-268`) already
reads `$CLAUDE_CONFIG_DIR` before `~/.claude`, with a comment explaining that a
user who relocated their data and gets told "none found" concludes the tool is
broken rather than that it looked in the wrong place. That comment is the
correct argument, it is already written, it is eleven lines from this feature in
the same file, and this feature does not apply it.

So the reasoning is sound about the thing it argues against and silent about the
thing it costs. It forecloses the design space with the most invasive
alternative available, rejects it correctly, and then treats the fixed list as
the residue rather than as one option among several. A reader of the comment
would not learn that `$CODEX_HOME` exists.

One smaller irony worth a line: the file frames a directory walk as the modest,
non-invasive option while stat-ing 11,516 entries of the user's home directory
on every run (§3). That is fine — it genuinely is less invasive — but "less
invasive" is the argument, not "not invasive", and the comment currently reads
as the latter.

**Recommendation.** Keep the no-exec rule and its justification verbatim. Add
env-var resolution per store (`$CODEX_HOME`, `$OLLAMA_HOME`) — three lines and a
field on `agentStore`. Rewrite the closing sentence so it says what is actually
true: *"a fixed list of default locations, plus the relocation variables each
tool documents. A store somewhere else entirely is a miss, and we would rather
miss it than execute a package manager to find it."* That is the same decision,
honestly costed, which is the standard the rest of this file sets.

---

## 5. What I would block on, and what I would merge with a comment

### Block

1. **§1.2 — a symlinked store root reports as absent.** Measured: a real store
   with real files behind `~/.grok` yields `[]`. Silently telling a user they
   do not have a tool they use daily is the failure the whole feature is written
   against, in the direction the tests do not cover. One line to fix
   (`filepath.EvalSymlinks` on the root, resolving the root only), plus a test.
2. **§2.3 — `TestDS7` is missing.** Deleting the glob matching entirely passes
   all six tests. The suite's stated first rule — report a store only when its
   files were counted — is enforced only against empty directories, and this is
   why §1.7's Cursor count can already disagree with its own doc comment without
   anything going red. Four lines of fixture.
3. **§1.1 — `500+` at the boundary and across roots.** 700 known files print as
   `500+`; 1200 files print as `500+` from a ceiling of 1000; exactly 500 gets a
   `+` it has not earned. On a feature whose thesis is that a checkable number
   must be checkable, three of four boundary cases are wrong. `(n, capped bool)`,
   five lines.

I would block on these three together, not separately: they are one review
round, roughly thirty lines, and all three are the same defect wearing different
hats — a claim in a comment that no measurement stands behind.

### Merge with a comment

- **`const cap = 500` shadows the builtin** (`discover.go:148`). The only
  builtin shadow in the repository. `revive` is enabled (`.golangci.yml:14`) and
  CI runs `golangci-lint` (`.github/workflows/ci.yml:98`); `redefines-builtin-id`
  is in revive's default rule set, so this will probably go red in CI. **I could
  not verify — `golangci-lint` is not installed on this machine** and I did not
  install it. Rename to `limit` and the question disappears. Run `make lint`
  before merging; the same run will likely flag the unused `path` parameter in
  the walk closure (`discover.go:150`) under `unused-parameter`.
- **§1.7 — anchor Cursor to `.cursor/projects`.** Makes the printed count (118)
  match the doc comment's own twice-confirmed number, drops a telemetry DB from
  a line that says "conversation transcripts only", immunises the count against
  the extension tree, and removes 14% of the walk.
- **§1.5 — no timeout on the walk** in a function that bounds its network probe
  at 2 s. A hung network mount hangs the command people run when things are
  broken.
- **§3 — the "bounded" comment describes a bound the code does not have.** Cap
  entries, or say what the cap actually caps.
- **§2.4 — DS5 asserts through the wrong field, DS4 passes vacuously, DS6
  re-runs discovery inside its own loop.**
- **§1.6 — generalise DS6.** The invariant "discovery must not undercount the
  reader it points at" is enforced for Codex and unenforced for Ollama, which
  has the same shape.
- **Line width.** The Codex line is now 104 columns —
  `agents  Codex  150 files in /Users/daniel/.codex/sessions, /Users/daniel/.codex/archived_sessions`
  — the longest `doctor` emits, on an 80-column terminal. `Path` is now a
  comma-joined list (`discover.go:133`) in a field documented as *"where this
  was found"*. Print the first root and `(+1 more)`, or put roots on their own
  line.

### Said once, and meant

The `Priceable` distinction, the refusal to name a command that does not exist
for Grok, the `n == 0 → no line` rule, and `TestDS6` are all better than what
most PRs in this area contain. The comment prose is doing real work rather than
narrating the code. The gap between the standard this file sets for itself and
the standard its tests enforce is the entire review — which is a good problem to
have, and a short one to close.
