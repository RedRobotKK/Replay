package card

import (
	"image"
	"image/color"
)

// Finding text inside a rendered card.
//
// Asserting on the constant a layout was given proves the constant, not the
// picture. A renderer that ignores its argument, or draws the install line off
// the bottom edge, or draws it in the background colour, satisfies a string
// check and ships a card nobody can install from.
//
// So the needle is rasterised through the same path the card uses, reduced to
// where the ink is, and looked for in the card reduced the same way. What is
// compared is the shape of the coverage, not the colour: the install line is
// light on dark in one design and dark on paper in the other, and a check that
// only worked for one of them would go quiet on the other exactly when the
// other broke.
//
// Two details are load-bearing. Advances are fractional, so a needle stamped
// at pen x=20 and the same text drawn on the card at x=92.0 + some accumulated
// fraction are antialiased differently; the needle is therefore stamped at
// several sub-pixel phases and any of them may match. And the bar is a
// proportion of the needle's ink rather than an exact compare, because those
// phases never align perfectly — which is why the discriminating needle has to
// be SHORT. One character of a 58-character line is 1.5% of its ink, below any
// usable tolerance, so `v=b` and `v=c` are told apart on the fragment that
// differs, not on the whole line.

const stampPhases = 4

// installStamps rasterises s in the face and size the given variant sets its
// install line in, at every sub-pixel phase, as ink masks.
func installStamps(v Variant, s string) ([][][]bool, error) {
	_ = v // both designs set the install line at the same size
	size := installSize
	out := make([][][]bool, 0, stampPhases)
	for p := 0; p < stampPhases; p++ {
		c, err := newCanvas(1200, 60, color.RGBA{0, 0, 0, 255})
		if err != nil {
			return nil, err
		}
		c.text(20+float64(p)/stampPhases, 12, s, faceMono, size, color.RGBA{255, 255, 255, 255})
		if m := trimmedMask(c.img, func(p color.RGBA) bool { return p.R > 128 }); m != nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// trimmedMask reduces an image to "is there ink here", by the caller's
// definition of ink, trimmed to the bounding box of that ink.
func trimmedMask(img *image.RGBA, inked func(color.RGBA) bool) [][]bool {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if inked(img.RGBAAt(x, y)) {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x+1), max(maxY, y+1)
			}
		}
	}
	if maxX <= minX || maxY <= minY {
		return nil
	}
	out := make([][]bool, maxY-minY)
	for y := range out {
		out[y] = make([]bool, maxX-minX)
		for x := range out[y] {
			out[y][x] = inked(img.RGBAAt(minX+x, minY+y))
		}
	}
	return out
}

// cardMasks renders the card as ink in both polarities, so one helper covers
// the dark design and the paper one.
func cardMasks(img *image.RGBA) [][][]bool {
	return [][][]bool{
		fullMask(img, func(p color.RGBA) bool { return luma(p) > 150 }), // light ink on dark
		fullMask(img, func(p color.RGBA) bool { return luma(p) < 110 }), // dark ink on paper
	}
}

// fullMask is trimmedMask without the trim, so coordinates still line up with
// the card.
func fullMask(img *image.RGBA, inked func(color.RGBA) bool) [][]bool {
	b := img.Bounds()
	out := make([][]bool, b.Dy())
	for y := range out {
		out[y] = make([]bool, b.Dx())
		for x := range out[y] {
			out[y][x] = inked(img.RGBAAt(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

// found reports whether any phase of the needle appears in any polarity of the
// card.
func found(hays, needles [][][]bool) bool {
	for _, n := range needles {
		for _, h := range hays {
			if matchAt(h, n) {
				return true
			}
		}
	}
	return false
}

// matchAt slides one needle over one haystack, scoring the overlap in both
// directions.
//
// Counting only "needle ink that is present in the card" is not enough, and the
// way it fails is specific: in Menlo the ink of `c` is very nearly a subset of
// the ink of `b`, because `b` is that same bowl with a stem added. A one-sided
// score therefore found `/c/c` inside a card that says `/c/b`, and the test
// that exists to tell the two arms apart said they were the same. Ink in the
// card that the needle does not have has to count against the match too, which
// is what the union in the denominator is for.
//
// The bar is 80% agreement. Two phases of the same string reach the high
// nineties; `/c/b` against `/c/c` scores in the sixties, because that extra
// stem is a large fraction of a five-character needle.
func matchAt(hay, needle [][]bool) bool {
	nh, nw := len(needle), len(needle[0])
	if nh > len(hay) || nw > len(hay[0]) {
		return false
	}
	// The needle's own ink count bounds how much disagreement a match can
	// survive: both <= needleInk, so at an 80% bar the disagreement can never
	// exceed a quarter of it. Bailing on that turns a scan of 750,000 offsets
	// from minutes into milliseconds, because nearly every offset is background
	// and fails within a few pixels.
	needleInk := 0
	for _, row := range needle {
		for _, v := range row {
			if v {
				needleInk++
			}
		}
	}
	if needleInk == 0 {
		return false
	}
	slack := needleInk / 4

	// A summed-area table of the card's ink, so the count inside any candidate
	// window is four lookups. Without it this scan is 750,000 windows of
	// several thousand pixels each and takes twenty seconds; almost all of
	// those windows are blank margin, and the count alone rules them out.
	sat := summedInk(hay)
	for oy := 0; oy+nh <= len(hay); oy++ {
		for ox := 0; ox+nw <= len(hay[0]); ox++ {
			// |A| and |B| can differ by at most the disagreement between them,
			// so a window whose ink count is too far from the needle's cannot
			// clear the bar however it is arranged.
			hayInk := sat[oy+nh][ox+nw] - sat[oy][ox+nw] - sat[oy+nh][ox] + sat[oy][ox]
			if hayInk-needleInk > slack || needleInk-hayInk > slack {
				continue
			}
			both, apart := 0, 0
		window:
			for y := 0; y < nh; y++ {
				for x := 0; x < nw; x++ {
					n, h := needle[y][x], hay[oy+y][ox+x]
					switch {
					case n && h:
						both++
					case n || h:
						apart++
						if apart > slack {
							// This offset can no longer reach the bar.
							break window
						}
					}
				}
			}
			if apart <= slack && both*100 >= (both+apart)*80 {
				return true
			}
		}
	}
	return false
}

// summedInk is a summed-area table over an ink mask, one row and column
// larger than the mask so every window is four unconditional lookups.
func summedInk(m [][]bool) [][]int {
	h, w := len(m), len(m[0])
	sat := make([][]int, h+1)
	sat[0] = make([]int, w+1)
	for y := 0; y < h; y++ {
		sat[y+1] = make([]int, w+1)
		rowSum := 0
		for x := 0; x < w; x++ {
			if m[y][x] {
				rowSum++
			}
			sat[y+1][x+1] = sat[y][x+1] + rowSum
		}
	}
	return sat
}

func luma(p color.RGBA) int {
	return (299*int(p.R) + 587*int(p.G) + 114*int(p.B)) / 1000
}
