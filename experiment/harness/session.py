"""The one path a spending experiment goes through.

policy.py encoded the measured levers and, until this module existed, was
imported by nothing that talks to the provider. Six probe scripts each hand-rolled
their own budget, call wrapper and warm logic, so the levers were enforced by
discipline. Discipline is what the budget race, the cold fan-out and the
undocumented-parameter risk all already cost us once.

Everything here is enforced by construction rather than by remembering:

  - `task_class` is a REQUIRED argument. There is no default, so the reasoning
    setting is never chosen by accident. An unclassified task is a caller
    decision, spelled out at the call site.
  - The shared block always leads and the variable part always follows, via
    policy.assemble, which refuses a prefix whose leading bytes vary.
  - Every response whose request disabled reasoning is checked. The parameter is
    undocumented; the check turns a silent 20x regression into a stop.
  - Worker count is validated against the published concurrency limit at
    construction, not discovered as HTTP 429 mid-run.
  - Spend is reserved before dispatch and settled at the provider-reported cost.
"""
import concurrent.futures as cf
import datetime as dt

import adapter
import fanout
import meter as meter_mod
import policy
import pricing
from fanout import Outcome


class Result(dict):
    """One call. Always has an outcome; never silently absent."""

    @property
    def ok(self):
        return self["outcome"] == Outcome.ANSWERED


