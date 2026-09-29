# Track B: billing reconstruction from saved artifacts

**Date** 2026-09-29. **Spend incurred by this track** zero. No inference call was
made. Every number below comes from a file already on disk.

**Scope.** The corpus is the 2026-09-29 probe session, provider `created`
window 03:26:36Z to 03:42:45Z. The 2026-09-28 experiments (DS-BASE, DS-SURF,
DS-CONC, DS-F1 to DS-F4b, WP-01, WP-02) are a separate session whose raw bodies
live in a different directory and are not reconstructed here.

**Evidence classes.** OBSERVED is a value literally present in a saved provider
response body or a balance response. DERIVED is arithmetic over OBSERVED values
and the published rate table. ASSUMED is a premise this campaign did not
establish. NOT_OBSERVED is evidence that was never captured, and it is never
read as zero or false.

**Reproduction.** `PYTHONDONTWRITEBYTECODE=1 python3
experiment/harness/track_b_audit.py <out.json>`, pointing at the probe
directory. It reads files and makes no network call.

---

## B1. Corpus reconstruction

### Procedure

Every `res-*.json` in the probe directory was parsed independently of the
existing summary artifacts. Each response was classified by dialect from the
presence of `prompt_cache_hit_tokens`, priced at its own `created` timestamp
through `experiment/harness/pricing.py`, and checked against four conservation
identities. The result was then cross-checked a second way, by summing the
`derived_usd` field of the seven per-run summary artifacts, which is an
independent path to the same total.

### Observations (OBSERVED)

| quantity | value |
| --- | ---: |
| `res-*.json` files | 479 |
| `req-*.json` files | 479 |
| response files carrying a `usage` block | 479 |
| response files carrying an `error` key | 0 |
| request stems without a matching response stem | 0 |
| `/v1/chat/completions` responses | 477 |
| `/anthropic/v1/messages` responses | 2 |
| `deepseek-flash` calls | 467 |
| `deepseek-v4-pro` calls | 12 |
| first provider `created` | 2026-09-29T03:26:36Z |
| last provider `created` | 2026-09-29T03:42:45Z |

Token totals, summed from the response bodies:

| field | chat dialect (477) | anthropic dialect (2) | corpus |
| --- | ---: | ---: | ---: |
| `prompt_tokens` | 6,288,253 | not reported by this dialect | n/a |
| cache hit | 5,885,824 | 6,400 | 5,892,224 |
| cache miss / fresh input | 402,429 | 449 | 402,878 |
| output | 56,477 | 32 | 56,509 |
| reasoning | 53,385 | not reported | 53,385 |
| cache creation | field absent | 0 | 0 |
| `total_tokens` | 6,344,730 | not reported | n/a |

Pricing regime, evaluated per call at its own timestamp: 479 of 479 fall in
peak. 2026-09-29 is a Tuesday and the window 03:26 to 03:42 UTC sits inside the
published 01:00 to 04:00 peak band.

Run structure, recovered by clustering the `created` timestamps on gaps greater
than 20 seconds. The clusters reproduce the seven summary artifacts exactly.

| run | calls | window (UTC) | DERIVED USD | cumulative |
| --- | ---: | --- | ---: | ---: |
| cacheprobe (DS-C1) | 29 | 03:26:36 to 03:27:09 | 0.006637 | 0.006637 |
| falsify (DS-C1) | 10 | 03:28:20 to 03:28:29 | 0.002804 | 0.009441 |
| clean (DS-C1) | 10 | 03:29:23 to 03:29:32 | 0.004146 | 0.013586 |
| arms (DS-OPT) | 72 | 03:32:04 to 03:32:43 | 0.043331 | 0.056917 |
| reconcile (DS-F5) | 200 | 03:34:10 to 03:34:17 | 0.039091 | 0.096008 |
| rscope (DS-RS, VOID) | 73 | 03:41:37 to 03:41:43 | 0.092117 | 0.188126 |
| rscope2 (DS-RS) | 85 | 03:42:40 to 03:42:45 | 0.041822 | 0.229947 |

Two of the 29 cacheprobe calls are the anthropic-dialect pair and carry no
`created` field, so they are placed in that run by file pairing and by the
summary artifact's own call count, not by timestamp.

