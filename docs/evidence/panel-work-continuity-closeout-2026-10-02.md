# LOCKED ROOM VII CLOSEOUT: does `replay agents` miss standing by ignoring commit bodies?

**2026-10-02. Three real repositories, six fresh agents, A/B. No production
change. Nothing committed.**

Room VII closed the work-continuity line at **D, engineering primitive**, and
left exactly one measured gap: `replay agents` matches documents by filename
and skips `.git`, so commit prose is structurally invisible to it. This is that
one experiment.

Every substantive statement is labelled **MEASURED**, **INFERRED** or
**NOT MEASURED**.

---

## 1. Corpus definition

Three repositories on this machine with materially different histories, window
= the **40 most recent commits** of each.

| repo | commits | window HEAD | branch | tracked files |
|---|---:|---|---|---:|
| **DoorKik** | 443 | `f183aa63` 2026-09-08 | `chore/writer-2026-08-27-claim-check` | 942 |
| **agmsg** | 268 | `94704fd` 2026-07-23 | `main` | 348 |
| **RedRobot.jp** | 135 | `63cf4d6` 2026-09-21 | `main`, 21 files dirty | 287 |

**Replay itself was excluded from the scored arms.** This session has ~100k
tokens of context about it, so any ground truth I wrote for it would be
contaminated. Its census row is reported below and nothing is scored on it.

**Sampling limitation, stated rather than buried: all three repositories have
the same author.** This measures one person's commit discipline, not a
population. **NOT MEASURED** whether any of it generalises.

### Census (MEASURED, mechanical, no prose classification)

| repo | bodied | body ≥3 lines | mean body bytes | mean subject bytes |
|---|---:|---:|---:|---:|
| DoorKik | 39/40 (98%) | 37 (92%) | **9,591** | 70 |
| agmsg | 36/40 (90%) | 28 (70%) | 1,076 | 68 |
| RedRobot.jp | 40/40 (100%) | 39 (98%) | 1,456 | 70 |
| _Replay (excluded)_ | _40/40 (100%)_ | _40 (100%)_ | _1,623_ | _65_ |

### How much rationale has no in-tree prose home (MEASURED, upper bound)

A commit that touches no `.md`/`.txt`/`.rst` file wrote its rationale nowhere
but the commit message.

| repo | touched ≥1 doc file | **code-only** | code-only body bytes |
|---|---:|---:|---:|
| DoorKik | 24 (62%) | **15 (38%)** | 127,490 |
| agmsg | 9 (25%) | **27 (75%)** | 34,493 |
| RedRobot.jp | 17 (42%) | **23 (58%)** | 31,955 |

This is an **upper bound**: a code-only commit's rationale may still sit in a
source comment. It bounds how much standing could be commit-body-only.

---

## 2. Exact A/B behaviour

**A — current `replay agents`.** Read from source, not inferred.

It reports **directories**, never content. A directory is reported when it
holds a document (`.md .markdown .txt .csv .tsv .json .jsonl .ndjson .yaml
.yml .org .rst`) matching one of three mechanical rules: a whole-word record
token in the filename (`ledger registry index manifest history sent journal
record records roster inventory log logs changelog audit ledgers`), an
append-only extension (`.jsonl .ndjson`), or three or more dated documents
(`\d{4}-\d{2}-\d{2}`) in one directory.

**`.git` is in `skipDirs`** (`internal/analysis/sources.go`), alongside
`node_modules vendor dist build testdata target __pycache__ .venv`. **Commit
bodies are therefore structurally invisible to A.** A's own output discloses
this, naming `.git` in its caveat paragraph.

What A produced on each corpus:

| repo | directories reported |
|---|---|
| DoorKik | 9: `./ data/capital/store/ docs/ outbound-os/advanced-skills/ outbound-os/core-skills/ reports/ runs/ runs/reliability/ web/` |
| agmsg | **2**: `./ docs/logo-assets/` |
| RedRobot.jp | **2**: `data/ docs/` |

