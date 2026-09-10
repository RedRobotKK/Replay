#!/usr/bin/env python3
"""Expected against actual, for every surface, with the drift named.

The question this answers is not "did it crash". It is: for each command and
flag, what did we expect, what happened, how far apart are those, is the gap
reproducible, and is it a defect or a decision.

That last distinction is the whole point. `replay prefix` exits 1 when a diff
voids the cached prefix: a harness that flags every non-zero exit as a failure
would report the product's most deliberate behaviour as a bug, and a harness
that ignores exit codes would miss a gate that stopped working. So an exit code
is compared against what that surface is *supposed* to do, not against zero.

Reproducibility is measured rather than assumed. Each surface runs three times;
a result that differs between runs is flaky, and flaky is a third category —
neither pass nor fail — because a check that reports a different answer each
time cannot be evidence for either.
"""
import argparse, json, os, re, subprocess, sys, tempfile, shutil, collections

DANGER = {"execute", "update", "apply", "yes", "write", "record", "contribute",
          "png", "out", "measure", "check-prices", "listen", "metrics-listen",
          "upstream", "install"}

# Surfaces whose non-zero exit is the feature, not the fault.
EXPECT_NONZERO = {("prefix", None)}

# Long-running listeners. Exercising these blind measures the harness or the
# machine, never the product: on a developer's box `serve` reports "bind:
# address already in use" because their own proxy holds :4000, and on a clean
# runner it blocks until the 45s timeout instead. Eight serve surfaces times
# four invocations is roughly twenty-four minutes of a CI job learning nothing,
# which is why the first attempt to run this harness was killed rather than
# read. Both readings are facts about the port.
SERVERS = {"serve"}

# Stdio servers. `mcp` speaks JSON-RPC on stdin; handed /dev/null it sees EOF,
# exits 0 and prints nothing. That is correct, and calling it SILENT reports
# the harness's own choice of stdin as the product's defect.
EXPECT_SILENT = {"mcp"}

# Needs a value; a bare flag would be a usage error rather than a surface test.
NEEDS_VALUE = {"screen": "cost", "color": "never", "cap": "2000", "to": "claude-haiku-4-5",
               "top": "5", "model": "claude-opus-5", "compare": "2026-09-01",
               "before": "__BEFORE__", "after": "__AFTER__", "card": "c", "tone": "measured",
               "dir": "__CORPUS__", "min-sessions": "2", "predicted": "-0.2",
               "ledger": "__TMP__", "policy-file": "__TMP__", "project": "__TMP__",
               "mask-patterns": "__TMP__", "candidates": "512", "max-age": "1h",
               "prior": "0", "relative": "0.1", "resolution": "512", "confirm": "2",
               "min": "0", "max": "65536", "max-probes": "1", "contribute-dir": "__TMP__"}


def surfaces(cli_md):
    s = open(cli_md, encoding="utf-8").read()
    out = []
    for b in re.split(r"^### ", s, flags=re.M)[1:]:
        name = b.split("\n")[0].strip()
        flags = re.findall(r"^\| `-([a-z0-9-]+)` \| ([a-z]+) \|", b, re.M)
        out.append((name, flags))
    return out


