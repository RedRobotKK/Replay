"""Clean-state validation: prove the interpreter ran the code that is on disk.

A mutation sweep decides whether a test is evidence. It rewrites a module,
runs the suite, restores the module, and reads the pass/fail. Every step of that
assumes the interpreter executes the bytes currently in the file. CPython does
not guarantee that.

THE DEFECT, REPRODUCED 2026-09-28 (see deepseek-suite/results/track-d.md):

  A timestamp-based .pyc records only two facts about its source: the mtime
  TRUNCATED TO A WHOLE SECOND, and the size in bytes. CPython reuses the cached
  bytecode whenever both still match. `cp` sets mtime to the moment of the copy,
  so a mutate-test-restore cycle that completes inside one second leaves a source
  file whose (whole-second mtime, size) pair still matches the previous step's
  .pyc, and the previous step's bytecode is executed instead.

  Both directions were reproduced, 20/20 trials with a plain `cp`:

    mutant on disk, ORIGINAL bytecode run   -> the mutation reads as SURVIVED
    original restored, MUTANT bytecode run  -> the control reads as FAILING

  The size field is only a partial guard. It rejects a mutation that changes the
  file's byte length, so an operator swap like `>` to `>=` is caught. It cannot
  help the RESTORE step at all, because the restored file is by definition the
  same length as the original, so the false-failing control can follow any
  mutation whatsoever.

THE PERMANENT GUARD is two lines, and scripts/harness-test carries them:

    export PYTHONDONTWRITEBYTECODE=1
    rm -rf experiment/harness/__pycache__

No .pyc is written, so none can be reused, so the question does not arise. This
module is the second half: a check that the guard was actually in force, and a
detector for a tree that has already been poisoned. A sweep that ran without the
guard is not repaired by adding the guard afterwards; its results are INVALID
and have to be re-run.

Free, offline, no credential, no spend. Run directly for a report:

    PYTHONDONTWRITEBYTECODE=1 python3 experiment/harness/cleanstate.py
"""
import importlib.util
import marshal
import os
import struct
import sys

HEADER_BYTES = 16
FLAG_HASH_BASED = 0b01
FLAG_CHECK_SOURCE = 0b10

# Verdicts, worst first. ACCEPTED_AND_STALE is the D1 defect and nothing else is.
ACCEPTED_AND_STALE = "ACCEPTED_AND_STALE"
ACCEPTED_AND_FRESH = "ACCEPTED_AND_FRESH"
REJECTED = "REJECTED"
NO_SOURCE = "NO_SOURCE"
UNREADABLE = "UNREADABLE"
FATAL = (ACCEPTED_AND_STALE,)


class CleanStateError(Exception):
    """The interpreter may not be running the code on disk.

    Raised rather than returned. A caller that can ignore this is a caller whose
    results are unfalsifiable, and the whole point of a mutation sweep is that
    its results are falsifiable.
    """


def read_header(pyc_path):
    """Parse a .pyc header, or None when it is not one this interpreter wrote."""
    try:
        with open(pyc_path, "rb") as fh:
            head = fh.read(HEADER_BYTES)
    except OSError:
        return None
    if len(head) < HEADER_BYTES or head[:4] != importlib.util.MAGIC_NUMBER:
        return None
    flags, a, b = struct.unpack("<III", head[4:16])
    hash_based = bool(flags & FLAG_HASH_BASED)
    return {"flags": flags, "hash_based": hash_based,
            "check_source": bool(flags & FLAG_CHECK_SOURCE),
            "source_mtime": None if hash_based else a,
            "source_size": None if hash_based else b,
            "source_hash": head[8:16] if hash_based else None}


def source_for(pyc_path):
    """The .py a cached .pyc claims to be the bytecode of."""
    d, name = os.path.split(pyc_path)
    if os.path.basename(d) != "__pycache__":
        return None
    stem = name.split(".")[0]
    return os.path.join(os.path.dirname(d), stem + ".py")


def would_be_reused(source_path, pyc_path, header=None):
    """Whether CPython would accept this .pyc for this source, WITHOUT compiling.

    This is the acceptance test reimplemented, not asked of the import system,
    because asking the import system is asking the component under suspicion.
    """
    h = header if header is not None else read_header(pyc_path)
    if h is None:
        return False
    try:
        st = os.stat(source_path)
    except OSError:
        return False
    if h["hash_based"]:
        if not h["check_source"]:
            return True                 # unchecked hash-based pyc: always used
        with open(source_path, "rb") as fh:
            return importlib.util.source_hash(fh.read()) == h["source_hash"]
    # The whole-second truncation is the defect. It is reproduced here exactly.
    return (h["source_mtime"] == int(st.st_mtime)
            and h["source_size"] == (st.st_size & 0xFFFFFFFF))


