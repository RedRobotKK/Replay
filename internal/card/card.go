package card

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

// Variant is which of the two designs to render.
//
// Two exist so that the choice between them can be measured rather than
// argued about, which is also why the letter travels in the install URL. One
// card with no comparison is an opinion with a picture attached.
type Variant string

const (
	// VariantB is the receipt: a dark terminal frame, the tool's own output,
	// and the re-billed tokens as the headline.
	VariantB Variant = "b"
	// VariantC is the confession: paper, a statement in the first person, and
	// the figures demoted to a footnote under a rule.
	VariantC Variant = "c"
)

// Variants lists the arms, in a fixed order so that anything iterating them —
// a test, a batch render, the random assignment in cmd/replay — sees the same
// set in the same order every time.
func Variants() []Variant { return []Variant{VariantB, VariantC} }

// Width and Height are the Open Graph / summary_large_image size. Every
// service the card is posted to crops or re-encodes anything else, and what it
// crops first is the bottom, where the install line is.
const (
	Width  = 1200
	Height = 630
)

// Data is everything the card is allowed to know.
//
// Every field is a number. That is the privacy guarantee, and it is structural
// rather than a matter of care: there is no string field, so there is nowhere
// for a filesystem path, a project name or a model id to arrive. The card is
// posted in public and the guarantee has to hold for inputs nobody tested.
//
// It also carries no spend total, for the reason set out at the top of
// cmd/replay/share.go: a total tells a reader the poster's burn rate and lets
// them infer team size and runway. A rate reads the same from a solo developer
// and from a team of fifty, which is what makes it comparable, and comparison
// is the only mechanism by which a number like this travels.
type Data struct {
	// AvoidableShare is the fraction of priced spend that was re-billed, 0..1.
	AvoidableShare float64
	// Tasks is the row count and LaneUnit says what a row is, exactly as
	// costSummary carries them. The two travel together because a count under
	// the wrong noun is the defect that pair was introduced to prevent, and a
	// card posted publicly is the last place for a label one release behind
	// the figure beside it.
	Tasks    int
	LaneUnit bool
	Breaks   int

	MedianUSD float64
	P90USD    float64

	// PeakAvoidableTokens is the largest number of tokens re-billed within a
	// single row — one session, or one agent lane. A peak rather than the
	// corpus sum on purpose: the corpus sum divided by the avoidable rate
	// reconstructs an order-of-magnitude spend total, which is the one figure
	// the share card refuses to carry. A peak divided by the rate reconstructs
	// nothing, because a reader does not know how many rows there were.
	PeakAvoidableTokens int
}

// The palette. Fixed, and the same values the prototype was designed against.
var (
	paper  = color.RGBA{250, 250, 249, 255}
	ink    = color.RGBA{20, 20, 22, 255}
	red    = color.RGBA{212, 20, 36, 255}
	mute   = color.RGBA{139, 144, 154, 255}
	dark   = color.RGBA{18, 18, 20, 255}
	dim    = color.RGBA{107, 114, 128, 255}
	light  = color.RGBA{229, 231, 235, 255}
	hot    = color.RGBA{255, 95, 109, 255}
	rule   = color.RGBA{228, 226, 222, 255}
	border = color.RGBA{63, 67, 75, 255}
	redLit = color.RGBA{255, 110, 120, 255} // the wordmark on dark, where RED goes muddy
)

