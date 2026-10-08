# Replay vs. Replay Doctor: comparison and a recommendation, 2026-10-08

**This file recommends a default canonical name. It does not change one.**
No file in this repository is renamed by this pass, and no package is
republished under a different name. The decision stays Daniel's; this is
the evidence-based comparison his own task specification asked for, with an
actual recommendation this time rather than a declined one.

## 1. What is actually live under each name, re-verified today

| Surface | Name live there | Verified how |
|---|---|---|
| GitHub repository name, `go install` path, CLI binary (`replay`) | **Replay** | `gh repo view RedRobotKK/Replay --json name` → `"Replay"`; `go.mod` → `module github.com/RedRobotKK/Replay`; CLI binary name is `replay` |
| GitHub repository description | **Replay Doctor** | `gh repo view` description: "Replay Doctor names the turn it broke on, the cause, and the tokens re-billed..." — the repo's own name and its own description disagree |
| npm registry | **replay-doctor** | `npm view replay-doctor dist-tags` → `latest: 0.8.0`, versions 0.6.0–0.8.0 live; four platform packages `@replay-doctor/<platform>` |
| PyPI registry | **replay-doctor** | `pypi.org/pypi/replay-doctor/json` → latest `0.8.0`, same version history |
| Public domain | **replay.doctor** | Live site, fetched directly today; CITATION.cff's own `url:` field points here |
| CITATION.cff | **Replay** (title), but `url` points at replay.doctor | Read directly: `title: Replay`, `url: "https://replay.doctor"` |
| `plugins/replay/skills/replay-doctor/SKILL.md` | Both, in the same file | Heading `# Replay Doctor`, first body sentence "Replay reads the transcripts..." |
| `plugins/replay/.claude-plugin/plugin.json` | Both | `"name": "replay"`, `"description"` starts "Replay Doctor: at the end of each session..." |
| `README.md` | **Replay**, with one sentence naming the alternate | Fixed this pass (see `gtm-02-public-surface-retrofit-2026-10-08.md`): title and body now agree, with an explicit note that npm and the web use "Replay Doctor" |

