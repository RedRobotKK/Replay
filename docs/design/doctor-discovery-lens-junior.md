# The doctor discovery block, read by someone who has to maintain it

**Lens:** two years in, comfortable with Go, first day in this repository, told
only "have a look at the new doctor discovery thing." No prior knowledge of
prompt-cache tooling.

**Under review:** `cmd/replay/discover.go`, `cmd/replay/discover_test.go`, and
the `agents` block in `cmd/replay/doctor.go:66-101`, on branch
`feat/doctor-agent-discovery`.

**Question this answers:** not "is it good code". Whether I could safely change
it in six months.

---

## Read this first: the branch moved while I was reviewing it

I started against a version where `agentStore` had a single `rel string`. By
the time I finished, seven things had changed. That matters for anyone reading
this later, because **most of what a fresh reader trips over was fixed during
the review**, and a report that still listed those as live defects would be
exactly the confidently-wrong artefact this lens exists to catch.

Fixed while I was reading, each of which I had found independently and can
confirm is now genuinely gone (not merely renamed):

| Was | Now |
|---|---|
| Codex scanned `.codex/sessions` only — doctor said 29 files, `replay codex` read 150 | `roots: codexRoots(home)` (`discover.go:102`) — the reader's own list. Doctor now prints 150 |
| Ollama globbed `*.log` — 12 files against the 6 `replay burn` reads | `patterns: []string{"server*.log"}` (`discover.go:125`), matching `burn.go:184` |
| Ollama `priceable: true` beside prose saying "no dollars" | `priceable: false` (`discover.go:128`) |
| A symlinked store vanished silently | `filepath.EvalSymlinks` at `discover.go:184`, crediting `defaultroot.go` |
| `const cap` shadowed the builtin | `const maxFiles` (`discover.go:179`) |
| `f.Files >= 500` inferred truncation from the total | `Capped bool` (`discover.go:57`), set by the walk that actually stopped |
| `Priceable` had no consumer at all | Consumed at `doctor.go:76-101` |

Two new tests arrived too: **DS6** (`discover_test.go:182`), which asserts
discovery's count matches the reader its `next:` line names, and **DS7**
(`:223`), which runs `replay <cmd> --help` for every `next:` that names a
command and fails if the dispatch rejects it.

Everything below is measured against the current state.

### Verification status, stated honestly

- `go test ./cmd/replay/ -run TestDS -count=1` — **passes**, DS1 through DS7.
- `go test ./cmd/replay/` — **I could not get a clean full-package run.** It
  exceeds the 600s default timeout on this machine and panics in
  `forEachSession` parsing the real 1,738-file Claude Code corpus. I first
  reported this package green, which was wrong: I had read an exit code from a
  shell pipeline that masked `go test`'s own status. I could not establish
  whether the timeout predates this branch. **Treat the full suite as unverified
  by me.** The targeted discovery tests are green and I ran those directly.
- `TestNoOrphanedDocuments` failed because of *this file*. See "One thing I
  broke" at the end.
- All experiments ran in `/tmp`. The only repository files I changed are this
  document and one row in `docs/design/README.md` linking it.

---

## 1. Could I add a fifth agent to `knownStores` without asking anyone?

**Mechanically, yes.** I copied the repo to `/tmp`, added OpenClaw, built, and
ran the discovery suite. It passed, and no test needed to change.

I picked OpenClaw because `~/.openclaw` is on this machine and
`docs/design/cli-tool-integration-survey.md:246` records its logs as
**"unexamined"**. I examined them. `~/.openclaw/agents/main/sessions/*.jsonl`
carries, per message:

```json
"usage": {"input": 16297, "output": 169, "cacheRead": 0, "cacheWrite": 0,
          "totalTokens": 16466,
          "cost": {"input": 0.0130376, "output": 0.000676, "total": 0.0137136}}
```

Input, output, cache read, cache write **and a dollar cost already computed**.
That is a more directly priceable local store than anything currently in
`knownStores`, and it is absent. I only know it because I opened the file
myself. Nothing in the repository pointed me there — the one document that
mentions the store says its contents are unknown.

### What I still had to guess

