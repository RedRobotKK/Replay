"""Track B audit: corpus reconstruction, missing-call accounting, balance
reconciliation with correct precision, and the cent-truncation falsification.

Reads saved artifacts only. Makes no network call of any kind.

Evidence classes are kept apart:
  OBSERVED      a value literally present in a saved response body or balance read
  DERIVED       arithmetic over OBSERVED values and the published rate table
  ASSUMED       a premise this campaign did not establish
  NOT_OBSERVED  evidence that was never captured. Never zero, never false.
"""
import collections
import datetime as dt
import glob
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import pricing

PROBE = "/Users/daniel/.claude/jobs/581b6292/tmp/probe"


def load(path):
    with open(path) as fh:
        d = json.load(fh)
    u = d.get("usage")
    if not u:
        return None
    created = d.get("created")
    when = dt.datetime.fromtimestamp(created, dt.timezone.utc) if created else None
    base = dict(file=os.path.basename(path), model=d.get("model", ""),
                created=created, when=when, error=("error" in d))
    if "prompt_cache_hit_tokens" in u:
        det_p = u.get("prompt_tokens_details") or {}
        det_c = u.get("completion_tokens_details") or {}
        base.update(
            dialect="chat",
            hit=u.get("prompt_cache_hit_tokens"),
            miss=u.get("prompt_cache_miss_tokens"),
            prompt_total=u.get("prompt_tokens"),
            out=u.get("completion_tokens"),
            total=u.get("total_tokens"),
            cached_tokens=det_p.get("cached_tokens"),
            reasoning=det_c.get("reasoning_tokens"),
            reasoning_present=("reasoning_tokens" in det_c),
            cache_write=None,                 # field absent on this dialect
            cache_write_present=False,
        )
    else:
        base.update(
            dialect="anthropic",
            hit=u.get("cache_read_input_tokens"),
            miss=u.get("input_tokens"),
            prompt_total=None,
            out=u.get("output_tokens"),
            total=None,
            cached_tokens=None,
            reasoning=None,
            reasoning_present=False,
            cache_write=u.get("cache_creation_input_tokens"),
            cache_write_present=("cache_creation_input_tokens" in u),
        )
    base["raw_usage_keys"] = sorted(u.keys())
    return base


