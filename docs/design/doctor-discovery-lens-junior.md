# The doctor discovery block, read by someone who has to maintain it

**Lens:** two years in, comfortable with Go, first day in this repository, told
only "have a look at the new doctor discovery thing." No prior knowledge of
prompt-cache tooling.

**Under review:** `cmd/replay/discover.go`, `cmd/replay/discover_test.go`, and
the `agents` block added to `cmd/replay/doctor.go` on branch
`feat/doctor-agent-discovery`.

**Question this answers:** not "is it good code". Whether I could safely change
it in six months.

## A note on timing, because it affects what you can reproduce

The files moved while I was reading them. When I started, `agentStore` had a
single `rel string` and Codex pointed only at `.codex/sessions`. Partway
through, `TestDS6_DiscoveryAgreesWithTheReaderItRecommends` appeared in
`discover_test.go:181` (red), and then `rel` became `rels []string` and Codex
gained `.codex/archived_sessions`. Everything below is measured against the
current state unless it says otherwise. The one place it matters is Q6: the
Codex half of that defect is fixed, the Ollama half is not.

Verification status, honestly: `go test ./cmd/replay/ -run TestDS -count=1`
passes, including DS6. The full package takes about 400 seconds; I ran it green
(exit 0) against a scratch copy carrying my added fifth agent. All experiments
below were run in `/tmp`; nothing in the repository was modified except this
file.

---

## 1. Could I add a fifth agent to `knownStores` without asking anyone?

**Mechanically, yes.** I copied the repo to `/tmp`, added OpenClaw, built, and
ran the suite. It passed, and no test needed to change — which is most of the
problem.

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
`knownStores`, and it is not there. I only know that because I opened the file
myself. Nothing in the repository pointed me at it.

### What I had to guess at

1. **How specific `rels` should be.** Codex points at `.codex/sessions`,
   Ollama at `.ollama/logs`, but Grok points at `.grok` and Cursor at `.cursor`
   (`discover.go:82-110`). Two conventions, no rule. For OpenClaw, `.openclaw`
   with `*.jsonl` also counts `logs/config-audit.jsonl` — a config-write audit
   trail that has nothing to do with agent usage — as evidence the agent is
   installed. I guessed `.openclaw/agents`. Nobody told me which is right.

2. **That `patterns` match the basename only.** `countStoreFiles`
   (`discover.go:160-165`) calls `filepath.Match(p, d.Name())`. I measured what
   that means:

   ```
   *.jsonl = 1    sub/*.jsonl = 0    **/*.jsonl = 0
   ```

   A pattern containing a slash matches nothing, silently. My first instinct
   was `agents/*/sessions/*.jsonl`, which is how I'd have written it in most
   codebases, and it would have produced a store that finds zero files, is
   dropped at `discover.go:127`, and never appears — with every test still
   green. This is not written down anywhere.

3. **What `priceable` is for.** See Q3. I could not work it out.

4. **Whether `next` is a command or a sentence.** Codex and Ollama give
   commands; Grok and Cursor give prose. `doctor.go:81` prints `next: %s` for
   all four, so a reader sees "next: replay codex" and "next: nothing to run:
   this is not a spend surface..." in the same column. Both defensible; no
   stated rule.

5. **Whether my count has to agree with the command I name.** It does — that is
   now DS6 — but DS6 checks Codex only, by name.

6. **What "confirmed by opening the store on a real machine"
   (`discover.go:77-79`) obliges me to do.** There is no evidence field on
   `agentStore`, no citation per entry, and no test. I had to decide for myself
   what would count as having done it.

### What was not written down that I needed

The two documents that would have told me how to justify an entry are
`docs/evidence/surface-census-2026-09-08.md` and
`docs/design/cli-tool-integration-survey.md`. They are excellent. Nothing in
the code sends you to them except one clause at `discover.go:79`, and nothing
says you should add a row to either when you add a store.