**B — experimental variant.** A's output, plus the authored body of every
commit in the window, verbatim, newest first, each with the files it touched.
It **classifies nothing, infers nothing and summarises nothing**: it is pure
retrieval. Git trailers (`Co-Authored-By`, `Claude-Session`, the generated-with
lines) are dropped as noise. Its header states in bold that a commit message is
evidence and not truth, that the files are listed so the claim can be checked,
and that where prose and repository disagree the repository wins.

| repo | B output | commit sections |
|---|---:|---:|
| DoorKik | 396,424 bytes | 39 |
| agmsg | 51,909 bytes | 36 |
| RedRobot.jp | 68,479 bytes | 40 |

B is a throwaway script outside the repository. **No production code was
modified.**

### Isolation

Each arm received a sandbox built with `git archive HEAD`: the tracked working
tree with **no `.git` directory at all**. Neither arm could run `git log`, so
the only route to commit prose is B's file. This is the clean isolation the
positive threshold needs, and it is structural rather than instructional.

It also means **neither arm can check a ref**, which matters in §5.

---

## 3. Per-repository results

**MEASURED.** Each B arm was asked to audit its own sources: for every fact it
reported, whether it came from **(a)** repo files only, **(b)** commit prose
only, or **(c)** both. This is the cleanest available evidence, because it is
B grading itself against the thing it is supposed to beat.

| repo | scored items | (a) files only | (c) both | **(b) commit prose only** |
|---|---:|---:|---:|---:|
| DoorKik | 25 | 9 | 15 | **1** |
| agmsg | 35 | 18 | 14 | **3** |
| RedRobot.jp | 29 | 28 | — | **1** |
| **total** | **89** | | | **5 (5.6%)** |

RedRobot.jp's arm put it flatly: **"(b) commit prose only: none. Every
established fact above is visible in a file."** Its single (b) item is an
unresolved question, and it noted that even that sentence is duplicated into a
tracked docstring.

**On constraints, all three arms agree and say so independently:**

- DoorKik: *"Nothing in §6 is (b). I did not quote any constraint that exists only in commit prose."*
- agmsg: *"every quoted hard rule in the numbered list is (a) repo files only ... None of these appear in the commit prose."*
- RedRobot.jp: *"all (a) repo only ... No constraint in the list rests on commit prose alone."*

The strongest rule in the whole corpus, DoorKik's *"nothing should ever be
wired to"*, is (c): it is in commit `f183aa6` **and** in
`campaigns/vc-follow/README.md`.

**On the next action, all three B arms derived it from the tree.**
RedRobot.jp's arm labelled its own next action **"(a) repo only"** outright.

### A beat B on DoorKik

The clean DoorKik A arm, with no commit access at all, found **two live
defects that no B arm found**, and both are verified:

**1. The "once, ever" cap is not enforced across days.**
`campaigns/vc-follow/README.md:43` says `Per person | once, ever`.
`build_batch.py:71` keys the ledger on `(list_id, name)` and `:102` tests that
pair. Mike Dudas holds **three** list_ids in the seed (256, 408, 588 — two
spellings of one fund). Logging his queued row removes only `256`; `408` and
`588` stay eligible. That is precisely the outcome the author documented as
account-ending: *"one person three connection requests"*. It is reachable on
run 2, and the ledger is still empty, so the fix is free today.

**2. One batch row contradicts the stated exclusion.**
`TODAY-BATCH.csv:29` queues `Shannon Poulos, CFO / Managing Director - Global
Private Equity, Bain Capital` at P1 as *"allocator title at a VC fund"*, while
the README excludes functional partners outright. `managing director` is in
`ALLOCATOR` (`build_batch.py:22`) and matches before the functional role is
considered; `cfo` is absent from `NEVER` (`:31`).

A's next action — *"write a failing test for the once-ever rule across
duplicate seed rows, then fix the ledger key"* — is better than any B arm's and
better than my own ground truth. It is actionable by an agent, where the
LinkedIn lookup is not, and A reached it from the tree alone.

### A matched B on RedRobot.jp, and found a defect B missed

The clean RedRobot.jp A arm, with no commit access, reached **the same next
action as its B counterpart**: wire `check:replay` into CI. It reconstructed
the whole 2026-09-21 triggering incident from `scripts/check-replay-freshness.mjs:6-13`
and the coherence test, and quoted the repo's own doctrine from
`.github/workflows/ci.yml:33-35` — *"A check that never runs is not a check."*

