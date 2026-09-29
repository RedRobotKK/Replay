"""Decisive test of the block model, with every input observed.

The falsification round mis-predicted one rung because the prediction needed a
word-to-token estimate, and that estimate was wrong by 111 tokens, which moved
the block floor by one. The model was not what failed; the instrument in front
of it was.

This removes the estimate. The warm prompt is a strict EXTENSION of the cold
prompt, so the shared prefix is exactly the cold call's own prompt, whose token
count the provider reported. The prediction is then made and printed BEFORE the
warm call is issued.
"""
import argparse, json, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import adapter, fanout, meter as meter_mod
from cacheprobe import filler

BLOCK = 128


def predict(shared_tokens):
    return BLOCK * max(0, shared_tokens // BLOCK - 1)


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
    m = meter_mod.Meter(a.ceiling, label="block model, fully observed")
    budget = fanout.Budget(a.ceiling, 0.004)
    chat = adapter.DeepSeekChat(a.curlrc, a.base)

    def call(prompt, note):
        if not budget.reserve():
            m.error(f"budget refused: {note}"); return None
        try:
            mm, _t, _p = chat.call(a.model, prompt, 16, a.tmp, allow_truncated=True)
        except Exception as e:
            budget.release(); m.error(f"{note}: {type(e).__name__}: {e}"); return None
        if mm.get("status") != 200:
            budget.release(); m.error(f"{note}: HTTP {mm.get('status')}"); return None
        budget.settle(m.record(mm, a.model, note))
        return mm

    m.banner("shared prefix is the cold prompt itself; prediction precedes the warm call")
    out = []
    for n in (300, 700, 1500, 3000, 5000):
        base = filler(n, seed=550000 + n)
        cold = call(base, f"{n}w cold (defines the prefix)")
        if cold is None:
            continue
        shared = (cold.get("fresh_in") or 0) + (cold.get("cache_read") or 0)
        p = predict(shared)
        print(f"    PREDICT: shared={shared} tokens -> cached={p} "
              f"({shared // BLOCK} blocks, minus the last)")
        warm = call(base + " and then answer: name one word above.", f"{n}w warm")
        if warm is None:
            continue
        obs = warm.get("cache_read") or 0
        hit = obs == p
        print(f"    OBSERVED: {obs}  {'MATCH' if hit else 'MISS'}")
        out.append({"rung": n, "shared_tokens": shared, "predicted": p,
                    "observed": obs, "match": hit})

    n_ok = sum(1 for r in out if r["match"])
    print(f"\n  exact, every input observed: {n_ok}/{len(out)}")
    s = m.summary()
    json.dump({"results": out, "summary": s, "budget": budget.snapshot()},
              open(a.out, "w"), indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