And one of them is already stale in a way that matters:
`cli-tool-integration-survey.md:221-222` says "no log directory exists on this
machine (`~/.oracle`, `~/.config/oracle`, `~/Library/Application
Support/oracle` all absent)". `~/.oracle/sessions` exists right now with 12
files — `meta.json` and `output.log` per session, `createdAt` 2026-09-09. So
the file I would use to check my own work has been overtaken by the machine it
describes.

---

## 2. The `reads` and `next` strings are prose in a Go file

**Where I'd look:** `docs/evidence/surface-census-2026-09-08.md` is the
verification source, and it is unusually good — it carries its own correction
in a block quote when its first conclusion about Grok turned out to be drawn
from an incomplete search.

**What I verified myself, without trusting anyone:**

| String | Where | My check | Verdict |
|---|---|---|---|
| Cursor "conversation transcripts only, no usage fields" | `discover.go:107` | 118 `.jsonl` under `~/.cursor`; 0 contain `usage`, `inputTokens`, `promptTokens` or `cache_read` | accurate |
| Grok "usage fields are present: inputTokens, cachedReadTokens, costUsdTicks" | `discover.go:95` | 74 `updates.jsonl` on disk, **72** contain `costUsdTicks` and `inputTokens` | accurate, with a caveat below |
| Codex "rollout logs: per-turn usage, and rate-limit events" | `discover.go:84` | `replay codex` prints per-turn billed totals and a quota block | accurate |
| Ollama "server logs: prompt sizes and eval durations. Local, so no dollars" | `discover.go:101` | `replay burn` reports 3,294 ollama requests, no dollar column | accurate as prose; see Q6 for the file count |

So yes — I can verify all four myself, in about ten minutes, with `grep` and by
running the commands. That is genuinely rare and it is the best thing about
this feature.

**The caveat, offered because this repo cares about it.** The census says "74
`updates.jsonl` files carry `inputTokens`, `cachedReadTokens`, `outputTokens`,
`reasoningTokens` and `costUsdTicks`". I count 72. The other two are 3-line
stubs under `~/.grok/sessions/%2FUsers%2Fdaniel%2FDevelopment/` with no usage
fields at all. The `reads` string in `discover.go:95` is unaffected — fields
*are* present — but the number in the evidence file is 72, not 74.

**But nothing checks the prose.** I ran mutations in `/tmp`:

- Replace Grok's `reads` with `"full per-turn dollar cost, verified against an
  xAI invoice"` — a claim `surface-census-2026-09-08.md` explicitly says is not
  established, since two aggregations of the same files differ by exactly 2x.
  **Tests stay green.**
- Replace Grok's `next` with `"replay doctor --agents"` — the exact
  nonexistent flag that `discover.go:90-94` says an earlier draft contained.
  **Tests stay green.**

The comment records the defect and congratulates the fix. No test was added to
stop it recurring. A future me, editing this prose, has nothing catching them.

---

## 3. `Priceable bool` is set per store. How would I know if it were wrong?

**You would not.** The field has no consumer.

```
$ grep -rn --include='*.go' 'Priceable\|priceable' cmd internal
```

Every hit is inside `discover.go` (declaration, four literals, one assignment
at `:134`) or `discover_test.go:144`. Nothing prints it. Nothing branches on
it. `doctor.go:73-86` never reads it.

**What breaks if someone sets it true for a surface with no usage data:**
nothing. That is the finding. I flipped Ollama's `priceable` to `false` in a
scratch copy — the suite stayed green. Flipping Cursor's to `true` *does* fail
DS5 (`discover_test.go:144`), but only because DS5 names Cursor; it is a test
about one row, not about the field.

**And it is already inconsistent today.** `discover.go:100-103`:

```go
name: "Ollama", rels: []string{".ollama/logs"}, patterns: []string{"*.log"},
reads:     "server logs: prompt sizes and eval durations. Local, so no dollars",
next:      "replay burn",
priceable: true,
```

