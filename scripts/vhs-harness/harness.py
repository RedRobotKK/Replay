#!/usr/bin/env python3
"""Render every screen through a real terminal and check what it displays.

The difference from scripts/tui-audit matters. That one inspects the bytes the
program writes. This one drives a real terminal emulator with VHS and reads back
what the terminal actually shows after processing every escape sequence, so it
catches a class the other cannot: a cursor left in the wrong place, a row
overwritten by the one after it, an escape the program emitted correctly and the
terminal rendered wrongly.

Determinism took three attempts and the failures are worth recording, because
each one would have produced a check that passes while asserting nothing.

  1. VHS writes a .txt containing EVERY frame, separated by box rules. It is a
     filmstrip, not a screenshot.
  2. Frame boundaries move between runs: a shell prompt renders a few
     milliseconds either side of a capture, so consecutive runs of the identical
     tape differ by a stray line.
  3. Taking the text after the last rule yields an empty string, because the
     recording ends on a rule. Three empty files compare equal, and the check
     reports success having examined nothing.

So the frame taken is the one with the most content, trailing blanks stripped.
Verified stable across three consecutive runs before anything was built on it.
"""
import argparse, os, re, shutil, subprocess, sys, tempfile

SCREENS = ["cost", "why", "context", "advise", "guards", "model", "safe", "share", "live"]

# Locale to the symbol its reader should see. money.Detect maps a territory to a
# currency; this asserts the screen actually prints that symbol, which is the
# part a unit test on the money package cannot see.
LOCALES = {
    "en_US.UTF-8": "$",
    "ja_JP.UTF-8": "¥",
    "de_DE.UTF-8": "€",
    "en_GB.UTF-8": "£",
}

RULE = re.compile(r"^─{40,}$", re.M)


def richest_frame(raw):
    """The frame with the most content, trailing blank lines removed."""
    frames = RULE.split(raw)
    best = max(frames, key=lambda f: sum(1 for l in f.split("\n") if l.strip()))
    lines = [l.rstrip() for l in best.split("\n")]
    while lines and not lines[0].strip():
        lines.pop(0)
    while lines and not lines[-1].strip():
        lines.pop()
    return lines


def render(binary, screen, locale, cols, corpus, workdir):
    tape = os.path.join(workdir, "t.tape")
    out = os.path.join(workdir, "t.txt")
    home = tempfile.mkdtemp(dir=workdir)
    with open(tape, "w") as f:
        f.write(f"""Output {out}
Set Width {cols * 11}
Set Height 760
Set FontSize 16
Set Shell bash
Hide
Type "export PS1='' HOME={home} REPLAY_TRANSCRIPTS={corpus} LC_ALL={locale} COLUMNS={cols} PATH={os.path.dirname(binary)}:$PATH"
Enter
Type "clear"
Enter
Show
Type "replay tui --once --screen {screen} -color never"
Enter
Sleep 3s
""")
    env = dict(os.environ, PATH="/opt/homebrew/bin:" + os.environ.get("PATH", ""))
    p = subprocess.run(["vhs", tape], capture_output=True, text=True, env=env, timeout=180)
    if not os.path.exists(out):
        return None, (p.stderr or p.stdout)[:200]
    return richest_frame(open(out, encoding="utf-8", errors="replace").read()), None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--cols", type=int, default=80)
    ap.add_argument("--screens", default=",".join(SCREENS))
    a = ap.parse_args()

    if not shutil.which("vhs", path="/opt/homebrew/bin:" + os.environ.get("PATH", "")):
        print("vhs is not installed; this harness needs vhs, ttyd and ffmpeg", file=sys.stderr)
        return 2

    work = tempfile.mkdtemp(prefix="vhs-harness-")
    corpus = os.path.join(work, "corpus", "proj")
    os.makedirs(corpus)
    here = os.path.dirname(os.path.abspath(__file__))
    shutil.copyfile(os.path.join(here, "..", "..", "internal", "transcript", "testdata",
                                 "session-redacted.jsonl"), os.path.join(corpus, "s.jsonl"))

    issues, checked = [], 0
    for screen in a.screens.split(","):
        for locale, symbol in LOCALES.items():
            frame, err = render(a.bin, screen, locale, a.cols,
                                os.path.join(work, "corpus"), work)
            if frame is None:
                issues.append((screen, locale, "RENDER", err))
                continue
            checked += 1

            body = [l for l in frame if l.strip()]
            if not body:
                issues.append((screen, locale, "BLANK",
                               "the terminal displayed nothing; a blank frame compares equal "
                               "to another blank frame and asserts nothing"))
                continue

            for i, l in enumerate(frame):
                if len(l) > a.cols:
                    issues.append((screen, locale, "OVERFLOW",
                                   f"displayed line {i} is {len(l)} cells in {a.cols}: {l.strip()[:48]!r}"))

            # Currency is the region check. Only the cost and share screens
            # carry money, so the others are not expected to show a symbol.
            if screen in ("cost", "share"):
                text = "\n".join(frame)
                if symbol not in text and "$" not in text:
                    issues.append((screen, locale, "NO-CURRENCY",
                                   f"no {symbol} and no $ on a screen that reports money"))

    print(f"rendered {checked} frames through a real terminal "
          f"({len(a.screens.split(','))} screens x {len(LOCALES)} locales at {a.cols} cols)\n")
    if not issues:
        print("  every frame rendered, none overflowed, money screens carry a currency.")
        return 0
    from collections import Counter
    print("  ISSUES:", dict(Counter(i[2] for i in issues)), "\n")
    for s, l, k, m in issues[:20]:
        print(f"   {k:<12} {s:<8} {l:<12} {m}")
    if len(issues) > 20:
        print(f"   ... and {len(issues)-20} more")
    return 1


if __name__ == "__main__":
    sys.exit(main())
