# 26. A surviving guard is permitted only with evidence, addressed one guard at a time

**Status:** Proposed
**Date:** 2026-09-20

> ADR number 0025 is reserved for a record prepared on 2026-09-18 and still
> awaiting review. This one takes 0026 so the two cannot collide.

## Context

[ADR-0014](0014-checks-must-be-able-to-fail.md) established that a check is not
evidence until it has been observed to fail, and `scripts/guard-reachability`
enforces it: every conditional a change introduces is neutralised in turn, and
one whose removal leaves the suite green fails the run.

The rule worked, and then it met a reader.

`internal/transcript/jev.go` parses a capture format whose whole contract is
that a record which violates it is refused and nothing partial comes back. Most
of the file is refusal. When its contract tests began running in the ordinary
suite, guard-reachability reported 48 surviving conditionals. Tests closed 23 of
them. The 25 that remained were not untested code, and they were not dead code
either. They were **defensive propagation**: an error check whose removal moves
the refusal one call deeper without changing whether the record is refused.

Four distinct situations were found under one reported verdict, and the reason
this record exists is that the reviewer could not tell them apart:

1. **Contract-level behavioural coverage.** A guard whose removal changes
   whether a record is accepted, or crashes the reader. Three were found this
   way: the scanner-error check (an oversized line was accepted as a partial
   result), the duplicate-key walk's decode check (the walk looped on a decoder
   that never advanced), and the identity namespace check (an id shorter than
   the prefix panicked on the slice that follows it). These are defects. They
   are closed with tests, never with an entry in a manifest.
2. **Defensive propagation, dominated.** A guard whose removal leaves every
   contract-level outcome unchanged on a measured corpus: the record is still
   refused, nothing partial is returned, and only which diagnostic names the
   failure differs. This is a claim about the code as it stands and a refactor
   can invalidate it.
3. **Structurally unreachable.** A guard no input can enter, by a property of a
   type or a dependency rather than by absence of effort. This is a claim about
   a contract.
4. **Diagnostic identity.** The thing that actually differs in case 2. A reader
   that says "missing input_tokens" and one that says "input_tokens: unexpected
   end of JSON input" have made the same decision about the record and told the
   producer different things about how to fix it. Diagnostic identity is worth
   keeping and is not worth a test per call site.

Two ways out were considered and rejected. Writing a test per survivor buys
assertions that a diagnostic string has not changed, which is a change-detector
rather than a contract; and deleting the guards trades a named refusal for a
decode failure two frames deeper, which is the reader getting worse.

## Decision

A surviving introduced guard may be permitted, and only by a per-guard entry in
`internal/guardcheck/testdata/guard-evidence.json` carrying its address, a
category (`dominated` or `structurally-unreachable`), a justification, and the
evidence the justification rests on.

The mechanism is deliberately narrow.

**The address names one guard.** It is `{pkg, func, cond, producer}`, where
`producer` is the normalised text of the statement that produced the value the
condition reads: the `if` statement's own init clause where it has one, and
otherwise the preceding statement in the same block. There is no wildcard, no
file-level entry, no package-level entry and no "waive this" flag.

**There is no ordinal.** An ordinal within the identity was the first design and
it was dropped on evidence: inserting an identical conditional ahead of an
evidenced one renumbers it, so the entry silently retargets a guard nobody
reviewed with every field of the address unchanged. The producing statement does
not move when something else is inserted, and it changes when the guard itself
changes, which is exactly when an entry should stop matching.

**`Identity` is untouched.** `guardcheck.Identity{Pkg, Func, Cond}` remains the
grouping key that base-tree grandfathering spends by count, and
`EvidenceAddress` is a separate concept in a separate file. Identity is not
unique per guard — 25 Jev survivors occupy 17 identities — and building
addressing on it would have exempted guards by collision.

**Everything fails closed.** A manifest that will not parse fails the run; so
does an entry with no justification, no evidence, an unknown category, or a
duplicate address. A survivor with no entry fails the run exactly as before. An
entry that matches no surviving introduced guard is *stale* and fails the run
too, for the reason the frozen-mutant catalogue fails on an anchor that no
longer resolves: an entry nobody can tie to a guard has stopped describing the
tree, and letting those accumulate is how this becomes the blanket waiver it
exists not to be.

**Order matters.** The manifest is consulted only after the base tree has had
its say, so an evidenced entry can never stand in for grandfathering the base
could have granted on its own.

**An evidenced guard does not disappear.** It is reported in its own category,
with its justification, on every run.

## Consequences

The reviewer's default is unchanged: a new guard that survives neutralisation
fails the build. What changes is that a reviewer who has measured why can say
so in a form the machine checks and the next reader can audit.

The categories age differently and that is the point of separating them. A
`dominated` entry is a claim about today's control flow; the honest response to
a refactor that invalidates one is a failing run, which is what stale detection
produces. A `structurally-unreachable` entry is a claim about a dependency's
contract and should outlive refactors of our own code.

An entry is not a substitute for a test, and the first pass proved it: of the 48
survivors, 23 were closed by tests, and three of those were closed because the
guard turned out to be load-bearing rather than defensive. One of the three was
found only because a corpus input was added specifically to try to break the
guard rather than to confirm it. The discipline that makes this policy safe is
that the corpus is built to falsify the domination claim, not to illustrate it.

Two limits are stated rather than papered over. A domination claim is scoped to
the corpus it names and is not a proof of universal impossibility. And a manifest
entry records that someone looked; it does not record that they looked hard. The
only defence against a careless entry is that the justification and the evidence
are written out where review can see them, which is why both fields are required
and neither may be empty.

## Status of the first application

Applied to the 25 surviving guards in `internal/transcript/jev.go`: 24
`dominated`, 1 `structurally-unreachable`. Measured on 2026-09-20 by neutralising
each condition as `false && (cond)` and re-running a 77-input corpus (75 contract
violations, 2 valid records). No input became accepted under any neutralisation,
and no input newly returned a partial result alongside an error.

---

[ADR index](README.md) · [Documentation index](../README.md)
