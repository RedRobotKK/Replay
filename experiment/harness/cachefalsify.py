"""Falsification round for the 128-token block model. See the prereg file.
FROZEN 2026-09-29. This script produced published findings and is kept as the
method that generated them. It predates session.py and hand-rolls its own budget
and call wrapper; do not copy that shape into anything new, and do not "fix" it,
because rewriting it changes what produced the numbers in the ledger.

New spending experiments go through session.Run, which enforces the measured
levers by construction.
"""
import argparse, json, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import adapter, fanout, meter as meter_mod
from cacheprobe import filler

PREDICT = {160: 0, 200: 128, 240: 128, 2048: 2176, 4096: 4608}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ceiling", type=float, default=0.25)
    ap.add_argument("--model", default="deepseek-flash")
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--base", default="https://api.deepseek.com")
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    os.makedirs(a.tmp, exist_ok=True)
    m = meter_mod.Meter(a.ceiling, label="cache model falsification")
    budget = fanout.Budget(a.ceiling, 0.004)
    chat = adapter.DeepSeekChat(a.curlrc, a.base)
    rec = []

    def call(prompt, note, **kw):
        if not budget.reserve():
            m.error(f"budget refused: {note}"); return None
        try:
            mm, _t, _p = chat.call(a.model, prompt, 16, a.tmp,
                                   allow_truncated=True, **kw)
        except Exception as e:
            budget.release(); m.error(f"{note}: {type(e).__name__}: {e}"); return None
        if mm.get("status") != 200:
            budget.release(); m.error(f"{note}: HTTP {mm.get('status')}"); return None
        budget.settle(m.record(mm, a.model, note))
        return mm

    m.banner("falsification: rungs the model has never seen")
    results = []
    for n, pred in sorted(PREDICT.items()):
        p = filler(n, seed=770000 + n)
        call(p + "\n\nQ1: one word.", f"{n}w cold")
        got = call(p + "\n\nQ2: another word.", f"{n}w warm")
        if got is None:
            continue
        obs = got.get("cache_read") or 0
        tot = (got.get("fresh_in") or 0) + obs
        ok = obs == pred
        mult = (obs % 128 == 0)
        results.append({"rung": n, "prompt_tokens": tot, "predicted": pred,
                        "observed": obs, "exact": ok, "multiple_of_128": mult})
        print(f"    rung {n:>5}w  prompt {tot:>6}  predicted {pred:>6}  "
              f"observed {obs:>6}  {'MATCH' if ok else 'MISS'}"
              f"  {'' if mult else '  <-- NOT a multiple of 128'}")

    hits = sum(1 for r in results if r["exact"])
    print(f"\n  exact predictions: {hits}/{len(results)}")
    print(f"  all multiples of 128: {all(r['multiple_of_128'] for r in results)}")
    s = m.summary()
    json.dump({"prereg": PREDICT, "results": results, "summary": s,
               "budget": budget.snapshot()}, open(a.out, "w"), indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
