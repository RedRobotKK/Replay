"""Prompt optimization campaign runner. See experiment/prompt-opt/PREREG.md.

Reasoning configuration is held at the provider default in every condition: no
reasoning parameter is sent. Only the instruction suffix changes.

The corpus is the shared prefix and is byte-identical across every trial, so
cache state is a constant of the design rather than something averaged over.
Every trial records its cache read and any departure is flagged.
"""
import argparse, hashlib, json, os, random, sys, time
import concurrent.futures as cf

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import promptcond, session, taskclasses
from fanout import Outcome
from policy import TaskClass

# Reasoning must stay at the default, so every call is dispatched under a task
# class whose policy sends no reasoning parameter.
NEUTRAL = TaskClass.AGGREGATE


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--ceiling", type=float, default=0.60)
    ap.add_argument("--reps", type=int, default=2)
    ap.add_argument("--conditions", default=",".join(promptcond.CONDITIONS))
    ap.add_argument("--seed", type=int, default=20260929)
    ap.add_argument("--workers", type=int, default=12)
    ap.add_argument("--label", default="prompt-opt")
    a = ap.parse_args()
    os.makedirs(a.tmp, exist_ok=True)

    corpus = taskclasses.load_corpus()
    tasks = taskclasses.task_set()
    conds = [c.strip() for c in a.conditions.split(",")]
    run = session.Run(curlrc=a.curlrc, tmp=a.tmp, ceiling_usd=a.ceiling,
                      label=a.label, workers=a.workers, quiet=True)

    print(f"  corpus {len(corpus):,} chars | {len(tasks)} tasks | "
          f"{len(conds)} conditions | {a.reps} reps")
    # Warm the shared prefix once. Every trial then hits the same cached value.
    w = run.ask(NEUTRAL, variable="Reply with the single word ok.", shared=corpus,
                max_tokens=16, note="warm")
    baseline_hit = w["cache_read"] or 0
    print(f"  warm: {w['fresh_in']} fresh, {baseline_hit} cached  "
          f"(this cache read is the constant every trial is checked against)")

    trials = [(t, c, r) for t in tasks for c in conds for r in range(a.reps)]
    random.Random(a.seed).shuffle(trials)      # no condition gets a contiguous block
    print(f"  {len(trials)} trials, shuffled with seed {a.seed}\n")

    rows, order = [], []
    lock = __import__("threading").Lock()

    def one(idx, t, c, rep):
        suffix = promptcond.render(c, t.prompt_variable_part)
        body_hash = hashlib.sha256((corpus + "\n\n" + suffix).encode()).hexdigest()[:16]
        r = run.ask(NEUTRAL, variable=suffix, shared=corpus,
                    max_tokens=taskclasses.MAX_TOKENS, check=t.checker,
                    qid=idx, note=f"{c}/{t.label}/r{rep}")
        with lock:
            order.append({"idx": idx, "cond": c, "task": t.label, "rep": rep})
        eq = promptcond.equivalence(c, t.prompt_variable_part)
        return {
            "idx": idx, "cond": c, "task": t.label, "cls": t.cls, "rep": rep,
            "outcome": r["outcome"], "passed": r["passed"],
            "fresh_in": r["fresh_in"], "cache_read": r["cache_read"],
            "out": r["out"], "reasoning": r["reasoning"], "usd": r["usd"],
            "wall_ms": r["wall_ms"], "status": r["status"],
            "suffix_chars": len(suffix), "body_sha256_16": body_hash,
            "cache_as_expected": (r["cache_read"] or 0) == baseline_hit,
            "equivalence": eq, "text": (r.get("text") or "")[:120],
        }

    t0 = time.time()
    with cf.ThreadPoolExecutor(max_workers=a.workers) as ex:
        futs = [ex.submit(one, i, t, c, rep) for i, (t, c, rep) in enumerate(trials)]
        for n, f in enumerate(cf.as_completed(futs), 1):
            rows.append(f.result())
            if n % 40 == 0:
                print(f"    {n}/{len(trials)}  ${run.meter.derived_usd:.5f}")
    wall = time.time() - t0

    s = run.summary()
    anomalies = [r for r in rows if not r["cache_as_expected"]]
    print(f"\n  wall {wall:.0f}s  DERIVED ${s['derived_usd']:.5f}")
    print(f"  cache-read anomalies: {len(anomalies)} of {len(rows)}"
          f"{'  <-- EXCLUDED from cost comparison' if anomalies else ''}")
    json.dump({"rows": rows, "order": order, "summary": s,
               "baseline_cache_read": baseline_hit, "seed": a.seed,
               "reps": a.reps, "conditions": conds, "wall_s": wall},
              open(a.out, "w"), indent=1, default=str)
    print(f"  wrote {a.out}")


if __name__ == "__main__":
    sys.exit(main())
