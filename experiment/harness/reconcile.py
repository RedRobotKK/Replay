"""Validate cache-hit PRICING against money. The last open blocker.

Every previous reconciliation batch ran with cache_read = 0, so the cheapest
rate in the table -- $0.006/1M on flash, off a $0.30 miss rate -- has never been
checked against a bill.

This batch is built so the answer cannot be ambiguous. Roughly 4M of its ~4.2M
input tokens are cache reads. Billed at the hit rate the batch costs about
$0.045; billed at the miss rate it costs about $1.18. A 25x gap is far outside
the balance endpoint's $0.01 resolution and outside any plausible settling lag,
which is what killed the first F4 comparison.

Balance is read before, then after a settling wait, and both are recorded.
"""
import argparse, json, os, subprocess, sys, time
import concurrent.futures as cf

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import adapter, fanout, meter as meter_mod, pricing
from cacheprobe import filler

N_Q = 200
PREFIX_WORDS = 17000
SETTLE_S = 90


def balance(curlrc):
    r = subprocess.run(["curl", "-sS", "-K", curlrc,
                        "https://api.deepseek.com/user/balance"],
                       capture_output=True, text=True)
    try:
        return float(json.loads(r.stdout)["balance_infos"][0]["total_balance"])
    except Exception as e:
        print(f"  balance read failed: {e}")
        return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ceiling", type=float, default=0.30)
    ap.add_argument("--model", default="deepseek-flash")
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--base", default="https://api.deepseek.com")
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--workers", type=int, default=32)
    a = ap.parse_args()
    os.makedirs(a.tmp, exist_ok=True)
    m = meter_mod.Meter(a.ceiling, label="cache-hit price reconciliation", quiet=True)
    budget = fanout.Budget(a.ceiling, 0.002)
    chat = adapter.DeepSeekChat(a.curlrc, a.base)
    NONE = {"reasoning_effort": "none"}

    b0 = balance(a.curlrc)
    print(f"  balance BEFORE  ${b0}")
    pre = filler(PREFIX_WORDS, seed=606060)

    def ask(i):
        if not budget.reserve():
            return {"i": i, "refused": True}
        try:
            mm, txt, _ = chat.call(a.model, pre + f"\n\nQ{i}: reply with the single "
                                   f"word ok.", 24, a.tmp, allow_truncated=True, **NONE)
        except Exception as e:
            budget.release(); return {"i": i, "error": str(e)}
        if mm.get("status") != 200:
            budget.release(); return {"i": i, "http": mm.get("status")}
        usd = m.record(mm, a.model)
        budget.settle(usd)
        return {"i": i, "cache_read": mm.get("cache_read") or 0,
                "fresh": mm.get("fresh_in") or 0, "out": mm.get("out") or 0,
                "usd": usd, "wall_ms": mm.get("wall_ms")}

    print(f"  warming a {PREFIX_WORDS}-word prefix ...")
    t0 = time.time()
    rows = [ask(0)]
    print(f"  warm: {rows[0].get('fresh')} fresh, {rows[0].get('cache_read')} cached")
    print(f"  fanning out {N_Q - 1} questions at {a.workers} workers ...")
    with cf.ThreadPoolExecutor(max_workers=a.workers) as ex:
        futs = [ex.submit(ask, i) for i in range(1, N_Q)]
        for n, f in enumerate(cf.as_completed(futs), 1):
            rows.append(f.result())
            if n % 40 == 0:
                print(f"    {n}/{N_Q - 1}  running ${m.derived_usd:.5f}")
    wall = time.time() - t0

    cr = sum(r.get("cache_read", 0) for r in rows)
    fr = sum(r.get("fresh", 0) for r in rows)
    ou = sum(r.get("out", 0) for r in rows)
    ok = sum(1 for r in rows if "usd" in r)
    hit_rate, miss_rate, out_rate = (
        pricing.PEAK if pricing.is_peak(__import__("datetime").datetime.now(
            __import__("datetime").timezone.utc)) else pricing.OFFPEAK)["deepseek-flash"]
    at_hit = (fr * miss_rate + cr * hit_rate + ou * out_rate) / 1e6
    at_miss = ((fr + cr) * miss_rate + ou * out_rate) / 1e6

    print(f"\n  calls {ok}/{N_Q}   wall {wall:.1f}s")
    print(f"  input {fr:,} fresh + {cr:,} cached   output {ou:,}")
    print(f"  DERIVED if cache reads bill at the HIT rate  (${hit_rate}/1M): ${at_hit:.5f}")
    print(f"  DERIVED if cache reads bill at the MISS rate (${miss_rate}/1M): ${at_miss:.5f}")
    print(f"  the two differ by {at_miss / at_hit:.1f}x\n")
    print(f"  settling {SETTLE_S}s before reading the balance ...")
    time.sleep(SETTLE_S)
    b1 = balance(a.curlrc)
    obs = (b0 - b1) if (b0 is not None and b1 is not None) else None
    print(f"  balance AFTER   ${b1}")
    print(f"  OBSERVED spend  ${obs:.2f}" if obs is not None else "  OBSERVED spend  unknown")
    if obs is not None:
        print(f"\n  observed / derived-at-hit  = {obs / at_hit:.3f}")
        print(f"  observed / derived-at-miss = {obs / at_miss:.3f}")
    json.dump({"balance_before": b0, "balance_after": b1, "observed": obs,
               "derived_at_hit": at_hit, "derived_at_miss": at_miss,
               "fresh": fr, "cached": cr, "out": ou, "calls_ok": ok,
               "wall_s": wall, "rows": rows}, open(a.out, "w"), indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
