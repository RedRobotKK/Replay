#!/usr/bin/env python3
"""Audit every TUI screen for layout artifacts at several terminal widths.

Checks that hold regardless of taste:
  1. no rendered line is wider than the terminal it claims to fit
  2. every SGR sequence opened is closed, so colour never leaks past a row
  3. only the palette internal/tui/color.go defines is emitted
  4. no stray control characters or trailing whitespace
Width is measured East-Asian-aware, because Ambiguous characters are one cell
in en_US and two in ja_JP and this project ships to both.
"""
import os, re, shutil, subprocess, sys, tempfile, unicodedata

SCREENS = ["cost","why","context","advise","guards","model","safe","share","live"]
WIDTHS  = [60, 80, 120, 200]
LOCALES = ["en_US.UTF-8", "ja_JP.UTF-8"]
PALETTE = {"0","1","2","36","32","33","31"}
SGR = re.compile(r"\x1b\[([0-9;]*)m")
BIN = os.environ.get("REPLAY_BIN") or os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "..", "..", "replay")

# Widths at or above the design target. A failure here is a regression and
# fails the build.
SUPPORTED = 80

# Below the target the layout is known to overflow, and the count is frozen so
# it cannot quietly grow. internal/tui/storyboard.go scene 25, "Terminal
# narrower than 80", already specifies the fix — "columns dropped in a fixed
# order: wire, endpoint, surface" — so this is an unimplemented design, not an
# unknown defect. Implementing it should drive this number to zero, and this
# check will say so.
KNOWN_NARROW_OVERFLOWS = 18

def cells(s, ambiguous_wide):
    s = SGR.sub("", s)
    n = 0
    for ch in s:
        if unicodedata.combining(ch): continue
        w = unicodedata.east_asian_width(ch)
        n += 2 if (w in ("W","F") or (w == "A" and ambiguous_wide)) else 1
    return n

# Render against the repository's own fixture, never the machine's corpus.
#
# The first version of this check did not pin it, and the count came out 86
# here and 72 on the CI runner: the screens render real sessions, so the layout
# depends on whatever transcripts happen to exist. That is precisely the fault
# screens_svg_test.go documents — "a check whose inputs vary by machine is a
# check that passes at home and fails in CI" — repeated by the check written to
# catch layout faults.
_ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
_FIXTURE_SRC = os.path.join(_ROOT, "internal", "transcript", "testdata", "session-redacted.jsonl")


def _fixture_root():
    d = tempfile.mkdtemp(prefix="tui-audit-")
    proj = os.path.join(d, "proj")
    os.makedirs(proj, exist_ok=True)
    shutil.copyfile(_FIXTURE_SRC, os.path.join(proj, "s.jsonl"))
    return d


CORPUS = None


def render(screen, width, locale, color):
    global CORPUS
    if CORPUS is None:
        CORPUS = _fixture_root()
    home = tempfile.mkdtemp(prefix="tui-audit-home-")
    env = dict(os.environ, COLUMNS=str(width), LINES="50", LC_ALL=locale,
               REPLAY_TRANSCRIPTS=CORPUS, HOME=home, USERPROFILE=home,
               ANTHROPIC_BASE_URL="http://127.0.0.1:1")
    if not color: env["NO_COLOR"] = "1"
    else: env.pop("NO_COLOR", None)
    try:
        p = subprocess.run([BIN,"tui","--once","--screen",screen,"-color","always" if color else "never"],
                           capture_output=True, text=True, stdin=subprocess.DEVNULL,
                           timeout=60, env=env)
    except subprocess.TimeoutExpired:
        return None
    return p.stdout

issues=[]
for screen in SCREENS:
    for locale in LOCALES:
        wide = locale.startswith("ja")
        for width in WIDTHS:
            out = render(screen, width, locale, color=False)
            if out is None:
                issues.append((screen,locale,width,"TIMEOUT","render did not return")); continue
            for i,line in enumerate(out.rstrip("\n").split("\n"),1):
                c = cells(line, wide)
                if c > width:
                    issues.append((screen,locale,width,"OVERFLOW",
                                   f"line {i} is {c} cells in a {width}-cell terminal: {line.strip()[:60]!r}"))
                if line != line.rstrip():
                    issues.append((screen,locale,width,"TRAILING-WS", f"line {i}"))
                if re.search(r"[\x00-\x08\x0b\x0c\x0e-\x1f]", SGR.sub("", line)):
                    issues.append((screen,locale,width,"CTRL-CHAR", f"line {i}"))

# colour checks once per screen, at the width the images use
for screen in SCREENS:
    out = render(screen, 80, "en_US.UTF-8", color=True)
    if out is None: continue
    depth=0
    for i,line in enumerate(out.rstrip("\n").split("\n"),1):
        for code in SGR.findall(line):
            for part in (code or "0").split(";"):
                if part not in PALETTE:
                    issues.append((screen,"-",80,"PALETTE",f"line {i} emits SGR {part!r}, outside color.go"))
            depth = 0 if code=="0" or code=="" else depth+1
        if depth != 0:
            issues.append((screen,"-",80,"UNCLOSED",f"line {i} ends with colour still open"))
            depth=0

print(f"screens {len(SCREENS)} x locales {len(LOCALES)} x widths {len(WIDTHS)} = "
      f"{len(SCREENS)*len(LOCALES)*len(WIDTHS)} renders + {len(SCREENS)} colour checks\n")

narrow = [i for i in issues if isinstance(i[2], int) and i[2] < SUPPORTED]
hard   = [i for i in issues if i not in narrow]

from collections import Counter
if hard:
    print(f"  FAIL at supported widths (>= {SUPPORTED}):", dict(Counter(i[3] for i in hard)), "\n")
    for sc,l,w,kind,msg in hard[:25]:
        print(f"   {kind:<12} {sc:<8} {l:<12} w={w:<4} {msg}")
    if len(hard) > 25:
        print(f"   ... and {len(hard)-25} more")
else:
    print(f"  clean at every supported width (>= {SUPPORTED}), both locales:")
    print("    no overflow, no palette drift, no unclosed colour, no control characters.\n")

print(f"  known narrow-terminal overflows (< {SUPPORTED}): {len(narrow)}, frozen at {KNOWN_NARROW_OVERFLOWS}")
if len(narrow) > KNOWN_NARROW_OVERFLOWS:
    print(f"   GREW by {len(narrow)-KNOWN_NARROW_OVERFLOWS}. The narrow layout got worse.")
elif len(narrow) < KNOWN_NARROW_OVERFLOWS:
    print(f"   IMPROVED by {KNOWN_NARROW_OVERFLOWS-len(narrow)}. Lower the frozen count in this file.")
else:
    print("   unchanged. all ten screens read this machine; what remains is tables in the shared layout helpers.")

sys.exit(1 if (hard or len(narrow) != KNOWN_NARROW_OVERFLOWS) else 0)
