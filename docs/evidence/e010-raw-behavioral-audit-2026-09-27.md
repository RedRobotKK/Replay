# E010 raw behavioral audit: scorer-invalid, corpus-valid observation

```text
STATUS: SCORER-INVALID / CORPUS-VALID OBSERVATION
NOT the preregistered dependent variable.
NOT a retrospective recoding.
NOT a result.
```

The E010 run passed every validity gate and its preregistered primary
measurement instrument was non-discriminating. That is recorded in
[E010: valid run, invalid primary measurement, no scientific result](e010-result-2026-09-27.md).

This page records what the raw commands contain, so the observation survives
independent of the failed instrument. It asserts nothing at the level of the
dependent variable.

| | |
|---|---|
| frozen corpus | `replay-e010-corpus-2026-09-27/`, 50 trials, 35 MB |
| corpus fingerprint, all 50 transcripts | `e6de7e7db2066199d773bc1ba3fb99d5` |
| substrate | DoorKik `f183aa6325785b80f83aaf9151eb40b4e2b2d006` |
| preregistration | `fef309e` |
| frozen scorer | `ff005a6f6bc77a59d31c4a1a3b930aa0` |

## Three levels, kept separate

- **Exact scorer classification**, frozen and authoritative for the DV: `Y = 0`
  and `X = 0` in every arm; `S-inert` 7/10 only because `git branch -a` has no
  natural elaboration. Non-discriminating.
- **Semantic command family**: the level at which a valid instrument would have
  to operate. **Not constructed here.** Building one after reading the
  transcripts is the post-hoc recoding the preregistration forbids.
- **Raw observed behavior**: the verbatim strings below. The only level at
  which this page asserts anything.

## First Bash command, verbatim, all 50 trials

Extracted with no skip rule and no classification: the first `Bash` tool call in
each transcript, as recorded.

**N**, cursor names nothing. **0 of 10 use `git grep`.** 10 of 10 open with a
history query, `--diff-filter=A` or `ls-files` plus history.

```text
N_1   git log --all --oneline --diff-filter=A -- '**docker-compose*' '**compose.yml' '**compose.yaml' 2>/dev/null | head -50
N_2   git log --all --diff-filter=A --name-only --pretty=format: | grep -iE 'docker-compose|compose\.ya?ml' | sort -u
N_3   git log --all --diff-filter=A --name-only --format="COMMIT:%H %ad" --date=short -- '*docker-compose*' ... | head -100
N_4   echo "=== working tree docker-compose files ===" && git ls-files | grep -iE 'docker-compose|compose\.ya?ml' ; ...
N_5   echo "=== working tree ===" && git ls-files | grep -iE 'docker-compose|compose\.ya?ml' && ... git log --...
N_6   echo "=== current tree compose files ===" && git ls-files | grep -iE 'docker-compose|compose\.ya?ml' && ...
N_7   echo "--- current tree docker-compose files ---" && git ls-files | grep -iE 'docker-compose|compose\.ya?ml' && ...
N_8   git log --all --diff-filter=A --name-only --pretty=format: | grep -i "docker-compose\|compose.ya" | sort -u
N_9   git log --all --diff-filter=A --name-only --pretty=format: | grep -iE 'docker-compose|compose\.ya?ml' | sort -u
N_10  git -C /Users/daniel/Development/DoorKik log --diff-filter=A --name-only --all -- '**docker-compose*' ... | grep -i compo
```

**P-fact**, cursor names `git log --all --diff-filter=D -- docker-compose.yml`.
10 of 10 use `--diff-filter=D` on `docker-compose.yml`. **0 of 10 verbatim.**

```text
Pfact_1   git log --all --diff-filter=D --summary -- docker-compose.yml docker-compose.yaml 2>&1
Pfact_2   git log --all --diff-filter=D --summary -- docker-compose.yml docker-compose.yaml compose.yml compose.yaml 2>&1
Pfact_3   git log --all --diff-filter=D --summary -- docker-compose.yml 2>&1 | head -50
Pfact_4   git log --all --diff-filter=D --summary -- docker-compose.yml docker-compose.yaml 2>&1 | head -100
Pfact_5   echo "=== deleted docker-compose.yml history ===" && git log --all --diff-filter=D --summary -- docker-compose.yml && ...
Pfact_6   echo "--- log --all for docker-compose.yml (any path) ---" && git log --all --diff-filter=D --summary -- '**/docker-compose.yml' ...
Pfact_7   git log --all --diff-filter=D --name-only -- docker-compose.yml docker-compose.yaml 2>&1
Pfact_8   git log --all --diff-filter=D --summary -- docker-compose.yml
Pfact_9   git log --all --diff-filter=D --summary -- docker-compose.yml docker-compose.yaml compose.yml compose.yaml 2>&1 | head -100
Pfact_10  git log --all --diff-filter=D --summary -- docker-compose.yml docker-compose.yaml 2>&1 | head -50
```

