"""Characterise the prefix cache before anything is built on it.

Prefix caching is the largest single lever available on this provider, and four
of its properties have never been measured: whether a hit is reported on each
dialect, how long a prefix must be before it caches at all, what busts it, and
how quickly it becomes available after the populating call returns.

`fanout.MIN_WARM_PREFIX_CHARS` is currently a guess. This measures it.

Every probe uses a tiny output cap. The subject is the usage record, not the
answer, so `allow_truncated=True` is correct here and nowhere else.
"""
import argparse
import json
import os
import random
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import adapter
import fanout
import meter as meter_mod

WORDS = ["provenance", "ledger", "cache", "prefix", "token", "measure",
         "observed", "derived", "estimated", "surface", "workload", "replay",
         "boundary", "conformance", "telemetry", "reconcile", "adapter"]


def filler(n_words, seed=20260929):
    """Deterministic, so a prefix is byte-identical across runs and arms."""
    r = random.Random(seed)
    return " ".join(r.choice(WORDS) for _ in range(n_words))


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
    m = meter_mod.Meter(a.ceiling, label="cache probe")
    budget = fanout.Budget(a.ceiling, 0.002)
    chat = adapter.DeepSeekChat(a.curlrc, a.base)
    anth = adapter.DeepSeekAnthropic(a.curlrc, a.base)
    rec = []

    def call(ad, prompt, note, max_tokens=16, **kw):
        if not budget.reserve():
            m.error(f"budget refused: {note}")
            return None
        try:
            mm, txt, _ = ad.call(a.model, prompt, max_tokens, a.tmp,
                                 allow_truncated=True, **kw)
        except Exception as e:
            budget.release()
            m.error(f"{note}: {type(e).__name__}: {e}")
            return None
        if mm.get("status") != 200:
            budget.release()
            m.error(f"{note}: HTTP {mm.get('status')}")
            return None
        usd = m.record(mm, a.model, note)
        budget.settle(usd)
        rec.append({"probe": note, "dialect": ad.name, **dict(mm), "usd": usd})
        return mm

    # P0 -- does anything work at all. One call, smallest possible.
    m.banner("P0 connectivity")
    if call(chat, "Reply with the single word: ok", "P0 smoke") is None:
        m.error("P0 failed; nothing further is worth spending on")
        json.dump(rec, open(a.out, "w"), indent=1)
        return 1

    # P1 -- is a hit reported, and on which dialect. Identical prefix, different
    # suffix, so a hit can only come from the prefix.
    m.banner("P1 hit reporting per dialect")
    pre = filler(3000)
    for ad in (chat, anth):
        call(ad, pre + "\n\nQ1: name one word above.", f"P1 {ad.name} cold")
        call(ad, pre + "\n\nQ2: name a different word above.", f"P1 {ad.name} warm")

    # P2 -- the minimum cacheable prefix. fanout's 2000-char threshold is a
    # guess; a wrong guess either skips a saving or serialises for nothing.
    m.banner("P2 minimum cacheable prefix")
    for n in (16, 32, 64, 128, 256, 512, 1024):
        p = filler(n, seed=90000 + n)      # a distinct prefix per rung
        call(chat, p + "\n\nQ1: one word.", f"P2 {n}w cold")
        got = call(chat, p + "\n\nQ2: another word.", f"P2 {n}w warm")
        if got:
            rec[-1]["rung_words"] = n

    # P3 -- what busts it. Each varies exactly one thing from a warm baseline.
    m.banner("P3 cache sensitivity")
    p3 = filler(3000, seed=31337)
    call(chat, p3 + "\n\nQ0: one word.", "P3 populate")
    call(chat, p3 + "\n\nQ1: one word.", "P3 control (identical prefix)")
    call(chat, " " + p3 + "\n\nQ2: one word.", "P3 prefix + one leading space")
    call(chat, p3 + "\n\nQ3: one word.", "P3 temperature changed", temperature=0.9)
    call(chat, p3 + "\n\nQ4: one word.", "P3 max_tokens changed", max_tokens=64)
    call(chat, p3[:-40] + "\n\nQ5: one word.", "P3 prefix truncated 40 chars")

    # P4 -- availability latency. How soon after a populating call does the
    # cache serve? Warm-then-fan is only correct if the answer is "immediately".
    m.banner("P4 availability after the populating call")
    p4 = filler(3000, seed=4242)
    call(chat, p4 + "\n\nQ0: one word.", "P4 populate")
    for delay in (0.0, 0.5, 2.0):
        time.sleep(delay)
        call(chat, p4 + f"\n\nQ{delay}: one word.", f"P4 +{delay:.1f}s")

    s = m.summary()
    json.dump({"records": rec, "summary": s, "budget": budget.snapshot()},
              open(a.out, "w"), indent=1)
    print(f"\nwrote {a.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
