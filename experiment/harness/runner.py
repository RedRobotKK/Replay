"""Experiment runner: arms, trials, and an auditable row per call.

Keeps four things separate on purpose: the adapter sends, the task supplies
content and its checker, pricing derives dollars, and this module only
orchestrates and records. Nothing here interprets a result.
"""
import concurrent.futures as cf, datetime as dt, json, os, statistics, uuid
import pricing

def run_arm(adapter, model, arm_name, order, task_fn, n, tmp, max_tokens=24,
            static_words=1500, workers=8):
    """One arm: n trials, each with a UNIQUE static block so a cold reading is
    genuinely cold. Reusing a block across trials would return warm and encode a
    false baseline, which is the error the 2026-09-05 fixtures recorded."""
    def one(i):
        seed = uuid.uuid4().int % (2**31)
        static, var, check = task_fn(seed, static_words)
        prompt = (static + "\n\n" + var) if order == "static_first" else (var + "\n\n" + static)
        when = dt.datetime.now(dt.timezone.utc)
        m, text, path = adapter.call(model, prompt, max_tokens, tmp)
        return dict(arm=arm_name, order=order, trial=i, seed=seed,
                    when=when.isoformat(), peak=pricing.is_peak(when),
                    passed=bool(check(text)) if m["status"] == 200 else None,
                    usd_derived=pricing.cost_usd(model, m["fresh_in"], m["cache_read"],
                                                 m["out"], when),
                    raw=path, **m)
    with cf.ThreadPoolExecutor(max_workers=workers) as ex:
        return list(ex.map(one, range(n)))

def summarise(rows):
    """Report the spread, not only the mean. NOT_MEASURED stays absent."""
    ok = [r for r in rows if r["status"] == 200]
    def col(k):
        v = [r[k] for r in ok if r.get(k) is not None]
        return v
    out = {"n": len(rows), "http_200": len(ok)}
    for k in ("fresh_in", "cache_read", "out", "wall_ms", "usd_derived"):
        v = col(k)
        out[k] = ({"mean": statistics.mean(v), "median": statistics.median(v),
                   "min": min(v), "max": max(v), "n": len(v)} if v else "NOT_MEASURED")
    p = [r["passed"] for r in ok if r["passed"] is not None]
    out["pass_rate"] = (f"{sum(p)}/{len(p)}" if p else "NOT_MEASURED")
    out["peak_mixed"] = len({r["peak"] for r in ok}) > 1
    return out
