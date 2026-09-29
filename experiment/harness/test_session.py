"""Fixture tests for the experiment runner. No credential, no network, no spend.

The point of this module is that the levers cannot be bypassed by forgetting
them, so these tests are mostly about what it REFUSES to do.

Run: python3 experiment/harness/test_session.py
"""
import io
import os
import sys
import threading
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import adapter
import policy
import session
from fanout import Outcome
from policy import TaskClass


class FakeAdapter:
    """Records the kwargs and prompt of every call so policy can be asserted."""
    name = "fake"

    def __init__(self, reasoning=0, status=200, raise_with=None, cached=0):
        self.calls = []
        self.reasoning, self.status = reasoning, status
        self.raise_with, self.cached = raise_with, cached
        self._lock = threading.Lock()

    def call(self, model, prompt, max_tokens, tmp, **kw):
        with self._lock:
            self.calls.append({"prompt": prompt, "kw": kw, "max_tokens": max_tokens})
        if self.raise_with:
            raise self.raise_with
        m = adapter.Measurement(status=self.status, model_returned=model,
                                fresh_in=100, cache_read=self.cached, out=10,
                                reasoning=self.reasoning, wall_ms=5)
        return m, "answer 42", "/dev/null"


def run(ad, ceiling=1.0, workers=4, model="deepseek-flash"):
    return session.Run(curlrc=None, tmp="/tmp", ceiling_usd=ceiling,
                       label="test", model=model, workers=workers, ad=ad,
                       quiet=True)


class TestPolicyCannotBeBypassed(unittest.TestCase):

    def test_task_class_is_required(self):
        """No default. A default would pick the reasoning setting silently, and
        the wrong pick scores 7/24 while looking like a saving."""
        r = run(FakeAdapter())
        with self.assertRaises(TypeError):
            r.ask(variable="q")

    def test_a_lookup_disables_reasoning_without_the_caller_asking(self):
        ad = FakeAdapter()
        run(ad).ask(TaskClass.LOOKUP, "q", shared="S" * 2000)
        self.assertTrue(policy.disables_reasoning(ad.calls[0]["kw"]))

    def test_an_aggregation_keeps_reasoning_even_if_the_caller_forgets(self):
        ad = FakeAdapter()
        run(ad).ask(TaskClass.AGGREGATE, "q", shared="S" * 2000)
        self.assertFalse(policy.disables_reasoning(ad.calls[0]["kw"]))

    def test_the_shared_block_always_leads(self):
        ad = FakeAdapter()
        run(ad).ask(TaskClass.LOOKUP, "THE-QUESTION", shared="THE-DOCUMENT")
        p = ad.calls[0]["prompt"]
        self.assertTrue(p.startswith("THE-DOCUMENT"),
                        "the variable part led; nothing would cache")
        self.assertTrue(p.rstrip().endswith("THE-QUESTION"))

    def test_an_unstable_prefix_is_refused_before_any_spend(self):
        ad = FakeAdapter()
        r = run(ad)
        with self.assertRaises(policy.PrefixInstability):
            r.ask(TaskClass.LOOKUP, "q", shared=" leading space")
        self.assertEqual(ad.calls, [], "nothing may reach the provider")
        self.assertEqual(r.budget.snapshot()["settled_usd"], 0.0)

    def test_workers_above_the_published_limit_fail_at_construction(self):
        """Not discovered as HTTP 429 mid-run, after paying for the failures."""
        with self.assertRaises(ValueError):
            run(FakeAdapter(), workers=3000, model="deepseek-flash")
        with self.assertRaises(ValueError):
            run(FakeAdapter(), workers=600, model="deepseek-v4-pro")
        run(FakeAdapter(), workers=2500, model="deepseek-flash")  # exactly at cap


class TestTheUndocumentedParameterGuardIsWired(unittest.TestCase):

    def test_a_lookup_that_reasoned_anyway_stops_the_run(self):
        """The silent-regression case. The provider dropped the undocumented
        parameter, reasoning resumed, and nothing else would have noticed."""
        ad = FakeAdapter(reasoning=800)
        with self.assertRaises(policy.ReasoningRegression):
            run(ad).ask(TaskClass.LOOKUP, "q", shared="S" * 2000)

    def test_reasoning_tokens_on_an_aggregation_are_correct_not_a_regression(self):
        ad = FakeAdapter(reasoning=800)
        row = run(ad).ask(TaskClass.AGGREGATE, "q", shared="S" * 2000)
        self.assertEqual(row["outcome"], Outcome.ANSWERED)


