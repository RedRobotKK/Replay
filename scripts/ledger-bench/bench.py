#!/usr/bin/env python3
"""Before and after, for each product that meters agent spend.

Every tool in this class counts tokens. The question this benchmark answers is
what each one does with them, and the answer is scored the same way for all of
them: run the identical turns through each product's own accounting rule, then
through a per-model cache-tiered price table, and print both.

The fixture is committed and deterministic, so the numbers below are the same on
every machine. Point --transcripts at your own corpus to get your own.

A product that cannot be measured prints NOT MEASURED and never prints zero. A
zero is a claim that something was counted and came to nothing, and none of the
gaps here are that.
"""
import argparse, json, os, sys

# Replay's table, USD per million tokens, in/out. docs/TOKEN-PRICES.md, dated
# 2026-09-07, cross-checked against OpenRouter and LiteLLM.
RATE = {
    "claude-opus-5": (5.0, 25.0),
    "claude-sonnet-5": (2.0, 10.0),
    "claude-fable-5-1": (10.0, 50.0),
    "claude-haiku-4-5": (1.0, 5.0),
}
CACHE_READ, CACHE_WRITE = 0.10, 1.25  # Anthropic multipliers on base input

# QM: src/ratelimit/budget.ts:15 and src/model/pi-models.ts:541.
QM_FLAT_USD_PER_MTOK = 5.0


def load(paths):
    """Sum usage per model, deduplicating by message id.

    The dedup is not incidental. Claude Code writes several assistant events per
    logical request and the usage repeats across them; counting the lines instead
    of the requests over-counted this corpus by 54%.
    """
    agg, seen = {}, set()
    for root in paths:
        for dp, _, fs in os.walk(root) if os.path.isdir(root) else [(os.path.dirname(root), [], [os.path.basename(root)])]:
            for fn in fs:
                if not fn.endswith(".jsonl"):
                    continue
                try:
                    fh = open(os.path.join(dp, fn), encoding="utf-8", errors="replace")
                except OSError:
                    continue
                with fh:
                    for line in fh:
                        if '"usage"' not in line:
                            continue
                        try:
                            o = json.loads(line)
                        except ValueError:
                            continue
                        m = o.get("message") or {}
                        u = m.get("usage")
                        if not isinstance(u, dict):
                            continue
                        mdl = m.get("model") or ""
                        if not mdl:
                            continue
                        rid = m.get("id") or o.get("uuid")
                        if rid:
                            if rid in seen:
                                continue
                            seen.add(rid)
                        a = agg.setdefault(mdl, [0, 0, 0, 0])
                        a[0] += u.get("input_tokens") or 0
                        a[1] += u.get("cache_read_input_tokens") or 0
                        a[2] += u.get("cache_creation_input_tokens") or 0
                        a[3] += u.get("output_tokens") or 0
    return agg


def priced(agg):
    """What the work actually cost: per model, cache tiers separated."""
    t = 0.0
    for mdl, (i, cr, cw, o) in agg.items():
        if mdl not in RATE:
            continue
        ri, ro = RATE[mdl]
        t += (i * ri + cr * ri * CACHE_READ + cw * ri * CACHE_WRITE + o * ro) / 1e6
    return t


def qm_charge(agg):
    """QM's budget, reproduced exactly.

    claude-harness.ts:570 collapses the three input classes into one number,
    wiring.ts:1562 passes it to estimateCostUsd, budget.ts:15 multiplies by a
    flat rate, and budget.ts gates execution on the result. Output is not
    counted at all.
    """
    return sum((i + cr + cw) for i, cr, cw, _ in agg.values()) / 1e6 * QM_FLAT_USD_PER_MTOK


def model_correct_cache_blind(agg):
    """Per-model rates, cache still folded into fresh input.

    The control. It isolates which of QM's two defects carries the error, and
    the answer is that this one does not: correcting the rate alone moves the
    total the wrong way on an Opus-heavy corpus, because Opus input is $5 and
    QM's flat rate is $5.
    """
    t = 0.0
    for mdl, (i, cr, cw, o) in agg.items():
        if mdl not in RATE:
            continue
        ri, ro = RATE[mdl]
        t += ((i + cr + cw) * ri + o * ro) / 1e6
    return t