### Conservation identities

All four checks ran over all 477 chat-dialect responses.

| identity | violations |
| --- | ---: |
| `prompt_cache_hit_tokens + prompt_cache_miss_tokens == prompt_tokens` | **0** |
| `prompt_tokens + completion_tokens == total_tokens` | **0** |
| `prompt_tokens_details.cached_tokens == prompt_cache_hit_tokens` | **0** |
| `completion_tokens_details.reasoning_tokens <= completion_tokens` | **0** |

No filename is reported because there is nothing to report. The identity also
holds on the aggregate: 5,885,824 + 402,429 = 6,288,253.

### Derived values

Cost per call is `fresh/1e6 * miss_rate + cache_read/1e6 * hit_rate + out/1e6 *
out_rate`, with the rate triple selected by model and by the call's own
timestamp. Worked example, the dearest call in the corpus, `res-60d2a62b.json`:
18,995 fresh, 0 cache read, 1 output, `deepseek-flash`, peak.

```text
18995/1e6 * 0.30  +  0/1e6 * 0.006  +  1/1e6 * 1.20  =  0.0056985 + 0 + 0.0000012
                                                      =  $0.0056997
```

| DERIVED quantity | value |
| --- | ---: |
| corpus total, per-file path | **$0.229946832** |
| corpus total, per-run-artifact path | **$0.229946832** |
| `deepseek-flash` (467 calls) | $0.222474048 |
| `deepseek-v4-pro` (12 calls) | $0.007472784 |
| cheapest single request | $0.0000303 |
| median single request | $0.000167796 |
| dearest single request | $0.0056997 |
| requests costing $0.01 or more | **0** |
| requests the rate table could not price | 0 |

**The existing figure of $0.22995 is CONFIRMED.** Two independent reconstruction
paths agree to the ninth decimal place: $0.229946832, which rounds to $0.22995.

### Evidence gaps

1. **Reasoning tokens are NOT_OBSERVED on 306 of 479 responses.** Only 173
   responses carry a `completion_tokens_details` object at all. The aggregate
   53,385 is the sum over the 173 that report it. `reconstruct.py` adds zero for
   the other 306, which promotes NOT_OBSERVED to zero in the reported reasoning
   total. Cost is unaffected, because reasoning tokens are a subset of
   `completion_tokens`, which is reported on every response and is what gets
   priced.
2. **Cache-creation tokens are OBSERVED as zero, not absent.** The field
   `cache_creation_input_tokens` is present on both anthropic-dialect responses
   with the value 0. The chat dialect has no such field, so cache creation there
   is NOT_OBSERVED rather than zero. Separately,
   `pricing.cost_usd` has no cache-creation argument, so a non-zero cache write
   would be silently unpriced. It is zero here, so the total is unaffected, but
   the cost function cannot express the quantity.
3. **Timestamps are NOT_OBSERVED on the 2 anthropic-dialect responses.** That
   dialect returns no `created` field. `reconstruct.py` substitutes the latest
   timestamp in the corpus, which is a silent promotion. Both calls fall inside
   the peak band under either the substituted timestamp or their true position
   in the cacheprobe run, so the regime and the cost are unchanged here. The
   substitution would matter on a run crossing 04:00 UTC.
4. **The peak classification carries an ASSUMED premise.** `pricing.is_peak`
   does not model Chinese public holidays, which the provider excludes from
   peak. If 2026-09-29 were a holiday, every call would be off-peak and the
   derived total would halve to about $0.1150. The reconciliation in B3 is
   itself evidence against that, because $0.1150 falls far outside the observed
   interval while $0.2299 falls inside it.

### Result

**CONFIRMED.** 479 responses, zero conservation violations, zero errors, zero
unpriced calls, and a corpus total of $0.229946832 reproduced by two independent
paths.

---

## B2. Missing-call accounting

### Procedure

Three independent checks. First, structural: `adapter.Adapter._post` writes
`req-<id>.json` to disk before dispatching, then hands curl `-o res-<id>.json`.
Any call that was dispatched through the harness therefore leaves a request
artifact whether or not the response arrived, so an orphan request stem is the
signature of a lost response. Second, census: the per-experiment call counts in
`EXPERIMENT-LEDGER.md` were reconciled against the saved corpus. Third, search:
the repository and the campaign artifacts were searched for any other record of
a call whose response was not written.

