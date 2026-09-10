#!/usr/bin/env python3
"""Unit economics for Replay, computed from declared inputs with provenance.

The point of this script is not the arithmetic — that part is trivial. It is
that every input carries where it came from, and a figure that depends on an
unknown input is refused rather than printed.

That refusal is the whole design. A spreadsheet lets you type 3% churn beside a
measured 7.94x and the two look identical on the page; a reader cannot tell
which one somebody observed. Here they cannot be confused, because a headline
that rests on a "?" input prints NOT MEASURED and says which input made it so.

Usage:
    model.py                 the full report
    model.py --check         exit 1 if any figure is presented as decided when
                             its inputs are not; for CI
"""
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
BAR = "─" * 74


def load():
    with open(os.path.join(HERE, "assumptions.json")) as f:
        return json.load(f)


def val(inputs, key):
    """Return (value, provenance). None value means the model must refuse."""
    e = inputs[key]
    return e["v"], e["p"]


def money(x):
    return f"${x:,.2f}"


def line(label, value, prov="", note=""):
    tag = {"M": "measured", "D": "decided", "A": "assumed", "?": "UNKNOWN"}.get(prov, "")
    print(f"  {label:<38} {value:>16}  {tag:<9} {note}")


def main():
    check = "--check" in sys.argv
    d = load()
    inputs = d["inputs"]
    sc = d["scenarios"]

    unknown = [k for k, v in inputs.items() if v["p"] == "?"]

    print(BAR)
    print(f"  Replay unit economics — inputs dated {d['_dated']}")
    print(BAR)

    # ---------------------------------------------------------------- revenue
    listp, lp = val(inputs, "list_price_usd_repo_month")
    dp, dpp = val(inputs, "design_partner_price_usd")
    share, sp = val(inputs, "design_partner_share")
    gm, gmp = val(inputs, "gross_margin")

    arpa = dp * share + listp * (1 - share)
    print("\n  REVENUE PER REPOSITORY / MONTH")
    line("list price", money(listp), lp)
    line("design-partner price", money(dp), dpp)
    line("share on design-partner price", f"{share:.0%}", sp)
    line("blended ARPA (today)", money(arpa), "D", "first cohort is 100% discounted")
    line("blended ARPA (at list)", money(listp), "D", "once the discount ends")
    line("gross margin", f"{gm:.0%}", gmp)
    line("contribution / repo / mo (list)", money(listp * gm), "A", "margin is assumed")

    # -------------------------------------------------------------------- LTV
    print("\n  LIFETIME VALUE — by churn band, at LIST price")
    churn, cp = val(inputs, "monthly_churn")
    if churn is None:
        print("     monthly_churn is UNKNOWN — no single LTV is stated.")
        print("     Bands below are scenarios, not forecasts:")
    for name, c in sc["churn_bands"].items():
        life = 1 / c
        ltv = listp * gm * life
        ltv_dp = dp * gm * life
        line(f"  {name} ({c:.0%}/mo, {life:.0f} mo)",
             money(ltv), "A", f"at $25: {money(ltv_dp)}")

    # -------------------------------------------------------------------- CAC
    print("\n  CAC — by channel, and what each demands of LTV")
    cac, cacp = val(inputs, "cac_usd")
    healthy = sc["churn_bands"]["healthy"]
    ltv_healthy = listp * gm * (1 / healthy)
    if cac is None:
        print("     cac_usd is UNKNOWN — nothing has ever been acquired.")
        print(f"     Ratios below use the 'healthy' band LTV of {money(ltv_healthy)}:")
    for name, c in sc["cac_bands"].items():
        ratio = ltv_healthy / c
        verdict = "viable" if ratio >= 3 else "DOES NOT CLOSE"
        payback = c / (listp * gm)
        line(f"  {name}", money(c), "A",
             f"LTV:CAC {ratio:>5.1f}x  payback {payback:>4.1f} mo  {verdict}")

    # ------------------------------------------------------------ the funnel
    print("\n  THE FUNNEL — measured, and it is the binding constraint")
    ai, aip = val(inputs, "attributed_installs_per_hour")
    ips, ipsp = val(inputs, "distinct_install_ips")
    stars, sp2 = val(inputs, "external_stars")
    line("attributed installs / hour", f"{ai}", aip, "from_card_24h, every hour measured")
    line("distinct install IPs (24h)", f"{ips}", ipsp, "328 installs, one source")
    line("external stars", f"{stars}", sp2, "the only star is the founder's")
    conv, convp = val(inputs, "free_to_paid_conversion")
    if conv is None:
        print("     free_to_paid_conversion is UNKNOWN, and with zero attributed")
        print("     installs it is not merely unmeasured — it is undefined. Any")
        print("     revenue forecast built on it multiplies a guess by zero users.")

    # ------------------------------------------------------- the value anchor
    print("\n  VALUE ANCHOR — what the customer gets, measured")
    tr, trp = val(inputs, "throttle_ratio")
    st, stp = val(inputs, "stranded_daily_usd_at_500_cap")
    av, avp = val(inputs, "avoidable_share_of_spend")
    line("cache-blind arithmetic runs", f"{tr}x high", trp, "57,958 requests")
    line("stranded / day at a $500 cap", money(st), stp, f"≈ {money(st*30)}/mo unusable")
    line("avoidable share of spend", f"{av:.2%}", avp, "one machine, one operator")
    line("list price as % of stranded value", f"{listp/(st*30):.2%}", "D",
         "the anchoring argument, in one figure")

    # ------------------------------------------------------------- the verdict
    print("\n" + BAR)
    if unknown:
        print("  VERDICT: NOT MEASURED — no LTV, CAC or payback figure here is a")
        print("  forecast. Four inputs are unknown and each is unknown for the same")
        print("  reason: nothing has been sold and nobody outside has run the tool.")
        for k in unknown:
            print(f"    ?  {k:<32} {inputs[k]['src']}")
        print("\n  What would make this model real, in order:")
        print("    1. one stranger runs it            → free_to_paid becomes definable")
        print("    2. ten strangers run it            → conversion gets a denominator")
        print("    3. one repo pays                   → cac_usd becomes measurable")
        print("    4. three months of that cohort     → monthly_churn becomes real")
    else:
        print("  VERDICT: all inputs resolved.")
    print(BAR)

    if check and unknown:
        # A model that always exits 0 cannot fail, and a business plan whose
        # unknowns are invisible is the same defect as a green test that
        # asserts nothing.
        print(f"\nunit-economics: {len(unknown)} input(s) unknown; "
              f"no figure here may be quoted as a forecast.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
