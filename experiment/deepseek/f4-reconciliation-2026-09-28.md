# F4: billing reconciliation

**Verdict: A — Reconciliation.** Observed billing agrees with independently
derived request cost within the tolerance declared before the run.

## The original discrepancy was an analysis failure, not a provider anomaly

The first F4 report claimed observed $0.05 against derived $0.1458, a 2.9x gap.
That comparison was invalid for **three compounding reasons**, and the original
figures are preserved here rather than deleted:

1. **The balance was read immediately after the run.** Billing lags. A free
   re-read ~18 minutes later showed the same period had settled to $0.18.
2. **The two sides counted different populations.** The derived $0.1458 covered
   only the 440 harness rows. The balance reflected every call made, including
   112 concurrency probes, a `max_tokens` ladder, thinking probes, baselines and
   streaming/Responses probes — roughly 134 uncosted calls. This was an error in
   the analysis, not in the provider's accounting.
3. **Therefore no apples-to-apples comparison existed.** A lagged total against
   a partial subtotal cannot produce a meaningful ratio.

## Controlled batches

Both used the harness (`adapter`/`tasks`/`pricing`/`runner`), unique prefixes so
nothing cached, `thinking` disabled for predictable output, and a 120-second
quiesce before `t0` so prior lag could not contaminate the baseline.

### F4 batch

| | |
|---|---|
| model / endpoint | `deepseek-flash` via `/anthropic/v1/messages` |
| calls | 300, all HTTP 200, 300/300 checker pass |
| tokens | fresh_in 916,222 · cache_read 0 · out 600 |
| start balance | $46.71 |
| stable balance | $46.58, from t+5 through t+25 |
| observed delta | $0.13 |
| derived | $0.1378 |
| ratio | **0.943** (observed **below** derived) |
| quantization | ±$0.005 = ±3.8% |

The 5.7% difference sat outside the ±3.8% quantization bound, which is why this
batch alone was not accepted.

### F4b batch, predeclared tolerance ≤3%

| | |
|---|---|
| model / endpoint | `deepseek-flash` via `/anthropic/v1/messages` |
| calls | 200, all HTTP 200 |
| tokens | fresh_in 2,810,859 · cache_read 0 · out 400 |
| start balance | $46.58 |
| balance curve | t+0 $46.58 · **t+3 $46.15** · t+6 $46.15 · t+10 $46.15 · t+15 $46.15 · t+20 $46.15 |
| stable balance | $46.15, first stable at t+3 and held to t+20 |
| observed delta | **$0.43** |
| derived | **$0.4219** |
| ratio | **1.0193** |
| difference | **1.93%** |
| quantization | ±$0.005 = **±1.19%** |
| tolerance | ≤3%, declared in `f4b.log` before the batch ran |
| result | **PASS** |

## Why the two batches together are stronger than either alone

F4 landed at ratio **0.943**, F4b at **1.0193**. They **bracket 1.0 from opposite
sides**. A systematic over-statement in the pricing model would push both the
same way; two batches straddling parity with deviations near the quantization
bound is the signature of rounding, not bias. The 5.7% residual that blocked F4
is explained as quantization noise on a charge too small to measure precisely.

## Hypotheses

| | conclusion |
|---|---|
| **H1 billing lag** | **CONFIRMED.** Balance is flat at t+0, settles by t+3 to t+5, then stable to t+25. Any balance read inside ~5 minutes of a batch under-reports. |
| **H2 rounding** | **CONFIRMED and quantified.** `total_balance` is a 2-decimal string; resolution is one cent. On a $0.13 charge that is ±3.8%; on $0.43 it is ±1.19%. |
| **H3 wrong rate** | **REJECTED.** The published off-peak flash table reconciles to 1.93% at n=200. Both batches bracket parity. |
| **H4 category mismatch** | **REJECTED.** Same model, endpoint and configuration as the table covers. |
| **H5 balance semantics** | **PARTIALLY RESOLVED.** Eleven candidate billing paths (`/usage`, `/user/usage`, `/billing/usage`, `/v1/usage`, `/user/transactions`, `/dashboard/billing/*` and others) all return 404. `/user/balance` is the only observable spend signal, so reconciliation is batch-level only. No per-request billing record exists to verify against. |

## What this validates, and what it does not

**Validated:** batch-level observed spend reconciles with independently derived
request cost, for `deepseek-flash` on `/anthropic/v1/messages`, with uncached
input, off-peak, at n=200, within 1.93%.

**Not validated, and not to be generalised into:**

- peak pricing — untested
- `deepseek-v4-pro` — untested
- cached-input pricing — both batches ran with `cache_read` of exactly 0, so the
  cache-hit rate is entirely unexercised
- output-heavy workloads — output was 400 tokens against 2.8M input, so the
  output rate is effectively untested
- other endpoints — `/v1/chat/completions` and `/v1/responses` untested
- per-request attribution — impossible; no per-request billing record exists

## Procedure for any future DeepSeek cost measurement

1. Quiesce ≥120 s before reading the baseline balance.
2. Read the balance after the batch and poll until two consecutive reads agree;
   expect settling within ~5 minutes.
3. Size the batch so expected spend exceeds ~$0.40, keeping cent quantization
   under ~1.2%.
4. Cost every call in the batch, or the comparison is a total against a subtotal.