### Observations (OBSERVED)

- 479 request stems and 479 response stems, and the two sets are identical.
  There are no orphans in either direction.
- All 479 responses parse as JSON and all 479 carry a `usage` block. There is no
  dispatched-but-unparsable response.
- `transport.py` constructs its HTTP adapter with `max_retries=0`, so the
  harness does not silently re-dispatch a request and bill it twice.
- Every summary artifact reports `errors: 0`, and the four carrying a budget
  block report `refused: 0` and `outstanding_usd: 0.0`.
- The ledger's 2026-09-29 rows account for the corpus as follows.

| ledger row | ledger n | saved responses | status |
| --- | ---: | ---: | --- |
| DS-C1 cache characterisation | 49 | 49 (29 + 10 + 10) | saved |
| DS-F2b reasoning-parameter shapes | 5 | **0** | **NOT_OBSERVED** |
| DS-OPT arms | 72 | 72 | saved |
| DS-F5 reconciliation batch | 200 | 200 | saved |
| DS-RS reasoning scope | 84 | 85 | saved, one more than the ledger row |
| DS-RS first run, recorded VOID | not in n | 73 | saved, absent from the n column |
| DS-DOC, DS-WF, DS-RW | 0 | 0 | no calls |

Saved 479, plus 5 unsaved, gives 484 calls known to have occurred in the
session. The ledger n column sums to 410 for the session, and the 74-call
difference is fully explained by the VOID first DS-RS run (73 calls, real spend,
deliberately excluded from the ledger n column) and the one extra DS-RS call
(85 saved against 84 reported).

### The 5 unsaved calls

DS-F2b tested four reasoning-parameter shapes and a no-parameter control, one
call each, by raw curl without `-o`. No file was written and none can be
recovered. What survives is the transcription in
`experiment/deepseek/optimization-arms-2026-09-29.md`.

| body | output tokens | reasoning tokens |
| --- | ---: | ---: |
| control, no parameter | 40 | 40 |
| `reasoning_effort: "none"` | 1 | not reported |
| `thinking: {"type": "disabled"}` | 1 | not reported |
| `reasoning_effort: "minimal"` | 32 | 30 |
| `enable_thinking: false` | 15 | 13 |

These output counts are transcribed from a campaign document, not read from a
saved body. Their input token counts are NOT_OBSERVED entirely. They are not
zero. A probe prompt is described as trivial, which is a qualitative statement
and not a measurement.

### Derived values

Output-side floor for the five calls, at the peak `deepseek-flash` output rate:

```text
(40 + 1 + 1 + 32 + 15) tokens = 89 tokens
89 / 1e6 * 1.20 = $0.0001068
```

Input contributes a strictly positive but unmeasured amount on top.

**LOWER BOUND on session spend, rate-table path:**

```text
$0.229946832  (479 saved responses, DERIVED)
+ $0.0001068  (89 transcribed output tokens on the 5 unsaved calls)
= $0.2300536   ->  LOWER BOUND $0.23005
```

**LOWER BOUND on session spend, balance path**, which needs no rate table: the
balance fell from $46.01 to $45.78, and under the resolution analysis in B3 the
true fall is strictly greater than **$0.2200**. This bound is weaker in value
and stronger in provenance, because it assumes nothing about published prices.

### What an upper bound would require

No upper bound is derivable from saved evidence alone. Closing it needs one of:

1. The raw response bodies of the 5 DS-F2b calls. They were never written. They
   cannot be recovered by reading, and re-running them would be new spend rather
   than recovery of old evidence.
2. A provider-side per-request usage or billing record. The surface map
   enumerated eight endpoints and found none. `/user/balance` returns an account
   total and no line items.
3. A conditional bound accepted on stated premises. If the published rate table
   is exact and no non-campaign activity touched the account in the window, then
   from the B3 interval the total is strictly below $0.2400, so the 5 unsaved
   calls together cost strictly less than `$0.2400 - $0.229947 = $0.010053`.
   That is a bound on a premise, not a measurement.

