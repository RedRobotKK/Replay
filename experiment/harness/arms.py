"""Baseline plus one-variable-at-a-time optimisation arms.

Every arm asks the same 12 questions about a structurally identical document and
is scored by the same machine checker, so an arm that is cheaper and worse shows
up as a pass-rate drop rather than averaging away into the cost figure.

Each arm gets its OWN document seed. Arms run in sequence share one cache, so an
arm reusing an earlier arm's document would inherit its warm prefix and read as
nearly free. That contamination would make every arm after the first look good.

FROZEN 2026-09-29. This script produced published findings and is kept as the
method that generated them. It predates session.py and hand-rolls its own budget
and call wrapper; do not copy that shape into anything new, and do not "fix" it,
because rewriting it changes what produced the numbers in the ledger.

New spending experiments go through session.Run, which enforces the measured
levers by construction.
"""
import argparse, json, os, random, string, sys, time
import concurrent.futures as cf

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import adapter, fanout, meter as meter_mod
from cacheprobe import WORDS

N_Q = 12


def document(seed, n_words=3000):
    """Filler with N uniquely-named records planted at even intervals."""
    r = random.Random(seed)
    recs = []
    for i in range(N_Q):
        key = "RECORD-" + "".join(r.choice(string.ascii_uppercase) for _ in range(6))
        val = r.randint(100000, 999999)
        recs.append((key, val))
    words = [r.choice(WORDS) for _ in range(n_words)]
    step = n_words // (N_Q + 1)
    for i, (k, v) in enumerate(recs):
        words.insert((i + 1) * step + i * 4, f"{k} has value {v} .")
    return " ".join(words), recs


def make_questions(doc, recs, order):
    """order='after' puts the variable query last, which is the cacheable shape."""
    qs = []
    for i, (k, v) in enumerate(recs):
        q = (f"Using only the document below, state the numeric value of {k}. "
             f"Answer with the number alone.")
        prompt = (doc + "\n\n" + q) if order == "after" else (q + "\n\n" + doc)
        qs.append(Q(i, prompt, str(v)))
    return qs


class Q:
    max_tokens = 512

    def __init__(self, qid, prompt, want):
        self.id, self.prompt, self.want = qid, prompt, want

    def check(self, text):
        return self.want in (text or "")


def run_arm(name, ad, model, qs, m, budget, mode, call_kw, tmp):
    """mode: 'seq' | 'par' | 'warm'. Returns the arm's row."""
    m.banner(f"{name}   [{mode}, {model}, {call_kw or 'defaults'}]")
    t0 = time.time()
    rows = []

    def ask(q, phase):
        if not budget.reserve():
            m.error(f"{name}: budget refused q{q.id}")
            return {"id": q.id, "ok": None, "refused": True}
        try:
            mm, txt, _ = ad.call(model, q.prompt, q.max_tokens, tmp,
                                 allow_truncated=True, **call_kw)
        except Exception as e:
            budget.release()
            m.error(f"{name} q{q.id}: {type(e).__name__}: {e}")
            return {"id": q.id, "ok": None, "error": str(e)}
        if mm.get("status") != 200:
            budget.release()
            m.error(f"{name} q{q.id}: HTTP {mm.get('status')}")
            return {"id": q.id, "ok": None, "http": mm.get("status")}
        budget.settle(m.record(mm, model, f"{name} q{q.id} {phase}"))
        return {"id": q.id, "ok": q.check(txt), "trunc": mm.get("truncated"),
                "cache_read": mm.get("cache_read") or 0,
                "fresh": mm.get("fresh_in") or 0,
                "out": mm.get("out") or 0,
                "reasoning": mm.get("reasoning") or 0,
                "wall_ms": mm.get("wall_ms")}

    if mode == "seq":
        rows = [ask(q, "seq") for q in qs]
    elif mode == "par":
        with cf.ThreadPoolExecutor(max_workers=N_Q) as ex:
            futs = [ex.submit(ask, q, "par") for q in qs]
            rows = [f.result() for f in futs]
    else:  # warm
        rows = [ask(qs[0], "warm")]
        with cf.ThreadPoolExecutor(max_workers=N_Q) as ex:
            futs = [ex.submit(ask, q, "fan") for q in qs[1:]]
            rows += [f.result() for f in futs]

    wall = time.time() - t0
    dec = [r for r in rows if r.get("ok") is not None]
    passed = sum(1 for r in dec if r["ok"])
    cr = sum(r.get("cache_read", 0) for r in rows)
    fr = sum(r.get("fresh", 0) for r in rows)
    ou = sum(r.get("out", 0) for r in rows)
    rs = sum(r.get("reasoning", 0) for r in rows)
    usd = sum(0 for _ in rows)  # arm cost read from the meter delta by caller
    return {"arm": name, "mode": mode, "model": model, "kw": call_kw,
            "wall_s": round(wall, 2), "passed": passed, "decided": len(dec),
            "cache_read": cr, "fresh_in": fr, "out": ou, "reasoning": rs,
            "hit_pct": round(100 * cr / max(1, cr + fr), 1), "rows": rows}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ceiling", type=float, default=1.50)
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--base", default="https://api.deepseek.com")
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    os.makedirs(a.tmp, exist_ok=True)
    m = meter_mod.Meter(a.ceiling, label="baseline and arms")
    budget = fanout.Budget(a.ceiling, 0.004)
    chat = adapter.DeepSeekChat(a.curlrc, a.base)
    NONE = {"reasoning_effort": "none"}

    plan = [
        ("B  baseline",      "seq",  "deepseek-flash",  {},     "before"),
        ("A1 order",         "seq",  "deepseek-flash",  {},     "after"),
        ("A2 parallel",      "par",  "deepseek-flash",  {},     "after"),
        ("A3 warm-then-fan", "warm", "deepseek-flash",  {},     "after"),
        ("A4 + no reasoning","warm", "deepseek-flash",  NONE,   "after"),
        ("A5 + v4-pro",      "warm", "deepseek-v4-pro", NONE,   "after"),
    ]
    out = []
    for i, (name, mode, model, kw, order) in enumerate(plan):
        doc, recs = document(seed=8100 + i * 137)   # own document => cold start
        qs = make_questions(doc, recs, order)
        before = m.derived_usd
        row = run_arm(name, chat, model, qs, m, budget, mode, kw, a.tmp)
        row["usd"] = round(m.derived_usd - before, 6)
        out.append(row)

    s = m.summary()
    print(f"\n{'arm':<20}{'wall':>7}{'cost':>11}{'hit%':>7}{'out tok':>9}"
          f"{'rsn':>8}{'pass':>8}")
    base = out[0]
    for r in out:
        print(f"{r['arm']:<20}{r['wall_s']:>6.1f}s{'$'+format(r['usd'],'.5f'):>11}"
              f"{r['hit_pct']:>7.1f}{r['out']:>9,}{r['reasoning']:>8,}"
              f"{str(r['passed'])+'/'+str(r['decided']):>8}")
    print(f"\n{'arm':<20}{'vs baseline cost':>18}{'vs baseline wall':>18}")
    for r in out[1:]:
        cx = base["usd"] / r["usd"] if r["usd"] else float("inf")
        wx = base["wall_s"] / r["wall_s"] if r["wall_s"] else float("inf")
        print(f"{r['arm']:<20}{cx:>17.2f}x{wx:>17.2f}x")
    json.dump({"arms": out, "summary": s, "budget": budget.snapshot()},
              open(a.out, "w"), indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
