"""Separate settling lag from per-request cent truncation. NOT YET RUN.

The open discrepancy: the isolated 200-call batch reconciled at 1.023, but the
session total was $0.05 observed against $0.0959 derived. Two explanations fit
and they make OPPOSITE predictions, so polling the balance for longer cannot
tell them apart -- lag and truncation both look like "the money is not there
yet" at any single moment.

  H1 settling lag       -- the charge posts eventually. Given time, observed
                           converges on derived for ANY batch shape.
  H2 cent truncation    -- a request costing less than $0.01 bills as zero.
                           Observed converges on derived only for batches whose
                           individual calls clear the cent.

The design is two batches with the SAME derived total and opposite call sizes:

  Batch BIG    few calls, each well over $0.01
  Batch SMALL  many calls, each well under $0.01

Under H1 both converge on their derived totals. Under H2, BIG converges and
SMALL stays far below. The balance is then polled on a schedule so a slow post
cannot be mistaken for a missing one.

Runs through session.Run, so the levers are enforced and the ceiling holds.
"""
import argparse
import json
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import session
from cacheprobe import filler
from policy import TaskClass

POLLS_S = [0, 60, 300, 900, 1800, 3600]


def balance(curlrc):
    r = subprocess.run(["curl", "-sS", "-K", curlrc,
                        "https://api.deepseek.com/user/balance"],
                       capture_output=True, text=True)
    try:
        return float(json.loads(r.stdout)["balance_infos"][0]["total_balance"])
    except Exception:
        return None


def batch(name, curlrc, tmp, ceiling, n_calls, prefix_words, max_tokens, workers):
    """One batch. Returns (derived_usd, summary)."""
    run = session.Run(curlrc=curlrc, tmp=tmp, ceiling_usd=ceiling,
                      label=f"settle:{name}", workers=workers, quiet=True)
    shared = filler(prefix_words, seed=hash(name) % 100000)
    # LOOKUP: the answer is a word in the shared block. Reasoning off keeps the
    # output small and predictable, which is what makes the two batches
    # comparable on input cost rather than on how much either happened to think.
    run.fan(TaskClass.LOOKUP, shared,
            [f"Q{i}: reply with the single word ok." for i in range(n_calls)],
            max_tokens=max_tokens)
    s = run.summary()
    print(f"  {name}: {s['calls']} calls, derived ${s['derived_usd']:.5f}, "
          f"per call ${s['derived_usd'] / max(1, s['calls']):.6f}")
    return s["derived_usd"], s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--ceiling", type=float, default=0.60)
    ap.add_argument("--confirm", action="store_true",
                    help="required; this script spends money")
    a = ap.parse_args()
    if not a.confirm:
        print("settle_test.py spends money. Re-run with --confirm.")
        print(f"Estimated: about $0.25 derived across two batches, ceiling "
              f"${a.ceiling:.2f}. Then {POLLS_S[-1] // 60} minutes of polling.")
        return 2

    os.makedirs(a.tmp, exist_ok=True)
    b0 = balance(a.curlrc)
    print(f"balance BEFORE ${b0}")

    # BIG: each call must clear $0.01 on its own. A cold 30k-token prefix at the
    # peak miss rate is about $0.009, so these run UNWARMED and large on purpose
    # -- the one place in this harness where a cold fan-out is the point.
    big_usd, big = batch("BIG", a.curlrc, a.tmp, a.ceiling / 2,
                         n_calls=12, prefix_words=30000, max_tokens=24, workers=12)
    # SMALL: matched total, tiny individual calls.
    small_usd, small = batch("SMALL", a.curlrc, a.tmp, a.ceiling / 2,
                             n_calls=400, prefix_words=400, max_tokens=24, workers=32)

    print(f"\n  derived BIG ${big_usd:.5f}  SMALL ${small_usd:.5f}")
    print(f"  polling the balance at {POLLS_S} seconds\n")
    obs = []
    t0 = time.time()
    for target in POLLS_S:
        while time.time() - t0 < target:
            time.sleep(min(15, target - (time.time() - t0)))
        b = balance(a.curlrc)
        spent = (b0 - b) if (b0 is not None and b is not None) else None
        obs.append({"t_s": target, "balance": b, "observed": spent})
        print(f"    t+{target:>5}s  balance ${b}  observed ${spent:.2f}"
              if spent is not None else f"    t+{target}s  balance read failed")

    total_derived = big_usd + small_usd
    final = obs[-1]["observed"]
    print(f"\n  derived total ${total_derived:.5f}   observed ${final:.2f}")
    print(f"  ratio {final / total_derived:.3f}" if total_derived else "")
    print("\n  H1 settling lag predicts the ratio approaches 1.0 by the last poll.")
    print("  H2 cent truncation predicts it plateaus near "
          f"${big_usd:.2f} -- the BIG batch alone -- and never reaches "
          f"${total_derived:.2f}.")
    print("  Neither is asserted here. The numbers go in the ledger and the "
          "reading is a separate, written judgement.")

    json.dump({"balance_before": b0, "polls": obs,
               "derived_big": big_usd, "derived_small": small_usd,
               "derived_total": total_derived,
               "big_summary": big, "small_summary": small},
              open(a.out, "w"), indent=1, default=str)
    print(f"\nwrote {a.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
