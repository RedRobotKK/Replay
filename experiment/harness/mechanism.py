"""Track A executor: cache mechanism experiments.

Runs through session.Run, so the budget, the meter, the reasoning guard and the
published concurrency cap all apply. Mechanism arms deliberately violate the
optimisation levers -- A1 sends a prompt with a leading space on purpose -- so
each call passes its complete prompt as the VARIABLE part with an empty shared
block. policy.assemble returns that untouched, which lets the experiment
perturb bytes without weakening the guard that protects ordinary runs.

Every arm prints its prediction BEFORE the observing call is issued.
"""
import argparse
import json
import os
import random
import sys
import threading
import time
import concurrent.futures as cf

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import session
from policy import TaskClass

WORDS = ["provenance", "ledger", "cache", "prefix", "token", "measure",
         "observed", "derived", "estimated", "surface", "workload", "replay",
         "boundary", "conformance", "telemetry", "reconcile", "adapter"]
BLOCK = 128
TAIL = " Reply with the single word ok."


def filler(n_words, seed):
    r = random.Random(seed)
    return " ".join(r.choice(WORDS) for _ in range(n_words))


def h_a(n):
    """Operator's proposed formula."""
    return BLOCK * (n // BLOCK)


def h_b(n):
    """Prior 17-rung fit: the final block is never served."""
    return BLOCK * max(0, n // BLOCK - 1)


class Track:
    def __init__(self, run, out_dir):
        self.run, self.out = run, out_dir
        self.records = []

    def call(self, label, prompt, extra=None, task=TaskClass.LOOKUP, max_tokens=8):
        r = self.run.ask(task, variable=prompt, shared="", max_tokens=max_tokens,
                         note=label, extra_kwargs=extra)
        rec = {"label": label, "outcome": r["outcome"], "hit": r["cache_read"],
               "miss": r["fresh_in"], "out": r["out"], "wall_ms": r["wall_ms"],
               "usd": r["usd"], "status": r["status"], "extra": extra or {}}
        self.records.append(rec)
        return rec

    def say(self, s):
        print(s, flush=True)

    # ---- A1 -------------------------------------------------------------
    def a1(self):
        self.say("\n=== A1 prefix identity ===")
        base = filler(2000, seed=1101)
        cold = self.call("A1 cold (defines prefix)", base + TAIL)
        n = (cold["miss"] or 0) + (cold["hit"] or 0)
        self.say(f"  cold n={n} tokens; predictions for each perturbation below")
        arms = [
            ("exact identical (positive control)", base + TAIL, "HIT expected"),
            ("one LEADING space", " " + base + TAIL, "0 expected: leading bytes differ"),
            ("one TRAILING space added", base + TAIL + " ", "HIT expected: change is at the end"),
            ("one char changed MID-prefix", base[:len(base)//2] + "Z" + base[len(base)//2+1:] + TAIL,
             "partial: blocks before the change may still serve"),
            ("capitalization changed at start", base[0].upper() + base[1:] + TAIL,
             "0 expected if matching is byte-exact"),
            ("punctuation inserted at start", "." + base + TAIL,
             "0 expected if matching is byte-exact"),
        ]
        for lab, prompt, pred in arms:
            self.say(f"  PREDICT {lab}: {pred}")
            r = self.call(f"A1 {lab}", prompt)
            self.say(f"  OBSERVED hit={r['hit']} miss={r['miss']}")

    # ---- A2 -------------------------------------------------------------
    def a2(self):
        self.say("\n=== A2 quantization: H-A vs H-B ===")
        self.say(f"  {'n':>6}{'H-A':>7}{'H-B':>7}{'observed':>10}{'verdict':>10}")
        verdicts = []
        # dense around 128/256/384/512/640; word counts chosen to straddle them
        for w in (80, 95, 100, 105, 110, 120, 130, 145, 160, 190, 200, 215,
                  230, 250, 280, 300, 330, 380, 420, 460, 500, 540):
            base = filler(w, seed=220000 + w)
            cold = self.call(f"A2 {w}w cold", base)
            n = (cold["miss"] or 0) + (cold["hit"] or 0)
            pa, pb = h_a(n), h_b(n)
            warm = self.call(f"A2 {w}w warm", base + TAIL)
            obs = warm["hit"] or 0
            v = ("H-A" if obs == pa and pa != pb else
                 "H-B" if obs == pb and pa != pb else
                 "both" if pa == pb and obs == pa else "NEITHER")
            verdicts.append({"n": n, "h_a": pa, "h_b": pb, "observed": obs,
                             "verdict": v, "multiple_of_128": obs % BLOCK == 0})
            self.say(f"  {n:>6}{pa:>7}{pb:>7}{obs:>10}{v:>10}")
        self.a2_verdicts = verdicts
        disc = [v for v in verdicts if v["h_a"] != v["h_b"]]
        for name in ("H-A", "H-B", "NEITHER"):
            self.say(f"  {name}: {sum(1 for v in disc if v['verdict']==name)}/{len(disc)} discriminating rungs")
        bad = [v for v in verdicts if not v["multiple_of_128"]]
        self.say(f"  cached counts not a multiple of 128: {len(bad)}"
                 f"{'  <-- QUANTIZATION ITSELF REFUTED' if bad else ''}")

    # ---- A4 -------------------------------------------------------------
    def a4(self):
        self.say("\n=== A4 warm-cache timing (completion dependence) ===")
        self.say("  PREDICT sequential B hits; concurrent B misses")
        base = filler(3000, seed=4401)
        self.call("A4 seq A", base)
        b = self.call("A4 seq B (after A completed)", base + TAIL)
        self.say(f"  OBSERVED sequential B hit={b['hit']}")

        base2 = filler(3000, seed=4402)
        order = []
        lock = threading.Lock()

        def fire(tag, prompt):
            with lock:
                order.append((tag, "dispatch", time.monotonic()))
            r = self.call(f"A4 conc {tag}", prompt)
            with lock:
                order.append((tag, "return", time.monotonic()))
            return r
        with cf.ThreadPoolExecutor(max_workers=2) as ex:
            fa = ex.submit(fire, "A", base2)
            fb = ex.submit(fire, "B", base2 + TAIL)
            ra, rb = fa.result(), fb.result()
        self.say(f"  OBSERVED concurrent A hit={ra['hit']}  B hit={rb['hit']}")
        # Dispatch order is not evidence of arrival order. Report the timeline.
        t0 = min(t for _, _, t in order)
        self.say("  timeline (ms from first dispatch): " +
                 ", ".join(f"{tag}:{ev}@{(t-t0)*1000:.0f}" for tag, ev, t in sorted(order, key=lambda x: x[2])))
        a_ret = next(t for tag, ev, t in order if tag == "A" and ev == "return")
        b_disp = next(t for tag, ev, t in order if tag == "B" and ev == "dispatch")
        self.say(f"  B dispatched BEFORE A returned: {b_disp < a_ret}"
                 f"  (this, not the code's order, is the control)")
        self.a4 = {"seq_b_hit": b["hit"], "conc_b_hit": rb["hit"],
                   "b_before_a_returned": b_disp < a_ret}

    # ---- A6 -------------------------------------------------------------
    def a6(self):
        self.say("\n=== A6 parameter invariance ===")
        base = filler(3000, seed=6601)
        self.call("A6 populate", base)
        ctrl = self.call("A6 control (no change)", base + TAIL)
        self.say(f"  control hit={ctrl['hit']}  PREDICT every arm below matches this")
        for lab, extra in [("temperature=0.9", {"temperature": 0.9}),
                           ("max_tokens=64", None),
                           ("top_p=0.5", {"top_p": 0.5}),
                           ("frequency_penalty=1.0", {"frequency_penalty": 1.0}),
                           ("presence_penalty=1.0", {"presence_penalty": 1.0}),
                           ("stop=['zzz']", {"stop": ["zzz"]})]:
            mt = 64 if lab.startswith("max_tokens") else 8
            r = self.call(f"A6 {lab}", base + TAIL, extra=extra, max_tokens=mt)
            same = r["hit"] == ctrl["hit"]
            self.say(f"  {lab:<26} hit={r['hit']:<8} unchanged={same}"
                     f"{'' if same else '   <-- AFFECTS CACHE IDENTITY'}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--ceiling", type=float, default=0.40)
    ap.add_argument("--arms", default="a1,a2,a4,a6")
    a = ap.parse_args()
    os.makedirs(a.tmp, exist_ok=True)
    run = session.Run(curlrc=a.curlrc, tmp=a.tmp, ceiling_usd=a.ceiling,
                      label="track-A", workers=8, quiet=True)
    t = Track(run, a.tmp)
    for arm in a.arms.split(","):
        getattr(t, arm.strip())()
    s = run.summary()
    print(f"\n  calls {s['calls']}  DERIVED ${s['derived_usd']:.5f}  "
          f"({100*s['derived_usd']/a.ceiling:.1f}% of ceiling)")
    json.dump({"records": t.records, "summary": s,
               "a2_verdicts": getattr(t, "a2_verdicts", None),
               "a4": getattr(t, "a4", None)}, open(a.out, "w"), indent=1, default=str)
    print(f"wrote {a.out}")


if __name__ == "__main__":
    sys.exit(main())
