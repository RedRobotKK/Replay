package card

import (
	"image"
	"image/color"
)

// resampleGray scales a grayscale glyph cell down to w x h by area averaging.
//
// The standard library has no scaler and this package may not import one, so
// this is the whole of it. It is an area average rather than a bilinear
// filter, and the difference matters here: the atlas master is 128px and the
// smallest text on a card is 18px, a ratio of about 7:1. Bilinear samples four
// source pixels per destination pixel regardless of the ratio, so at 7:1 it
// reads four pixels out of every forty-nine and throws the rest away — thin
// stems land between the samples and disappear, and the ones that survive
// alias into a stipple. Averaging the covered area instead is the same length
// of code and is what antialiasing a downsample actually means.
//
// Only downsampling is exercised: every size the cards use is below the 128px
// nominal. Upscaling here would give blocky nearest-neighbour output, which is
// the honest result of an area filter run backwards and a reason to regenerate
// the atlas larger rather than to reach for a different filter.
func resampleGray(src *image.Gray, w, h int) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 || b.Empty() {
		return dst
	}
	sx := float64(b.Dx()) / float64(w)
	sy := float64(b.Dy()) / float64(h)
	for y := 0; y < h; y++ {
		// The source band this destination row covers, in fractional pixels.
		y0, y1 := float64(y)*sy, float64(y+1)*sy
		for x := 0; x < w; x++ {
			x0, x1 := float64(x)*sx, float64(x+1)*sx
			var sum, weight float64
			for py := int(y0); py < int(y1)+1 && py < b.Dy(); py++ {
				// How much of this source row falls inside the band.
				wy := overlap(float64(py), float64(py+1), y0, y1)
				if wy <= 0 {
					continue
				}
				for px := int(x0); px < int(x1)+1 && px < b.Dx(); px++ {
					wx := overlap(float64(px), float64(px+1), x0, x1)
					if wx <= 0 {
						continue
					}
					sum += wy * wx * float64(src.GrayAt(b.Min.X+px, b.Min.Y+py).Y)
					weight += wy * wx
				}
			}
			if weight > 0 {
				// +0.5 rounds to nearest; truncation alone would darken every
				// glyph by half a level, which over a whole card reads as a
				// lighter, thinner face than the one in the atlas.
				dst.SetGray(x, y, color.Gray{Y: uint8(sum/weight + 0.5)})
			}
		}
	}
	return dst
}

// overlap is the length shared by two intervals on a line, or zero.
func overlap(a0, a1, b0, b1 float64) float64 {
	lo, hi := a0, a1
	if b0 > lo {
		lo = b0
	}
	if b1 < hi {
		hi = b1
	}
	if hi <= lo {
		return 0
	}
	return hi - lo
}