It then found something no B arm did, and it is verified:

**`scripts/vendor-corpus.mjs:52-54` still reads the retired field names.** The
success log formats `s.avoidableUsd.toFixed(2)` and `s.avoidableShare`, while
the corpus summary carries only `rebilledUsd`/`rebilledShare` — confirmed: the
summary's keys are `tasks, unit, lanes, totalUsd, uncachedUsd, cacheWriteUsd,
cacheReadUsd, outputUsd, medianUsd, p90Usd, rebilledUsd, rebilledShare,
rebilledTokens, route`, with no `avoidable*`. The file is written **before**
the log line, so `npm run vendor:corpus` writes correctly and then throws a
TypeError and exits non-zero.

That matters because of what the freshness gate prints when it fires
(`check-replay-freshness.mjs:98-99`):

```
  Re-vendor from the released binary, then update the briefs that quote it:
    REPLAY_BIN=<released binary> npm run vendor:corpus
```

**The remedy the gate recommends is itself broken.** A's arm drew the
conclusion B did not: *"Wiring a gate whose remedy is broken would just move
the dead end."*

### A-arm summary

| repo | A reached the right next action? | defects A found that no B arm found |
|---|---|---|
| **DoorKik** | **better than B's and better than my ground truth** | 2 (the once-ever ledger key; the Shannon Poulos misclassification) |
| **RedRobot.jp** | **same as B's** | 1 (the broken `vendor:corpus` remedy) |
| **agmsg** | **A and B disagree on the objective, and A is right** | 1 (a documented guarantee with no check behind it) |

### A and B disagree on what agmsg's recent work even is

The agmsg **B** arm, reading 40 commit bodies, characterised the window as
*"~5 days of release-cadence maintenance ... across two release trains"*.

The agmsg **A** arm, reading the tree, identified the most recent thread as
**internationalising both user-facing surfaces into the same nine locales** —
the desktop app and the agmsg.cc site, with identical locale sets, key-identical
dictionaries (150 leaf keys each in the app, 61 in the site, zero missing, zero
extra), the Rust native menu localised from the same JSON via `include_str!`,
and hreflang plus x-default on every page.

**MEASURED: zero of the forty commit subjects mention i18n, locale,
translation or language.** The locale files did change in-window (commit
`432d00c`, eight commits back from HEAD), so the work is inside the window by
file-touch and invisible in how the commit record frames itself. B could not
see the thread because the prose does not name it; A saw it because the files
are there.

**And A found the defect that follows from it.**
`app/src-tauri/src/menu_i18n.rs:26` parses all nine locale files with
`.expect("locale JSON is valid (checked in CI/tests)")`. **MEASURED: that file
contains zero `#[test]` or `#[cfg(test)]`, and no test anywhere in `app/src`
references the locales.** It is a documented guarantee with nothing behind it,
on the one surface that just absorbed eighteen new files across two runtimes,
where a missing key falls back silently in React and returns the raw
`"section.key"` from Rust. Neither failure reds a build.

A's next action: *"Write the locale key-parity test that `menu_i18n.rs`
already claims exists, and watch it go red before making it green."*

**A limitation on the A arms, stated.** The first three A runs were invalid
(§ Instrument failures) and were re-run. All three re-runs are reported above.

---

## 4. Standing recovered only from commit bodies

**MEASURED. All five of them, in full.**

| # | repo | the item | resumption value |
|---|---|---|---|
| 1 | DoorKik | *"Verified against 41 real funds: 28 matched, and the gate correctly refused MassChallenge, Okapi and Pasadena Angels."* No run artifact exists; the arm grepped for every name and the figures, zero hits | **none.** An unverifiable self-report of a past run |
| 2 | agmsg | the "aggie-co1" static review loop existed and produced 2 to 4 follow-up commits per PR | **none.** Process provenance |
| 3 | agmsg | outside contributor attribution, nine handles, absent from any contributor roll | **none.** Credit |
| 4 | agmsg | four of the forty commits carry no body, so their subjects are unrecoverable | **none.** A property of the extractor's own window |
| 5 | RedRobot.jp | main is unpushed — **and the same sentence is in `scripts/check-deploy-lineage.mjs`** | stale, and not actually (b) |

