# E4-01: can the existing primitives represent partial and evolving work state?

**2026-10-02. Campaign Phase 4. Result: the TAXONOMY is adequate, the CARRIER is
not. No new state taxonomy was built.**

## Question

The campaign names eleven work states (known, unknown, unresolved, stale,
contradictory, inferred, observed, reconstructed, pending, abandoned,
superseded) and instructs: do not invent a new taxonomy unless the existing
semantics demonstrably cannot express the distinction. This is that
demonstration, run before any design work.

## Null hypothesis

The existing primitives cannot express the eleven states, so a new taxonomy is
required.

## What already exists

`internal/stateledger` (present in the tree, uncommitted, authored by another
session) carries three vocabularies:

| | states | question it decides |
|---|---|---|
| `Standing` | ASSERTED, SUPPORTED, CONTRADICTED, SUPERSEDED, UNRESOLVED | what is currently believed |
| `Kind` | locates, executable, dispositive | what a check is CAPABLE of |
| `Outcome` | supports, contradicts, inconclusive, not-run | what a check returned |

plus `Claim.History []Entry`, and `Claim.Replacement *string` where nil means
the replacement is genuinely unknown. `internal/claims.EvidenceBasis` carries
OBSERVED / RECONSTRUCTED / RECONCILED / INFERRED separately.

## Method

A fingerprint function reduces a claim to everything an outside consumer can
read about it. Two states that produce the same fingerprint are not
represented, merely spelled. Eight states were constructed through the
package's own API and fingerprinted.

## Result: expressible, and distinct — with the full Ledger in hand

All eight produced distinct fingerprints. `Settle` also refuses a non-dispositive
check, so "the check ran" cannot close a question the check could never answer:
the action-is-not-outcome invariant is already enforced inside the state model.

| state | fingerprint |
|---|---|
| directly observed | `standing=SUPPORTED open=false history=2 check=dispositive/supports` |
| unknown | `standing=ASSERTED open=false history=1 check=none` |
| unresolved | `standing=UNRESOLVED open=true history=2 check=executable/inconclusive` |
| pending | `standing=ASSERTED open=false history=1 check=dispositive/not-run` |
| contradicted | `standing=CONTRADICTED open=true history=2 check=dispositive/contradicts` |
| superseded, replacement known | `standing=SUPERSEDED replacement=set history=2` |
| stale, replacement unknown | `standing=SUPERSEDED replacement=nil history=2` |
| abandoned | `standing=ASSERTED open=true history=1 check=none` |

**So the null is rejected. The taxonomy is adequate and no new one is needed.**

## Negative control on my own discriminator, which is where the finding is

The test above passes partly because the fingerprint consults the Ledger's side
tables. Restricting it to what a `Claim` itself carries — the shape any
serialized consumer would see — collapses three states into one:

```
INDISTINGUISHABLE: [unknown, pending, abandoned]
  all produce  standing=ASSERTED replacement=nil history=1
```

"Never investigated", "action taken, outcome not yet observable" and "dropped
without settling" are the same value on the claim. They are separable only by
consulting `l.open` and `l.checks`.

## Why that matters, verified

| | |
|---|---|
| JSON tags in `internal/stateledger/ledger.go` | **0** |
| `Marshal` / `WriteFile` / `json.` in non-test code | **none** |
| exported accessor for `checks` or for the open-question list | **none** (`HasOpenQuestion` returns a bool) |
| production callers | **none**; the only mention outside the package is a comment at `internal/claims/registry.go:11` |

`open` and `checks` are unexported and unserialized. So a claim that leaves the
Ledger — written to disk, handed to a fresh agent, carried across a session
boundary — arrives as ASSERTED with no way to tell "nobody has looked" from
"an action is outstanding" from "this was dropped".

That is the campaign's own invariant, missing evidence is not negative evidence,
failing at the carrier rather than at the vocabulary. It is the same shape as the
previous campaign's Phase 3 conclusion: the meanings do not need unifying, the
carrier does.

## Conclusion

- **Phase 4 taxonomy work: UNNECESSARY.** A duplicate primitive, which is one of
  this campaign's stated stop conditions.
- **The real Phase 4 gap: NO CARRIER.** `internal/stateledger` cannot cross a
  process boundary at all.
- **Phase 4C is therefore untestable as written** until the carrier exists. A
  fresh-agent continuity experiment has nothing to hand the fresh agent.

## Limitations

Expressibility only. This says nothing about whether these distinctions are
USEFUL to a fresh agent, which is an outcome question and is separately
**NOT MEASURED** — see the R10 trial, which stopped at a preregistered ceiling
with the control arm at 5/5 and never ran its treatment.

Eight states were constructed, not eleven: observed / reconstructed / inferred
live in `claims.EvidenceBasis`, a different and already-separate vocabulary, and
were not re-tested here.

## Next

Do not build a taxonomy. The open question is whether the carrier is worth
building, and that is gated on an outcome signal that does not exist in any
corpus examined.
