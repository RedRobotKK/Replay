package tui

// The layout pattern, chosen rather than arrived at.
//
// Stefanie Jane names seven that successful terminal apps fall into, and warns
// that picking the wrong one is like picking the wrong data structure: every
// decision downstream gets harder. The first draft of this surface picked none,
// which is worse than picking badly, because nothing downstream could be
// checked against it.
//
// This is Header + Scrollable List, the htop and tig pattern: a fixed header
// carrying the answer and the summary, a scrollable body carrying the evidence
// for it, and a function bar at the bottom.
//
// It is the right one here for a reason particular to this tool. Every screen
// answers one question, and the answer is a figure with a table under it. That
// is exactly the shape the pattern was made for: "view a list of things with
// summary stats". The alternatives were considered and rejected:
//
//	Persistent multi-panel   would need several things worth watching at once.
//	                         There is one: the traffic log. The rest are answers
//	                         to questions, and an answer nobody asked is noise.
//	Miller columns           needs tree-shaped data. A cost report is not a tree.
//	Drill-down stack         k9s earns this with a real hierarchy. Ours is eight
//	                         sibling questions, not a nesting.
//	Widget dashboard         btop's case: many streams about one system. Ours is
//	                         one stream and eight ways of asking about it.
//	IDE three-panel          navigate, work, inspect. Nothing here is edited.
//	Overlay/popup            right for "?" and for confirmations, and it is used
//	                         for exactly those. Wrong for the screens themselves,
//	                         which need to persist while you read them.
//
// Spatial consistency follows from the choice: the header block is the same
// geometry on every screen, the table starts on the same row, and the footer is
// always the last line. A reader learns the shape once. That is not decoration,
// it is the reason TestStoryboard_TheHeaderBlockHasOneGeometryEverywhere exists,
// and the reason it caught state 1 putting its third column at 54 while state 2
// put it at 48.
const Pattern = "header + scrollable list"

// Glyph tiers.
//
// This corrects an over-generalisation rather than adding a feature. The design
// banned every non-ASCII character because box-drawing is East Asian Ambiguous
// and doubles in width in a CJK locale, which was measured and is true. Turning
// that into a prohibition threw away the third tier of "design in layers":
// usable in monochrome, readable at 16 colours, beautiful at true colour.
//
// The same discipline applies to glyphs. ASCII is the floor, and the floor is
// what everything is built on, so the frame elements stay ASCII and the tests
// enforcing that stay. What was wrong was concluding that nothing else may ever
// appear. A sparkline drawn from fractional blocks is a tier-three flourish over
// a tier-one frame, and it is only safe where the layout does not depend on its
// width.
const (
	// TierFrame is structure: rules, gutters, padding, column edges. ASCII
	// only, always, because every column position depends on it.
	TierFrame = iota
	// TierMeaning is the semantic layer: status words, notes, figures. ASCII,
	// with colour carrying meaning that is also readable without it.
	TierMeaning
	// TierFlourish is ornament that no layout depends on: sparklines, meters,
	// progress. May use Unicode blocks where the terminal reports it can, and
	// must degrade to ASCII rather than shear.
	TierFlourish
)

// Meter renders a proportion at the flourish tier, in ASCII.
//
// Eight steps of a block, done with characters that are one cell everywhere. It
// is uglier than the fractional-block version and it is the version that cannot
// break a column, which is the trade this surface makes on purpose.
func Meter(fraction float64, width int) string {
	if width <= 0 {
		return ""
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	filled := int(fraction*float64(width) + 0.5)
	out := make([]byte, 0, width)
	for i := 0; i < width; i++ {
		if i < filled {
			out = append(out, '#')
		} else {
			out = append(out, '.')
		}
	}
	return string(out)
}