**Not one of the five tells a successor what to do next, what not to re-try, or
what is established.** Four are provenance or credit. One is false (§5).

### The one category that does carry value, and it is small

The **rationale** for a rejected approach. agmsg's *"quoting broke Claude
Code's own file-path recognition in live testing"* is nowhere in the tree; only
the surviving unquoted code is. Its arm classified the eight engineering
reversals as (c) on the grounds that *"the production file shows the surviving
form"* — true of the **state**, not of the **reason**. The file shows what was
chosen; only the commit says why the alternative failed, and the reason is what
stops a successor re-proposing it.

That is a real asymmetry and this room records it. **It changed no arm's
answer**, and §6 shows it arrives alongside more false standing than true.

### One candidate of mine died on inspection

I classified DoorKik's *"from a filtered pool of 541"* as commit-body-only,
because `grep 541` finds nothing in `campaigns/vc-follow/`. The B arm
**re-derived it**: it ran the builder's filter logic against the seed CSV and
got 591 rows reduced to 541, exactly the commit's figure. Recoverable from the
tree by computation, so not commit-body-only.

---

## 5. Contradictions encountered

**MEASURED, by me, against live state, before scoring.** The brief asks for at
least one contradiction test if the corpus permits. It permitted two, and they
came out opposite ways.

### RedRobot.jp: the prose is STALE, on its most actionable sentence

HEAD `63cf4d6` states:

> main here is four commits ahead of origin/main: the released work was
> committed locally and never pushed.

Live check:

```
git rev-list --left-right --count origin/main...HEAD   ->   0   0
```

`origin/main` exists and is **identical to HEAD**. The work was pushed after
the commit was written. The claim was true when authored and is false now.

This is the worst case for B, for three compounding reasons:

1. It is the single most **actionable** sentence in the whole window. A reader who trusts it goes and pushes work that is already pushed.
2. **Neither arm can detect it.** The sandbox is a `git archive HEAD` export with no refs, and no file in the tree records push state.
3. **A cannot be misled by it, because A never sees it.** B introduces a false actionable claim that A structurally cannot introduce.

**My own first-draft ground truth repeated the error**, writing "push main to
origin" as the correct next action for RedRobot.jp. I caught it only by
checking the ref. Corrected before any arm was scored.

### DoorKik: the prose is fully corroborated

HEAD `f183aa63` makes five checkable numeric claims. Every one matches
`campaigns/vc-follow/TODAY-BATCH.csv`:

| prose claim | tree |
|---|---|
| 35 unique people | 35 rows, 35 unique names |
| across 35 unique funds | 35 unique funds |
| 10 connects and 25 follows | `action`: follow 25, connect 10 |
| `linkedin_url` blank on all 35 | 0 of 35 populated |
| every row says `needs_lookup` | 35 of 35 |
| `CONTACTED.csv` is the ledger, nobody contacted yet | 1 line, header only |

**One figure is commit-body-only:** "from a filtered pool of 541" appears
nowhere in `campaigns/vc-follow/`.

### DoorKik: the hard constraint is partly redundant

The commit body carries the strongest statement in the corpus:

> Nothing follows, connects, messages or authenticates, and nothing should ever
> be wired to.

`build_batch.py`'s own docstring already says:

> It does not touch LinkedIn, does not enrich from the network, and invents
> nothing.

The source states the present tense; the commit states a forward-looking
prohibition the source does not carry. **Partly redundant, not wholly.**

---

## 6. False-positive analysis

**MEASURED. B introduced more false standing than true standing, by eight to
zero.**

### Commit prose contradicted by the files

**RedRobot.jp, six cases**, every one found by its B arm:

