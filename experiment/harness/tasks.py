"""Benchmark ladder: every task carries its own machine checker.

A task is (prompt_parts, checker). prompt_parts separates the STATIC block from
the VARIABLE query so an experiment can reorder them without changing content,
which is the only way a placement intervention holds semantics fixed.

Checkers return True/False, never a score, so an arm that costs less and answers
worse is visible as a pass-rate drop rather than averaged away.
"""
import random, re, string, uuid

def _filler(n, seed):
    # seed is per-trial, so the static block differs every trial. A fixed seed
    # here made every trial share one prefix, which cached in BOTH arms and hid
    # the ordering effect entirely. Found 2026-09-28 when counting showed
    # identical cache reads in both orders.
    r = random.Random(seed)
    words = ["provenance", "ledger", "cache", "prefix", "token", "measure",
             "observed", "derived", "estimated", "surface", "workload", "replay"]
    return " ".join(r.choice(words) for _ in range(n))

def extraction(seed, static_words=1500):
    """A: mechanical. A fixed corpus with one planted, uniquely-named field."""
    r = random.Random(seed)
    key = "RECORD-" + "".join(r.choice(string.ascii_uppercase + string.digits) for _ in range(8))
    static = (f"Reference corpus.\n{_filler(static_words, seed)}\n"
              f"Registry entry {key} has status APPROVED and owner unit 4417.\n"
              f"{_filler(static_words, seed + 1)}\n")
    var = f"Question: what is the owner unit for {key}? Reply with the number only."
    return static, var, (lambda t: "4417" in t)

def counting(seed, static_words=1500):
    """A: mechanical. Count a marker whose count is fixed and known."""
    r = random.Random(seed)
    n = r.randint(3, 9)
    body = _filler(static_words, seed) + " " + " ".join(["MARKER"] * n) + " " + _filler(200, seed + 1)
    static = "Reference corpus.\n" + body + "\n"
    var = "Question: how many times does the exact token MARKER appear above? Reply with the digit only."
    return static, var, (lambda t, n=n: re.search(rf"\b{n}\b", t) is not None)

def ordering(seed, static_words=1200):
    """A: mechanical. Sort supplied integers; checker matches the sequence."""
    r = random.Random(seed)
    nums = r.sample(range(10, 99), 5)
    static = "Reference corpus.\n" + _filler(static_words, seed) + "\n"
    var = ("Question: sort these ascending and reply with them comma-separated, "
           "nothing else: " + ", ".join(str(x) for x in nums))
    want = ",".join(str(x) for x in sorted(nums))
    return static, var, (lambda t, w=want: w.replace(" ", "") in t.replace(" ", ""))

LADDER = {"extraction": extraction, "counting": counting, "ordering": ordering}