class TestSpendControl(unittest.TestCase):

    def test_the_ceiling_refuses_rather_than_overspending(self):
        r = run(FakeAdapter(), ceiling=0.0)
        row = r.ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(row["outcome"], Outcome.REFUSED_BUDGET)
        self.assertEqual(r.budget.snapshot()["refused"], 1)

    def test_an_http_error_releases_its_reservation(self):
        r = run(FakeAdapter(status=503))
        row = r.ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(row["outcome"], Outcome.ERROR)
        self.assertEqual(r.budget.snapshot()["outstanding_usd"], 0.0,
                         "a request the provider rejected must not hold budget")

    def test_a_truncated_answer_settles_because_it_was_billed(self):
        m = adapter.Measurement(status=200, out=64, fresh_in=10, cache_read=0,
                                finish_reason="length")
        r = run(FakeAdapter(raise_with=adapter.Truncated(m, "/dev/null")))
        row = r.ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(row["outcome"], Outcome.TRUNCATED)
        self.assertIsNone(row["passed"], "a truncated answer decides nothing")
        self.assertGreater(r.budget.snapshot()["settled_usd"], 0.0)


class TestFanOut(unittest.TestCase):

    def test_a_long_shared_block_warms_before_fanning(self):
        ad = FakeAdapter()
        r = run(ad)
        rows = r.fan(TaskClass.LOOKUP, "S" * 9000, [f"q{i}" for i in range(5)])
        self.assertEqual(len(rows), 5)
        self.assertEqual(rows[0]["note"], "warm")
        self.assertEqual([x["note"] for x in rows[1:]], ["fan"] * 4)

    def test_a_short_shared_block_does_not_warm(self):
        ad = FakeAdapter()
        rows = run(ad).fan(TaskClass.LOOKUP, "tiny", [f"q{i}" for i in range(4)])
        self.assertTrue(all(x["note"] == "fan" for x in rows),
                        "warming below the 256-token floor buys nothing and "
                        "costs one serial round trip")

    def test_every_question_gets_a_row_even_when_the_ceiling_is_hit(self):
        r = run(FakeAdapter(), ceiling=0.0)
        rows = r.fan(TaskClass.LOOKUP, "S" * 9000, [f"q{i}" for i in range(6)])
        self.assertEqual(len(rows), 6, "a missing row is indistinguishable from "
                                       "a question nobody asked")
        self.assertTrue(all(x["outcome"] == Outcome.REFUSED_BUDGET for x in rows))

    def test_mismatched_checks_are_refused(self):
        r = run(FakeAdapter())
        with self.assertRaises(ValueError):
            r.fan(TaskClass.LOOKUP, "S" * 9000, ["a", "b"], checks=[None])


class TestScoring(unittest.TestCase):

    def test_undecided_outcomes_stay_out_of_the_denominator(self):
        r = run(FakeAdapter())
        r.ask(TaskClass.LOOKUP, "q1", "S" * 2000, check=lambda t: True)
        r.ask(TaskClass.LOOKUP, "q2", "S" * 2000, check=lambda t: False)
        r2 = run(FakeAdapter(status=500))
        r2.ask(TaskClass.LOOKUP, "q3", "S" * 2000, check=lambda t: True)
        rate, p, n = r.pass_rate()
        self.assertEqual((p, n), (1, 2))
        self.assertAlmostEqual(rate, 0.5)
        self.assertEqual(r2.pass_rate(), (None, 0, 0),
                         "an errored call decides nothing, so there is no rate")

    def test_a_checker_that_raises_leaves_the_answer_undecided(self):
        def boom(_t):
            raise ValueError("bad regex")
        r = run(FakeAdapter())
        row = r.ask(TaskClass.LOOKUP, "q", "S" * 2000, check=boom)
        self.assertEqual(row["outcome"], Outcome.ANSWERED,
                         "the call succeeded and was paid for")
        self.assertIsNone(row["passed"])
        self.assertIn("check raised", row["error"])
        self.assertEqual(r.pass_rate()[2], 0)