**Six of nine surfaces checked already say "Replay Doctor" somewhere, and
three of those six say both names in the same file.** This is not a typo
anywhere it appears more than once (npm's four platform packages and PyPI
are both deliberate, already-published package names, not a slip), and it
is not confined to one layer (code identity, package identity, and web
identity each independently chose a name, and two of the three chose
differently from the repository's own name).

## 2. Product clarity and positioning

**"Replay"** is accurate to the mechanism (it replays provider-side caching
behaviour from a transcript) but is a generic, highly overloaded word with
no connection to the specific problem (prompt-cache billing) a stranger is
trying to solve. **"Replay Doctor"** adds the diagnostic framing the product
actually leads with everywhere it speaks to a user first: the CLI's own
`--help` opens "if the agent bill went up and nothing errored, a prompt
cache broke" — a diagnosis, not a replay. The npm package description,
written for an audience with zero context, uses "Replay Doctor" for exactly
this reason: it is the more self-explanatory of the two names to someone who
has never heard of either.

## 3. Consistency across surfaces

Consistency currently favors neither name outright, because neither is used
everywhere: "Replay" wins the code-identity surfaces (GitHub repo, Go module
path, CLI binary, CITATION.cff title), "Replay Doctor" wins the
distribution-identity surfaces (npm, PyPI, the public domain, the GitHub
repo's own description). Picking either name as canonical and then
updating every other surface to match it is the same amount of
documentation work either way; picking "Replay Doctor" additionally avoids
touching three live package registries (see migration cost, below).

## 4. Search and discoverability (INFERRED, not measured)

**This section is inferred from the surfaces already checked, not from
search-ranking data.** This session's web-search budget was exhausted
before a query could be run to measure it directly, so treat this as
reasoned inference, not verified evidence, and re-run the query later if a
measured answer is wanted.

"Replay" as a bare search term is a common English word and a common
product name across unrelated categories (gaming replay tools, video
replay services, sports replay systems) with no inherent connection to LLM
cost or prompt caching; a search for "replay" alone would not surface this
project without an additional qualifier. "Replay Doctor" is a two-word
compound with no obvious prior claimant in the coding-agent-cost space,
already matches a registered domain (replay.doctor) and two live package
names (npm, PyPI), and is the more distinctive string of the two by
construction. The inference, not a measurement: "Replay Doctor" more
reliably reaches someone searching for this specific problem, and "Replay"
more easily reaches someone who already knows the GitHub repository by
name.

## 5. Migration cost and risk

| Direction | Cost | Risk |
|---|---|---|
| Canonicalize on **Replay Doctor** | Rename the GitHub repository (redirects automatically; `go install` paths referencing the old path would break unless a redirect or vanity import path is added), update `go.mod`'s module path (a breaking change for any existing `go install github.com/RedRobotKK/Replay` user unless an alias/redirect is kept), update CITATION.cff's title, update the CLI binary name (breaking for anyone with `replay` on their `$PATH` or in scripts) | Breaks existing installs and scripts that reference `replay` or the current module path; GitHub's own repo rename redirect mitigates the web-facing part but not the Go module path or binary name |
| Canonicalize on **Replay** | Rename the npm package (new package name, old `replay-doctor` would need to be deprecated in place pointing at the new one, since npm does not support renaming a published package or its four platform companions), rename the PyPI package (same constraint), migrate or alias the replay.doctor domain, update the GitHub repository description and every doc/plugin file currently using "Replay Doctor" | Lower technical risk (nothing with import-path or `$PATH` binary-name consequences breaks), but loses two already-indexed, already-versioned package registry identities and would need `replay-doctor` on both registries to keep pointing new installs at wherever "Replay" ends up published |

**Migration cost is asymmetric, and it favors keeping "Replay Doctor" as the
canonical distribution name and "Replay" as the canonical code/repository
name** rather than collapsing to one everywhere — exactly the split that
already exists today, made explicit instead of accidental.

## 6. Conflicting names currently in use (summary)

Already itemized in section 1. Restated as the one-line version: the
product has one codebase and two live, externally-committed names, split
along a code/distribution boundary that happens to track where the name was
first needed (the repository existed before the npm package or the domain
did), not along any product-identity boundary.

## 7. Recommendation

**Recommended default: treat "Replay" as the canonical code identity
(GitHub repository, Go module path, CLI binary name, CITATION.cff title)
and "Replay Doctor" as the canonical product/distribution identity (npm,
PyPI, the public domain, and anywhere the product is introduced to someone
who does not already know the repository).** This is not a coin flip
between two equally-costed options: section 5's asymmetry is the deciding
fact. Renaming three already-published, versioned package identities
(`replay-doctor` and its four platform companions on npm, `replay-doctor` on
PyPI) to follow a repository rename is strictly more disruptive, and less
reversible, than documenting that the existing split is intentional and
making every file that currently contradicts itself (the three identified
in section 1) say so explicitly, the way `README.md` now does.

**What this recommendation is not:** it is not a decision. The owner
approval this task's own operating constraints require before any name is
changed anywhere has not been sought or given, and nothing in this pass
renames anything. If Daniel wants full consolidation under one name instead
(for brand simplicity over package-identity continuity), that is a
legitimate different answer this evidence does not foreclose, it is just the
costlier one by the measure above.

## 8. One concrete next action available without a naming decision

Independent of which name Daniel eventually picks as canonical, every
surface identified in section 1 that contradicts *itself* (names both in
one file, in a way that reads as an error rather than a deliberate note)
should say so explicitly, the way `README.md` was fixed to in this pass.
Two remain: `plugins/replay/skills/replay-doctor/SKILL.md` (heading says one
name, first sentence says the other, with no explanatory note) and
`plugins/replay/.claude-plugin/plugin.json` (same pattern, in its
`name`/`description` pair). Fixing those two does not require resolving the
canonical-name question, only stopping two files from silently disagreeing
with themselves; left open here rather than fixed, since the task's
constraint on not silently choosing a name on the owner's behalf extends to
not using self-consistency edits as a backdoor to pick one.

---

[Evidence index](README.md) ·
[GTM-02 retrofit](gtm-02-public-surface-retrofit-2026-10-08.md) ·
[Release criteria](../../RELEASE-CRITERIA.md)