`Priceable`'s own doc comment (`discover.go:54-56`) says it is "whether the
surface carries usage the tool can cost". The `reads` string one line above the
flag says "no dollars", and `replay burn` prints ollama's quota column as "none
exists". So the flag says one thing and the prose beside it says the other, in
the same struct literal, and no test can see it — because nothing reads the
flag.

To a fresh reader this field looks like a guard. It is a comment with a type.
Either wire it to something (the `next:` line for an unpriceable surface could
be derived from it rather than written twice) or delete it, because right now
it invites exactly the mistake of trusting it.

---

## 4. The doc comment at the top of `discover.go`

### What it told me that I could not have worked out from the code

**The prohibition, `discover.go:33-40`.** "It does not read `$PATH`, run `brew
list`, or execute any binary." This is the most valuable paragraph in the file,
because it states a boundary a reader would otherwise cross without noticing —
enumerating installed software is the obvious next feature, and this says why
not. And it is *checkable*: the imports are `os`, `path/filepath`, `sort`,
`strings`, and nothing else. A reviewer can confirm the claim in five seconds.

**The two rules, `discover.go:21-31`.** Why the code counts files instead of
stat'ing the directory. The reason is a claim about people — a dotfile outlives
the tool that made it — which no amount of reading the code would give me.

### What is stale, unverifiable, or asserting something I cannot check

**`discover.go:13-15` — "a survey on 2026-09-09 found five agent CLIs ... two
of which write usage data the readers here can already parse."** The five are
never enumerated in `discover.go`. `discover_test.go:14-16` names them — Codex,
Grok, Cursor, Oracle, OpenClaw — and `knownStores` holds four, omitting Oracle
and OpenClaw. I could not tell whether that omission is a decision or an
oversight. (Having opened both stores myself: Oracle's `~/.oracle/sessions`
holds `meta.json` with `model` but no token counts; OpenClaw's session JSONL
holds full usage *and* cost. The second looks like an oversight.)

The second half of that sentence describes the state *before* this branch:
`codex.go` and `burn.go` already read Codex and Ollama. Discovery did not make
those readable. It's a true sentence in the wrong tense, and it reads as a
claim about what this feature achieved.

**`discover.go:31` — "the overclaim `docs/SURFACES.md` was corrected for the
same day this was written."** I checked. `git log -1 --date=short --
docs/SURFACES.md` gives 2026-09-09. Holds.

**`discover.go:77-79` — "Each entry was confirmed by opening the store on a
real machine, not from a vendor's documentation."** The strongest claim in the
file and the only one a reader cannot check at all. There is no per-entry
record of who opened what, or when. It asks for trust in exactly the register
the rest of the file spends its energy refusing to ask for.

**`discover.go:141-146` is now stale.** "The count stops at the cap" — the cap
is `const cap = 500` at `:148`, and it is applied *per root*. Since `rels`
became plural, Codex's ceiling is 1000. I measured it: 1,200 files across the
two Codex roots reports `Files = 1000`. Meanwhile `doctor.go:80` still tests
`f.Files >= 500` and prints `"500+ files"`, a bare literal duplicating the
constant in the other file. Nothing broke here yet, but the comment, the
constant and the display rule have already come apart.

(Also: `const cap` shadows the builtin `cap`. Harmless today. There is a branch
named `fix/lint-shadowed-builtin` in this repo, so somebody cares; I could not
run `golangci-lint` to check, as it is not installed here.)

---

## 5. The tests

### The one that taught me most about *why*: DS3

`TestDS3_AnEmptyStoreIsNotAnAgent` (`discover_test.go:96-104`) is nine lines
long and it is the only test whose reason is not derivable from the code. "The
directory outliving the tool is the common case: someone tries an agent,
removes it, and the dotfile stays." That single sentence explains the entire
shape of `discoverAgents` — why it counts rather than stats, why `n == 0` is a
`continue` and not a finding, why `Files` is documented as "never zero"
(`discover.go:48`). Without it I would have read the file count as a nicety and
optimised it away.

DS6 (`:181`) is the runner-up, and it is better written than the others in one
specific respect: it states the invariant in general form — "discovery must not
undercount a surface relative to the reader it points at" — and then says
explicitly that the invariant is *not* "Codex has two directories". That is the
sentence that stops the next person fixing the instance and missing the class.
It doesn't quite live up to itself; see below.

### The one I would have written differently: DS4

`TestDS4_EveryFindingIsActionable` (`:111-127`) iterates over what
`discoverAgents` returned. So it only ever checks stores that were **found on
the test's fake home**. A store entry that finds nothing is never examined.

I proved this. I added to `knownStores`:

```go
{
    name: "Bogus", rels: []string{".bogus"}, patterns: []string{"sessions/*.jsonl"},
    reads: "", next: "", priceable: true,
},
```

Empty `reads`, empty `next`, and a pattern that can never match anything. The
suite passes clean. The test named "every finding is actionable" is satisfied
by an entry that can never produce a finding.

I would iterate `knownStores` directly and assert, for every entry: non-empty
`name`, `reads` and `next`; at least one `rel`; and no pattern containing a
`/`, because such a pattern silently matches nothing. That version fails for a
bad entry regardless of what happens to be on the test machine's disk, which is
the point of a table-driven config.

Two smaller things. DS5 (`:135`) asserts against the literal name "Cursor"
rather than against a property, and its prose check is a three-way substring
OR that at least three of the four current entries would satisfy by accident.
And DS6, having named a general invariant, is hardcoded to Codex — I'd loop
every store that names a real command in `next` and compare against that
command's own root list. Which brings me to:

---

## 6. The first thing I would break by accident

**The `next:` line quietly ceasing to describe what the named command reads.**

This is not hypothetical. It is live today for Ollama:

```
agents        Ollama  12 files in /Users/daniel/.ollama/logs
              server logs: prompt sizes and eval durations. Local, so no dollars
              next: replay burn
