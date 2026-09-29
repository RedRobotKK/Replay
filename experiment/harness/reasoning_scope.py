"""Does `reasoning_effort: none` cost accuracy? Depends on the task class.

Ground truth is computed from the corpus here, not written by hand, so a
question cannot be scored against a fact the author mis-remembered.
"""
import argparse, json, os, re, sys
import concurrent.futures as cf

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import adapter, fanout, meter as meter_mod

REPS = 3
MAXTOK = 4096   # raised: at 1024, reasoning-on truncated 37.5% of aggregative calls


def build_corpus(root, files):
    parts = []
    for f in files:
        parts.append(f"===== FILE: {f} =====\n" + open(os.path.join(root, f)).read())
    return "\n\n".join(parts)


def make_questions(corpus):
    """Return [(class, question, ground_truth_checker, label)]."""
    lines = corpus.split("\n")
    qs = []

    # --- M: mechanical. The answer sits at one place in the text. ---
    #
    # The first version of this class asked which function was declared on a
    # given LINE NUMBER of a 48,000-character document. That is a positional
    # counting task, not a lookup: reasoning-on truncated 12/12 and
    # reasoning-off scored 0/12. The cell measured the question design.
    #
    # These are real typed string constants from the corpus. Finding one is a
    # single locate-and-copy with no counting and no inference.
    for name, val in re.findall(r'^\t+(\w+)\s+(?:Kind|Status)\s*=\s*"([^"]+)"',
                                corpus, re.M)[:6]:
        qs.append(("M", f"The document declares a constant named {name} whose "
                        f"value is a double-quoted string literal. State only "
                        f"that literal's contents, without the quotes.",
                   (lambda v: (lambda t: v.lower() in (t or "").lower()))(val),
                   f"M const {name}={val}"))

    # --- R: aggregative. The answer is nowhere in the text. ---
    func_lines = [(i, l) for i, l in enumerate(lines) if l.startswith("func ")]
    n_func = sum(1 for l in lines if l.startswith("func "))
    qs.append(("R", "Count the lines in the document that begin with the exact "
                    "five characters 'func ' at the very start of the line. "
                    "Answer with the number alone.",
               (lambda n: (lambda t: str(n) in re.findall(r"\d+", t or "")))(n_func),
               f"R count func lines={n_func}"))

    n_exp = sum(1 for l in lines if re.match(r"func (?:\([^)]*\)\s*)?[A-Z]", l))
    qs.append(("R", "Count the lines beginning with 'func ' that declare a "
                    "function whose name starts with an UPPERCASE letter "
                    "(a receiver in parentheses may come first). Number alone.",
               (lambda n: (lambda t: str(n) in re.findall(r"\d+", t or "")))(n_exp),
               f"R count exported={n_exp}"))

    n_type = len(re.findall(r"^type \w+", corpus, re.M))
    qs.append(("R", "Count the lines in the document that begin with the exact "
                    "five characters 'type ' at the very start of the line. "
                    "Answer with the number alone.",
               (lambda n: (lambda t: str(n) in re.findall(r"\d+", t or "")))(n_type),
               f"R count types={n_type}"))

    n_imp = len(re.findall(r"^===== FILE:", corpus, re.M))
    qs.append(("R", "Count how many times the exact marker '===== FILE:' appears "
                    "at the start of a line. Answer with the number alone.",
               (lambda n: (lambda t: str(n) in re.findall(r"\d+", t or "")))(n_imp),
               f"R count files={n_imp}"))

    # successor questions: order matters, answer is not adjacent to the question
    for i in range(min(4, len(func_lines) - 1)):
        a = re.match(r"func (?:\([^)]*\)\s*)?([A-Za-z_]\w*)", func_lines[i][1])
        b = re.match(r"func (?:\([^)]*\)\s*)?([A-Za-z_]\w*)", func_lines[i + 1][1])
        if not (a and b):
            continue
        qs.append(("R", f"Find the line declaring the function named {a.group(1)}. "
                        f"State only the name of the function declared on the NEXT "
                        f"line in the document that begins with 'func '.",
                   (lambda w: (lambda t: w.lower() in (t or "").lower()))(b.group(1)),
                   f"R successor of {a.group(1)}={b.group(1)}"))
    return qs


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--ceiling", type=float, default=0.60)
    ap.add_argument("--model", default="deepseek-flash")
    ap.add_argument("--curlrc", required=True)
    ap.add_argument("--root", required=True)
    ap.add_argument("--base", default="https://api.deepseek.com")
    ap.add_argument("--tmp", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    os.makedirs(a.tmp, exist_ok=True)

    corpus = build_corpus(a.root, ["internal/advisor/advisor.go",
                                   "internal/transcript/wire.go"])
    qs = make_questions(corpus)
    print(f"  corpus {len(corpus):,} chars")
    print(f"  questions: {sum(1 for c,_,_,_ in qs if c=='M')} mechanical, "
          f"{sum(1 for c,_,_,_ in qs if c=='R')} aggregative")
    for c, _, _, lab in qs:
        print(f"    [{c}] {lab}")

    m = meter_mod.Meter(a.ceiling, label="reasoning scope", quiet=True)
    budget = fanout.Budget(a.ceiling, 0.004)
    chat = adapter.DeepSeekChat(a.curlrc, a.base)

    def ask(cls, q, check, lab, off, rep):
        kw = {"reasoning_effort": "none"} if off else {}
        if not budget.reserve():
            return {"cls": cls, "off": off, "ok": None, "why": "budget"}
        try:
            mm, txt, _ = chat.call(a.model, corpus + "\n\n" + q, MAXTOK, a.tmp, **kw)
        except adapter.Truncated as t:
            budget.settle(m.record(t.measurement, a.model))
            return {"cls": cls, "off": off, "ok": None, "why": "truncated", "lab": lab}
        except Exception as e:
            budget.release()
            return {"cls": cls, "off": off, "ok": None, "why": str(e)[:60], "lab": lab}
        if mm.get("status") != 200:
            budget.release()
            return {"cls": cls, "off": off, "ok": None, "why": f"http{mm.get('status')}"}
        budget.settle(m.record(mm, a.model))
        return {"cls": cls, "off": off, "ok": bool(check(txt)), "lab": lab,
                "out": mm.get("out") or 0, "rsn": mm.get("reasoning") or 0,
                "wall_ms": mm.get("wall_ms"), "text": (txt or "")[:60]}

    print("\n  warming the corpus prefix ...")
    ask(*qs[0][:1], qs[0][1], qs[0][2], qs[0][3], False, 0) if False else None
    _w = chat.call(a.model, corpus + "\n\nReply with the single word ok.", 32,
                   a.tmp, allow_truncated=True, reasoning_effort="none")
    budget.settle(m.record(_w[0], a.model))
    print(f"  warm: {_w[0].get('fresh_in')} fresh, {_w[0].get('cache_read')} cached")

    jobs = [(c, q, ck, lab, off, r)
            for (c, q, ck, lab) in qs for off in (False, True) for r in range(REPS)]
    print(f"  running {len(jobs)} calls ...")
    rows = []
    with cf.ThreadPoolExecutor(max_workers=24) as ex:
        futs = [ex.submit(ask, *j) for j in jobs]
        for n, f in enumerate(cf.as_completed(futs), 1):
            rows.append(f.result())
            if n % 30 == 0:
                print(f"    {n}/{len(jobs)}  ${m.derived_usd:.5f}")

    print(f"\n  {'cell':<26}{'pass':>10}{'rate':>8}{'undec':>7}{'out tok':>9}{'rsn':>8}")
    cells = {}
    for cls in ("M", "R"):
        for off in (False, True):
            sel = [r for r in rows if r["cls"] == cls and r["off"] == off]
            dec = [r for r in sel if r["ok"] is not None]
            p = sum(1 for r in dec if r["ok"])
            out = sum(r.get("out", 0) for r in sel)
            rsn = sum(r.get("rsn", 0) for r in sel)
            rate = p / len(dec) if dec else None
            cells[(cls, off)] = {"passed": p, "decided": len(dec),
                                 "rate": rate, "out": out, "rsn": rsn,
                                 "undecided": len(sel) - len(dec)}
            nm = f"{cls} reasoning {'OFF' if off else 'ON '}"
            print(f"  {nm:<26}{str(p)+'/'+str(len(dec)):>10}"
                  f"{(f'{rate:.0%}' if rate is not None else '--'):>8}"
                  f"{len(sel)-len(dec):>7}{out:>9,}{rsn:>8,}")

    s = m.summary()
    json.dump({"cells": {f"{k[0]}_{'off' if k[1] else 'on'}": v
                         for k, v in cells.items()},
               "rows": rows, "summary": s, "budget": budget.snapshot()},
              open(a.out, "w"), indent=1)
    return 0


if __name__ == "__main__":
    sys.exit(main())
