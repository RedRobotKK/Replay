"""Parallel bounded inspection: many small checked questions, one budget.

This is the shape the measurements actually reward. DeepSeek sustained 64
concurrent requests with no throttling and median latency that FELL from 662 ms
to 575 ms, and a cached read costs 50x less than a miss on flash. Both favour
many small questions over one long session.

The two measured failure modes point the same way. It did not self-terminate
when told to stop, and reasoning starved the output budget three times. Neither
can happen here: every question is a single call with its own checker, so there
is no loop to run away and no synthesis turn to starve.

Every question carries a checker. A fan-out without one produces opinions at
scale, which is worse than producing none.
"""
import concurrent.futures as cf
import datetime as dt

import adapter
import pricing


class Question:
    """One bounded question and the check that decides whether it was answered.

    check(text) -> True | False | None. None means the checker itself could not
    decide, which is NOT_MEASURED and must never be folded into a pass rate.
    """

    def __init__(self, qid, prompt, check, max_tokens=400):
        self.qid, self.prompt, self.check, self.max_tokens = qid, prompt, check, max_tokens


class Budget:
    """A ceiling enforced before dispatch, not discovered afterwards.

    Spend is committed the moment a request is sent, so a ceiling checked after
    the fact is a report rather than a control. Workers reserve an estimate
    first; the estimate is reconciled against the reported usage on return.
    """

    def __init__(self, ceiling_usd, est_per_call_usd):
        self.ceiling, self.est = ceiling_usd, est_per_call_usd
        self.committed = 0.0
        self.actual = 0.0
        self.refused = 0

    def reserve(self):
        if self.committed + self.est > self.ceiling:
            self.refused += 1
            return False
        self.committed += self.est
        return True

    def settle(self, usd):
        self.actual += usd or 0.0


def fan_out(ad, model, questions, ceiling_usd, tmp, workers=16,
            est_per_call_usd=0.002, **call_kw):
    """Ask every question in parallel, under one budget, each answer checked.

    Returns one row per question. A question refused by the budget is recorded
    as refused rather than dropped, because a silently shortened fan-out looks
    identical to one where the model declined to answer.
    """
    budget = Budget(ceiling_usd, est_per_call_usd)

    def ask(q):
        base = dict(qid=q.qid, when=dt.datetime.now(dt.timezone.utc).isoformat())
        if not budget.reserve():
            return dict(base, status=None, outcome="REFUSED_BUDGET", passed=None)
        try:
            m, text, raw = ad.call(model, q.prompt, q.max_tokens, tmp, **call_kw)
        except adapter.Truncated as t:
            # The O1 guard. A truncated answer is not a short answer, and it is
            # not folded into the pass rate.
            return dict(base, outcome="TRUNCATED", passed=None,
                        raw=t.raw_path, **t.measurement)
        usd = pricing.cost_usd(model, m["fresh_in"], m["cache_read"], m["out"])
        budget.settle(usd)
        try:
            passed = q.check(text)
        except Exception:
            passed = None          # a checker that raised decided nothing
        return dict(base, outcome="ANSWERED", passed=passed,
                    usd_derived=usd, raw=raw, **m)

    with cf.ThreadPoolExecutor(max_workers=workers) as ex:
        rows = list(ex.map(ask, questions))
    return rows, budget


def summarise(rows, budget):
    """Counts, never a score. Each outcome stays its own fact."""
    answered = [r for r in rows if r.get("outcome") == "ANSWERED"]
    decided = [r for r in answered if r.get("passed") is not None]
    return {
        "asked": len(rows),
        "answered": len(answered),
        "truncated": sum(1 for r in rows if r.get("outcome") == "TRUNCATED"),
        "refused_budget": sum(1 for r in rows if r.get("outcome") == "REFUSED_BUDGET"),
        "http_error": sum(1 for r in answered if r.get("status") not in (200, None)),
        # Denominator is decided answers only. A truncated or refused question
        # says nothing about correctness and must not dilute the rate.
        "pass_rate": (f"{sum(1 for r in decided if r['passed'])}/{len(decided)}"
                      if decided else "NOT_MEASURED"),
        "undecided": len(answered) - len(decided),
        "derived_usd": round(budget.actual, 6),
        "ceiling_usd": budget.ceiling,
        "models_returned": sorted({r.get("model_returned") for r in answered
                                   if r.get("model_returned")}),
        "fingerprints": sorted({r.get("system_fingerprint") for r in answered
                                if r.get("system_fingerprint")}),
    }