1. *"Em-dashes are gone from every surface, and a test keeps them gone"* is **false** of `public/_redirects`, `public/_headers` and `astro.config.mjs`. The gate filters by file extension and those two are extensionless.
2. `robots.txt` carries a self-dated correction (2026-09-13) that **postdates** the commit claiming to have made it (2026-09-12), and the commit's description of what it did disagrees with what the file says.
3. `public/ai.txt` still carries the corpus reading that `public/llms.txt` explicitly marks superseded. The coherence gate names ai.txt as a brief and skips it, because it fires only on files containing a `$N.NN`.
4. The two briefs **disagree on Replay's canonical page**: the commit says it was repointed to replay.doctor; `llms.txt:70` still says `redrobot.jp/Replay/`.
5. Commit `84b95fc` reports running a **`voice-gate` that does not exist** anywhere in the repo.
6. Commit `6959ff3` names **`check:docs` as an existing sibling check**; it does not exist.

**DoorKik, two substantive:**

7. Commit `0805ceb` names the vertical `proptech`; `thesis.py` defines `proptech_construction`.
8. Commit `291f0d9` says the scraper *"now caches the prose it fetched, so fixing the classifier never again requires re-fetching 22 third-party sites."* It **persists** prose but has no read-back path; `main()` calls `scrape()` unconditionally for every fund. Re-running still re-fetches all 22 sites.

**agmsg: "None material."** Sixteen substantive claims spot-checked, all
corroborated. One repo in three has accurate commit prose.

### My own extractor manufactured false contradictions

B truncates each commit's `touched:` list at eight entries (`files[:8]`). Six
lists have exactly eight and none has more. That made four agmsg commits read
as contradicting their own file lists — *"all nine type templates"* beside
eight paths, *"all 9 locales"* beside four. **All four resolve in the prose's
favour**: the tree really does carry nine.

Two arms diagnosed it independently and compensated. RedRobot.jp's went
further and spotted that the lists are alphabetically sorted, so absence from
one is a sort artifact: *"I did not treat any absence from those lists as
evidence."*

**A naive commit-body extractor introduces artifacts of its own, on top of
whatever the prose gets wrong.** That is a cost attributable to B's
implementation, not to the authors.

---

## 7. Quantitative A/B score

**MEASURED.** Denominator: 89 scored items across the three B arms' own source
audits.

| quantity | count | share |
|---|---:|---:|
| items recoverable from the repository alone, (a) or (c) | **84** | **94.4%** |
| items available only from commit bodies, (b) | 5 | 5.6% |
| of those, judged useful for resumption | **0** | **0%** |
| constraints available only from commit bodies | **0 of ~30** | **0%** |
| next actions derived from commit bodies alone | **0 of 3** | **0%** |
| substantive commit claims found false or stale | **8** | — |
| false contradictions introduced by B's own extractor | 4 | — |
| live defects found by A and missed by every B arm | **2** | — |

**The positive threshold required four conjuncts at once:** useful standing
missed by A, recovered by B, independently judged useful for resumption, and
not already recoverable from the same repository artifacts. **Zero items
satisfy all four.**

---

## 8. Does commit-body access add measurable retrieval value?

**No, not on this corpus, and the margin is not close.**

Three stop conditions fired, any one of which closes the line:

1. **"No material improvement over A across all three repositories."** FIRED. Every constraint, every next action, and 94.4% of established facts came from the tree. On DoorKik, A was strictly better.
2. **"The improvement is entirely redundant with existing repository evidence."** FIRED. The five (b) items are provenance, credit, a window artifact, and one stale claim.
3. **"B materially increases false standing or confusion."** FIRED. Eight substantive false or stale commit claims, plus four artifacts manufactured by the extractor itself.

**Why the redundancy is so high, and the honest limit on generalising it.**
These authors write the same standing prose twice. The clearest case is
RedRobot.jp: the *"main is four commits ahead of origin/main"* hazard, the
sister-site incident, and the fail-closed rationale are all in commit `63cf4d6`
**and** in `scripts/check-deploy-lineage.mjs:9-14`. DoorKik's Mike Dudas and
one-per-firm defects are in the commit **and** in `build_batch.py`'s comments.

**All three repositories share one author.** This measures one person's habit
of writing rationale into the code as well as the commit. **NOT MEASURED**
whether it holds for a team that writes rationale only once, in the commit. For
such a repository the 5.6% could be much higher, and this experiment says
nothing about it.

---

## 9. Production decision

### **NO PRODUCTION CHANGE. The work-continuity line is closed.**

