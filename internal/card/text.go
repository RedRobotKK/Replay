package card

import (
	"image"
	"image/color"
)

// canvas is the card being drawn. Every coordinate the layouts use is in final
// pixels, top-left origin, matching the prototype the designs were laid out in.
type canvas struct {
	img   *image.RGBA
	faces map[string]*face
}

func newCanvas(w, h int, bg color.RGBA) (*canvas, error) {
	f, err := loadFaces()
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := &canvas{img: img, faces: f}
	c.rect(0, 0, w, h, bg)
	return c, nil
}

// rect fills a half-open rectangle with an opaque colour.
func (c *canvas) rect(x0, y0, x1, y1 int, col color.RGBA) {
	col.A = 255
	b := c.img.Bounds()
	for y := max(y0, b.Min.Y); y < min(y1, b.Max.Y); y++ {
		for x := max(x0, b.Min.X); x < min(x1, b.Max.X); x++ {
			c.img.SetRGBA(x, y, col)
		}
	}
}

// outline draws a one-pixel border, the box the install line sits in.
func (c *canvas) outline(x0, y0, x1, y1 int, col color.RGBA) {
	c.rect(x0, y0, x1, y0+1, col)
	c.rect(x0, y1-1, x1, y1, col)
	c.rect(x0, y0, x0+1, y1, col)
	c.rect(x1-1, y0, x1, y1, col)
}

// text draws s with its pen at x and the top of the ascender at y, which is
// where PIL's default "la" anchor puts it. The prototype's coordinates are
// therefore usable unchanged, and that is the point: the layouts below are a
// port, and a port that has to re-derive every y offset is a redesign.
//
// Returns the pen position after the last glyph, so a run of differently
// styled words can be laid out without measuring twice.
func (c *canvas) text(x, y float64, s string, faceName string, size int, col color.RGBA) float64 {
	f := c.faces[faceName]
	if f == nil || size <= 0 {
		return x
	}
	cells := f.cells(size)
	scale := float64(size) / float64(f.nominal)
	ox := float64(f.meta.OriginX) * scale
	oy := float64(f.meta.OriginY) * scale
	pen := x
	for i := 0; i < len(s); i++ {
		gi := f.index(s[i])
		if gi < 0 {
			continue
		}
		// Round the placement per glyph but carry the pen as a float. The
		// alternative — rounding the advance — drifts by up to half a pixel a
		// character, which is three characters of error across an install line.
		c.blend(cells[gi], int(pen-ox+0.5), int(y-oy+0.5), col)
		pen += f.advance(s[i], size)
	}
	return pen
}

// width is what text would advance, without drawing. Used to centre and to
// right-align, and to check at build time that a line fits its box.
func (c *canvas) width(s string, faceName string, size int) float64 {
	f := c.faces[faceName]
	if f == nil {
		return 0
	}
	var w float64
	for i := 0; i < len(s); i++ {
		w += f.advance(s[i], size)
	}
	return w
}

// blend composites one glyph cell over the canvas, using the cell's gray value
// as coverage. Source-over with an opaque destination, which is every case
// here: the card has no transparency and never will, because the platforms
// that display it composite onto backgrounds nobody controls.
func (c *canvas) blend(cell *image.Gray, x, y int, col color.RGBA) {
	b := cell.Bounds()
	clip := c.img.Bounds()
	for cy := b.Min.Y; cy < b.Max.Y; cy++ {
		dy := y + cy - b.Min.Y
		if dy < clip.Min.Y || dy >= clip.Max.Y {
			continue
		}
		for cx := b.Min.X; cx < b.Max.X; cx++ {
			a := uint32(cell.GrayAt(cx, cy).Y)
			if a == 0 {
				continue
			}
			dx := x + cx - b.Min.X
			if dx < clip.Min.X || dx >= clip.Max.X {
				continue
			}
			dstc := c.img.RGBAAt(dx, dy)
			c.img.SetRGBA(dx, dy, color.RGBA{
				R: mix(dstc.R, col.R, a),
				G: mix(dstc.G, col.G, a),
				B: mix(dstc.B, col.B, a),
				A: 255,
			})
		}
	}
}

// mix blends src over dst at coverage a (0..255), in integer arithmetic so the
// result is bit-identical on every machine. The card has to render the same
// bytes twice, and a float path here would be the one place that is not
// obviously true.
func mix(dst, src uint8, a uint32) uint8 {
	return uint8((uint32(src)*a + uint32(dst)*(255-a) + 127) / 255)
}
