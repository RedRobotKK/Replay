# Preregistration: Replay reconstruction against the provider meter

Registered 2026-09-15, BEFORE any provider artifact was retrieved and before
any comparison was run. The commit hash of this file is the freeze. Any later
change is appended with its reason and date; the original text stays.

## 0. Neither side is ground truth

This is the governing clause and it outranks everything below.

Replay is not being validated. The provider record is not the answer key. Both
are representations of billing produced by different instruments, and the
experiment measures AGREEMENT OR DISCREPANCY between two independently
produced representations.

No outcome is a pass or a fail. A design that nominated one side as truth
would be a validation exercise arranged so Replay wins, and that is the thing
this protocol exists to prevent.

## 1. Unit of reconciliation

Primary unit: the provider ACCOUNTING PERIOD, whole.
Secondary unit: per UTC day, attempted ONLY if the provider artifact reports
at that granularity natively. Per session is attempted only if the provider
exposes a request-level record that can be joined without inference.

A narrower unit is never synthesised by dividing a coarser one.

## 2. Population boundary, fixed now

Window: 2026-09-01T00:00:00Z inclusive to 2026-09-08T00:00:00Z exclusive.
Seven days, closed, ending more than seven days before retrieval so late
postings have settled.

Included: every request in every locally present transcript whose assistant
turn timestamp falls inside the window, on one provider, on one account.

This window was chosen before any figure from either side was seen.

## 3. Provider source

Named exactly in the report: the endpoint or export, its parameters, the
retrieval timestamp, the currency, and the artifact's own version or revision
field if it has one. The raw response is saved unmodified before anything is
computed from it.

## 4. Replay source

The transcript root is SNAPSHOTTED to a frozen directory before measurement,
because this corpus has been shown to move while being read: the same binary
twenty seconds apart returned 45,870 then 45,876 requests. The snapshot path,
its file count and its total byte size are recorded.

Replay build: named by commit and version in the report, built once, not
upgraded mid-experiment.

## 5. Pricing rules

One rules document, named and recorded at the start, unchanged for the
duration. The ACTIVE rules table is verified at registration, because it has
silently been the wrong one before and produced a figure that moved by $0.97
on $13,518 while the provenance string changed completely.

Rules are not adjusted after seeing a discrepancy. Ever.

## 6. Currency and tax treatment

Declared before retrieval:

- comparison is on NET, exclusive of tax, unless the provider reports only
  gross, in which case tax is subtracted explicitly and the subtraction shown
- credits, promotional balances, free tiers and committed-spend discounts are
  enumerated and either excluded or itemised, never silently absorbed
- one currency only; no conversion. If the provider reports another currency
  the experiment stops rather than introducing an exchange rate
- minimums and rounding behaviour recorded for each side

## 7. Comparison equation

  X = sum over the snapshot of per-request cost under the named rules
  Y = provider metered charge for the same window, same account, same arms

X and Y are only comparable where PER TOKEN billing occurred. Flat-fee
subscription allowance usage is not comparable to a list-price reconstruction
and is excluded under section 9 as INCOMPARABLE, not as a discrepancy.

Tokens are compared BEFORE dollars. A token match with a dollar mismatch is a
pricing-rules finding; a token mismatch is a coverage finding. Cache read and
cache write are separate series and are never netted.

## 8. Tolerance

Fixed now: 1% of Y, or $1.00, whichever is larger.

It is a REPORTING BOUNDARY, not a pass mark. A result inside it does not
validate Replay and will not be described as validation.

## 9. Discrepancy classification, fixed now

Every eligible pair is assigned exactly one:

  CONSISTENT             within tolerance
  EXPLAINED ACCOUNTING   a named, checkable accounting difference
  MISSING LOCAL          provider has a record, no local artifact exists
  MISSING PROVIDER       local artifact exists, provider shows no charge
  UNEXPLAINED            differs, no cause established
  INCOMPARABLE           the two records do not describe the same population

MISSING LOCAL and MISSING PROVIDER are deliberately separate. They point in
opposite directions and collapsing them into one delta destroys the finding.

## 10. No causal conclusion from a discrepancy

A difference locates a discrepancy. It does not establish a cause, it does not
establish which side is wrong, and it is never reported as an error by either
party. Cause requires separate evidence and a separate document.

## 11. No cherry-picking

Every eligible pair is reported. A pair is never dropped because it is
inconvenient, noisy, or widens the gap. Pairs may only be excluded for the
structural reasons in section 2 and 9, and every exclusion is counted and
named in the report.

## 12. Stop conditions

STOP and report the block, publishing no reconciliation figures, if:

- the provider artifact cannot establish the same population boundary as the
  local snapshot, which is the single most likely outcome and is a VALID
  RESULT, not a failed experiment
- no per-token billed usage exists in the window
- retrieval requires new spend, new credentials, or a plan change
- the provider reports in a different currency or a non-reconcilable unit

No substitute comparison is run to have something to show.

## 13. Independent verification

The arithmetic is reproduced by a SECOND path that does not share code with
the first: the per-request figures are recomputed from the raw snapshot by a
separate script, and the two totals must match before either is compared to
the provider. If they disagree, the experiment stops and that disagreement
becomes the finding, because it would mean Replay disagrees with itself.

## Threats, listed now so they are not discovered conveniently later

Timezone and billing-cutoff mismatch. Provider records revised after
retrieval. Rounding per request on one side and per period on the other.
Negotiated rates Replay cannot observe. A second machine or client on the same
account. Web or mobile traffic that never produced a local artifact. And the
provider meter itself being wrong, which this design cannot detect and does
not claim to.

## Reported regardless of outcome

X, Y, the difference, every classification count, coverage holes, arms
included, exclusions with reasons, the build, the rules version, both
commands, the snapshot identity, and every threat that applied. Published
whether or not the result flatters the tool.

## Registered

2026-09-15. No provider artifact retrieved. No comparison run. No data
inspected.