def _digest(code):
    """A structural fingerprint of a code object, stable across compiles.

    co_filename and co_firstlineno are excluded: they differ between a cached
    compile and a fresh one for reasons that are not a behaviour change. Sets and
    frozensets among the constants are sorted, because their marshalled order is
    not guaranteed stable across processes.
    """
    consts = []
    for c in code.co_consts:
        if hasattr(c, "co_code"):
            consts.append(_digest(c))
        elif isinstance(c, (set, frozenset)):
            consts.append(("set", sorted(repr(x) for x in c)))
        else:
            consts.append(repr(c))
    return (code.co_name, code.co_argcount, code.co_kwonlyargcount,
            code.co_flags, bytes(code.co_code), tuple(consts),
            tuple(code.co_names), tuple(code.co_varnames))


def bytecode_matches_source(source_path, pyc_path):
    """Whether the cached bytecode is what this source compiles to right now.

    Returns None when the comparison cannot be made, which is not a pass.
    """
    try:
        with open(pyc_path, "rb") as fh:
            fh.seek(HEADER_BYTES)
            cached = marshal.load(fh)
        with open(source_path, "rb") as fh:
            src = fh.read()
        fresh = compile(src, source_path, "exec", dont_inherit=True)
    except Exception:
        return None
    try:
        return _digest(cached) == _digest(fresh)
    except Exception:
        return None


def inspect(pyc_path):
    """Classify one cached .pyc. See the verdict constants above."""
    h = read_header(pyc_path)
    src = source_for(pyc_path)
    rec = {"pyc": pyc_path, "source": src, "verdict": UNREADABLE,
           "reused": None, "matches": None}
    if h is None or src is None:
        return rec
    if not os.path.exists(src):
        rec["verdict"] = NO_SOURCE
        return rec
    rec["reused"] = would_be_reused(src, pyc_path, h)
    if not rec["reused"]:
        rec["verdict"] = REJECTED
        return rec
    rec["matches"] = bytecode_matches_source(src, pyc_path)
    # None (uncomparable) is treated as stale. An unverifiable cache entry that
    # WILL be used is exactly as unsafe as one known to differ.
    rec["verdict"] = ACCEPTED_AND_FRESH if rec["matches"] else ACCEPTED_AND_STALE
    return rec


def find_pycache(*dirs):
    out = []
    for d in dirs:
        for root, names, _ in os.walk(d):
            if os.path.basename(root) == "__pycache__":
                out.append(root)
            names[:] = [n for n in names if n != ".git"]
    return sorted(out)


def audit(*dirs):
    """Every cached .pyc under these directories, classified."""
    out = []
    for cache in find_pycache(*dirs):
        for name in sorted(os.listdir(cache)):
            if name.endswith(".pyc"):
                out.append(inspect(os.path.join(cache, name)))
    return out


def assert_clean(*dirs, strict=True):
    """Raise unless a run under these directories can be trusted.

    strict (the default) additionally refuses any __pycache__ at all. That is the
    right setting for a mutation sweep: the only cache entry that cannot mislead
    a sweep is the one that does not exist.
    """
    problems = []
    if not sys.dont_write_bytecode:
        problems.append(
            "bytecode writing is ENABLED (PYTHONDONTWRITEBYTECODE unset). This "
            "run can write a .pyc that a later step inside the same whole second "
            "will reuse against different source.")
    findings = audit(*dirs)
    for r in findings:
        if r["verdict"] in FATAL:
            problems.append(
                f"{r['pyc']}: CPython would reuse this bytecode and it is NOT "
                f"what {r['source']} compiles to. Any result measured in this "
                f"tree is about code that is not on disk.")
    if strict:
        for cache in find_pycache(*dirs):
            problems.append(f"{cache}: a bytecode cache exists at all; a sweep "
                            f"must run with none. rm -rf it and re-run.")
    if problems:
        raise CleanStateError("\n  ".join(["clean-state check failed:"] + problems))
    return findings


def main(argv=None):
    argv = list(argv if argv is not None else sys.argv[1:])
    strict = "--strict" in argv
    dirs = [a for a in argv if not a.startswith("-")] or \
        [os.path.dirname(os.path.abspath(__file__))]
    print(f"dont_write_bytecode = {sys.dont_write_bytecode}")
    findings = audit(*dirs)
    if not findings:
        print("no cached bytecode found; nothing can be reused")
    for r in findings:
        print(f"  {r['verdict']:<20} {r['pyc']}")
    bad = [r for r in findings if r["verdict"] in FATAL]
    if bad or (strict and (findings or not sys.dont_write_bytecode)):
        print("\nNOT CLEAN. Results measured in this tree are INVALID.")
        return 1
    print("\nclean")
    return 0


if __name__ == "__main__":
    sys.exit(main())
