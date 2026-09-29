"""A7/A8: is the prefix cache scoped to the endpoint, the model, or the account?

These are the campaign's strongest falsification tests. A7's prior observation is
n=1 and is treated as NOT_OBSERVED for the general claim. A8 has never been run.

Endpoint and model are constructor arguments on session.Run, so each scope gets
its own Run sharing one ceiling budget by splitting it.
"""
import argparse, json, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import session
from cacheprobe import filler
from policy import TaskClass

TAIL = " Reply with the single word ok."


def mk(curlrc, tmp, ceiling, label, model="deepseek-flash", dialect="chat"):
    return session.Run(curlrc=curlrc, tmp=tmp, ceiling_usd=ceiling, label=label,
                       model=model, dialect=dialect, workers=4, quiet=True)


def ask(run, prompt, note, mt=8):
    r = run.ask(TaskClass.LOOKUP, variable=prompt, shared="", max_tokens=mt, note=note)
    print(f"    {note:<46} hit={r['cache_read']:<7} miss={r['fresh_in']:<7} "
          f"status={r['status']}")
    return r


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--ceiling", type=float, default=0.30)
    a = ap.parse_args()
    c = a.ceiling / 4
    out = {}

    print("=== A7 cross-ENDPOINT (same model, same key) ===")
    print("  PREDICT: B hits on a prefix populated by A, in both directions")
    chat = mk(a.curlrc, a.tmp, c, "A7-chat", dialect="chat")
    anth = mk(a.curlrc, a.tmp, c, "A7-anth", dialect="anthropic")
    p1 = filler(3000, seed=770001)
    ask(chat, p1, "chat COLD (populates)")
    w = ask(chat, p1 + TAIL, "chat WARM (control: endpoint works)")
    x = ask(anth, p1 + TAIL, "anthropic, prefix populated by chat")
    p2 = filler(3000, seed=770002)
    ask(anth, p2, "anthropic COLD (populates)")
    w2 = ask(anth, p2 + TAIL, "anthropic WARM (control)")
    x2 = ask(chat, p2 + TAIL, "chat, prefix populated by anthropic")
    out["a7"] = {"chat_warm": w["cache_read"], "anth_on_chat_prefix": x["cache_read"],
                 "anth_warm": w2["cache_read"], "chat_on_anth_prefix": x2["cache_read"]}
    print(f"  chat->anthropic crossed: {bool(x['cache_read'])}   "
          f"anthropic->chat crossed: {bool(x2['cache_read'])}")
    print("  CONFOUND, registered in PLAN.md: the two dialects wrap the prefix in")
    print("  different request bodies. 'Equivalent prefix' means the same user text;")
    print("  byte-identity of the serialised prompt is ASSUMED, not established.")

    print("\n=== A8 cross-MODEL (same endpoint, same key) ===")
    print("  PREDICT: no sharing. Model-scoped caching is the common implementation.")
    fl = mk(a.curlrc, a.tmp, c, "A8-flash", model="deepseek-flash")
    pro = mk(a.curlrc, a.tmp, c, "A8-pro", model="deepseek-v4-pro")
    p3 = filler(3000, seed=880001)
    ask(fl, p3, "flash COLD (populates)")
    fw = ask(fl, p3 + TAIL, "flash WARM (control)")
    pc = ask(pro, p3 + TAIL, "v4-pro, prefix populated by flash")
    p4 = filler(3000, seed=880002)
    ask(pro, p4, "v4-pro COLD (populates)")
    pw = ask(pro, p4 + TAIL, "v4-pro WARM (control)")
    fc = ask(fl, p4 + TAIL, "flash, prefix populated by v4-pro")
    out["a8"] = {"flash_warm": fw["cache_read"], "pro_on_flash_prefix": pc["cache_read"],
                 "pro_warm": pw["cache_read"], "flash_on_pro_prefix": fc["cache_read"]}
    crossed = bool(pc["cache_read"]) or bool(fc["cache_read"])
    print(f"  flash->v4-pro crossed: {bool(pc['cache_read'])}   "
          f"v4-pro->flash crossed: {bool(fc['cache_read'])}")
    print(f"\n  SCOPE VERDICT: {'account-global' if crossed else 'model-scoped'}"
          f" (cross-model sharing {'observed' if crossed else 'NOT observed'})")

    total = sum(r.summary()["derived_usd"] for r in (chat, anth, fl, pro))
    print(f"\n  DERIVED total ${total:.5f}")
    out["derived_usd"] = total
    json.dump(out, open(a.out, "w"), indent=1)
    print(f"wrote {a.out}")


if __name__ == "__main__":
    sys.exit(main())