```

`burn.go:184` globs `server*.log`. There are 6 of those. The other 6 are
`app*.log`, and I re-ran the check from
`surface-census-2026-09-08.md` Finding 1 against all six of them: **zero**
lines matching `prompt eval` or `n_past`. They are application lifecycle logs.

So doctor counts 12 files and hands the reader a command that reads 6. That is
DS6's defect exactly — "two commands disagreeing about their own disk
discredits every other line in the report" — surviving in the surface DS6
doesn't cover, in the same release that fixed it for Codex. The census even
states the conclusion ahead of time: "The narrower glob is right."

**Second thing I'd break: a symlinked store.** `countStoreFiles`
(`discover.go:147`) walks `root` as given. `filepath.WalkDir` lstats its root,
so if `~/.grok` is a symlink it sees a non-directory, descends into nothing,
and the store vanishes with no error. Measured:

```
discoverAgents on symlinked ~/.grok = []
holdsTranscripts on same path       = true
```

`defaultroot.go:90-102` documents this precise bug and its consequence — bare
`replay` said "recorded no sessions yet" while `doctor` counted 1,681
transcripts in the same shell — and fixes it with `filepath.EvalSymlinks`.
`cmd/replay/symlinkroot_test.go` exists to freeze it. Discovery reintroduced
it. On this machine none of the four stores are symlinks, so it is invisible
until someone moves one to an external disk.

**Third: patterns.** Covered in Q1. A slash in a pattern is a silent zero, and
I would have written one on my first attempt.

---

## 7. Duplication

Yes, and it is the part I'd worry about most for a six-month horizon.

### Store locations — this is now the fifth parallel list

| What | Where |
|---|---|
| Claude Code roots (4 candidates + `REPLAY_TRANSCRIPTS`) | `cmd/replay/defaultroot.go:54-79` |
| Codex roots (2) | `cmd/replay/codex.go:23-40` |
| Ollama glob | `cmd/replay/burn.go:184` |
| Claude Code inside the desktop sandbox | `cmd/replay/desktoproot.go:37-60` |
| **Codex + Ollama + Grok + Cursor, re-declared** | `cmd/replay/discover.go:80-111` |

`knownStores` spells Codex's two roots as string literals instead of calling
`codexRoots(home)`, which already returns exactly those two and carries a
comment explaining why there are two. The two lists can drift apart again, and
the only thing standing between them is DS6 — which compares *counts*, not
roots, and only for Codex.

Two consequences a fresh reader hits immediately:

- The desktop sandbox stores that `desktoproot.go` knows about are not in
  `knownStores`. Note `desktoproot.go:26` already says "Discovery reports it;
  the user asks for it" — the word "discovery" now means two unrelated
  mechanisms in this package.
- `REPLAY_TRANSCRIPTS` (`defaultroot.go:16`) lets a user say where their Claude
  corpus is. There is no analogue for agent stores. Someone whose `~/.codex`
  lives elsewhere is invisible to the `agents` block and has no way to say so.

### File counting — four walkers, one of which is different

| Function | Where | Behaviour |
|---|---|---|
| `countFiles` | `doctor.go:218` | `filepath.Glob`, one level |
| `countNestedTranscripts` | `doctor.go:189` | `WalkDir`, skips dot dirs, excludes the top level |
| `holdsTranscripts` | `defaultroot.go:85-113` | `WalkDir`, **resolves symlinks**, stops at first hit |
| `countStoreFiles` | `discover.go:147` | `WalkDir`, capped at 500/root, basename glob, **no symlink handling** |

The fourth is the only one that ignores the symlink lesson the third exists to
record.

### Documentation

`docs/design/surface-taxonomy-3-providers.md:141` already tabulates on-disk
discovery roots — with `file:line` citations into `defaultroot.go`, `codex.go`
and `burn.go`. `knownStores` is a sixth copy of that information, and nothing
ties the code to the table or the table to the code.

### A second `doctor`

`internal/tui/measured.go:119-162` renders `replay tui -screen doctor`. I ran
it: no agents block. Two commands called "doctor", one of which now knows about
four agent stores and one of which does not.

### Also worth knowing before you start

`docs/design/doctor-discovery-lens-beginner.md` already exists and already
reaches the 29-vs-150 Codex finding at `:154-155`. I found it by grepping the
repo for store paths, not because anything linked to it.

---

## Things I could not work out, stated plainly

These are findings about the code, not about my reading of it.

- **What `Priceable` is for.** Not whether it is set correctly — what it is
  *meant to do*. It has no consumer anywhere in the repository.
- **Whether the `rels` granularity difference is deliberate.** Codex and Ollama
  point at a specific store subdirectory; Grok and Cursor point at the whole
  dotfile directory. I could not tell whether that is a considered choice about
  scan cost versus a convention that drifted.
- **Whether the 500 cap is per store or per root,** from reading alone. I had
  to measure it. It is per root, so Codex's true ceiling is 1000, and
  `doctor.go:80` still hardcodes 500.
- **Whether omitting Oracle and OpenClaw from `knownStores` is a decision.**
  `discover_test.go:15-16` names five CLIs; the code carries four.
- **Whether adding a store obliges me to update `docs/SURFACES.md`,
  `docs/requirements.md` §9, or the census.** Nothing in the code or tests says
  so, and `docs_drift_test.go` covers commands in the help text, not agent
  stores.
- **What "confirmed by opening the store on a real machine" would require of
  me** if I added an entry tomorrow. There is no record of what was done for
  the existing four, so there is no example to follow.

## What I would want before changing this

Three things, all small:

1. **Make `next:` self-verifying.** Generalise DS6 to loop every store whose
   `next` names a real Replay command, and compare against that command's own
   root list — Ollama would go red today.
2. **Validate `knownStores` as a table**, independent of what is on disk: no
   empty `reads`/`next`, at least one `rel`, no `/` in a pattern.
3. **Either wire `Priceable` or delete it.** As it stands it is a field a fresh
   reader will trust and a maintainer can set wrongly with no consequence, which
   is the combination that eventually puts a wrong number in front of a user.

---

[Design](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