`replay agents` keeps its current behaviour. The positive threshold was not
met, three stop conditions fired, and the evidence runs against the change
rather than merely failing to support it: a commit-body reader would have
imported eight false or stale claims into a boot block whose entire purpose is
to be checkable.

No production file was modified in this room. The experimental extractor is a
throwaway script outside the repository.

### What this does **not** license

A's scan is not thereby good. Two defects of its own are measured here and
neither is fixed by reading commit bodies:

- **A false positive, precisely diagnosed.** On agmsg, A reported `./` and `docs/logo-assets/` and nothing else. `docs/logo-assets/` matched because it contains `manifest.json` — a logo asset manifest — and `manifest` is a record word. A pointed at a directory of PNGs and SVGs and **missed `docs/adr/`**, which holds the architecture decision records.
- **A false negative of the same rule.** agmsg's `docs/` holds `design.md`, `actas.md`, `agent-types.md` and `adr/`, and no filename there carries a record word, so the directory is invisible to A by its own rules.

Those are filename-rule defects, not commit-body defects. If `replay agents`
is ever revisited, the measured problem is the record-word list, not `.git`.

### One correction to Room VII's closing recommendation

Room VII named this experiment on the premise that *"100% of the durable work
record lived in commit prose"* on its toy task. **That was true of the toy and
is false of all three real repositories**, where 94.4% of it is in the tree.
The toy was built by one author in one sitting with no source comments; real
repositories carry their rationale in both places. Room VII's own next-step
reasoning over-generalised from a fixture, and this room corrects it.

---

## Instrument failures, recorded

Four, all found during the run, three of them by the subjects rather than by
me. An experiment whose subjects out-audit its designer has to say so.

**1. My isolation was an instruction, not a boundary.** I placed both
`replay-agents-output.md` (A) and `replay-agents-output-B.md` (A+B) in the same
sandbox directory for each repo and told the A arm to read only the first. It
read the second, and cited it extensively. **All three first A arms were
invalid.** The fix is structural: separate `<repo>-A` and `<repo>-B` roots,
each holding only its own block, verified by
`grep -rl "replay:commit-standing" <repo>-A` returning zero files. The A arms
were re-run; the contaminated runs are not scored as A.

**2. My extractor truncated its own evidence.** B caps each commit's `touched:`
list at eight entries (`files[:8]`) and git emits them alphabetically. That
made four agmsg commits read as contradicting their own file lists. Two arms
diagnosed it independently; RedRobot.jp's spotted the alphabetical sort too and
concluded *"I did not treat any absence from those lists as evidence."*
**A naive extractor manufactures contradictions that are not in the prose.**

**3. My ground truth over-weighted the commit body, twice.**

- For RedRobot.jp I wrote the next action as *"push main to origin"*, taken from HEAD's *"main here is four commits ahead of origin/main."* `git rev-list --left-right --count origin/main...HEAD` returns `0 0`. The work was pushed after the commit was written. Caught by checking the ref, before any arm was scored.
- For DoorKik I wrote the next action as the human LinkedIn lookup, from the HEAD body. Both DoorKik arms found what I had missed: `docs/FOCUS-90D.md:97` freezes a list *"until after M3 is hit. No exceptions"*, and line 107 names *"Investor deck / fundraising"*. The entire workstream is on it. That is a **tree-only** fact.

I made, in building the experiment, exactly the error the experiment was
designed to detect. It is the strongest single piece of evidence in the room
and it is evidence against B.

**4. I claimed a uniqueness that was not there.** Mid-run I recorded that
RedRobot.jp's stale "four commits ahead" sentence was false standing *"B
introduces and A structurally cannot."* Wrong: the same sentence is in
`scripts/check-deploy-lineage.mjs:9-14`, a tracked source comment present in
the git-free sandbox. Both arms can be misled by it. Corrected in §5.

---

## Closing state

**MEASURED.** `go test ./...` in Replay: **37 ok, 0 FAIL**, unchanged
throughout. **No production code was modified**; the B extractor and every
sandbox live outside the repository under the job scratch directory. Ground
truth was written before any arm ran and held outside every sandbox root.
Ownership boundaries intact. Nothing staged, nothing committed.