class TestNoScriptBypassesTheRunner(unittest.TestCase):
    """The architecture, enforced against the source.

    policy.py existed for a turn imported by nothing that talks to the provider,
    because six scripts each built their own adapter and called it directly. A
    comment saying "use session.Run" would not have prevented that. This does:
    a new spending script either goes through the runner, or has to declare
    itself frozen in its own docstring, which it cannot do honestly.
    """

    ALLOWED = {"session.py", "adapter.py"}

    def test_only_the_runner_constructs_an_adapter(self):
        here = os.path.dirname(os.path.abspath(__file__))
        offenders, checked = [], 0
        for name in sorted(os.listdir(here)):
            if not name.endswith(".py") or name.startswith("test_"):
                continue
            checked += 1
            with open(os.path.join(here, name)) as fh:
                src = fh.read()
            if name in self.ALLOWED or "FROZEN 2026" in src[:2000]:
                continue
            for i, line in enumerate(src.splitlines(), 1):
                stripped = line.strip()
                if stripped.startswith("#"):
                    continue
                if "adapter.DeepSeek" in stripped:
                    offenders.append(f"{name}:{i}: {stripped[:70]}")
        self.assertEqual(offenders, [],
                         "these build a provider adapter directly instead of "
                         "going through session.Run, so the levers, the budget "
                         "and the undocumented-parameter guard do not apply to "
                         "them:\n  " + "\n  ".join(offenders))
        self.assertGreater(checked, 5, "the scan found almost nothing; a guard "
                                       "that inspects no files proves nothing")

    def test_the_superseded_runner_cannot_be_used(self):
        import runner
        with self.assertRaises(NotImplementedError):
            runner.run_arm()
        with self.assertRaises(NotImplementedError):
            runner.summarise()


class TestFanPropagatesExtraKwargs(unittest.TestCase):
    """Every call in a fan-out must carry the caller's request parameters.

    `ask` propagated them and `fan` silently dropped them. A caller could then
    only vary a request parameter per arm by lying about the task class, which
    changes the reasoning setting as a side effect and makes the arm measure two
    things at once.
    """

    def test_every_call_warm_and_fanned_receives_them(self):
        ad = FakeAdapter()
        r = run(ad)
        r.fan(TaskClass.AGGREGATE, "S" * 9000, [f"q{i}" for i in range(4)],
              extra_kwargs={"top_p": 0.25})
        self.assertEqual(len(ad.calls), 4)
        for i, c in enumerate(ad.calls):
            self.assertEqual(c["kw"].get("top_p"), 0.25,
                             f"call {i} lost the caller's request parameter")

    def test_they_do_not_override_the_task_class_reasoning_setting(self):
        """The policy decision stays with the task class; extra_kwargs adds to
        it rather than replacing it."""
        ad = FakeAdapter()
        run(ad).fan(TaskClass.LOOKUP, "S" * 9000, ["q0", "q1"],
                    extra_kwargs={"top_p": 0.25})
        for c in ad.calls:
            self.assertTrue(policy.disables_reasoning(c["kw"]))
            self.assertEqual(c["kw"].get("top_p"), 0.25)


class TestMalformedConfiguration(unittest.TestCase):
    """A misconfigured run must fail at construction, not mid-flight.

    A run that discovers its own misconfiguration after dispatching has already
    spent money on the requests that failed.
    """

    def test_an_unknown_dialect_is_refused(self):
        with self.assertRaises(KeyError):
            session.Run(curlrc=None, tmp="/tmp", ceiling_usd=1.0, label="t",
                        dialect="not-a-dialect", quiet=True)

    def test_an_unknown_model_gets_no_concurrency_cap_and_says_so(self):
        """An unpriced, unknown model has no published limit to check against.

        It must not silently inherit another model's cap. The run is allowed,
        because refusing every unknown model would block legitimate probing,
        but nothing pretends a bound was verified.
        """
        r = session.Run(curlrc=None, tmp="/tmp", ceiling_usd=1.0, label="t",
                        model="deepseek-does-not-exist", workers=9999,
                        ad=FakeAdapter(), quiet=True)
        self.assertIsNone(policy.PUBLISHED_CONCURRENCY.get("deepseek-does-not-exist"))
        self.assertEqual(r.workers, 9999)

    def test_an_unpriced_model_yields_no_derived_cost(self):
        """pricing.cost_usd returns None for a model it has no row for, and the
        meter must not turn that into a zero. A zero would read as a free call."""
        import pricing
        self.assertIsNone(pricing.cost_usd("deepseek-does-not-exist", 100, 0, 10))


