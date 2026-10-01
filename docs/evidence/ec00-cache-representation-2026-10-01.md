# Can the index carry an unpriced-but-token-bearing unit?

**Discovery only. No production code was edited. `cmd/replay/cost.go` remains
byte-identical to 8582262.**

**Answer: yes, and the architecture already provides every mechanism needed.
No architectural expansion is required.**

---

## 1. What reaches `cache.put`

`cmd/replay/cost.go:727` — `cache.put(path, u, reqIDs, unjoinable)`.

The whole `costUnit`, plus the request ids and the unjoinable count.

**It runs only on the cold success path, after the gate at :680.** So today an
unpriced session is never cached at all, because it returns before reaching
line 727. That is why the warm path has nothing to replay.

## 2. What survives serialization

`cmd/replay/costcache.go:29` — `cachedUnit{Size, Mod, Unit costUnit, ReqIDs,
Unjoinable}`.

**`Unit` is the entire `costUnit`, serialized as JSON.** Every field with a
tag survives a round trip. **A new flag on `costUnit` therefore needs no
serialization work at all**, which is the single most important finding here.

## 3. Where `unpriced` is computed cold

`cost.go:709`, a local `int` incremented inside the cold `forEachSession`
walk. Declared at :658.

## 4. Where the warm path reconstructs it

**Nowhere. That is the entire gap.**

The warm loop at cost.go:645-655 calls `join.addCached(ids, unjoinable)` to
replay the join disclosures, and has **no equivalent for `unpriced`**. The
counter is also declared at :658, AFTER the warm loop, so it is not even in
scope there.

## 5. Can an existing field represent the state?

No existing field carries it, and one boolean on `costUnit` is sufficient.

**There is no false-$0 risk**, because the flag is precisely what stops `0`
from meaning a measured zero: a row with `CostUSD == 0` and `Unpriced == true`
says the dollars are unavailable, and a row with `CostUSD == 0` and
`Unpriced == false` says the session genuinely cost nothing.

## 6. Is the unjoinable mechanism reusable?

**The pattern transfers exactly. The field does not, and the difference is
worth stating rather than glossing.**

| | `Unjoinable` | proposed `Unpriced` |
|---|---|---|
| Shape | per-file **count**, 0..n | per-unit **boolean** |
| Why it is stored | those requests are deliberately absent from `ReqIDs`, so the count cannot be derived | the unit would otherwise not be cached at all |
| Replay mechanism | `join.addCached(ids, unjoinable)` | `if u.Unpriced { unpriced++ }` |

So reusing the **structure** (cache the datum, replay it warm) is safe.
Reusing the **field** would not be: one counts requests inside an otherwise
sound file, the other marks a whole unit.

The precedent is not merely analogous, it is the same lesson already learned
twice in this file. `cachedUnit.ReqIDs` and `cachedUnit.Unjoinable` both carry
comments saying a warm run that could not state them "would report the corpus
as fully joinable, which is the disclosure quietly dropped again."

## 7. Old-index safety, which turns out to need no work

`costIndexKey()` includes **`unitSchema()`**, which reflects over `costUnit`'s
JSON tags, sorts them and joins them. **Adding a field changes that string,
which changes the index key, which causes `load()` to discard a stale index
wholesale** at costcache.go:85.

The function's own comment describes this exact situation as settled
precedent:

> "v2 rather than v1 because this branch also changed the SHAPE: a cached unit
> now carries whether its request id was measured, and an index written before
> that cannot answer the join."

A cached unit would now carry whether its price was known, and an index
written before that cannot answer the disclosure. **Identical case, identical
response, and the machinery is already in place.**

---

## State-transition table

| State | Token evidence | Price | USD | Counts as priced? | Discloses unpriced? | Survives cache? |
|---|---|---|---|---|---|---|
| **priced** | yes | yes | measured | **yes** | no | yes, today |
| **unpriced** | yes | no | **absent, flagged** | **no** | **yes** | **not today: never cached** |
| **genuine zero** | yes | yes | `$0` measured | **yes** | no | yes, today |
| **unmeasurable** | no | — | absent | no | `unreadable`, separate counter | **no, and correctly so**: `cache.put` runs only on success, so an unreadable file is re-read every run |

The fourth row is verified, not assumed: cost.go:665 states it in code —
"Nothing is cached for them either, cache.put runs only on success, so a cold
count is the whole count." **Unreadable needs no change.**

---

## The smallest production change, specified and NOT applied

Six edits, all in `cmd/replay`, no new abstraction, no index-format work.

1. `costUnit`: add `Unpriced bool \`json:"unpriced,omitempty"\``.
2. `cost.go:658`: move the `unpriced, unreadable := 0, 0` declaration **above**
   the warm loop. It is currently below it and therefore out of scope there.
3. Warm loop at ~:651: `if u.Unpriced { unpriced++ }`, the exact analogue of
   `join.addCached` one line above it.
4. `cost.go:680`: flag instead of `return nil`, so the unit reaches
   `cache.put` at :727.
5. `cost.go:717`: hoist `u.RebilledTokens = deficit` out of the price branch.
6. `summarise`: skip unpriced rows when accumulating dollar statistics, and
   set `s.Tasks = len(costs)` so `tasks` keeps meaning priced rows.

**Conservation of semantic state, cold to serialized to warm, is proven by
construction rather than by the count happening to match:**

- the flag lives on `costUnit`, which is serialized whole (finding 2);
- the warm loop replays it, mirroring the mechanism already used for the join
  disclosures (finding 6);
- an index written by any earlier binary self-invalidates (finding 7).

## What this does NOT establish

That the change is correct. It establishes that the representation EXISTS and
that the cold-serialized-warm path can carry it. The six edits above have not
been applied, and the previous attempt failed on a regression that only the
full suite surfaced, so the next step is to apply them and let the suite
decide, not to assume this analysis is sufficient.