// InstallLine is the only thing on the card that does anything.
//
// It carries no query string, and that is the whole design of it. The obvious
// form was `curl -fsSL "https://redrobot.jp/replay.sh?src=card&v=b" | sh`, and
// every problem with it came from the two tracking parameters. zsh is the
// default shell on macOS and globs a bare ?, aborting with "no matches found"
// before curl ever runs, so the URL had to be quoted; the & meant the quotes
// could not be dropped either. That is 57 characters with four shell
// metacharacters in them, on an image that people retype from a photograph of
// a phone screen. Quoting solves the shell problem and creates a transcription
// problem, and both are caused by the attribution, not by the install.
//
// The path form has no metacharacter to protect, so it needs no quotes: 39
// characters, nothing but letters, slashes and one pipe. Attribution is
// unchanged, because /c/<arm> is resolved by the site rather than by a query
// the client has to carry. The arm letter is still the only thing that ties an
// install back to the design that was on screen.
//
// -fsSL stays. Without -f an HTML error page is piped into sh, and without -L a
// redirect ends the install; they are two characters each and they are the
// difference between a failure that says so and one that does something.
//
// TWO CORRECTIONS, 2026-09-13, and the second one mattered more.
//
// The host was redrobot.jp and the canonical install host is replay.doctor.
//
// And /c/<arm> was designed here and never created on either site. It returned
// 404 on redrobot.jp and on replay.doctor, so the one artifact in this project
// built to be posted in public carried an install command that did nothing. It
// now resolves, through public/_redirects on replay.doctor, as a 302 to
// /replay.sh?src=card-<arm> so the arm survives as attribution. The -L above is
// what makes that work, which is the second reason it stays.
//
// The /c/ segment is gone, and the reason is the layout rather than taste.
// replay.doctor is two characters longer than redrobot.jp, which took the line
// from 39 characters to 41 and from a 1175px right edge to 1232px on a 1200px
// card. TestNoCardOverrunsItsRightMargin caught it. The alternative was
// dropping the type size from 47px to about 44px, and the paragraph above
// argues at length that 47 is the largest that fits and that this line is the
// one thing on the card that does anything. So the path absorbed the two
// characters instead, and the line is 39 characters again at the same size.
//
// Single-letter paths at the site root are therefore a reserved namespace.
// There are two of them and they are cheap, but a future page called /b would
// break every card already posted.
//
// Nothing caught it. The install-host test added earlier the same day walks
// markdown, and this is Go, so the most public install line in the project sat
// outside the check written to protect install lines.
func InstallLine(v Variant) string {
	return "curl -fsSL https://replay.doctor/" + string(v) + " | sh"
}

// installSize is the type size of that line, and it is the number the rest of
// the layout is built around rather than the other way up.
//
// A 1200px card renders at about 375px in a mobile feed, a 3.2x downscale. The
// prototype set this line at 21px, which arrives at 6.5px on screen: visibly a
// command, not readably one, and a card whose call to action needs a tap to
// read has no call to action. 47px is the largest that fits — 39 characters at
// Menlo's 0.602em advance is 1092px against 1056px of content width, so the
// line runs to a 36px right margin rather than the 72px the rest of the card
// uses, which is the one asymmetry worth having.
//
// It is worth stating what this does NOT reach. 47px in Menlo is a 36px cap
// height, not the 48px a brief asked for; 48px of cap needs a 63px size and
// 1478px of advance, which does not fit on a 1200px card at any margin. 36px
// caps arrive at 11px on a 375px screen, which is about the cap height of
// ordinary body text in a browser, and that is the honest claim: legible
// without tapping, not large.
const installSize = 47

// Render draws the card. The returned image is 1200x630 and fully opaque.
func Render(v Variant, t Tone, d Data) (*image.RGBA, error) {
	switch v {
	case VariantB:
		return renderB(sayB(t, d))
	case VariantC:
		return renderC(sayC(t, d))
	default:
		return nil, fmt.Errorf("unknown card design %q: the designs are b and c", v)
	}
}

// Encode renders and writes a PNG.
func Encode(w io.Writer, v Variant, t Tone, d Data) error {
	img, err := Render(v, t, d)
	if err != nil {
		return err
	}
	// The default encoder settings, deliberately unset. These files get
	// regenerated on every run, and a reviewer needs identical input to
	// produce an identical file so that a design change is distinguishable
	// from encoder noise.
	return png.Encode(w, img)
}

// unit names what Tasks counts, matching the noun the text share card uses.
func (d Data) unit() string {
	if d.LaneUnit {
		return "agent lanes"
	}
	return "sessions"
}

func (d Data) unitSingular() string {
	if d.LaneUnit {
		return "agent lane"
	}
	return "session"
}

// rateText is the headline figure, and it refuses to round a finding away.
// The same three cases as the text card: a real fraction under one percent
// would print as "0%", which reports a measurement as nothing.
func (d Data) rateText() string {
	pct := d.AvoidableShare * 100
	switch {
	case pct == 0:
		return "0%"
	case pct < 1:
		return "<1%"
	default:
		return fmt.Sprintf("%.0f%%", pct)
	}
}

