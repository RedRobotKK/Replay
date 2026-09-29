"""Fixture tests for the fan-out. No credential, no network, no spend.

Run: python3 experiment/harness/test_fanout.py
"""
import os
import sys
import threading
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import adapter
import fanout
from fanout import Budget, Outcome


class Q:
    def __init__(self, qid, prompt, check=lambda t: True, max_tokens=64):
        self.id, self.prompt, self.check, self.max_tokens = qid, prompt, check, max_tokens


class FakeAdapter:
    """Records when each call starts and finishes, so ordering is testable."""

    def __init__(self, behaviour=None, delay=0.0):
        self.behaviour = behaviour or {}
        self.delay = delay
        self.events = []
        self._lock = threading.Lock()

    def call(self, model, prompt, max_tokens, tmp, **kw):
        with self._lock:
            self.events.append(("start", prompt[:40], time.monotonic()))
        time.sleep(self.delay)
        with self._lock:
            self.events.append(("end", prompt[:40], time.monotonic()))
        b = self.behaviour.get(prompt)
        if isinstance(b, BaseException):
            raise b
        m = adapter.Measurement(status=200, model_returned="fake", cache_read=0,
                                cache_write=100, fresh_in=10, out=5, wall_ms=1)
        return m, b if isinstance(b, str) else "ok", "/dev/null"


LONG = "X" * 3000


class TestBudgetIsEnforcedAcrossThreads(unittest.TestCase):
    """The ceiling is the money guard and it is exercised under contention.

    The pre-rewrite reserve() was an unlocked read-modify-write. Removing the
    lock from the rewritten Budget makes this test fail, which is the only
    reason to trust it.
    """

    def test_no_more_reservations_than_the_ceiling_allows(self):
        """Run against a probe that forces the race to occur every time.

        A plain contention loop is not evidence here: removing the lock leaves
        the test passing, because CPython's GIL makes the unlocked window
        between the read and the write too small to hit reliably. The probe
        subclasses Budget and puts a preemption point inside that window --
        timing only, never logic, since reserve() is inherited unchanged.

        Locked, threads queue and exactly 10 of 40 are granted. Unlocked, all
        40 read the same committed figure during the sleep and all 40 commit.
        """

        class Probe(fanout.Budget):
            @property
            def _committed(self):
                time.sleep(0.003)
                return self._c

            @_committed.setter
            def _committed(self, v):
                self._c = v

        b = Probe(ceiling_usd=10 * 0.002, est_per_call_usd=0.002)
        granted = []
        gate = threading.Barrier(40)
        lock = threading.Lock()

        def worker():
            gate.wait()
            if b.reserve():
                with lock:
                    granted.append(1)

        ts = [threading.Thread(target=worker) for _ in range(40)]
        for t in ts:
            t.start()
        for t in ts:
            t.join()
        self.assertEqual(len(granted), 10,
                         "the ceiling allows exactly 10 reservations of $0.002; "
                         "more means reserve() is not atomic")
        self.assertEqual(b.refused, 30)

    def test_money_is_exact_not_floating_point(self):
        """The defect this found: ten additions of 0.002 exceed a ceiling of
        0.02 in binary floating point, so the tenth call of a ten-call budget
        was refused. Wrong in the safe direction, and still wrong."""
        self.assertIsInstance(fanout._nano(0.002), int,
                              "money is held as integer nano-USD; a float "
                              "reintroduces the accumulation error")
        # Each of these ladders drifts if summed as floats.
        for est, n in [(0.002, 10), (0.0001, 7), (0.07, 3), (0.029, 11)]:
            b = Budget(ceiling_usd=est * n, est_per_call_usd=est)
            got = sum(1 for _ in range(n + 5) if b.reserve())
            self.assertEqual(got, n,
                             f"a ceiling of {n} x ${est} must admit exactly {n} "
                             f"reservations, got {got}")

    def test_settling_below_the_estimate_frees_room(self):
        b = Budget(ceiling_usd=0.010, est_per_call_usd=0.002)
        for _ in range(5):
            self.assertTrue(b.reserve())
        self.assertFalse(b.reserve())      # 5 estimates fill the ceiling
        for _ in range(5):
            b.settle(0.0001)               # they were far cheaper in truth
        self.assertTrue(b.reserve(),
                        "settling at real cost must return headroom, or a run "
                        "of cheap calls stays blocked by pessimistic estimates")


class TestWarmThenFan(unittest.TestCase):

    def test_the_warm_call_completes_before_any_fan_call_starts(self):
        """The whole point of the module. If a fan call starts before the warm
        call ends, every fan call misses the cache and the saving is gone."""
        ad = FakeAdapter(delay=0.05)
        qs = [Q(i, LONG + f" question {i}") for i in range(6)]
        rows, _, rep = fanout.fan_out(ad, "m", qs, 1.0, "/tmp", workers=6)
        self.assertTrue(rep["warmed"])
        warm_end = next(t for k, _, t in ad.events if k == "end")
        fan_starts = [t for k, p, t in ad.events
                      if k == "start" and not p.startswith(("X" * 40)[:40]) or True][1:]
        first_fan_start = sorted(t for k, _, t in ad.events if k == "start")[1]
        self.assertGreaterEqual(first_fan_start, warm_end,
                                "a fan call started before the warm call "
                                "completed; the prefix was not yet cached")
        self.assertEqual(len(rows), 6)

    def test_warming_is_skipped_when_nothing_is_shared(self):
        """Warming a non-shared prefix buys nothing and costs a serial round
        trip, so it must not happen by default."""
        ad = FakeAdapter()
        qs = [Q(i, f"unrelated question {i}") for i in range(4)]
        _, _, rep = fanout.fan_out(ad, "m", qs, 1.0, "/tmp")
        self.assertFalse(rep["warmed"])
        self.assertIn("shared prefix", rep["warm_skipped_reason"])

    def test_a_refused_warm_call_does_not_fan_out(self):
        ad = FakeAdapter()
        qs = [Q(i, LONG + f" q{i}") for i in range(5)]
        rows, budget, _ = fanout.fan_out(ad, "m", qs, 0.0, "/tmp")
        self.assertEqual(len(rows), 5)
        self.assertTrue(all(r["outcome"] == Outcome.REFUSED_BUDGET for r in rows))
        self.assertEqual(len(ad.events), 0, "no call may reach the provider")