1. **How specific `roots` should be.** Codex delegates to `codexRoots(home)`
   and Ollama names `.ollama/logs`, but Grok points at all of `.grok`
   (`discover.go:108-110`) and Cursor at all of `.cursor` (`:131`). Two
   conventions, no stated rule. For OpenClaw, a broad `.openclaw` with
   `*.jsonl` also counts `logs/config-audit.jsonl` — a config-write audit
   trail with nothing to do with agent usage — as evidence the agent is
   installed. I guessed `.openclaw/agents`. Nobody told me which is right, and
   the two existing conventions each argue for a different answer.

2. **That a pattern containing `/` silently matches nothing.**
   `discover.go:79` now says "matched against the base name", which is more
   than was there before, but the consequence is not spelled out. I measured
   it on current code:

   ```text
   *.jsonl = 1    sub/*.jsonl = 0    **/*.jsonl = 0
   ```

   My first instinct was `agents/*/sessions/*.jsonl`, which is how I would
   write it in most codebases. That store finds zero files, is dropped at
   `discover.go:157`, never appears in the report, and **every test stays
   green**. See Q6.

3. **What obligation "confirmed by opening the store on a real machine"
   (`discover.go:87-89` region) places on me.** There is no evidence field on
   `agentStore`, no per-entry citation, and no test. I had to invent my own
   standard for having done it.

4. **Whether a new entry obliges me to update `docs/SURFACES.md`,
   `docs/requirements.md` §9, or the census.** Nothing says. `docs_drift_test.go`
   enforces that documents are linked and that the guide covers every command —
   nothing ties a `knownStores` row to any document.

### What was not written down that I needed

The two files that would have told me how to justify an entry are
`docs/evidence/surface-census-2026-09-08.md` and
`docs/design/cli-tool-integration-survey.md`. Both are excellent; the census
even carries its own correction in a block quote when its first Grok conclusion
turned out to rest on an incomplete search. Nothing in the code sends you to
either, and nothing says to add a row when you add a store.

**And one of them is already overtaken by the machine it describes.**
`cli-tool-integration-survey.md:221-222` says "no log directory exists on this
machine (`~/.oracle`, `~/.config/oracle`, `~/Library/Application
Support/oracle` all absent)". `~/.oracle/sessions` exists right now: 12 files,
`meta.json` and `output.log` per session, `createdAt` 2026-09-09. So the
document I would use to check my own work is stale, and nothing detects that.

---

## 2. The `reads` and `next` strings are prose in a Go file

**Where I'd look:** `docs/evidence/surface-census-2026-09-08.md`.

**What I verified myself, trusting nobody:**

| String | Where | My own check | Verdict |
|---|---|---|---|
| Cursor "conversation transcripts only, no usage fields" | `discover.go:134` | 118 `.jsonl` under `~/.cursor`; **0** contain `usage`, `inputTokens`, `promptTokens` or `cache_read` | accurate |
| Grok "usage fields are present: inputTokens, cachedReadTokens, costUsdTicks" | `discover.go:117` | 74 `updates.jsonl` on disk; **72** contain `costUsdTicks` and `inputTokens` | accurate; see caveat |
| Codex "rollout logs: per-turn usage, and rate-limit events" | `discover.go:103` | `replay codex` prints per-turn billed totals and a quota block | accurate |
| Ollama "server logs: prompt sizes and eval durations. Local, so no dollars" | `discover.go:126` | `replay burn` reports ollama with quota "none exists" and no dollar column | accurate |

So all four are checkable by hand in about ten minutes with `grep` and by
running the commands. That is rare and it is the best property this feature
has.

**The caveat, offered because this repository cares about exactly this.** The
census states that 74 `updates.jsonl` files carry `inputTokens`,
`cachedReadTokens`, `outputTokens`, `reasoningTokens` and `costUsdTicks`. I
count **72**. The other two are 3-line stubs under
`~/.grok/sessions/%2FUsers%2Fdaniel%2FDevelopment/` with no usage fields at
all. The `reads` string is unaffected — the fields are present — but the number
in the evidence file is 72.

