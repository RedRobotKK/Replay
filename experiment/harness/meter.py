"""Live spend meter. Prints what each call cost, as it happens.

Cost here is DERIVED: arithmetic over provider-reported usage and a published
rate. It is not a charge. `/user/balance` is the only OBSERVED cost source and
its resolution floor is near $0.01, coarser than most single requests, so the
meter reconciles against the balance at run boundaries rather than per call.

The distinction is kept visible in the output rather than explained once in a
footnote, because a derived running total looks exactly like a bill.
"""
import sys
import threading
import time

import pricing


def _fmt_usd(v):
    return f"${v:,.6f}" if v < 0.01 else f"${v:,.4f}"


class Meter:
    def __init__(self, ceiling_usd, label="", stream=sys.stderr, quiet=False):
        self.ceiling = float(ceiling_usd)
        self.label = label
        self.stream = stream
        self.quiet = quiet
        self._lock = threading.Lock()
        self.t0 = time.time()
        self.calls = 0
        self.fresh_in = 0
        self.cache_read = 0
        self.out = 0
        self.reasoning = 0
        self.derived_usd = 0.0
        self.errors = 0

    def record(self, m, model, note=""):
        """Add one completed call and print its line. Returns its derived cost."""
        import datetime as dt
        when = dt.datetime.now(dt.timezone.utc)
        fi = m.get("fresh_in") or 0
        cr = m.get("cache_read") or 0
        ou = m.get("out") or 0
        rs = m.get("reasoning") or 0
        usd = pricing.cost_usd(model, fi, cr, ou, when) or 0.0
        with self._lock:
            self.calls += 1
            self.fresh_in += fi
            self.cache_read += cr
            self.out += ou
            self.reasoning += rs
            self.derived_usd += usd
            n, total = self.calls, self.derived_usd
            hitrate = self.cache_read / max(1, self.cache_read + self.fresh_in)
        if not self.quiet:
            pct = 100 * total / self.ceiling if self.ceiling else 0
            self._w(f"  [{n:>3}] in {fi:>6,} fresh + {cr:>6,} cached  out {ou:>5,}"
                    f"{'  rsn ' + format(rs, ',') if rs else ''}"
                    f"  {m.get('wall_ms', 0):>6,}ms"
                    f"  {_fmt_usd(usd)}"
                    f"   run {_fmt_usd(total)} ({pct:4.1f}% of ceiling)"
                    f"  hit {hitrate:5.1%}  {note}")
        return usd

    def error(self, what):
        with self._lock:
            self.errors += 1
        self._w(f"  [ERR] {what}")

    def _w(self, s):
        self.stream.write(s + "\n")
        self.stream.flush()

    def banner(self, text):
        self._w(f"\n=== {text} ===")

    def summary(self):
        import datetime as dt
        with self._lock:
            el = time.time() - self.t0
            billable_in = self.cache_read + self.fresh_in
            hit = self.cache_read / max(1, billable_in)
            lines = [
                "",
                f"--- {self.label} ---",
                f"  calls              {self.calls:,}   errors {self.errors}",
                f"  input              {self.fresh_in:,} fresh + {self.cache_read:,} cached "
                f"= {billable_in:,}  (cache hit {hit:.1%})",
                f"  output             {self.out:,}"
                + (f"  of which reasoning {self.reasoning:,}" if self.reasoning else ""),
                f"  DERIVED cost       {_fmt_usd(self.derived_usd)}  "
                # record() already guards this division; summary() did not, and a
                # run constructed with a zero ceiling is exactly the run whose
                # summary matters most -- it refused every call.
                f"({100 * self.derived_usd / self.ceiling if self.ceiling else 0:.1f}% "
                f"of ${self.ceiling:.2f} ceiling)",
                f"  pricing regime     {'PEAK' if pricing.is_peak(dt.datetime.now(dt.timezone.utc)) else 'OFF-PEAK'}"
                f"  (table {pricing.TABLE_DATE})",
                f"  wall               {el:.1f}s",
                "  NOTE: cost is DERIVED from published rates, not a provider charge.",
            ]
        self._w("\n".join(lines))
        return {"calls": self.calls, "fresh_in": self.fresh_in,
                "cache_read": self.cache_read, "out": self.out,
                "reasoning": self.reasoning, "derived_usd": self.derived_usd,
                "errors": self.errors, "wall_s": el}