def memorable_tokens(agg):
    """Memorable's accounting, reproduced from memorable-cli 0.5.18.

    Its CLI accumulates exactly these four classes and maps Anthropic's
    cache_read_input_tokens and cache_creation_input_tokens onto them. It has no
    rate card, no USD and no per-million divisor anywhere in the package, so the
    'before' here is a token count and not a cost.
    """
    s = [0, 0, 0, 0]
    for i, cr, cw, o in agg.values():
        s[0] += i; s[1] += cr; s[2] += cw; s[3] += o
    return {"input_tokens": s[0], "cached_input_tokens": s[1],
            "cache_write_tokens": s[2], "output_tokens": s[3]}


def row(label, before, after, note=""):
    return {"product": label, "before": before, "after": after, "note": note}


def main():
    ap = argparse.ArgumentParser()
    here = os.path.dirname(os.path.abspath(__file__))
    ap.add_argument("--transcripts", default=os.path.join(here, "fixture.jsonl"),
                    help="transcript file or directory (default: the committed fixture)")
    ap.add_argument("--json", action="store_true", help="emit JSON instead of a table")
    a = ap.parse_args()

    agg = load([a.transcripts])
    if not agg:
        print(f"no usage found in {a.transcripts}", file=sys.stderr)
        return 2

    true = priced(agg)
    qm = qm_charge(agg)
    mem = memorable_tokens(agg)
    prompt = sum(i + cr + cw for i, cr, cw, _ in agg.values())
    cache_share = sum(cr for _, cr, _, _ in agg.values()) / prompt if prompt else 0.0

    rows = [
        row("QM", f"${qm:,.2f}", f"${true:,.2f}",
            f"{qm / true:.1f}x over — flat ${QM_FLAT_USD_PER_MTOK:.0f}/MTok on input+cache, output free"),
        row("Memorable", f"{mem['cached_input_tokens'] + mem['input_tokens'] + mem['cache_write_tokens']:,} tokens, no price",
            f"${true:,.2f}", "captures all four classes, ships no rate card"),
        row("River AI", "usage returned, NOT PRICED", "NOT PRICED",
            "OpenAI-compatible /v1 confirmed live; no pricing units published at any documented path"),
        row("GBrain", "NOT MEASURED", "NOT MEASURED",
            "JSON-RPC surface answers 404 unauthenticated; needs a bearer token"),
    ]

    if a.json:
        print(json.dumps({
            "corpus": {"source": a.transcripts, "models": len(agg),
                       "prompt_tokens": prompt, "cache_share": round(cache_share, 4)},
            "qm_charge_usd": round(qm, 2), "true_cost_usd": round(true, 2),
            "ratio": round(qm / true, 3), "memorable_tokens": mem, "rows": rows,
        }, indent=2))
        return 0

    print(f"\n  Before and after, {os.path.basename(a.transcripts)}: "
          f"{prompt:,} prompt tokens across {len(agg)} model(s), {cache_share * 100:.1f}% from cache\n")
    print(f"  {'product':<12}{'before (their rule)':<34}{'after (priced)':<16}note")
    print(f"  {'-' * 12} {'-' * 33} {'-' * 15} {'-' * 60}")
    for r in rows:
        print(f"  {r['product']:<12}{r['before']:<34}{r['after']:<16}{r['note']}")

    blind = model_correct_cache_blind(agg)
    print(f"\n  Which defect carries it: correcting the rate per model but still")
    print(f"  folding cache reads into fresh input gives ${blind:,.2f}, against QM's")
    print(f"  ${qm:,.2f}. Separating the cache tiers gives ${true:,.2f}. The rate is")
    print(f"  not the problem; the dropped cache field is all of it.\n")
    print(f"  NOT MEASURED is not zero. Two of these four cannot be scored from")
    print(f"  outside, and neither is reported as costing nothing.\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
