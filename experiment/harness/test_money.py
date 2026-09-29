"""python3 experiment/harness/test_money.py

Fixture-only: no network, no credential, no spend.

Every monetary path in the harness was audited for float-accumulation hazards
(Track D, D2). The audit classified each site as a COMPARISON AGAINST A
THRESHOLD, which can change a decision, or a DISPLAY/AGGREGATE FIGURE, which
cannot. These tests pin the ones that can change a decision, plus the two that
were found broken.

The original defect, already fixed in fanout.Budget: ten additions of 0.002 give
0.020000000000000004, which is over a ceiling of 0.02, so a ten-call budget
granted nine reservations. test_fanout covers that the integer nano-USD
conversion holds. What follows is the rest of the audit.
"""
import datetime as dt
import io
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import fanout  # noqa: E402
import meter as meter_mod  # noqa: E402
import pricing  # noqa: E402
import reconstruct  # noqa: E402


class TestThePricingRegimeBoundaryIsExact(unittest.TestCase):
    """is_peak compares a FLOAT against four thresholds and decides a 2x rate.

    `h = hour + minute / 60` is float arithmetic feeding `1 <= h < 4` and
    `6 <= h < 10`. That is the dangerous class: a misclassification at a boundary
    would price a whole run at twice or half the correct rate. It happens to be
    safe, and "happens to be" is not a property anyone should have to re-derive,
    so it is pinned here against an exact integer rule.
    """

    @staticmethod
    def _exact(weekday_minutes):
        h, m = divmod(weekday_minutes, 60)
        return (h == 1 or h == 2 or h == 3) or (6 <= h < 10)

    def test_every_minute_of_a_weekday_agrees_with_integer_arithmetic(self):
        base = dt.datetime(2026, 9, 28, tzinfo=dt.timezone.utc)   # a Monday
        self.assertEqual(base.weekday(), 0)
        for minutes in range(24 * 60):
            when = base + dt.timedelta(minutes=minutes)
            self.assertEqual(pricing.is_peak(when), self._exact(minutes),
                             f"regime disagrees at {when:%H:%M} UTC")

    def test_the_four_boundaries_land_on_the_documented_side(self):
        d = lambda h, m=0, s=0: dt.datetime(2026, 9, 28, h, m, s,
                                            tzinfo=dt.timezone.utc)
        self.assertFalse(pricing.is_peak(d(0, 59)))
        self.assertTrue(pricing.is_peak(d(1, 0)), "01:00 is inside peak")
        self.assertTrue(pricing.is_peak(d(3, 59)))
        self.assertFalse(pricing.is_peak(d(4, 0)), "04:00 is outside peak")
        self.assertFalse(pricing.is_peak(d(5, 59)))
        self.assertTrue(pricing.is_peak(d(6, 0)))
        self.assertTrue(pricing.is_peak(d(9, 59)))
        self.assertFalse(pricing.is_peak(d(10, 0)))

    def test_seconds_are_discarded_and_that_never_crosses_a_boundary(self):
        """is_peak ignores seconds. Harmless only because every boundary falls on
        a whole hour, so no second can sit on the far side of one."""
        for h, m in ((3, 59), (9, 59), (0, 59), (5, 59)):
            at00 = pricing.is_peak(dt.datetime(2026, 9, 28, h, m, 0,
                                               tzinfo=dt.timezone.utc))
            at59 = pricing.is_peak(dt.datetime(2026, 9, 28, h, m, 59,
                                               tzinfo=dt.timezone.utc))
            self.assertEqual(at00, at59, f"seconds changed the regime at {h}:{m}")

    def test_the_weekend_is_off_peak_at_every_hour(self):
        sat = dt.datetime(2026, 10, 3, tzinfo=dt.timezone.utc)
        self.assertEqual(sat.weekday(), 5)
        for hour in range(24):
            self.assertFalse(pricing.is_peak(sat + dt.timedelta(hours=hour)))


class TestTheBudgetIsExactWhileTheMeterDrifts(unittest.TestCase):
    """Both figures exist, they disagree, and only one of them decides anything.

    OBSERVED in the campaign's own artifacts (tmp/probe/): arms.json reports
    settled_usd 0.043330728 and derived_usd 0.043330728000000006 for the same
    72 calls; rscope.json reports 0.0921174 against 0.09211739999999996. The
    difference is float accumulation in meter.derived_usd. It is tolerable
    BECAUSE derived_usd is printed and never compared to a ceiling.
    """

    def test_the_settled_total_is_exact_where_the_float_sum_is_not(self):
        per_call = 0.000601815
        n = 72
        drifted = 0.0
        b = fanout.Budget(10.0, 0.002)
        for _ in range(n):
            drifted += per_call
            self.assertTrue(b.reserve())
            b.settle(per_call)
        exact = round(per_call * n, 9)
        self.assertNotEqual(drifted, exact, "pick a value that actually drifts")
        self.assertEqual(b.settled, exact,
                         "the budget must not inherit the float sum's drift")

    def test_the_drift_can_never_change_a_reservation_decision(self):
        """The failure the nano conversion exists to prevent, at the exact
        ceiling where it first bit: ten calls of 0.002 against 0.02."""
        b = fanout.Budget(0.02, 0.002)
        self.assertEqual(sum(1 for _ in range(10) if b.reserve()), 10,
                         "the tenth reservation is inside the ceiling")
        self.assertFalse(b.reserve(), "the eleventh is not")
        self.assertEqual(b.refused, 1)

    def test_every_published_rate_converts_to_nano_without_loss(self):
        rates = [v for t in (pricing.PEAK, pricing.OFFPEAK)
                 for row in t.values() for v in row]
        for r in rates + [0.002, 0.004, 0.02, 0.25, 0.30, 0.60, 1.50]:
            self.assertEqual(fanout._nano(r), round(r * 1_000_000_000),
                             f"{r} does not survive the nano conversion")


