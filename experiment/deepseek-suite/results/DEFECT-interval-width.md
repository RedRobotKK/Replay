# DEFECT: the reconciliation interval is half its true width

**Status: OPEN.** Fix deferred to avoid a concurrent-edit clobber; Track D is
still writing to `experiment/harness/reconstruct.py`. This record exists so the
defect cannot be lost if the fix is interrupted.

## The error

`reconstruct.within_resolution` uses `HALF_CENT_NANO = 5_000_000` and builds the
interval as `(observed - half_cent, observed + half_cent)`, width $0.01.

That is the resolution of a **single** balance reading. The quantity being
reconciled is a **difference of two** readings, and each carries its own
independent quantisation error.

    before observed 46.01  ->  true in [46.005, 46.015)
    after  observed 45.78  ->  true in [45.775, 45.785)
    difference           ->  true in (0.2200, 0.2400)      width $0.02

The correct interval is `(D - 2q, D + 2q)` where `q` is half the display
resolution, and it is open at both ends. It is identical under rounding or
truncation, which is worth keeping because the provider's rounding rule is
NOT_OBSERVED.

## Why it matters

The bug is not neutral. It **flatters every reconciliation** by halving the
band a derived figure has to land in. Any claim of the form "derived lies inside
the observed resolution interval" was tested against a band twice as tight as
the evidence supports, so the claim is conservative rather than wrong, but the
stated precision is not what the instrument has.

This is my error, propagated. I wrote `[$0.225, $0.235)` in the E-H3 record and
in `reconstruct.py`, having proceeded as though only the closing reading were
uncertain.

## Fix

Interval `(D - 2q, D + 2q)`, `q` derived from the observed field format rather
than hardcoded. `total_balance` is a JSON **string with two decimals**, verified
in 7 raw bodies, so `q = 0.005`. A regression test must pin the width at `4q`
and fail if the interval is built from a single reading's error.
