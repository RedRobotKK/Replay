"""Concurrent fan-out across questions, with the cache accounted for.

The naive shape -- put every question in a thread pool and start them all at
once -- is wrong for any workload whose prompts share a prefix, and sharing a
prefix is the normal case: one document, one codebase, one spec, many questions
about it.

DeepSeek's cache is populated by a completed request. Requests issued
concurrently all begin before any of them has completed, so every one of them
misses and every one of them pays full input price for the shared prefix. The
cache then holds the prefix and nothing left in the run needs it.

Measured on the published rates, for a prefix that all questions share:

    30 questions, 10,000-token prefix:  $0.04500 all-parallel, $0.00237 warmed
    64 questions, 30,000-token prefix:  $0.28800 all-parallel, $0.01017 warmed

So this module runs one question alone first, lets it complete, and only then
fans out the rest. The warm call is a real question whose answer is kept -- not
a throwaway probe -- so warming costs latency, never an extra call.

Warming is skipped when the questions share no substantial prefix, because then
it buys nothing and costs one call's latency in serial.
"""
import concurrent.futures as cf
import threading

import adapter


# MEASURED 2026-09-29, 17 rungs, 17 exact. DeepSeek's prefix cache is quantised
# to 128-token blocks and the FINAL block is never served from cache:
#
#     cached_tokens = 128 * max(0, floor(shared_prefix_tokens / 128) - 1)
#
# So a shared prefix must span at least two blocks -- 256 tokens -- before any
# hit occurs at all. Below that, warming buys nothing.
BLOCK_TOKENS = 128
MIN_WARM_PREFIX_TOKENS = 2 * BLOCK_TOKENS

# The harness has no tokenizer, so the dispatch decision uses a character proxy.
# It is deliberately biased toward warming: 3 chars per token is the dense case
# (code, minified JSON), and assuming density means warming on prefixes that
# might not qualify rather than skipping ones that do. The asymmetry justifies
# it -- a needless warm costs one call of latency, a missed warm costs full
# input price on the shared prefix for every question in the fan-out.
#
# The earlier value here was 2000, a guess. It happened to sit near the real
# boundary for English prose and would have been wrong by 2.6x on code.
CHARS_PER_TOKEN_DENSE = 3
MIN_WARM_PREFIX_CHARS = MIN_WARM_PREFIX_TOKENS * CHARS_PER_TOKEN_DENSE