class TestOneFailureCannotDiscardPaidResults(unittest.TestCase):

    def test_a_caught_exception_costs_one_row_not_the_run(self):
        qs = [Q(i, f"q{i}") for i in range(5)]
        ad = FakeAdapter(behaviour={"q2": RuntimeError("connection reset")})
        rows, _, _ = fanout.fan_out(ad, "m", qs, 1.0, "/tmp", workers=5)
        self.assertEqual(len(rows), 5, "every question gets a row; a missing row "
                                       "is indistinguishable from one never asked")
        t = fanout.tally(rows)
        self.assertEqual(t[Outcome.ERROR], 1)
        self.assertEqual(t[Outcome.ANSWERED], 4)
        self.assertIn("connection reset", next(r for r in rows if r["id"] == 2)["error"])

    def test_an_exception_that_escapes_ask_costs_one_row_not_the_run(self):
        """The case submit/as_completed exists for.

        ask() catches Exception, so an ordinary failure never reaches the
        executor and does not distinguish map from as_completed. A BaseException
        does: SystemExit, which a checker calling sys.exit() raises for real,
        passes straight through ask(). Under ex.map it propagates out of the
        result iterator and discards every call already paid for.
        """
        def bail(_t):
            sys.exit("checker called sys.exit")

        qs = [Q(0, "q0"), Q(1, "q1", check=bail), Q(2, "q2"), Q(3, "q3")]
        rows, budget, _ = fanout.fan_out(FakeAdapter(), "m", qs, 1.0, "/tmp",
                                         workers=4, cost_fn=lambda _m: 0.001)
        self.assertEqual(len(rows), 4)
        self.assertEqual(fanout.tally(rows)[Outcome.ANSWERED], 3,
                         "three calls succeeded and were billed; one checker "
                         "exiting must not discard them")
        bad = next(r for r in rows if r["id"] == 1)
        self.assertEqual(bad["outcome"], Outcome.ERROR)
        self.assertIn("escaped ask()", bad["error"])

    def test_a_truncated_answer_is_not_a_failure_and_is_still_billed(self):
        qs = [Q(i, f"q{i}") for i in range(3)]
        m = adapter.Measurement(status=200, out=64, fresh_in=10, cache_read=0,
                                cache_write=0, finish_reason="length")
        ad = FakeAdapter(behaviour={"q1": adapter.Truncated(m, "/dev/null")})
        rows, budget, _ = fanout.fan_out(ad, "m", qs, 1.0, "/tmp",
                                         cost_fn=lambda _m: 0.003)
        self.assertEqual(fanout.tally(rows)[Outcome.TRUNCATED], 1)
        self.assertIsNone(next(r for r in rows if r["id"] == 1)["passed"],
                          "a truncated answer decides nothing")
        self.assertAlmostEqual(budget.snapshot()["settled_usd"], 0.009,
                               msg="a truncated call reached the provider and "
                                   "was billed; it must settle, not release")


class TestScoring(unittest.TestCase):

    def test_undecided_outcomes_are_outside_the_denominator(self):
        qs = [Q(0, "q0", check=lambda t: True),
              Q(1, "q1", check=lambda t: False),
              Q(2, "q2", check=lambda t: True)]
        ad = FakeAdapter(behaviour={"q2": RuntimeError("boom")})
        rows, _, _ = fanout.fan_out(ad, "m", qs, 1.0, "/tmp")
        rate, passed, n = fanout.pass_rate(rows)
        self.assertEqual((passed, n), (1, 2),
                         "the errored question is undecided; counting it as a "
                         "failure would let a network fault read as incapacity")
        self.assertAlmostEqual(rate, 0.5)

    def test_a_checker_that_raises_decides_nothing(self):
        def explode(_t):
            raise ValueError("bad regex")
        qs = [Q(0, "q0", check=explode), Q(1, "q1", check=lambda t: True)]
        rows, _, _ = fanout.fan_out(ad := FakeAdapter(), "m", qs, 1.0, "/tmp")
        r0 = next(r for r in rows if r["id"] == 0)
        self.assertEqual(r0["outcome"], Outcome.ANSWERED,
                         "the call succeeded and was paid for; the checker "
                         "failing does not undo that")
        self.assertIsNone(r0["passed"])
        self.assertIn("check raised", r0["error"])
        _, _, n = fanout.pass_rate(rows)
        self.assertEqual(n, 1)


class TestPrefixDetection(unittest.TestCase):

    def test_common_prefix(self):
        self.assertEqual(fanout.common_prefix_len(["abcd", "abce", "abcf"]), 3)
        self.assertEqual(fanout.common_prefix_len(["abc", "xyz"]), 0)
        self.assertEqual(fanout.common_prefix_len(["abc"]), 0,
                         "one text shares a prefix with nothing")
        self.assertEqual(fanout.common_prefix_len([]), 0)
        self.assertEqual(fanout.common_prefix_len(["ab", "abcd"]), 2,
                         "the shorter text bounds the shared prefix")


if __name__ == "__main__":
    unittest.main(verbosity=2)