class TestTheReconciliationIntervalDoesNotDependOnFloatLuck(unittest.TestCase):
    """reconstruct.py decides `lo <= derived < hi` for lo/hi = observed +/- 0.005.

    That is a threshold comparison on float-accumulated money, so it is in the
    dangerous class. The residual error is bounded far below the interval, and
    this pins that: the float comparison must agree with exact integer
    nano-arithmetic across the whole range of balances the campaign saw.
    """

    @staticmethod
    def _the_float_version_that_was_wrong(observed, derived):
        """Exactly what reconstruct.py used to compute. Kept as the falsifier."""
        lo, hi = observed - 0.005, observed + 0.005
        return lo <= derived < hi

    def test_the_interval_includes_its_own_inclusive_lower_bound(self):
        """The defect, at the value that exposed it.

        0.05 - 0.005 is 0.045000000000000005 in binary floating point, which is
        ABOVE the bound it represents. A derived total of exactly $0.045 was
        reported outside an interval it is the endpoint of.
        """
        inside, lo, _hi = reconstruct.within_resolution(0.05, 0.045)
        self.assertTrue(inside, "the inclusive lower bound must be inside")
        self.assertEqual(lo, 0.045)
        self.assertFalse(self._the_float_version_that_was_wrong(0.05, 0.045),
                         "the old float form must still demonstrate the defect, "
                         "or this test is no longer testing anything")

    def test_it_agrees_with_integer_nano_arithmetic_at_every_cent(self):
        for cents in range(0, 200):
            observed = cents / 100
            obs_n = fanout._nano(observed)
            for offset in (-5_000_001, -5_000_000, -1, 0, 1,
                           4_999_999, 5_000_000, 5_000_001):
                derived = (obs_n + offset) / 1e9
                expected = -5_000_000 <= offset < 5_000_000
                self.assertEqual(
                    reconstruct.within_resolution(observed, derived)[0],
                    expected, f"interval wrong at ${observed} {offset}n")

    def test_the_upper_bound_stays_exclusive(self):
        """The bound is half-open. A guard that widened it to closed would pass
        the test above and quietly accept a cent more than the instrument can
        distinguish."""
        self.assertFalse(reconstruct.within_resolution(0.05, 0.055)[0])
        self.assertTrue(reconstruct.within_resolution(0.05, 0.0549999)[0])

    def test_the_campaign_figures_are_classified_correctly(self):
        # reconcile.json: 46.00 -> 45.96 observed, 0.039091104 derived at the
        # cache-hit rate, 1.13994 at the miss rate. The balance subtraction
        # alone produces 0.03999999999999915, which is the kind of value this
        # function has to survive.
        observed = 46.00 - 45.96
        self.assertNotEqual(observed, 0.04, "the subtraction really does drift")
        self.assertTrue(reconstruct.within_resolution(observed, 0.039091104)[0],
                        "DS-F5: billing at the cache-hit rate reconciles")
        self.assertFalse(reconstruct.within_resolution(observed, 1.13994)[0],
                         "DS-F5: billing at the miss rate does not")


class TestAZeroCeilingDoesNotCrashTheReport(unittest.TestCase):
    """Meter.record() guarded this division. Meter.summary() did not.

    A run built with a zero ceiling refuses every call, which is precisely the
    run whose summary someone needs to read, and asking for it raised
    ZeroDivisionError instead.
    """

    def test_summary_survives_a_zero_ceiling(self):
        m = meter_mod.Meter(0.0, label="refused everything",
                            stream=io.StringIO(), quiet=True)
        s = m.summary()
        self.assertEqual(s["calls"], 0)
        self.assertEqual(s["derived_usd"], 0.0)

    def test_a_nonzero_ceiling_still_reports_its_percentage(self):
        """The positive control: the guard must not blank out the real figure."""
        out = io.StringIO()
        m = meter_mod.Meter(0.10, label="x", stream=out, quiet=True)
        m.record({"fresh_in": 1_000_000, "cache_read": 0, "out": 0},
                 "deepseek-flash",
                 )
        m.summary()
        self.assertIn("% of $0.10 ceiling", out.getvalue())
        self.assertNotIn("(0.0% of", out.getvalue(),
                         "a real spend must not print as 0.0% of the ceiling")


if __name__ == "__main__":
    unittest.main(verbosity=2)
