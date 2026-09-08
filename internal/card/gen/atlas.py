#!/usr/bin/env /usr/bin/python3
"""Rasterise ASCII 32..126 into grayscale glyph atlases for internal/card.

This is a DEV-TIME tool. It is not run at build time, it is not imported by
anything in the module, and it contributes nothing to the binary's dependency
graph. Its *output* — three PNGs and one JSON sidecar — is committed next to
the Go source and embedded with //go:embed, which is how `replay` draws text
without a font library.

That indirection is the whole point. `go.mod` is three lines and there is no
`go.sum`, and "no dependencies" is a claim published on the README and on
redrobot.jp. A social card is not worth weakening it for, so the font rasteriser
runs here, on a developer's machine, once, and ships as pixels.

Run it with the interpreter that has PIL:

    /usr/bin/python3 internal/card/gen/atlas.py

Regenerating is only necessary if a face, the glyph range or the nominal size
changes. The committed output is the artefact; this file is the receipt for it.
"""

import json
import os
import sys

from PIL import Image, ImageDraw, ImageFont

# 128px nominal. Every size the cards use is smaller than this, so the Go side
# only ever downsamples — see resample.go for why that direction matters.
NOMINAL = 128
FIRST, LAST = 32, 126  # printable ASCII, 95 glyphs
PAD = 4  # slack around the ink so a rounded scale never clips a stem

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")

# Faces, in the order the cards ask for them. Each entry lists candidates so a
# missing font fails loudly here rather than silently substituting something
# that changes every coordinate on the card.
FACES = [
    ("bold", [
        ("/System/Library/Fonts/Supplemental/Arial Bold.ttf", 0),
        ("/System/Library/Fonts/HelveticaNeue.ttc", 1),
    ]),
    ("regular", [
        ("/System/Library/Fonts/HelveticaNeue.ttc", 0),
        ("/System/Library/Fonts/Supplemental/Arial.ttf", 0),
    ]),
    ("mono", [
        ("/System/Library/Fonts/Menlo.ttc", 0),
        ("/System/Library/Fonts/Supplemental/Courier New.ttf", 0),
    ]),
]


def load(candidates):
    for path, index in candidates:
        if not os.path.exists(path):
            continue
        try:
            return ImageFont.truetype(path, NOMINAL, index=index), path, index
        except Exception as err:  # a present-but-unreadable font is still a miss
            print("  skipping %s#%d: %s" % (path, index, err), file=sys.stderr)
    raise SystemExit(
        "no usable font among %s. Refusing to fall back to a default face: the "
        "card layout is hand-placed against specific metrics, and a substituted "
        "font would move every coordinate while still rendering something that "
        "looks plausible." % ([c[0] for c in candidates],)
    )


def build(name, candidates):
    font, path, index = load(candidates)
    chars = [chr(c) for c in range(FIRST, LAST + 1)]

    # PIL's default anchor is "la": x at the pen, y at the top of the ascender.
    # Every bbox below is relative to that point, which is also the origin the
    # Go drawing code positions glyphs against.
    boxes = [font.getbbox(ch) for ch in chars]
    inked = [b for b in boxes if b[2] > b[0] and b[3] > b[1]]
    min_x = min(b[0] for b in inked)
    max_x = max(b[2] for b in inked)
    min_y = min(b[1] for b in inked)
    max_y = max(b[3] for b in inked)

    origin_x = -int(min_x) + PAD if min_x < 0 else PAD
    origin_y = -int(min_y) + PAD if min_y < 0 else PAD
    cell_w = int(max_x) + origin_x + PAD + 1
    cell_h = int(max_y) + origin_y + PAD + 1

    atlas = Image.new("L", (cell_w * len(chars), cell_h), 0)
    draw = ImageDraw.Draw(atlas)
    for i, ch in enumerate(chars):
        draw.text((i * cell_w + origin_x, origin_y), ch, font=font, fill=255)

    png = os.path.join(OUT, "atlas-%s.png" % name)
    atlas.save(png, optimize=True)

    return {
        "file": os.path.basename(png),
        "cellW": cell_w,
        "cellH": cell_h,
        # Where the pen sits inside a cell. Subtracting these (scaled) from the
        # draw position is what makes glyphs with negative left bearings, and
        # descenders, land where the layout says they do.
        "originX": origin_x,
        "originY": origin_y,
        # Fractional on purpose: rounding an advance per glyph accumulates into
        # visibly wrong word spacing over a 58-character install line.
        "advances": [round(font.getlength(ch), 4) for ch in chars],
        "source": "%s#%d" % (path, index),
    }, png


def main():
    out = {"nominal": NOMINAL, "first": FIRST, "last": LAST, "faces": {}}
    for name, candidates in FACES:
        meta, png = build(name, candidates)
        out["faces"][name] = meta
        print("%-8s %s  %dx%d cells  %d bytes"
              % (name, meta["source"], meta["cellW"], meta["cellH"], os.path.getsize(png)))
    side = os.path.join(OUT, "atlas.json")
    with open(side, "w") as fh:
        json.dump(out, fh, indent=1, sort_keys=True)
        fh.write("\n")
    print("wrote", side)


if __name__ == "__main__":
    main()
