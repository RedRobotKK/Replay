"""Fixture tests for the optimisation policy. No credential, no network, no spend.

Each test names the measurement its rule came from, so a future change that
makes one fail has to argue with evidence rather than with an opinion.

Run: python3 experiment/harness/test_policy.py
"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import adapter
import policy
import reconstruct
from policy import TaskClass


class TestReasoningIsClassConditional(unittest.TestCase):
    """reasoning-scope-2026-09-29.md: 18/18 vs 17/18 on lookup, 24/24 vs 7/24
    on aggregation."""

    def test_lookup_disables_reasoning(self):
        self.assertTrue(policy.disables_reasoning(policy.config_for(TaskClass.LOOKUP)))

    def test_aggregate_keeps_reasoning(self):
        self.assertFalse(policy.disables_reasoning(policy.config_for(TaskClass.AGGREGATE)))

    def test_unclassified_fails_toward_the_expensive_correct_setting(self):
        """The load-bearing default.

        Reasoning-off answered 39 as 67 and 14 as 1, with nothing marking the
        answer wrong. An unclassified task defaulting to off would buy silence,
        not savings.
        """
        self.assertFalse(policy.disables_reasoning(policy.config_for(TaskClass.UNKNOWN)),
                         "an unclassified task must not silently disable reasoning")
        self.assertFalse(policy.disables_reasoning(policy.config_for("typo-not-a-class")),
                         "an unrecognised class must fall through to reasoning ON")


class TestTheUndocumentedParameterIsGuarded(unittest.TestCase):
    """docs-reconciliation-2026-09-29.md: the vendor documents only the ENABLING
    direction. The disabling shapes work and are not promised."""

    def test_a_response_that_reasoned_anyway_is_a_stop(self):
        m = adapter.Measurement(reasoning=412, out=500)
        with self.assertRaises(policy.ReasoningRegression):
            policy.check_reasoning_honoured(m, {"reasoning_effort": "none"})

    def test_the_guard_is_silent_when_the_parameter_is_honoured(self):
        m = adapter.Measurement(reasoning=0, out=12)
        policy.check_reasoning_honoured(m, {"reasoning_effort": "none"})
        policy.check_reasoning_honoured(adapter.Measurement(out=12),
                                        {"thinking": {"type": "disabled"}})

    def test_the_guard_does_not_fire_when_reasoning_was_requested(self):
        """Reasoning tokens on a request that asked for reasoning are correct,
        not a regression."""
        m = adapter.Measurement(reasoning=9000, out=9100)
        policy.check_reasoning_honoured(m, {"reasoning_effort": "high"})
        policy.check_reasoning_honoured(m, {"thinking": {"type": "enabled"}})
        policy.check_reasoning_honoured(m, {})


class TestPromptAssembly(unittest.TestCase):
    """optimization-arms: ordering alone is 4.6x at no latency cost.
    cache-characterisation: one leading space drops the hit rate to zero."""

    def test_the_variable_part_goes_last(self):
        got = policy.assemble("SHARED DOCUMENT", "the question")
        self.assertTrue(got.startswith("SHARED DOCUMENT"),
                        "the shared block must lead, or nothing caches")
        self.assertTrue(got.rstrip().endswith("the question"))

    def test_a_prefix_with_leading_whitespace_is_refused(self):
        """Matching is byte-exact. A prefix whose first byte varies caches
        nothing, and the cost of that is invisible at the call site."""
        for bad in (" doc", "\ndoc", "\tdoc"):
            with self.assertRaises(policy.PrefixInstability):
                policy.assemble(bad, "q")

    def test_no_shared_block_returns_the_variable_part_unchanged(self):
        self.assertEqual(policy.assemble("", "q"), "q")


class TestWarmDecision(unittest.TestCase):
    """cache-characterisation: below 256 tokens nothing caches at all."""

    def test_a_substantial_shared_prefix_warms(self):
        ps = ["X" * 5000 + f" q{i}" for i in range(4)]
        warm, why = policy.should_warm(ps)
        self.assertTrue(warm, why)

    def test_a_short_shared_prefix_does_not(self):
        ps = ["tiny " + f"q{i}" for i in range(4)]
        warm, why = policy.should_warm(ps)
        self.assertFalse(warm)
        self.assertIn("below", why)

    def test_one_prompt_never_warms(self):
        self.assertFalse(policy.should_warm(["X" * 9000])[0])

    def test_the_threshold_is_two_blocks_not_a_guess(self):
        self.assertEqual(policy.BLOCK_TOKENS, 128)
        self.assertEqual(policy.MIN_WARM_PREFIX_TOKENS, 256,
                         "the final block is never served, so one block caches "
                         "nothing and the floor is two")


class TestReview(unittest.TestCase):

    def test_it_catches_reasoning_off_on_an_aggregative_task(self):
        f = policy.review({"task_class": TaskClass.AGGREGATE,
                           "call_kwargs": {"reasoning_effort": "none"},
                           "prompts": ["a", "b"]})
        self.assertTrue(any("7/24" in x for x in f), f)

    def test_it_catches_a_cold_fan_out(self):
        f = policy.review({"task_class": TaskClass.LOOKUP,
                           "call_kwargs": {"reasoning_effort": "none"},
                           "prompts": ["X" * 9000 + f" q{i}" for i in range(6)],
                           "warm": False})
        self.assertTrue(any("warm one call" in x for x in f), f)

    def test_it_catches_workers_above_the_published_limit(self):
        f = policy.review({"task_class": TaskClass.AGGREGATE, "prompts": ["a"],
                           "model": "deepseek-v4-pro", "workers": 900})
        self.assertTrue(any("429" in x for x in f), f)

    def test_a_correct_plan_produces_no_findings(self):
        """A reviewer that flags everything is as useless as one that flags
        nothing; the clean case must actually come back clean."""
        f = policy.review({"task_class": TaskClass.LOOKUP,
                           "call_kwargs": {"reasoning_effort": "none"},
                           "prompts": ["X" * 9000 + f" q{i}" for i in range(6)],
                           "warm": True, "workers": 32, "model": "deepseek-flash"})
        self.assertEqual(f, [], f)


class TestPublishedFactsAreNotMeasurements(unittest.TestCase):

    def test_concurrency_limits_match_the_vendor_docs(self):
        """DS-CONC tested 64 and reported no ceiling. These are the published
        numbers it was nowhere near."""
        self.assertEqual(policy.PUBLISHED_CONCURRENCY["deepseek-flash"], 2500)
        self.assertEqual(policy.PUBLISHED_CONCURRENCY["deepseek-v4-pro"], 500)


class TestReconciliationInterval(unittest.TestCase):
    """The band a balance DIFFERENCE can distinguish.

    Two cent-resolution readings carry two independent errors, so the interval
    is 4q wide. An earlier version used 2q and flattered every reconciliation.
    """

    def test_the_interval_is_four_half_resolutions_wide(self):
        import reconstruct
        _, lo, hi = reconstruct.within_resolution(0.23, 0.23)
        self.assertAlmostEqual(hi - lo, 0.02, places=9,
                               msg="a difference of two cent-resolution readings "
                                   "spans $0.02, not $0.01")
        self.assertAlmostEqual(lo, 0.22, places=9)
        self.assertAlmostEqual(hi, 0.24, places=9)

    def test_the_campaign_figure_reconciles_inside_the_correct_band(self):
        import reconstruct
        inside, lo, hi = reconstruct.within_resolution(0.23, 0.229946832)
        self.assertTrue(inside, f"$0.229946832 must lie in ({lo}, {hi})")

    def test_the_interval_is_open_at_both_ends(self):
        """Open, so the result does not depend on whether the provider rounds
        or truncates, which is NOT_OBSERVED."""
        import reconstruct
        inside_lo, lo, hi = reconstruct.within_resolution(0.23, 0.22)
        inside_hi, _, _ = reconstruct.within_resolution(0.23, 0.24)
        self.assertFalse(inside_lo, "the lower bound must be excluded")
        self.assertFalse(inside_hi, "the upper bound must be excluded")


if __name__ == "__main__":
    unittest.main(verbosity=2)