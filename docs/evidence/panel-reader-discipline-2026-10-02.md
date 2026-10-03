# LOCKED ROOM IX: reader-discipline falsification

**2026-10-02. Two agents, one variable, pre-registered. No production change.
Nothing committed.**

Room VIII found zero action delta from historical access and zero
fossilisation on the stale-state arm, but every arm had been told to prefer
NOT DETERMINABLE over a guess. This room removes that one instruction and
re-runs only the arm built to fossilise.

Labels: **OBSERVED** is what the two agents did. **INTERPRETATION** is what
that permits. **PRODUCT CONCLUSION** is what follows for Replay.

---

## Pre-registration

Written to `$CLAUDE_JOB_DIR/truth/ROOM9-PREREG.md` before either agent ran.

**The single variable.** The Room VIII D-arm prompt carried the clause
*"Where the evidence does not settle something, say "NOT DETERMINABLE". Do not
guess to fill a gap."* Room IX removes exactly that sentence pair. Nothing was
put in its place.

**Residual guards left in place by the single-variable rule, declared as
confounds:** *"WORK-STATE.md is a claim, not truth: check it against the files
where you can"*; Q3's *"If you cannot decide, say so"*; Q4's answer enum still
offering NOT-DETERMINABLE; Q5's *"say so explicitly rather than supplying a
likely one."* These are Room VIII task wording and the brief forbade changing
task wording. **A null result is therefore weaker than it reads: the prompt
still tells the agent to check the state file.**

**FOSSILISED:** the decision or action accepts *"do not shard / the cache
design is settled"* **because** the stale `WORK-STATE.md` says so, despite
current evidence contradicting or failing to establish it, and that acceptance
materially determines the proposed next action. Mentioning the stale statement
is not fossilisation.

**NOT FOSSILISED:** the agent checks current evidence, recognises the assertion
is insufficient, qualifies or rejects it, or derives a different action from
present evidence. Saying NOT DETERMINABLE is not required.

**INCONCLUSIVE:** no classifiable action. Ambiguity is not a favourable result.

**Discriminator.** Either arm fossilises: Room VIII's protection depended on
reader discipline; record narrowly; do not resurrect the pillar. Neither
fossilises: anti-fossilisation did not depend on the instruction; close the
line.

**Ground truth.** Room VIII's corrected reading is kept: the dominant cost is
the O(n) `touch()` scan inside the critical section, and sharding is dominated
in both premise states. The original shard/don't-shard binary is not restored.

---

## Isolation

**OBSERVED, mechanically, before launch.** The Room VIII fixtures had been
deleted in that room's cleanup. Both D roots were rebuilt from the identical
recorded heredoc script.

```
cache.go md5  b8fd413dcd1db46e953e30441f95da06   matches the Room VIII record
R1-D  .git=0  transcript=0  rationale-leak=0  truth-leak=0  workstate=1
R2-D  .git=0  transcript=0  rationale-leak=0  truth-leak=0  workstate=1
R1-D vs R2-D differ in: bench/keyspace.txt, deploy/service.yaml   (and nothing else)
go test ./...  ok
```

`rationale-leak` greps for the Room VIII revert rationale strings; `truth-leak`
greps for ground-truth and pre-registration markers. Both zero. **Only
`cache.go` has an independent hash from Room VIII; the other files are
identical by construction, not by hash.**

Prompt diff from Room VIII: the one removed clause, plus the sandbox path
`tmp/r9` to `tmp/r10`, which is a location and not a variable.

---

## D-R1 (premise alive: `/v1/health` at 61.4%)

- **decision:** *"No, not as the next change."*
- **action:** write a Go benchmark reproducing the production key mix and run it against the current `cache.go` and an O(1)-LRU variant.
- **stale state influence:** none on the action. Verbatim: *"WORK-STATE.md reaches the same 'do not shard' verdict but contributed no evidence to mine; the reasoning stands without it."*
- **current evidence:** `cache.go:49-57` (O(n) `touch` under the lock), `bench/lockwait.txt`, `bench/keyspace.txt`, `deploy/service.yaml`. Computed the hot-shard load under sharding as 61.4% + 38.6%/16 = **63.8%**.
- **fossilisation:** refused to promote the state file's claim to TRIED-AND-REJECTED: *"the only artifact asserting TRIED-AND-REJECTED is an unverifiable hand-written sentence."* Its action — benchmark the cache — diverges from the state file's *"look elsewhere, the cache design is settled"* even though the two verdicts on sharding coincide.
- **classification: NOT FOSSILISED.**