// shortTokens matches the formatting `replay cost` prints to the terminal. The
// card and the report are read side by side and a figure that is abbreviated
// two different ways reads as two different figures.
func shortTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// wordmark is the brand block both designs open with.
func (c *canvas) wordmark(x, y float64, col color.RGBA) {
	c.text(x, y, "REPLAY", faceBold, 22, col)
	c.text(x+112, y+3, "by RedRobot K.K.", faceRegular, 18, mute)
}

// renderB — the receipt.
//
// A dark terminal frame showing the tool's own summary, then the tokens as the
// headline. It argues by evidence: this is what the output looks like, here is
// what it found. Every line on it is a figure the command actually produced.
//
// The install line is set first and everything else is fitted around it. That
// inverts the prototype, where the command was a 21px footnote under the art,
// and it is the right way up: the picture exists to get the command read.
func renderB(s bScript) (*image.RGBA, error) {
	c, err := newCanvas(Width, Height, dark)
	if err != nil {
		return nil, err
	}
	c.rect(0, 0, 8, Height, red)
	c.wordmark(72, 46, redLit)

	c.text(72, 104, s.Prompt, faceMono, 22, dim)
	c.text(72, 146, s.Stats[0], faceMono, 26, light)
	c.text(72, 184, s.Stats[1], faceMono, 26, dim)

	// The figure sits beside the copy or above it, decided by measuring it
	// rather than by which tone asked.
	//
	// An abbreviated figure is four or five characters and leaves a column for
	// three lines of copy beside it. An exact one is twelve, and at 96px it
	// runs to x=552 — straight through copy set at x=370. Overlapping text
	// still renders, so the failure is silent, and it is not a rekt-only
	// failure: a measured card whose peak passes 100M abbreviates to "987.7M"
	// and overlaps in exactly the same way.
	if c.width(s.Figure, faceBold, 96) <= gutter {
		c.text(72, 240, s.Figure, faceBold, 96, hot)
		for i, leg := range s.Legs {
			col := light
			if i == len(s.Legs)-1 {
				col = dim
			}
			c.text(370, float64(250+44*i), leg, faceBold, 34, col)
		}
	} else {
		c.text(72, 220, s.Figure, faceBold, 76, hot)
		for i, leg := range s.Legs {
			col := light
			if i == len(s.Legs)-1 {
				col = dim
			}
			c.text(72, float64(306+38*i), leg, faceBold, 34, col)
		}
	}

	// The box is sized to the line rather than the line to the box: 1092px of
	// advance from x=72, plus even padding.
	c.outline(56, 420, 1180, 508, border)
	c.text(72, 432, s.Install, faceMono, installSize, light)

	c.text(72, 552, s.Foot, faceRegular, 20, dim)
	return c.img, nil
}

// gutter is how much room the copy column leaves for a figure set beside it:
// the copy starts at x=370, the figure at x=72, and 24px of air between them.
const gutter = 370 - 72 - 24

// renderC — the confession.
//
// Paper, a statement in the first person, and the figures demoted to a
// footnote under a rule. It argues by admission rather than by evidence, which
// is why the two are worth comparing: they are not the same card in two
// colourways, they are two different reasons to click.
func renderC(s cScript) (*image.RGBA, error) {
	c, err := newCanvas(Width, Height, paper)
	if err != nil {
		return nil, err
	}
	c.rect(0, 0, 8, Height, red)
	c.wordmark(72, 46, red)

	c.text(72, 118, s.Head[0], faceBold, 68, ink)
	c.text(72, 196, s.Head[1], faceBold, 68, ink)
	c.text(72, 284, s.Sub, faceRegular, 30, mute)

	c.rect(72, 348, 1129, 352, rule)

	// Laid out by measurement rather than at fixed stops: a four-digit session
	// count or the longer "agent lanes" noun would run one segment into the
	// next, and the failure is silent — overlapping text still renders.
	x := c.text(72, 378, s.Lead, faceBold, 56, red) + 20
	for _, seg := range s.Segs {
		x = c.text(x, 396, seg, faceRegular, 26, ink) + 36
	}

	c.text(72, 500, s.Install, faceMono, installSize, ink)
	return c.img, nil
}