### Result

**PARTIALLY SUPPORTED.** The set of unsaved calls is fully enumerated at 5, and
the enumeration is structurally defensible because the harness writes a request
artifact before dispatch and no orphan exists. The cost of those 5 calls remains
NOT_OBSERVED on the input side, so the corpus total is a lower bound and not a
total. Reported as **at least $0.23005**.

---

## B3. Balance reconciliation with correct precision

### Procedure

The display semantics of `/user/balance` were established from saved raw
response bodies rather than from recollection. The resolution interval of a
difference of two readings was then derived from those semantics, and the
reconciliation was expressed as that interval.

### Observations (OBSERVED)

Seven raw `/user/balance` response bodies are saved on disk. Every one has the
same shape:

```json
{"is_available":true,"balance_infos":[{"currency":"USD",
 "total_balance":"46.89","granted_balance":"0.00","topped_up_balance":"46.89"}]}
```

`total_balance` is a **JSON string**, not a number, and it carries exactly two
decimal places in all seven bodies. `granted_balance` is `"0.00"` in all seven
and `topped_up_balance` equals `total_balance` in all seven, so there is no
promotional credit drawing down in parallel with the paid balance.
`reconcile.py` reads this field through `float(...)`, which destroys the string
form, so the artifacts that record balances as numbers are not evidence of the
wire format. The seven raw bodies are.

Balance readings relevant to this session:

| reading | time | provenance |
| ---: | --- | --- |
| $46.01 | session start | reported in the task brief, no raw body on disk |
| $46.00 | before the DS-F5 batch, about 03:34:08Z | `probe/reconcile.json` |
| $45.96 | 90s after the DS-F5 batch | `probe/reconcile.json` |
| $45.83 | between | reported in the task brief, no raw body on disk |
| $45.78 | 2026-09-29T05:13:53Z | reported in the task brief |
| $45.78 | 05:15:35Z, poll 0 | `probe/balance_poll.jsonl` |
| $45.78 | 05:17:35Z, poll 1 | `probe/balance_poll.jsonl` |
| $45.78 | 05:19:36Z, poll 2 | `probe/balance_poll.jsonl` |
| $45.78 | 05:21:36Z, poll 3 | `probe/balance_poll.jsonl` |
| $45.78 | 05:23:37Z, poll 4 | `probe/balance_poll.jsonl` |

**The poll file is incomplete as a record.** It holds 5 rows spanning 8 minutes
at a 2-minute cadence, from 05:15:35Z to 05:23:37Z, and the poller was still
running when this track read it. All 5 rows read 45.78. The file records a
parsed float and not the raw body, so it carries no independent evidence of the
wire format.

### Derivation of the resolution interval

Let `q = 0.01` be the display quantum. A displayed reading `B` corresponds to a
true balance `T`:

- under round-half-up, `T` lies in `[B - q/2, B + q/2)`
- under truncation toward zero, `T` lies in `[B, B + q)`

Which of the two the provider uses is **NOT_OBSERVED**. It does not need to be,
because the reconciliation concerns a *difference* of two readings. In both
semantics each reading's error spans a half-open interval of width `q`, and the
two errors are independent, so for `D = B0 - B1`:

```text
T0 - T1  lies in  (D - q, D + q)     width 2q = $0.02, in either semantics
```

This is the correction that matters. `reconstruct.within_resolution` computes
`[obs - 0.005, obs + 0.005)` from `HALF_CENT_NANO = 5_000_000`, which applies a
single reading's resolution to a difference of two readings and so understates
the interval by a factor of two. The bug is in the direction that makes a
reconciliation look tighter than the instrument allows. The constant should be a
full cent, `10_000_000` nano-USD, and the comparison should be strict at both
ends rather than half-open, because neither endpoint is attainable: the lower
end needs the first reading at its minimum and the second at its unattainable
supremum, and the upper end needs the reverse.

That function was rewritten by another track while this one was running, to fix
a float boundary defect by comparing in integer nano-USD. That fix is correct
and unrelated. The interval width is still wrong.

### Derived values

