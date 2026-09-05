# Vectorising the session data

**2026-09-04.** Three different questions hide inside "vectorise", and they have different answers.
Measured before deciding.

## Question 1: embed the session content?

**No, and it is not close.** Replay never stores message text: `Block.Text` is `json:"-"` and
`stripText` runs on both the request and response paths. **Embedding requires the text.** Adding it
would invert the privacy position the entire product rests on, in exchange for semantic search over
prompts a user already has in their own transcripts.

## Question 2: represent each session as a feature vector, and find similar ones?

**Yes. This is the right idea and it is what the product direction already needs.**

"Sessions with this waste trajectory usually go on to waste more" is a nearest-neighbour claim. The
vector is a handful of numbers already computed: error share over binned turns, re-read rate, break
count, cached share, turn bucket, tool count, held milliseconds.

**Call it a feature vector, not an embedding.** Nothing is learned and no model is involved. That
distinction matters when explaining it, because "we embed your sessions" invites exactly the question
this design exists to avoid.

### Two things that decide whether it works, neither of which is the algorithm

**Normalisation, which is where this usually fails.** The features have wildly different scales:
`error_share` is 0 to 1, turn count is 1 to 500, prompt tokens span three orders of magnitude.
**Euclidean distance over unnormalised features is dominated by whichever column has the widest
range**, so the "nearest" session would be whichever had a similar token count, and the waste signal
would be noise underneath it. Per-feature standardisation is not a refinement here, it is the
difference between working and not.

**Sample size against dimension count, which is the real constraint.** k-NN needs roughly ten samples
per dimension to mean anything, and a hundred to be comfortable:

| Features | Minimum sessions | Comfortable |
|---:|---:|---:|
| 5 | 50 | 500 |
| 10 | 100 | 1,000 |
| 20 | 200 | 2,000 |
| 384 (a real embedding) | 3,840 | 38,400 |

**Replay has eleven sessions from one machine.** A heavy user generates about 1,500 a year. So a
five-to-ten feature vector becomes meaningful for a single active user within months, and a
twenty-feature one needs a corpus.

**And in sparse high dimensions, "nearest" stops meaning anything.** Measured on random points: in
2 dimensions the farthest point is 21x the distance of the nearest; at 20 dimensions it is 1.96x; at
384 it is **1.16x**. When every point is roughly equidistant, a nearest-neighbour result is noise
shaped like an answer. **Keep the vector small on purpose.**

## Question 3: use a vector database?

**No.** Brute force is faster than the argument for an index at this scale.

| Rows × dims | Size | Brute-force scan |
|---|---:|---:|
| 1,500 × 20 | 0.2 MB | **~0.1 ms** in Go |
| 15,000 × 20 | 2.4 MB | **~1 ms** |
| 150,000 × 20 | 24 MB | **~10 ms** |

A vector index exists to avoid scanning millions of rows in hundreds of dimensions. **Fifteen
thousand rows in twenty dimensions is a nested loop**, and it costs zero dependencies, no index to
rebuild, no recall/latency tradeoff to tune and no approximate results to explain. The same argument
that ruled out SQLite in ADR-0010 rules this out harder, because the payoff is smaller.

## What this means in practice

- **One small feature vector per session**, five to ten dimensions, stored in the derived summary
  from ADR-0010. No new file, no new dependency.
- **Standardise per feature** against the corpus, and say which corpus, exactly as the cache rules
  carry a date.
- **Brute-force k-NN**, and revisit only past roughly a million rows, which is centuries of use for
  one person.
- **A similarity vector is a stronger identifier than the scalars it came from**, because it is
  designed to make sessions distinguishable. ADR-0009 already binned the turn series for the corpus;
  **the feature vector must be binned at least as coarsely, or stay local entirely.** The safest
  version is that vectors never leave the machine and only the aggregate percentiles come back.

## The honest position today

**With eleven sessions, similarity search would return confident nonsense.** Enough points exist to
compute a distance and far too few for it to mean anything, which is the most dangerous amount of
data to have. **Build the feature vector, use it locally once a user has a few hundred sessions of
their own, and do not put a similarity claim in front of anyone before then.**

---

[Architecture](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