class Run:
    """A budgeted, policy-enforcing experiment run."""

    DIALECTS = {"chat": adapter.DeepSeekChat, "anthropic": adapter.DeepSeekAnthropic}

    def __init__(self, curlrc, tmp, ceiling_usd, label, model="deepseek-flash",
                 dialect="chat", est_per_call_usd=0.002, workers=32,
                 base="https://api.deepseek.com", ad=None, quiet=False):
        cap = policy.PUBLISHED_CONCURRENCY.get(model)
        if cap is not None and workers > cap:
            raise ValueError(
                f"workers={workers} exceeds the published concurrency limit of "
                f"{cap} for {model}. The excess returns HTTP 429, and a run that "
                f"discovers its own limit mid-flight has already paid for the "
                f"requests that failed.")
        self.model, self.tmp, self.workers = model, tmp, workers
        self.ad = ad if ad is not None else self.DIALECTS[dialect](curlrc, base)
        self.meter = meter_mod.Meter(ceiling_usd, label=label, quiet=quiet)
        self.budget = fanout.Budget(ceiling_usd, est_per_call_usd)
        self.rows = []

    # -- one call ---------------------------------------------------------
    def ask(self, task_class, variable, shared="", max_tokens=512, check=None,
            qid=None, note="", extra_kwargs=None):
        """Send one question. `task_class` is required and decides reasoning."""
        prompt = policy.assemble(shared, variable)
        kw = policy.config_for(task_class)
        if extra_kwargs:
            kw.update(extra_kwargs)

        if not self.budget.reserve():
            return self._row(qid, task_class, Outcome.REFUSED_BUDGET, note=note)
        try:
            m, text, raw = self.ad.call(self.model, prompt, max_tokens, self.tmp, **kw)
        except adapter.Truncated as t:
            # Reached the provider and was billed, so it settles rather than
            # releasing. It is not a deliverable.
            usd = self.meter.record(t.measurement, self.model, note or "truncated")
            self.budget.settle(usd)
            return self._row(qid, task_class, Outcome.TRUNCATED, m=t.measurement,
                             usd=usd, note=note)
        except Exception as e:
            # Unknown whether the provider billed it. Holding the reservation
            # can under-run; releasing it can overspend. Under-running is the
            # safe error for scarce capital.
            return self._row(qid, task_class, Outcome.ERROR, note=note,
                             error=f"{type(e).__name__}: {e}")

        if m.get("status") != 200:
            self.budget.release()
            return self._row(qid, task_class, Outcome.ERROR, m=m, note=note,
                             error=f"HTTP {m.get('status')}")

        # The undocumented parameter did what it did when we measured it, or
        # every cost figure downstream of here is wrong.
        policy.check_reasoning_honoured(m, kw)

        usd = self.meter.record(m, self.model, note)
        self.budget.settle(usd)
        row = self._row(qid, task_class, Outcome.ANSWERED, m=m, usd=usd,
                        note=note, raw=raw)
        if check is not None:
            # A checker that raises decides nothing. It must not turn a paid,
            # answered call into a failure or score it False.
            try:
                row["passed"] = bool(check(text))
            except Exception as e:
                row["passed"] = None
                row["error"] = f"check raised: {type(e).__name__}: {e}"
        row["text"] = text
        return row

    # -- many calls, warmed -----------------------------------------------
    def fan(self, task_class, shared, variables, max_tokens=512, checks=None,
            warm=None):
        """Ask every variable against one shared block, warming it first.

        Warming is decided by the measured 256-token floor unless the caller
        overrides it. Fanning out cold pays full input price on the shared block
        for every question and buys only latency.
        """
        variables = list(variables)
        checks = list(checks) if checks else [None] * len(variables)
        if len(checks) != len(variables):
            raise ValueError("checks must be one per variable, or omitted")
        if not variables:
            return []

        prompts = [policy.assemble(shared, v) for v in variables]
        decided, why = policy.should_warm(prompts)
        do_warm = decided if warm is None else warm
        self.meter.banner(f"fan {len(variables)} [{task_class}] "
                          f"{'warm-then-fan' if do_warm else 'parallel'}: {why}")

        out, start = [], 0
        if do_warm:
            out.append(self.ask(task_class, variables[0], shared, max_tokens,
                                checks[0], qid=0, note="warm"))
            start = 1
            if out[0]["outcome"] == Outcome.REFUSED_BUDGET:
                # The ceiling is already reached. Fanning out would produce a
                # column of refusals and say the same thing more slowly.
                out += [self._row(i, task_class, Outcome.REFUSED_BUDGET)
                        for i in range(1, len(variables))]
                return out

        if start < len(variables):
            with cf.ThreadPoolExecutor(max_workers=self.workers) as ex:
                futs = {ex.submit(self.ask, task_class, variables[i], shared,
                                  max_tokens, checks[i], i, "fan"): i
                        for i in range(start, len(variables))}
                for f in cf.as_completed(futs):
                    i = futs[f]
                    try:
                        out.append(f.result())
                    except BaseException as e:
                        # ask() should never raise. If it does, one question
                        # loses its row, not every call already paid for.
                        out.append(self._row(i, task_class, Outcome.ERROR,
                                             error=f"escaped ask(): {type(e).__name__}: {e}"))
        out.sort(key=lambda r: (r["id"] is None, r["id"]))
        return out

    # -- bookkeeping ------------------------------------------------------
    def _row(self, qid, task_class, outcome, m=None, usd=None, note="",
             error=None, raw=None):
        m = m or {}
        r = Result(id=qid, task_class=task_class, outcome=outcome, usd=usd,
                   note=note, error=error, passed=None, raw=raw,
                   model=self.model,
                   when=dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds"),
                   peak=pricing.is_peak(dt.datetime.now(dt.timezone.utc)),
                   status=m.get("status"), model_returned=m.get("model_returned"),
                   fresh_in=m.get("fresh_in"), cache_read=m.get("cache_read"),
                   cache_write=m.get("cache_write"), out=m.get("out"),
                   reasoning=m.get("reasoning"), wall_ms=m.get("wall_ms"))
        self.rows.append(r)
        return r

    def pass_rate(self):
        """Decided answers only. A refusal, a truncation, an error and a checker
        that raised are four kinds of undecided, not four kinds of wrong."""
        dec = [r for r in self.rows
               if r["outcome"] == Outcome.ANSWERED and r["passed"] is not None]
        if not dec:
            return None, 0, 0
        p = sum(1 for r in dec if r["passed"])
        return p / len(dec), p, len(dec)

    def tally(self):
        t = {Outcome.ANSWERED: 0, Outcome.TRUNCATED: 0,
             Outcome.REFUSED_BUDGET: 0, Outcome.ERROR: 0}
        for r in self.rows:
            if r["outcome"] in t:
                t[r["outcome"]] += 1
        return t

    def summary(self):
        s = self.meter.summary()
        rate, p, n = self.pass_rate()
        s.update({"tally": self.tally(), "budget": self.budget.snapshot(),
                  "pass_rate": rate, "passed": p, "decided": n})
        return s