```text
D  = 46.01 - 45.78 = $0.23                     OBSERVED difference
q  = 0.01                                      OBSERVED display quantum
true spend in the window  ->  (0.23 - 0.01, 0.23 + 0.01)
                          ->  ($0.2200, $0.2400)      open interval, width $0.02

DERIVED corpus (479 saved)                     $0.229947
DERIVED lower bound including the 5 unsaved     $0.230054
```

**$0.230054 lies inside ($0.2200, $0.2400).** The point ratio
`0.23 / 0.229947 = 1.00023` is not the claim and should not be quoted as one.
The claim is the interval and the fact that the derived figure sits in it.

Headroom, on the premise that the rate table is exact and that no non-campaign
spend touched the account: the observed upper edge minus the derived corpus
gives `$0.2400 - $0.229947 = $0.010053` of room for everything not in the
corpus.

### Settlement

The prior session-level reconciliation, recorded in `EXPERIMENT-LEDGER.md` as
OPEN at a factor of 1.9, compared $0.05 observed against $0.0959 derived. That
comparison is reproduced exactly here. At the moment of the $45.96 reading the
cumulative derived total was $0.096008, and `46.01 - 45.96 = $0.05`, giving
1.92. The gap closes completely once settlement finishes. The same account,
read 99 minutes after the last call, gives $0.23 observed against $0.229947
derived. Settlement lag explains the gap and per-request cent truncation does
not, and B6 shows why truncation cannot.

The lag is large and is directly OBSERVED. At the $46.00 reading, taken just
before the DS-F5 batch, $0.056917 of derived cost had already been incurred and
only $0.01 had appeared on the balance. Roughly $0.047 was outstanding at that
instant.

This also strengthens DS-F5 rather than weakening it. Because of that lag the
$0.04 fall observed across the DS-F5 window cannot be attributed cleanly to the
DS-F5 batch, so the isolated 1.023 ratio is weaker than it reads. The
session-level test is stronger and points the same way: pricing the 3,744,384
cache reads at the miss rate instead of the hit rate would add about $1.10 to
the corpus and put the total near $1.33, which is two orders of magnitude
outside the observed interval.

### Evidence gaps

1. **The session-start reading of $46.01 has no raw body on disk.** It reaches
   this track as a transcribed number. The reconciliation is sensitive to it.
   Anchoring instead on the artifact-backed $46.00 would give the interval
   ($0.2100, $0.2300), and the derived lower bound $0.230054 would fall just
   outside it. That is not a valid alternative anchor, because $46.00 was read
   after four runs had already spent $0.056917, but it shows the margin is thin
   enough that the missing raw body matters. Future balance reads should write
   the response body to disk.
2. **The $45.83 reading is untimed** and cannot be placed against the run
   sequence.
3. **Sole occupancy of the account is ASSUMED.** Nothing in the evidence rules
   out other spend on the same key in the window.
4. **Settlement completeness is evidenced, not proven.** Five identical readings
   over 8 minutes, 91 minutes after the last call, is good evidence that
   settlement finished. It is not a guarantee, and the withdrawn DS-F4 died of
   exactly this assumption made too early.

### Result

**CONFIRMED, as an interval.** Observed spend lies in **($0.2200, $0.2400)**.
The derived lower bound of $0.230054 lies inside it. The previously OPEN
session-level reconciliation at a factor of 1.9 is closed, and the mechanism is
settlement lag.

---

## B6. Cent-resolution falsification without spending

### Hypothesis under test

Per-request cent truncation causes sub-cent calls to bill as zero. Formally, the
amount billed for a request of true cost `c` is `floor(100 * c) / 100`.

### Procedure

The hypothesis makes a point prediction about the corpus that the corpus already
answers. Every request was priced individually and the distribution of
per-request cost was compared against the $0.01 quantum. The predicted session
total under the hypothesis was then compared against the observed balance
movement. No new call is involved at any step.

### Observations (OBSERVED)

| quantity | value |
| --- | ---: |
| requests in the corpus | 479 |
| dearest single request, DERIVED | $0.0056997 |
| requests whose DERIVED cost is $0.01 or more | **0** |
| balance at session start | $46.01 |
| balance at 05:13:53Z and across 5 later polls | $45.78 |