**`next` is now guarded; `reads` is not.** DS7 (`discover_test.go:223`) runs
`replay <cmd> --help` for every `next:` naming a command and fails if the
dispatch rejects it. That closes the `replay doctor --agents` hole the file's
own comment describes, and it closes it in the general form rather than for one
row — good.

Nothing checks `reads`. On current code I replaced Grok's with `"full per-turn
dollar cost, verified against an xAI invoice"` — a claim the census explicitly
refuses, since two aggregations of the same files differ by exactly 2x and the
tick scale has never been checked against an invoice. **Tests stay green.** The
prose that makes the strongest promises to a user is the only part with no
mechanical check at all.

---

## 3. `Priceable bool` is set per store. How would I know if it were wrong?

**This changed mid-review, and it changed the answer.** When I started, nothing
read the field: it was written, plumbed through, and consumed nowhere. It is
now consumed at `doctor.go:96-101`, which prints:

```text
              2 of 4 can be priced; the rest are conversation only
```

That is a real improvement, and the comment beside it ("Consume `Priceable`
rather than carry it as decoration") names the right reason.

**But it moves the risk rather than removing it.** The field now produces a
sentence a user reads, and only one of the four rows is protected. I flipped
Grok's `priceable` to `false` on current code:

- `doctor` printed **"1 of 4 can be priced"** instead of "2 of 4".
- **The whole DS suite stayed green.**

Flipping Cursor's to `true` does fail DS5 (`discover_test.go:145`) — but DS5
asserts against the literal name "Cursor". It is a test about one row, not
about the field. So `Priceable` went from inert-and-wrong-invisibly to
live-and-wrong-visibly, which is better for a user who notices and worse for
one who doesn't.

**What breaks if someone sets it true for a surface with no usage data:** the
count in that line goes up, a reader concludes Replay can cost a surface it
cannot, and nothing in the suite objects. The check that would catch it is the
one DS5 gestures at but does not generalise: for every store, `priceable`
must agree with what its `reads` string says. Cursor is checked that way
already (`:147-152`); the other three are not.

---

## 4. The doc comment at the top of `discover.go`

### What it told me that the code could not

**The prohibition (`discover.go:35-39`).** "It does not read `$PATH`, run `brew
list`, or execute any binary." The most valuable paragraph in the file: it
states a boundary a maintainer would otherwise cross without noticing, because
enumerating installed software is the obvious next feature. And it is
**checkable in five seconds** — the imports are `os`, `path/filepath`, `sort`,
`strings`, and nothing else.

**The two rules (`discover.go:18-30`).** Why the code counts files rather than
stat'ing a directory. The reason is a claim about people — a dotfile outlives
the tool that made it — which no amount of reading Go would give me.

### What is stale, unverifiable, or unfalsifiable

**"A survey on 2026-09-09 found five agent CLIs ... two of which write usage
data the readers here can already parse" (`discover.go:12-15`).** The five are
never enumerated here. `discover_test.go:14-16` names them — Codex, Grok,
Cursor, Oracle, OpenClaw — and `knownStores` carries four, omitting Oracle and
OpenClaw. I could not tell whether that is a decision or an oversight. Having
opened both: Oracle's `~/.oracle/sessions` has `model` but no token counts;
OpenClaw's session JSONL has full usage *and* cost. The second looks like an
oversight.

The second clause describes the state *before* this branch — `codex.go` and
`burn.go` already read Codex and Ollama. Discovery did not make them readable.
A true sentence in a tense that credits this feature with something it did not
do.

**"Each entry was confirmed by opening the store on a real machine"
(`discover.go:87-89` region).** The strongest claim in the file and the only
one a reader cannot check at all. No per-entry record of what was opened, by
whom, or when. It asks for trust in precisely the register the rest of the file
spends its energy refusing to ask for.

**"without walking 3.8 GB to do it" (`discover.go:175`) — the mechanism is
wrong.** `maxFiles` bounds **matches**, not entries visited: the `n >= maxFiles`
check at `:187` fires on the match counter. I measured it — a directory of
5,003 files of which 3 match returns `n=3, capped=false` after visiting all
5,003. `~/.grok` holds 8,179 files and yields 74 matches, so the cap **never
fires** and the entire 4.2 GB tree is walked on every `replay doctor`.

The conclusion still holds — I timed `replay doctor` at **0.13s**, because
walking directory entries is cheap and no file is opened. But the sentence
explains the speed with a bound that is not doing the work. A maintainer who
adds a store with a rare pattern under a huge tree will trust a guarantee that
does not exist. This is the one line in the file I would rewrite before
changing anything else, precisely *because* the code is fast: the number
reassures, and the reason given for it is not the real reason.

---

## 5. The tests

### The one that taught me most about *why*: DS3

`TestDS3_AnEmptyStoreIsNotAnAgent` (`discover_test.go:97-105`) is nine lines
and the only test whose reason is not derivable from the code: "someone tries
an agent, removes it, and the dotfile stays." That sentence explains the whole
shape of `discoverAgents` — why it counts rather than stats, why `n == 0` is a
`continue` and not a finding, why `Files` is documented as never zero. Without
it I would have read the file count as a nicety and optimised it into an
`os.Stat`.

DS6 (`:182`) is the runner-up and is better written in one specific way: it
states the invariant generally — discovery must not disagree with the reader it
recommends — and then says explicitly that the invariant is *not* "Codex has
two directories". DS7 then generalises properly across all four rows. That pair
is the strongest thing in the file.

### The one I would have written differently: DS4

`TestDS4_EveryFindingIsActionable` (`:112-128`) iterates over what
`discoverAgents` **returned**. It therefore only ever checks stores that
produced a finding on the test's fake home. A store entry that finds nothing is
never examined.

I proved this on current code. Added to `knownStores`:

```go
{
    name: "Bogus", roots: []string{filepath.Join(home, ".bogus")},
    patterns:  []string{"sessions/*.jsonl"},
    reads:     "",
    next:      "",
    priceable: true,
},
```

Empty `reads`, empty `next`, a pattern that can never match, and a `priceable`
that inflates the "N of 4" line. **The whole suite passes clean.** A test named
"every finding is actionable" is satisfied by an entry that can never produce a
finding.

I would iterate `knownStores(home)` directly and assert, for every entry:
non-empty `name`, `reads` and `next`; at least one root; and no pattern
containing `/`, because such a pattern silently matches nothing. That version
fails for a bad entry regardless of what happens to be on the test machine's
disk — which is the whole point of a table-driven config.

Smaller: DS5 asserts against the literal string "Cursor" rather than a
property, and its prose check is a three-way substring OR that at least three of
the four current rows would satisfy by accident.

---

## 6. The first thing I would break by accident

**Adding a fifth store that silently does nothing.** This is now the sharpest
remaining edge, because the two failure modes compose:

1. A pattern with `/` in it matches nothing (measured above), so the store
   never appears.
2. DS4 only inspects stores that appeared, so nothing checks the entry.

The result is a `knownStores` row that looks completely reasonable in review,
ships, and does not exist at runtime. Neither DS6 nor DS7 helps — both iterate
findings, so a store producing no finding is invisible to them too. **This is
the specific thing I would do wrong, and the suite would tell me I was fine.**

Second: `reads` prose drifting from the surface it describes. DS7 pinned the
`next:` half of that promise; the half making the stronger claim is unpinned.

Third, and smaller: the `maxFiles` bound is per root and per match. If a future
store points at a large tree with a common pattern, the "500+" is honest now
(`Capped` is set by the walk that actually stopped), but the walk cost is
governed by tree size, not by the cap the comment credits.

---

## 7. Duplication

**Largely fixed during the review, and worth recording as the model for the
rest.** `knownStores` now calls `codexRoots(home)` (`discover.go:102`) instead
of re-spelling the two directories, and Ollama's pattern is pinned to
`burn.go:184` by a comment. The comment above `knownStores` (`:84-97`) states
the rule: roots and patterns come from the readers themselves wherever one
exists. That is the right rule and it is now followed for both surfaces that
have a reader.

What remains:

**Two functions that ask the same question differently.**

| Function | Where | Behaviour |
|---|---|---|
| `holdsTranscripts` | `defaultroot.go:85` | `WalkDir`, resolves symlinks, stops at first hit |
| `countStoreFiles` | `discover.go:176` | `WalkDir`, resolves symlinks, counts to `maxFiles` |

These are now consistent in the way that matters (the symlink fix was ported),
but they are still two walkers, and the second only became correct after
reintroducing and re-fixing the first's documented bug. Alongside them,
`doctor.go:218 countFiles` (Glob, one level) and `doctor.go:189
countNestedTranscripts` (WalkDir, skips dot dirs) make four.

**Store locations still live in four places.** `defaultroot.go:54`
(Claude Code roots), `codex.go:23` (Codex), `burn.go:184` (Ollama), and
`desktoproot.go:37` (Claude Code inside the desktop sandbox). Discovery now
consumes the middle two. It does not know about the fourth: the desktop
sandbox stores are absent from `knownStores`. Note `desktoproot.go:26` already
says "Discovery reports it; the user asks for it" — the word "discovery" now
means two unrelated mechanisms in one package.

**No `REPLAY_TRANSCRIPTS` for agent stores.** `defaultroot.go:16` lets a user
say where their Claude corpus lives. There is no analogue here, so someone
whose `~/.codex` sits elsewhere is invisible to the `agents` block with no way
to say so.

**A second `doctor` that did not get the feature.**
`internal/tui/measured.go:119-162` renders `replay tui -screen doctor`. I ran
it: no agents block. Two commands called "doctor", one of which now knows about
four agent stores and one of which does not.

**Documentation.** `docs/design/surface-taxonomy-3-providers.md:141` already
tabulates these roots with `file:line` citations into `defaultroot.go`,
`codex.go` and `burn.go`. Nothing ties that table to the code or the code to
the table.

**A prior review already exists.** `docs/design/doctor-discovery-lens-beginner.md`
reaches the 29-vs-150 Codex finding at `:154-155`. I found it by grepping for
store paths, not because anything linked to it.

---

## One thing I broke, reported because it is the same class of defect

Writing this document made `TestNoOrphanedDocuments`
(`cmd/replay/docs_drift_test.go:146`) fail. The repository requires every `.md`
under `docs/` to be reachable by a real Markdown link from another file, and it
counts link syntax rather than prose mentions — because an earlier version
passed on a filename that merely appeared in a sentence. I fixed it by adding
one row to `docs/design/README.md`. That is the only repository file I changed
besides this one.

Two things follow. First, a fresh contributor cannot add a document here
without knowing that rule, and nothing states it outside the test's own
comment. Second, the check runs one way only: **`docs/design/README.md` links
three documents that do not exist** — `quota-estimator-blue.md`,
`quota-estimator-red.md` and `quota-data-census.md`. An orphan is caught; a
dangling link is not. For a repository whose entire discipline is "a claim a
reader checks and finds hollow discredits the claims around it", an index with
three dead links is the same defect as the one this feature was built to avoid.

---

## Things I could not work out, stated plainly

These are findings about the code, not about my reading of it.

- **Whether the `roots` granularity difference is deliberate.** Codex and
  Ollama name a specific store directory; Grok and Cursor name the whole
  dotfile directory. I could not tell whether that is a considered trade
  between scan cost and coverage, or a convention that drifted.
- **Whether omitting Oracle and OpenClaw is a decision.**
  `discover_test.go:14-16` names five CLIs; the code carries four; OpenClaw's
  store carries usage *and* dollar cost.
- **Whether `go test ./cmd/replay/` passes on this branch at all.** It exceeds
  the 600s default timeout on this machine, in transcript parsing unrelated to
  discovery. I could not determine whether that predates the branch.
- **What a new entry obliges me to do about the evidence documents.** No
  example to follow, since there is no record of what was done for the existing
  four.

## What I would want before changing this

1. **Validate `knownStores` as a table**, independent of what is on disk: no
   empty `reads`/`next`, at least one root, and no `/` in any pattern. This is
   the one change that would have caught the mistake I actually made.
2. **Generalise DS5's prose check to every row**, so `priceable` must agree
   with what `reads` says — the field now prints a sentence to users and only
   one of four rows is guarded.
3. **Fix the `maxFiles` comment** to describe the bound that exists. The code
   is fast; the stated reason is not why.

---

[Design](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
