"""python3 experiment/harness/test_fanout.py

Fixture-driven. No API call, no credential.

Covers the controls that make a fan-out safe to point at a live endpoint: the
budget stops dispatch rather than reporting overspend afterwards, a truncated
answer never counts as a wrong answer, and an undecided checker never counts as
either.
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import adapter  # noqa: E402
import fanout  # noqa: E402


class FakeAdapter:
    """Replays a scripted outcome per call, counting dispatches."""

    def __init__(self, behaviour):
        self.behaviour, self.calls = behaviour, 0

    def call(self, model, prompt, max_tokens, tmp, **kw):
        self.calls += 1
        b = self.behaviour(self.calls)
        if b == "truncate":
            m = adapter.Measurement(finish_reason="length", out=max_tokens,
                                    truncated=True, status=200)
            raise adapter.Truncated(m, os.path.join(tmp, "raw.json"))
        m = adapter.Measurement(fresh_in=1000, cache_read=0, out=10, status=200,
                                requested_model=model, model_returned=model,
                                system_fingerprint="fp-1", finish_reason="stop",
                                truncated=False)
        return m, b, os.path.join(tmp, "raw.json")


def q(n, check, text_for=None):
    return [fanout.Question(f"q{i}", "prompt", check) for i in range(n)]


class TestBudgetStopsDispatch(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_ceiling_refuses_before_sending(self):
        """Spend commits when the request is sent, so the check must precede it.

        A ceiling verified after the fact is a report, not a control.
        """
        ad = FakeAdapter(lambda n: "ok")
        rows, budget = fanout.fan_out(
            ad, "deepseek-flash", q(20, lambda t: True), ceiling_usd=0.005,
            tmp=self.tmp, workers=4, est_per_call_usd=0.001)
        s = fanout.summarise(rows, budget)
        self.assertEqual(ad.calls, 5, "budget allowed 5 calls at 0.001 under a 0.005 ceiling")
        self.assertEqual(s["refused_budget"], 15)
        self.assertEqual(s["asked"], 20, "refused questions are recorded, not dropped")

    def test_a_shortened_fanout_is_visible(self):
        """A silently shortened run looks the same as a run nobody answered."""
        ad = FakeAdapter(lambda n: "ok")
        rows, budget = fanout.fan_out(
            ad, "deepseek-flash", q(10, lambda t: True), ceiling_usd=0.003,
            tmp=self.tmp, workers=2, est_per_call_usd=0.001)
        s = fanout.summarise(rows, budget)
        self.assertGreater(s["refused_budget"], 0)
        self.assertEqual(s["answered"] + s["refused_budget"], s["asked"])


class TestOutcomesStaySeparate(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_truncated_is_not_a_failed_answer(self):
        """A call that ran out of room did not answer wrongly. It did not answer."""
        ad = FakeAdapter(lambda n: "truncate" if n % 2 == 0 else "ok")
        rows, budget = fanout.fan_out(
            ad, "deepseek-flash", q(10, lambda t: True), ceiling_usd=1.0,
            tmp=self.tmp, workers=1, est_per_call_usd=0.001)
        s = fanout.summarise(rows, budget)
        self.assertEqual(s["truncated"], 5)
        self.assertEqual(s["pass_rate"], "5/5",
                         "the denominator is decided answers, not questions asked")

    def test_an_undecided_checker_is_not_a_failure(self):
        """check() returning None is NOT_MEASURED and leaves the rate alone."""
        ad = FakeAdapter(lambda n: "ok")
        rows, budget = fanout.fan_out(
            ad, "deepseek-flash",
            [fanout.Question(f"q{i}", "p", (lambda t: None) if i < 6 else (lambda t: True))
             for i in range(10)],
            ceiling_usd=1.0, tmp=self.tmp, workers=1, est_per_call_usd=0.001)
        s = fanout.summarise(rows, budget)
        self.assertEqual(s["undecided"], 6)
        self.assertEqual(s["pass_rate"], "4/4")

    def test_a_raising_checker_decides_nothing(self):
        """A broken checker must not be scored as the model being wrong."""
        ad = FakeAdapter(lambda n: "ok")
        def boom(t):
            raise ValueError("checker bug")
        rows, budget = fanout.fan_out(
            ad, "deepseek-flash", q(4, boom), ceiling_usd=1.0, tmp=self.tmp,
            workers=1, est_per_call_usd=0.001)
        s = fanout.summarise(rows, budget)
        self.assertEqual(s["pass_rate"], "NOT_MEASURED")
        self.assertEqual(s["undecided"], 4)


class TestIdentityIsCarried(unittest.TestCase):
    def test_models_and_fingerprints_are_collected(self):
        """So a later routing question is answerable from the fan-out record."""
        ad = FakeAdapter(lambda n: "ok")
        rows, budget = fanout.fan_out(
            ad, "deepseek-flash", q(3, lambda t: True), ceiling_usd=1.0,
            tmp=tempfile.mkdtemp(), workers=1, est_per_call_usd=0.001)
        s = fanout.summarise(rows, budget)
        self.assertEqual(s["models_returned"], ["deepseek-flash"])
        self.assertEqual(s["fingerprints"], ["fp-1"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