### The arithmetic

Under the hypothesis, every one of the 479 requests bills zero, because every
one costs strictly less than $0.01:

```text
floor(100 * 0.0056997) / 100 = floor(0.56997) / 100 = 0 / 100 = $0.00
```

and the dearest request is the hardest case, so the same holds for all 479.

```text
predicted session total under the hypothesis  =  479 * $0.00  =  $0.00
```

The observed balance movement is:

```text
46.01 - 45.78 = $0.23
and under the B3 resolution analysis the true fall is strictly greater than $0.22
```

`$0.22 > $0.00`. The prediction fails by the entire session.

The 5 unsaved calls cannot rescue the hypothesis. Under the hypothesis they bill
zero too, unless one of them individually reached $0.01. For a single
`deepseek-flash` call to reach $0.01 at the peak miss rate it would need about
33,300 fresh input tokens:

```text
0.01 / (0.30 / 1e6) = 33,333 tokens
```

against a parameter-shape probe whose entire output was 1 to 40 tokens. Even
granting the most generous case, one call billing $0.01 leaves $0.21 of observed
balance movement with no source.

### What this does and does not settle

Refuted: truncation to whole cents **at the per-request level**.

Not tested by this evidence: truncation or rounding at a coarser granularity,
for example per settlement batch or per billing period. The corpus cannot
distinguish those, because it observes only two endpoints of one session. A test
would need a session whose derived total sits near a half-cent boundary, with
balance reads bracketing it and confirmed settlement, and it would need to be
repeated. That is a separate experiment and it is not required to answer the
question as posed.

### Evidence gaps

The per-request costs are DERIVED, not billed amounts. The provider publishes no
per-request charge on any endpoint in the surface map. The refutation therefore
rests on the rate table being approximately correct. It is robust to large error
in that table. The dearest request in the corpus would have to be understated by
a factor of `0.01 / 0.0056997 = 1.75` before that one call reached a cent, and
the median request by a factor of `0.01 / 0.000167796 = 59.6` before a typical
call did. A rate table wrong by 59.6x would not have reconciled against the
balance in B3.

### Result

**REFUTED. NO ADDITIONAL SPEND REQUIRED.** The existing corpus falsifies
per-request cent truncation on its own. 479 of 479 requests cost under $0.01, so
the hypothesis predicts a session total of exactly $0.00, and the balance fell by
more than $0.22.

---

## Summary

| task | result |
| --- | --- |
| B1 corpus reconstruction, $0.22995 | **CONFIRMED** at $0.229946832 by two independent paths, zero conservation violations |
| B2 missing-call accounting | **PARTIALLY SUPPORTED**, 5 calls enumerated and unrecoverable, lower bound $0.23005 |
| B3 balance reconciliation | **CONFIRMED as an interval**, ($0.2200, $0.2400), derived lower bound inside it |
| B6 cent-truncation falsification | **REFUTED**, no additional spend required |

### Corrections this track proposes to existing artifacts

1. `reconstruct.within_resolution` halves the reconciliation interval.
   `HALF_CENT_NANO = 5_000_000` gives `[obs - 0.005, obs + 0.005)` for a
   difference of two cent-resolution readings, where the correct interval is the
   open `(obs - 0.01, obs + 0.01)`. The integer nano-USD comparison in that
   function is a separate and correct fix landed by another track.
2. `experiment/harness/reconstruct.py` sums reasoning tokens as zero for the 306
   responses that do not report them, promoting NOT_OBSERVED to zero.
3. `experiment/harness/reconstruct.py` substitutes the corpus's latest timestamp
   for responses with no `created` field, which is a silent promotion that would
   misprice a run crossing a regime boundary.
4. `experiment/harness/pricing.py` has no argument for cache-creation tokens, so
   that priced quantity cannot be expressed. It is zero in this corpus.
5. `EXPERIMENT-LEDGER.md` records the session-level reconciliation as OPEN at a
   factor of 1.9. It is now closed at the interval in B3, and the mechanism was
   settlement lag.
6. Balance reads should persist the raw response body. The session-start $46.01
   reading, on which the reconciliation depends, survives only as a transcribed
   number.