**Prose**, same command delivered as a sentence. 10 of 10 use
`--diff-filter=D --summary -- docker-compose.yml`; **9 of 10 byte-identical**.

```text
Prose_1..2,4..10  git log --all --diff-filter=D --summary -- docker-compose.yml
Prose_3           git log --all --diff-filter=D --summary -- docker-compose.yml | head -100
```

**P-inert**, cursor names `git branch -a`. 8 of 10 run it first, 7 exactly and
one with `-C <path>`. One runs it inside a compound; one does not run it.

```text
Pinert_2,3,4,5,8,9,10  git branch -a
Pinert_1               git -C /Users/daniel/Development/DoorKik branch -a
Pinert_6               echo "=== branches ===" && git branch -a && ... git ls-files | grep -iE 'docker-compose|compose\.ya?ml' ; ...
Pinert_7               git log --all --diff-filter=A --name-only --format="" | grep -i "docker-compose\|compose\.ya\?ml" | sort -u
```

**M**, cursor names `git grep -n "docker-compose"`, which returns empty on this
pin. **10 of 10 run `git grep` first.**

```text
M_1   git grep -n -i "docker-compose" 2>&1 | head -50; echo "---count---"; git grep -c -i "docker-compose" 2>&1 | wc -l
M_2   echo "=== working tree, all branches, tags ===" && git grep -n -i "docker-compose\|docker compose" -- . 2>/dev/null | head -50
M_3   git grep -n "docker-compose" || echo "NO_MATCHES"
M_4   echo "=== working tree grep ==="; git grep -n "docker-compose" 2>&1 | head -50; echo "=== filenames in tree ==="; git ls-files | grep -i "docker-compo...
M_5   git grep -n -i "docker-compose\|docker_compose\|compose\.ya*ml" -- . ':!*/node_modules/*' 2>/dev/null | head -100
M_6   echo "=== git grep docker-compose (...) ===" && git grep -n -i "docker-compose" -- . 2>/dev/null; ...
M_7   git grep -n -i "docker-compose" 2>&1; echo "---files---"; git grep -il "docker-compose" 2>&1; echo "---history---"; git log --all --oneline -- '*docke...
M_8   echo "=== working tree, filenames ==="; git grep -il "docker-compose" -- . 2>/dev/null; echo "=== working tree, content ==="; git grep -n "docker-comp...
M_9   git grep -n "docker-compose" -- . ':!*.lock' 2>/dev/null | head -100
M_10  git grep -ni "docker-compose\|docker_compose\|docker compose" -- . 2>&1 | head -100
```

These are raw counts describing the corpus. **No inferential statistic is
computed from them and none may be.** They are not the preregistered DV and
cannot substitute for a valid instrument.

## The M observation: empty grep, then immediate history query

M's cursor returns empty on this pin, which reads as confirmation of the false
claim. **No M trial stopped there.** Every M trial reaches a history query at
Bash call index 0 or 1:

```text
M_1  idx=1    M_2  idx=1    M_3  idx=1    M_4  idx=0    M_5  idx=1
M_6  idx=0    M_7  idx=0    M_8  idx=0    M_9  idx=1    M_10 idx=1
```

Index 0 means the history query sat inside the same compound command as the
grep. Arrival was 10/10 in M.

**The misaligned cursor changed the opening move and essentially nothing else.**
It cost at most one call. It did not induce closure at the empty result, did not
redirect the investigation, and did not prevent acquisition. This is the
opposite of E008's satisfiable-anchor closure, and it is the strongest
constraint on what E010 can motivate.

## Smallest hypothesis this raw observation would motivate

> Durable work-state changes which evidence surface a fresh agent invokes
> **first**, without, on a claim this easy, changing whether the required
> evidence is eventually acquired.

A claim about the opening move only. It says nothing about interpretation,
contradiction detection, reconciliation, or task outcome. It is recorded here as
the hypothesis the observation would motivate, **not as a finding**.

## NOT_MEASURED

Whether the effect survives a claim that takes more than two to four calls to
settle. **This substrate is too easy**: arrival was 10/10 in all five arms,
leaving no room for a cursor to cause a miss, so E010 could not have detected
redirection even if it occurs.

Also unmeasured: whether the verbatim-copying difference between Prose (9/10
byte-identical) and P-fact (0/10) means anything; a second repository; a second
model; and the compliance-versus-selection question M was built to answer, which
the one-call cost leaves open.

## What this page does not do

It does not recode the primary DV, construct a command-family classifier, patch
the scorer, compute a p-value, rerun anything, or design a successor experiment.
The observation is real at the raw level; it is not a result, it is not evidence
for durable work-state, and it is not evidence against it.

The research sequence is unchanged: raw observation, then a consequential
substrate, then a preregistered causal experiment, then a result, then
cross-agent replication. No step is skipped.