def run(binary, args, env, timeout=45):
    try:
        p = subprocess.run([binary] + args, capture_output=True, text=True,
                           stdin=subprocess.DEVNULL, timeout=timeout, env=env)
        return p.returncode, (p.stdout or "") + (p.stderr or "")
    except subprocess.TimeoutExpired:
        return None, "TIMEOUT"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--runs", type=int, default=3)
    ap.add_argument("--out", default="", help="where to write the JSON record")
    a = ap.parse_args()

    work = tempfile.mkdtemp(prefix="drift-")
    corpus = os.path.join(work, "corpus", "proj")
    os.makedirs(corpus)
    here = os.path.dirname(os.path.abspath(__file__))
    root = os.path.join(here, "..", "..")
    shutil.copyfile(os.path.join(root, "internal", "transcript", "testdata", "session-redacted.jsonl"),
                    os.path.join(corpus, "s.jsonl"))
    before = os.path.join(work, "before.json"); after = os.path.join(work, "after.json")
    open(before, "w").write('{"mcpServers":{"a":{}}}')
    open(after, "w").write('{"mcpServers":{"a":{},"b":{}}}')
    home = os.path.join(work, "home"); os.makedirs(home)

    env = dict(os.environ, HOME=home, USERPROFILE=home, LC_ALL="en_US.UTF-8",
               REPLAY_TRANSCRIPTS=os.path.join(work, "corpus"), NO_COLOR="1",
               COLUMNS="100", ANTHROPIC_BASE_URL="http://127.0.0.1:1")

    def sub(v):
        return (v.replace("__BEFORE__", before).replace("__AFTER__", after)
                 .replace("__CORPUS__", os.path.join(work, "corpus"))
                 .replace("__TMP__", os.path.join(work, "x")))

    results = []
    for cmd, flags in surfaces(os.path.join(root, "docs", "CLI.md")):
        if cmd in SERVERS:
            results.append(dict(surface=cmd, verdict="NOT RUN",
                                note="long-running listener; a blind run measures the port, not the surface"))
            for fname, _ in flags:
                results.append(dict(surface=f"{cmd} --{fname}", verdict="NOT RUN",
                                    note="long-running listener; a blind run measures the port, not the surface"))
            continue
        cases = [(cmd, [])]
        for fname, ftype in flags:
            if fname in DANGER:
                results.append(dict(surface=f"{cmd} --{fname}", verdict="NOT RUN",
                                    note="network, write or config change; not exercised blind"))
                continue
            args = [f"-{fname}"] + ([sub(NEEDS_VALUE[fname])] if fname in NEEDS_VALUE else [])
            cases.append((f"{cmd} --{fname}", [cmd] + args))
        for label, args in cases:
            if not args:
                args = [cmd]
            seen = []
            for _ in range(a.runs):
                code, out = run(a.bin, args, env)
                seen.append((code, len(out.strip()) > 0))
            codes = {c for c, _ in seen}
            if len(codes) > 1:
                results.append(dict(surface=label, verdict="FLAKY",
                                    note=f"exit codes differed across {a.runs} runs: {sorted(codes)}"))
                continue
            code = seen[0][0]
            produced = seen[0][1]
            _, text = run(a.bin, args, env)
            low = text.lower()
            if code is None:
                results.append(dict(surface=label, verdict="TIMEOUT", note="did not return in 45s"))
            elif code == 0 and produced:
                results.append(dict(surface=label, verdict="OK", note=""))
            elif code == 0 and not produced and cmd in EXPECT_SILENT:
                results.append(dict(surface=label, verdict="OK", note="exit 0 on stdin EOF, which is the contract"))
            elif code == 0 and not produced:
                results.append(dict(surface=label, verdict="SILENT",
                                    note="exit 0 and printed nothing: a success indistinguishable from a no-op"))
            elif cmd == "prefix":
                results.append(dict(surface=label, verdict="EXPECTED REFUSAL",
                                    note=f"exit {code} is the gate firing, which is the feature"))
            elif "invalid usage" in low or "is required" in low or "unknown command" in low:
                # The classifier learns from what the surface printed. A command
                # that refuses because it was handed no argument is behaving
                # correctly; calling that drift would report the harness's own
                # mistake as the product's, which is how a check ends up
                # measuring the person who wrote it.
                results.append(dict(surface=label, verdict="NEEDS ARGUMENT",
                                    note="refused with a usage message: correct, the harness invoked it bare"))
            else:
                results.append(dict(surface=label, verdict="DRIFT",
                                    note=f"exit {code}, reproducible across {a.runs} runs: {text.strip().splitlines()[0][:70] if text.strip() else 'no output'}"))

    counts = collections.Counter(r["verdict"] for r in results)
    print(f"surfaces exercised: {len(results)}   runs each: {a.runs}\n")
    for k in ("OK", "EXPECTED REFUSAL", "NEEDS ARGUMENT", "SILENT", "DRIFT", "FLAKY", "TIMEOUT", "NOT RUN"):
        if counts.get(k):
            print(f"  {k:<18} {counts[k]}")
    for k in ("FLAKY", "TIMEOUT", "SILENT", "DRIFT"):
        rows = [r for r in results if r["verdict"] == k]
        if rows:
            print(f"\n  {k}:")
            for r in rows[:14]:
                print(f"    {r['surface']:<34} {r['note']}")
            if len(rows) > 14:
                print(f"    ... and {len(rows)-14} more")
    json.dump(results, open(a.out or os.path.join(here, "last-run.json"), "w"), indent=1)

    # A harness that always returns 0 cannot fail, and a check that cannot fail
    # is not evidence. DRIFT, FLAKY and TIMEOUT each mean the surface did not do
    # what docs/CLI.md says it does, or did not do the same thing twice.
    # SILENT is deliberately not fatal: a success indistinguishable from a no-op
    # is worth reporting and is not by itself a broken surface.
    failed = sum(counts.get(k, 0) for k in ("DRIFT", "FLAKY", "TIMEOUT"))
    if failed:
        print(f"\n{failed} surface(s) drifted, flaked or timed out.")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
