"""Reconstruct provider-billed usage from saved raw responses.

Reads nothing but files already on disk and makes no API call, so it is free and
re-runnable by anyone holding the artifacts.

The evidence classes are kept apart on purpose and never merged:

  OBSERVED-PROVIDER  a number the provider put in a response body, or the
                     balance endpoint returned
  DERIVED            arithmetic over OBSERVED-PROVIDER tokens and the published
                     rate table
  NOT_OBSERVED       a call whose response was never saved. Absent from the
                     reconstruction, and therefore a LOWER bound on it, not a
                     zero.

A reconstruction that silently treats NOT_OBSERVED as zero reports a total that
looks complete and is not, which is the error that makes a reconciliation
meaningless.
"""
import argparse
import collections
import datetime as dt
import glob
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import pricing


def load(path):
    """Parse one saved response into a usage record, or None."""
    try:
        with open(path) as fh:
            d = json.load(fh)
    except Exception:
        return None
    u = d.get("usage")
    if not u:
        return None
    created = d.get("created")
    when = dt.datetime.fromtimestamp(created, dt.timezone.utc) if created else None
    if "prompt_cache_hit_tokens" in u:          # /v1/chat/completions, INCLUSIVE
        hit = u.get("prompt_cache_hit_tokens") or 0
        miss = u.get("prompt_cache_miss_tokens") or 0
        rec = {"dialect": "chat", "hit": hit, "miss": miss,
               "prompt_total": u.get("prompt_tokens") or 0,
               "out": u.get("completion_tokens") or 0,
               "reasoning": (u.get("completion_tokens_details") or {}).get(
                   "reasoning_tokens") or 0,
               "cache_write": 0}
    else:                                       # /anthropic/v1/messages, EXCLUSIVE
        rec = {"dialect": "anthropic",
               "hit": u.get("cache_read_input_tokens") or 0,
               "miss": u.get("input_tokens") or 0,
               "prompt_total": None,
               "out": u.get("output_tokens") or 0,
               "reasoning": 0,
               "cache_write": u.get("cache_creation_input_tokens") or 0}
    rec.update(model=d.get("model", ""), when=when, file=os.path.basename(path),
               error=("error" in d))
    return rec


def reconstruct(paths):
    recs = [r for r in (load(p) for p in paths) if r]
    agg = collections.Counter()
    per_model = collections.defaultdict(float)
    regimes = collections.Counter()
    violations = []
    derived = 0.0
    latest = max((r["when"] for r in recs if r["when"]), default=None)

    for r in recs:
        # The INCLUSIVE dialect publishes a total that must equal its parts.
        # A violation means the counting model is wrong, and every cost figure
        # built on it is wrong with it.
        if r["dialect"] == "chat" and r["hit"] + r["miss"] != r["prompt_total"]:
            violations.append(r)
        for k in ("hit", "miss", "out", "reasoning", "cache_write"):
            agg[k] += r[k]
        # Each call is priced at ITS OWN timestamp. Peak is a time-of-day
        # property, so a run that crosses 04:00 UTC is billed at two rates and a
        # flat assumption over-states one half of it.
        when = r["when"] or latest
        if when:
            regimes["peak" if pricing.is_peak(when) else "off_peak"] += 1
        c = pricing.cost_usd(r["model"], r["miss"], r["hit"], r["out"], when) or 0.0
        derived += c
        per_model[r["model"]] += c

    return {"n_responses": len(recs), "tokens": dict(agg),
            "derived_usd": derived, "per_model": dict(per_model),
            "regimes": dict(regimes), "conservation_violations": len(violations),
            "errors": sum(1 for r in recs if r["error"]),
            "first": min((r["when"] for r in recs if r["when"]), default=None),
            "last": latest,
            "by_dialect": dict(collections.Counter(r["dialect"] for r in recs))}


# The balance endpoint reports cents, so an observed delta of $X means true
# spend lies in [X - 0.005, X + 0.005). NANO is integer nano-USD: the interval
# is a THRESHOLD COMPARISON on money, and money is not float arithmetic.
#
# Built with floats it was wrong on its own boundary. At an observed $0.05,
# `0.05 - 0.005` evaluates to 0.045000000000000005, which is ABOVE the bound it
# represents, so a derived total landing exactly on the inclusive lower bound
# was reported OUTSIDE its own interval -- a reconciliation reading as failed
# when it passed.
NANO = 1_000_000_000
HALF_CENT_NANO = 5_000_000


def within_resolution(observed_usd, derived_usd):
    """Is the DERIVED total inside what the OBSERVED balance can distinguish?

    Returns (inside, lo_usd, hi_usd). The comparison is exact; the two bounds
    are floats because they are only ever printed.
    """
    obs = int(round(observed_usd * NANO))
    der = int(round(derived_usd * NANO))
    lo, hi = obs - HALF_CENT_NANO, obs + HALF_CENT_NANO
    return lo <= der < hi, lo / NANO, hi / NANO


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", required=True, help="directory of saved res-*.json")
    ap.add_argument("--glob", default="res-*.json")
    ap.add_argument("--balance-before", type=float)
    ap.add_argument("--balance-after", type=float)
    ap.add_argument("--unrecorded-calls", type=int, default=0,
                    help="calls known to have happened whose response was not saved")
    ap.add_argument("--out")
    a = ap.parse_args()

    r = reconstruct(sorted(glob.glob(os.path.join(a.dir, a.glob))))
    t = r["tokens"]
    print("== OBSERVED-PROVIDER ==")
    print(f"   responses with a usage block   {r['n_responses']}")
    print(f"   by dialect                     {r['by_dialect']}")
    print(f"   responses carrying an error    {r['errors']}")
    print(f"   window (provider `created`)    {r['first']} -> {r['last']}")
    print(f"   pricing regimes                {r['regimes']}")
    print(f"   conservation violations        {r['conservation_violations']}"
          f"  (hit+miss must equal prompt_tokens on the chat dialect)")
    for k in ("miss", "hit", "cache_write", "out", "reasoning"):
        print(f"   {k:<30} {t.get(k,0):>12,}")
    print()
    print("== DERIVED (published rates x observed tokens) ==")
    print(f"   total                          ${r['derived_usd']:.5f}")
    for m, v in sorted(r["per_model"].items()):
        print(f"   {m:<30} ${v:.5f}")

    if a.unrecorded_calls:
        print()
        print(f"== NOT_OBSERVED ==")
        print(f"   {a.unrecorded_calls} call(s) are known to have occurred and their "
              f"responses were not saved.")
        print(f"   The DERIVED total above is therefore a LOWER BOUND, not a total.")

    if a.balance_before is not None and a.balance_after is not None:
        obs = a.balance_before - a.balance_after
        print()
        print("== RECONCILIATION ==")
        print(f"   OBSERVED balance delta         ${obs:.2f}")
        print(f"   DERIVED from responses         ${r['derived_usd']:.5f}")
        # Quoting a ratio without this interval implies a precision the
        # instrument does not have. See within_resolution above.
        inside, lo, hi = within_resolution(obs, r["derived_usd"])
        print(f"   OBSERVED resolution interval   [${lo:.3f}, ${hi:.3f})")
        print(f"   DERIVED lies inside it         {inside}")
        if r["derived_usd"]:
            print(f"   ratio observed/derived         {obs / r['derived_usd']:.4f}"
                  f"   (point estimate; the interval above is the real claim)")
        print("   NOTE: the balance endpoint lags. A reconciliation is provisional "
              "until repeated reads stop moving.")

    if a.out:
        json.dump(r, open(a.out, "w"), indent=1, default=str)
        print(f"\nwrote {a.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