def cached_tokens_for(shared_prefix_tokens):
    """What the provider will serve from cache for a prefix of this length."""
    return BLOCK_TOKENS * max(0, shared_prefix_tokens // BLOCK_TOKENS - 1)


def common_prefix_len(texts):
    """Length of the longest prefix every text shares.

    Cheap: compares only the lexicographic extremes, since any character all
    strings agree on is a character the smallest and largest also agree on.
    """
    texts = [t for t in texts if t]
    if len(texts) < 2:
        return 0
    lo, hi = min(texts), max(texts)
    n = 0
    for a, b in zip(lo, hi):
        if a != b:
            break
        n += 1
    return n


class BudgetExceeded(Exception):
    """The ceiling would be crossed. Raised only by strict callers."""


NANO = 1_000_000_000  # integer nano-USD; floats do not add up to a ceiling


def _nano(usd):
    """USD as exact integer nano-dollars.

    Money is not float arithmetic. Ten additions of 0.002 produce
    0.020000000000000004, which is over a ceiling of 0.02, so the tenth
    reservation of a ten-call budget was refused. The error was in the safe
    direction -- under-running, not overspending -- and it was still a guard
    that did not do what it said. Nano resolution is well below the cost of the
    cheapest possible call (roughly 2,800 nano-USD), so nothing real rounds away.
    """
    return int(round(float(usd) * NANO))


class Budget:
    """A spending ceiling enforced across threads.

    Every mutation holds the lock. The previous version did not, and
    reserve() is a read-modify-write: two threads could both read a committed
    figure below the ceiling and both commit, so the ceiling was advisory. A
    ten-slot budget granted nine reservations to nine racing threads in one
    demonstration and would have granted more under load.

    Reservations are estimates. settle() replaces an estimate with the figure
    the provider actually reported, so a run of cheap calls does not stay
    blocked by pessimistic estimates and a run of expensive ones stops sooner.
    """

    def __init__(self, ceiling_usd, est_per_call_usd):
        self._ceiling = _nano(ceiling_usd)
        self._est = _nano(est_per_call_usd)
        self._lock = threading.Lock()
        self._committed = 0   # reserved, not yet settled
        self._settled = 0     # provider-reported, actually spent
        self.refused = 0

    @property
    def ceiling(self):
        return self._ceiling / NANO

    @property
    def settled(self):
        return self._settled / NANO

    @property
    def committed(self):
        return self._committed / NANO

    def reserve(self):
        with self._lock:
            if self._committed + self._settled + self._est > self._ceiling:
                self.refused += 1
                return False
            self._committed += self._est
            return True

    def settle(self, actual_usd):
        """Convert one outstanding reservation into a real cost."""
        with self._lock:
            self._committed = max(0, self._committed - self._est)
            self._settled += _nano(actual_usd)

    def release(self):
        """Give back a reservation for a call that never reached the provider."""
        with self._lock:
            self._committed = max(0, self._committed - self._est)

    def snapshot(self):
        with self._lock:
            return {"ceiling_usd": self._ceiling / NANO,
                    "settled_usd": self._settled / NANO,
                    "outstanding_usd": self._committed / NANO,
                    "refused": self.refused}


class Outcome:
    ANSWERED = "ANSWERED"
    TRUNCATED = "TRUNCATED"
    REFUSED_BUDGET = "REFUSED_BUDGET"
    ERROR = "ERROR"


def _row(q, **kw):
    base = {"id": q.id, "outcome": None, "passed": None, "status": None,
            "model_returned": None, "cache_read": None, "cache_write": None,
            "fresh_in": None, "out": None, "wall_ms": None, "usd": None,
            "phase": None, "error": None}
    base.update(kw)
    return base


def fan_out(ad, model, questions, ceiling_usd, tmp, workers=16,
            est_per_call_usd=0.002, cost_fn=None, warm=True, **call_kw):
    """Ask every question, warming the shared prefix first.

    Returns (rows, budget, report). One row per question, always: a question
    that was refused, truncated or errored still gets a row, because a missing
    row is indistinguishable from a question nobody asked.

    `cost_fn(measurement) -> usd` prices a completed call. Without it the budget
    can only track reservations, which is why it is required for any real run
    and optional for fixture tests.
    """
    budget = Budget(ceiling_usd, est_per_call_usd)
    questions = list(questions)
    report = {"warmed": False, "warm_skipped_reason": None,
              "shared_prefix_chars": 0, "cache_write_on_warm": None}

    if not questions:
        return [], budget, report

    shared = common_prefix_len([q.prompt for q in questions])
    report["shared_prefix_chars"] = shared

    def ask(q, phase):
        """One call. Returns a row; raises nothing the caller must handle."""
        if not budget.reserve():
            return _row(q, outcome=Outcome.REFUSED_BUDGET, phase=phase)
        try:
            m, text, raw = ad.call(model, q.prompt, q.max_tokens, tmp, **call_kw)
        except adapter.Truncated as t:
            # The call reached the provider and was billed, so it settles
            # rather than releasing. The answer is not a deliverable.
            usd = cost_fn(t.measurement) if cost_fn else 0.0
            budget.settle(usd)
            return _row(q, outcome=Outcome.TRUNCATED, phase=phase, usd=usd,
                        **_usage(t.measurement))
        except Exception as e:
            # Unknown whether the provider billed this. Releasing the
            # reservation is the choice that can overspend; holding it is the
            # choice that can under-run. Under-running is the safe error for
            # scarce capital, so the reservation is held and the run reports it.
            return _row(q, outcome=Outcome.ERROR, phase=phase,
                        error=f"{type(e).__name__}: {e}")

        usd = cost_fn(m) if cost_fn else 0.0
        budget.settle(usd)
        row = _row(q, outcome=Outcome.ANSWERED, phase=phase, usd=usd,
                   **_usage(m))
        # A checker that raises decides nothing. It must not turn a paid,
        # answered call into a failure, and it must not silently score False.
        try:
            row["passed"] = bool(q.check(text))
        except Exception as e:
            row["passed"] = None
            row["error"] = f"check raised: {type(e).__name__}: {e}"
        return row

    rows = []
    rest = questions

    if warm and len(questions) > 1 and shared >= MIN_WARM_PREFIX_CHARS:
        first = questions[0]
        rest = questions[1:]
        warm_row = ask(first, "warm")
        rows.append(warm_row)
        report["warmed"] = True
        report["cache_write_on_warm"] = warm_row.get("cache_write")
        if warm_row["outcome"] == Outcome.REFUSED_BUDGET:
            # The ceiling was already reached. Fanning out would produce a
            # column of refusals; returning now says the same thing cheaper.
            rows.extend(_row(q, outcome=Outcome.REFUSED_BUDGET, phase="fan")
                        for q in rest)
            return rows, budget, report
    elif not warm:
        report["warm_skipped_reason"] = "disabled by caller"
    elif len(questions) < 2:
        report["warm_skipped_reason"] = "single question"
    else:
        report["warm_skipped_reason"] = (
            f"shared prefix {shared} chars < {MIN_WARM_PREFIX_CHARS}")

    if rest:
        # submit/as_completed, not map. map re-raises the first exception when
        # the result iterator reaches it and discards every later result, so one
        # unexpected failure would throw away calls already paid for. Each
        # future is resolved on its own.
        with cf.ThreadPoolExecutor(max_workers=workers) as ex:
            futs = {ex.submit(ask, q, "fan"): q for q in rest}
            for fut in cf.as_completed(futs):
                q = futs[fut]
                try:
                    rows.append(fut.result())
                except BaseException as e:  # ask() should never raise; prove it
                    rows.append(_row(q, outcome=Outcome.ERROR, phase="fan",
                                     error=f"escaped ask(): {type(e).__name__}: {e}"))

    order = {q.id: i for i, q in enumerate(questions)}
    rows.sort(key=lambda r: order.get(r["id"], 1 << 30))
    return rows, budget, report


def _usage(m):
    return {"status": m.get("status"), "model_returned": m.get("model_returned"),
            "cache_read": m.get("cache_read"), "cache_write": m.get("cache_write"),
            "fresh_in": m.get("fresh_in"), "out": m.get("out"),
            "wall_ms": m.get("wall_ms")}


def pass_rate(rows):
    """Decided answers only.

    A refusal, a truncation, an error and a checker that raised are all
    undecided. Counting any of them as a failure would let a budget ceiling or a
    network fault read as a capability result.
    """
    decided = [r for r in rows if r["outcome"] == Outcome.ANSWERED
               and r["passed"] is not None]
    if not decided:
        return None, 0, 0
    passed = sum(1 for r in decided if r["passed"])
    return passed / len(decided), passed, len(decided)


def tally(rows):
    t = {Outcome.ANSWERED: 0, Outcome.TRUNCATED: 0,
         Outcome.REFUSED_BUDGET: 0, Outcome.ERROR: 0}
    for r in rows:
        if r["outcome"] in t:
            t[r["outcome"]] += 1
    return t