class TestMissingTelemetry(unittest.TestCase):
    """Absent provider fields are NOT_OBSERVED, never zero and never false."""

    def test_a_response_without_reasoning_does_not_trip_the_invariant(self):
        """`reasoning` is absent on the Anthropic dialect. Treating absence as
        a violation would fail every such call; treating it as zero would claim
        the parameter was honoured when nothing was reported."""
        m = adapter.Measurement(status=200, fresh_in=10, out=5)
        self.assertIsNone(m.get("reasoning"))
        policy.check_reasoning_honoured(m, {"reasoning_effort": "none"})

    def test_absent_identity_fields_stay_absent_in_the_row(self):
        ad = FakeAdapter()
        ad.call = lambda *a, **k: (adapter.Measurement(status=200, fresh_in=1, out=1),
                                   "x", "/dev/null")
        row = run(ad).ask(TaskClass.AGGREGATE, "q")
        for f in ("model_returned", "cache_read", "reasoning"):
            self.assertIsNone(row[f], f"{f} was invented where none was reported")

    def test_a_missing_usage_block_is_unaccounted_and_holds_its_reservation(self):
        """A 200 reporting no usage is a billed call that cannot be priced.

        It must NOT bank as a free ANSWERED call, and its reservation must be
        HELD rather than released: the provider answered, so the money is gone.
        Holding can under-run the ceiling; releasing can overspend it. For
        scarce capital under-running is the safe error.
        """
        ad = FakeAdapter()
        ad.call = lambda *a, **k: (adapter.Measurement(status=200), "x", "/dev/null")
        r = run(ad)
        row = r.ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(row["outcome"], Outcome.ERROR)
        self.assertIn("UNACCOUNTED", row["error"])
        self.assertIsNone(row["fresh_in"], "no usage may be invented")
        snap = r.budget.snapshot()
        self.assertGreater(snap["outstanding_usd"], 0.0,
                           "the reservation must be held, not released")
        self.assertEqual(snap["settled_usd"], 0.0,
                         "an unpriceable call must not settle as $0.00")


class TestCacheRegimeIsRecordedNotInferred(unittest.TestCase):
    """Dispatch and observed cache state are two facts, kept apart.

    The campaign measured the block relation entirely under sequential
    dispatch, then saw a count that did not fit it under concurrency. A run
    that collapses the two cannot tell them apart afterwards, so the runtime
    records both and never derives one from the other.
    """

    def test_a_warm_then_fan_run_labels_its_dispatch(self):
        ad = FakeAdapter(cached=500)
        r = run(ad)
        rows = r.fan(TaskClass.AGGREGATE, "S" * 9000, [f"q{i}" for i in range(4)])
        self.assertEqual(rows[0]["dispatch"], session.DISPATCH_WARM)
        for x in rows[1:]:
            self.assertEqual(x["dispatch"], session.DISPATCH_CONCURRENT_FAN)
        s = r.summary()
        self.assertEqual(s["dispatch"][session.DISPATCH_WARM], 1)
        self.assertEqual(s["dispatch"][session.DISPATCH_CONCURRENT_FAN], 3)

    def test_a_lone_call_is_sequential_by_default(self):
        row = run(FakeAdapter()).ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(row["dispatch"], session.DISPATCH_SEQUENTIAL)

    def test_observed_cache_state_comes_from_the_provider(self):
        cold = run(FakeAdapter(cached=0)).ask(TaskClass.AGGREGATE, "q")
        warm = run(FakeAdapter(cached=4096)).ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(cold["cache_state"], session.CACHE_COLD)
        self.assertEqual(warm["cache_state"], session.CACHE_WARM)

    def test_a_missing_usage_block_is_unknown_not_cold(self):
        """The load-bearing distinction. Calling an unmeasured call 'cold'
        asserts a measurement nobody made, and would let a run report a cache
        regime it never observed."""
        self.assertEqual(session.observed_cache_state(adapter.Measurement(status=200)),
                         session.CACHE_UNKNOWN)
        self.assertEqual(session.observed_cache_state({}), session.CACHE_UNKNOWN)
        self.assertNotEqual(session.observed_cache_state(adapter.Measurement()),
                            session.CACHE_COLD)

    def test_the_summary_reports_regimes_separately(self):
        """Never one averaged cache figure across regimes."""
        r = run(FakeAdapter(cached=500))
        r.fan(TaskClass.AGGREGATE, "S" * 9000, ["a", "b"])
        s = r.summary()
        self.assertIn("dispatch", s)
        self.assertIn("cache_state", s)
        self.assertIsInstance(s["dispatch"], dict)


if __name__ == "__main__":
    unittest.main(verbosity=2)