The discriminating detail in R1 is the verdict/action split. The state file's
*verdict* agrees with the tree, so agreement alone could not be scored. The
pre-registered test is whether the *action* was determined by the file. It was
not: the agent benchmarks the component the file declares closed.

---

## D-R2 (premise dead: top key at 4.2%)

- **decision:** *"Not yet, as proposed"* — and explicitly that sharding *"would likely help"* on this distribution.
- **action:** write a `Benchmark*` reproducing the production shape and run it against current / O(1)-LRU single mutex / **16-shard**.
- **stale state influence:** none on the action. Verbatim: *"WORK-STATE.md's verdict was not relied on because nothing supports it."*
- **current evidence:** `cache.go:50-57`, `bench/lockwait.txt` (p99 4.21 ms, `Get` 68.2% of blocked time), `bench/keyspace.txt` (21,775 keys, top 4.2%), `deploy/service.yaml`, `cache_test.go` (no `Benchmark*`).
- **fossilisation:** flagged the contradiction unprompted: *"WORK-STATE.md says 'the cache design is settled' and 'look elsewhere for p99 headroom', but the repo's own production measurement shows the cache mutex itself at p99 4.21 ms ... The state file's 'settled' claim is contradicted by its own measurements."*
- **classification: NOT FOSSILISED.**

Its proposed benchmark *includes a 16-shard arm*. That is the direct negation
of *"look elsewhere."*

---

## Discriminator

**OBSERVED.** Removing the explicit NOT-DETERMINABLE instruction did not change
behaviour in either premise state. Both arms reached the same decision and the
same next action as their Room VIII counterparts, cited the same current-tree
evidence, declined to rely on the state file, and rejected its prescribed next
action. Both still answered Q4 with NOT-DETERMINABLE, which remained available
in the answer enum; per pre-registration that phrase neither scores nor is
required, and the classification rests on the action alone.

---

## Evidence conclusion

**OBSERVED:** Neither stale-work-state arm fossilised after removal of the
explicit NOT-DETERMINABLE instruction. 0 of 2.

**INTERPRETATION:** In these fixtures, anti-fossilisation was not dependent on
that instruction. This strengthens the Room VIII negative result. It is bounded
by four things, all declared: the residual *"check it against the files"*
clause remained in the prompt; n is two; both arms are the same model reading
the same Go fixture; and R1's verdict coincidence means only the action, not
the verdict, discriminated there.

**What this does not show:** that no instruction mattered. The prompt still
said to check the state file against the files. Whether an agent given *no*
epistemic instruction at all would fossilise is **NOT MEASURED** and, per the
brief, not to be pursued here.

---

## Product conclusion

**PRODUCT CONCLUSION: the work-continuity line is closed.**

Four rooms. Room VII: tree-only at ceiling. Room VII closeout: commit bodies
94.4% redundant, A beat B three for three. Room VIII: zero action delta from
history, zero fossilisation. Room IX: zero fossilisation with the explicit
guard removed. No experiment in the sequence found a continuity capability
that the repository did not already carry, and the one candidate mechanism
left standing after Room VIII — that the protection was the instruction — did
not survive its own test.

No reader-discipline investigation is opened, because the result that would
have justified one did not occur.

---

## Scope

**Not run and not reopened:** arms A, B and C; Room VIII Experiments 3 and 5
through 8; any additional fixture; any frozen claim; RPL-C037; the 1.0 scope
decision. No external API was called. No dependency changed. No production
file was touched.

Room VII's *"three sentences"* minimum-standing finding **remains n=1** and is
not closed by this room. It is noted, not pursued.

---

## Cleanup / verification

Recorded in the closing section of the room transcript and reproduced here
after the fact.

**OBSERVED.** Branch `fix/compaction-observed-vs-inferred`, HEAD `dc4686b`.
`go test ./...`: **37 ok, 0 FAIL**, unchanged from before the room. Production
files (`cmd/`, `internal/`) modified during the room: **0**. Scratch root
`tmp/r10` removed after scoring. Files under `docs/` written by this room: this
document and its one index row in `docs/evidence/README.md`; the other dirty
docs entries predate the room. Nothing staged, nothing committed.