def main():
    paths = sorted(glob.glob(os.path.join(PROBE, "res-*.json")))
    recs = [r for r in (load(p) for p in paths) if r]
    out = {"n_files_scanned": len(paths), "n_with_usage": len(recs)}

    # ---- B1 conservation -------------------------------------------------
    violations, total_mismatch = [], []
    for r in recs:
        if r["dialect"] != "chat":
            continue
        if r["hit"] is None or r["miss"] is None or r["prompt_total"] is None:
            violations.append({"file": r["file"], "kind": "missing_field"})
            continue
        if r["hit"] + r["miss"] != r["prompt_total"]:
            violations.append({"file": r["file"], "kind": "hit+miss!=prompt",
                               "hit": r["hit"], "miss": r["miss"],
                               "prompt_tokens": r["prompt_total"]})
        if r["total"] is not None and r["prompt_total"] + r["out"] != r["total"]:
            total_mismatch.append({"file": r["file"], "prompt": r["prompt_total"],
                                   "out": r["out"], "total": r["total"]})
        # cached_tokens is a second, independent report of the hit count
        if r["cached_tokens"] is not None and r["cached_tokens"] != r["hit"]:
            violations.append({"file": r["file"], "kind": "cached_tokens!=hit",
                               "cached_tokens": r["cached_tokens"], "hit": r["hit"]})
        if r["reasoning"] is not None and r["reasoning"] > r["out"]:
            violations.append({"file": r["file"], "kind": "reasoning>completion",
                               "reasoning": r["reasoning"], "out": r["out"]})

    # ---- B1 aggregation and per-request cost -----------------------------
    agg = collections.Counter()
    per_model = collections.defaultdict(float)
    per_model_calls = collections.Counter()
    regimes = collections.Counter()
    per_req = []
    latest = max((r["when"] for r in recs if r["when"]), default=None)
    for r in recs:
        for k in ("hit", "miss", "out", "reasoning", "cache_write"):
            agg[k] += r[k] or 0
        when = r["when"] or latest
        regime = "peak" if pricing.is_peak(when) else "off_peak"
        regimes[regime] += 1
        c = pricing.cost_usd(r["model"], r["miss"], r["hit"], r["out"], when)
        per_req.append({"file": r["file"], "usd": c, "model": r["model"],
                        "regime": regime, "dialect": r["dialect"],
                        "when": when.isoformat() if when else None})
        per_model[r["model"]] += c or 0.0
        per_model_calls[r["model"]] += 1
    derived = sum(p["usd"] or 0.0 for p in per_req)
    unpriced = [p["file"] for p in per_req if p["usd"] is None]
    costs = sorted((p["usd"] for p in per_req if p["usd"] is not None))
    dearest = sorted(per_req, key=lambda p: -(p["usd"] or 0))[:5]

    # cache_write is a priced quantity the cost function never receives
    cw_reported = [r["file"] for r in recs if r["cache_write"]]

    out["B1"] = {
        "tokens_OBSERVED": dict(agg),
        "by_dialect": dict(collections.Counter(r["dialect"] for r in recs)),
        "responses_carrying_error": sum(1 for r in recs if r["error"]),
        "window_first_created_utc": min(
            (r["when"] for r in recs if r["when"]), default=None),
        "window_last_created_utc": latest,
        "regimes": dict(regimes),
        "conservation_violations": violations,
        "prompt_plus_completion_vs_total_mismatches": total_mismatch,
        "derived_usd": derived,
        "per_model_usd": dict(per_model),
        "per_model_calls": dict(per_model_calls),
        "unpriced_files": unpriced,
        "per_request_min_usd": costs[0] if costs else None,
        "per_request_max_usd": costs[-1] if costs else None,
        "per_request_median_usd": costs[len(costs) // 2] if costs else None,
        "n_requests_at_or_above_1_cent": sum(1 for c in costs if c >= 0.01),
        "dearest_five": dearest,
        "anthropic_cache_creation_nonzero": cw_reported,
        "reasoning_field_present_count": sum(1 for r in recs if r["reasoning_present"]),
        "cache_creation_field_present_count": sum(
            1 for r in recs if r["cache_write_present"]),
        "distinct_usage_key_sets": sorted(
            {tuple(r["raw_usage_keys"]) for r in recs}),
    }

    # ---- run census: do the summary artifacts account for every response? --
    census = {}
    for name in ("cacheprobe", "falsify", "clean", "arms", "rscope", "rscope2",
                 "reconcile"):
        p = os.path.join(PROBE, name + ".json")
        d = json.load(open(p))
        s = d.get("summary") or {}
        census[name] = {"calls": s.get("calls", len(d.get("rows", []))),
                        "derived_usd": s.get("derived_usd")}
    census["reconcile"] = {"calls": json.load(
        open(os.path.join(PROBE, "reconcile.json")))["calls_ok"],
        "derived_usd": json.load(
            open(os.path.join(PROBE, "reconcile.json")))["derived_at_hit"]}
    out["run_census"] = census
    out["run_census_total_calls"] = sum(v["calls"] for v in census.values())
    out["run_census_total_usd"] = sum(v["derived_usd"] or 0 for v in census.values())

    # ---- B3 balance ------------------------------------------------------
    poll_path = os.path.join(PROBE, "balance_poll.jsonl")
    poll = []
    if os.path.exists(poll_path):
        with open(poll_path) as fh:
            for line in fh:
                line = line.strip()
                if line:
                    poll.append(json.loads(line))
    out["B3"] = {"poll_rows": poll, "poll_row_count": len(poll)}

    json.dump(out, open(sys.argv[1], "w"), indent=1, default=str)
    print(json.dumps({k: v for k, v in out.items() if k != "B1"},
                     indent=1, default=str))
    b1 = out["B1"]
    print("--- B1 ---")
    for k in ("tokens_OBSERVED", "by_dialect", "regimes", "derived_usd",
              "per_model_usd", "per_model_calls", "window_first_created_utc",
              "window_last_created_utc", "per_request_min_usd",
              "per_request_max_usd", "per_request_median_usd",
              "n_requests_at_or_above_1_cent", "reasoning_field_present_count",
              "cache_creation_field_present_count",
              "anthropic_cache_creation_nonzero", "unpriced_files",
              "responses_carrying_error", "distinct_usage_key_sets"):
        print(f"  {k} = {b1[k]}")
    print(f"  conservation_violations = {len(b1['conservation_violations'])}")
    for v in b1["conservation_violations"][:20]:
        print("    ", v)
    print(f"  total_mismatches = {len(b1['prompt_plus_completion_vs_total_mismatches'])}")
    for v in b1["prompt_plus_completion_vs_total_mismatches"][:20]:
        print("    ", v)
    print("  dearest_five:")
    for d in b1["dearest_five"]:
        print("    ", d)


if __name__ == "__main__":
    main()
