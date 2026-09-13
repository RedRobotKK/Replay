package tui

import (
	"os"
	"strings"
)

// The semantic layer named in layout.go as TierMeaning: "status words, notes,
// figures. ASCII, with colour carrying meaning that is also readable without
// it." It was specified there and not built, so until now every screen was one
// undifferentiated block in which the figure that costs money and the figure
// that is fine looked the same.
//
// Two rules govern everything here, and both exist because of how this fails.
//
// Colour is never the only carrier. Every distinction it draws is already in
// the words: "re-billed" says what it is, a break count is a number, the
// selected row already has a ">" in the gutter. Roughly one man in twelve
// cannot separate red from green, terminals are themed to taste, and this
// output gets piped into files where colour does not exist at all. A screen
// that needs its colours is a screen that has already failed for those readers.
//
// Colour occupies no cells. An SGR sequence is bytes in a string that the
// terminal consumes, so a field coloured BEFORE it is padded shifts every
// column after it, and the machine that wrote it cannot see the damage because
// its terminal acts on the bytes rather than showing them. Paint after padding,
// always. TestCL1 enforces this by stripping the escapes and requiring the
// plain render back, byte for byte.

// Style is a role in the reading, not a colour.
//
// Naming these after what they mean rather than after what they look like is
// what lets the palette be re-chosen once, here, without auditing every screen
// for the word "red".
type Style uint8

// The roles. Ordered by how loud they are.
const (
	// Plain is unstyled body text: the default, and most of every screen.
	Plain Style = iota
	// Faint is chrome the reader should be able to look past: rules, the
	// footer, key hints, provenance notes. Lowering these is what makes the
	// figures rise without shouting.
	Faint
	// Strong is the answer to the screen's question, and there is one per
	// screen. Bold rather than coloured, so it survives a monochrome
	// terminal and any theme.
	Strong
	// Accent marks the thing the reader is pointing at: the selected row,
	// the active tab. Position already says it; this makes it findable.
	Accent
	// Good is a measured absence of trouble. Used sparingly, because a
	// screen full of green trains the reader to ignore it.
	Good
	// Warn is worth a look and is not yet a problem.
	Warn
	// Alarm is money already spent twice, or a claim the tool cannot stand
	// behind. The loudest thing on a screen and the rarest.
	Alarm
)

// codes are 16-colour SGR bodies, chosen for the floor rather than the ceiling.
//
// "Usable in monochrome, readable at 16 colours, beautiful at true colour" is
// the layered rule this file inherits, and the 16-colour rung is where it has
// to be right: 256-colour and truecolour terminals render these faithfully,
// while a 16-colour one renders them at all. Nothing here assumes a background,
// because a palette that only works on black is a palette that fails on half
// the terminals in use.
var codes = map[Style]string{
	Faint:  "2",  // faint
	Strong: "1",  // bold, no hue: works where colour does not
	Accent: "36", // cyan
	Good:   "32", // green
	Warn:   "33", // yellow
	Alarm:  "31", // red
}

// PaletteCodes lists every SGR body the palette can emit, for the test that
// pins the set. Exported through the package's test rather than as API,
// because the point is that the colours live in one place.
func PaletteCodes() []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		out = append(out, c)
	}
	return out
}

// Painter decides whether colour is emitted, and paints when it is.
//
// A value rather than a package global, so a test can hold one of each kind at
// once and the answer never depends on the order tests ran in.
type Painter struct{ on bool }

// NoPaint is a painter that emits nothing. It is the zero value on purpose:
// forgetting to construct one degrades to plain text rather than to a panic or
// to stray escapes in a captured file.
var NoPaint = Painter{}

// NewPainter resolves the colour decision once, from the three inputs that get
// a vote, in the order they win.
//
//	NO_COLOR set, to anything at all, disables colour whatever else says.
//	  no-color.org's rule is that presence is enough, and a tool that honours
//	  it conditionally has not honoured it. It is an accessibility setting and
//	  it outranks a flag typed on one command line.
//	TERM=dumb means the terminal has told us it cannot.
//	mode is the user's request: "always", "never", or "auto".
//	  "auto", the default, means colour only when stdout is a terminal, so a
//	  pipe into a file, a README capture or an agent gets clean bytes.
func NewPainter(mode string, stdoutIsTerminal bool) Painter {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return NoPaint
	}
	if os.Getenv("TERM") == "dumb" {
		return NoPaint
	}
	switch mode {
	case "always":
		return Painter{on: true}
	case "never":
		return NoPaint
	default:
		return Painter{on: stdoutIsTerminal}
	}
}

// On reports whether this painter emits anything, for callers choosing between
// two layouts rather than two colours.
func (p Painter) On() bool { return p.on }

// Paint wraps s in the role's SGR, or returns it untouched.
//
// Pass a field that is ALREADY padded to its column width. Padding a painted
// string counts the escape bytes as characters and silently narrows the
// visible field.
//
// An empty string is returned unpainted: wrapping nothing in escapes emits
// bytes that render as nothing, which is how a "colourless" screen ends up
// failing an escape-free assertion.
func (p Painter) Paint(st Style, s string) string {
	code, ok := codes[st]
	if !p.on || !ok || s == "" {
		return s
	}
	// Surrounding whitespace is left outside the escapes.
	//
	// Not a nicety. The palette sets foreground only, so painting a space
	// changes nothing a reader can see, and leaving the padding inside the
	// sequence broke something real: a padded column ends in spaces, the
	// --once path trims trailing whitespace from every line, and a line
	// ending in "\x1b[0m" no longer looks like it ends in spaces. The trim
	// stopped firing and the header row kept 17 characters of padding that
	// the uncoloured render did not have.
	//
	// TestCL1 caught it, which is the whole reason that test compares renders
	// instead of counting columns.
	head := len(s) - len(strings.TrimLeft(s, " \t"))
	tail := len(s) - len(strings.TrimRight(s, " \t"))
	if head+tail >= len(s) {
		return s // all whitespace: nothing to colour
	}
	body := s[head : len(s)-tail]
	return s[:head] + "\x1b[" + code + "m" + body + "\x1b[0m" + s[len(s)-tail:]
}

// Severity maps a count to a role, so every screen agrees what "a lot" means.
//
// The thresholds are deliberately blunt and deliberately here rather than at
// each call site, because the alternative is four screens quietly disagreeing
// about when a number turns red. Zero is Good and not Plain: a measured zero
// is a result, and the reader should be able to see it is a result.
func Severity(n int) Style {
	switch {
	case n == 0:
		return Good
	case n < 10:
		return Warn
	default:
		return Alarm
	}
}

// StripSGR removes colour from a rendered screen.
//
// Only used by tests and by callers capturing a screen to a file. It is here
// rather than in a test so that the definition of "what this layer emits" and
// the definition of "how to undo it" cannot drift apart.
func StripSGR(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] == ';' || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// active is the painter the screens render with.
//
// A package-level value, and worth saying why rather than leaving it to be
// found. Painter is a value type so the decision can be constructed and tested
// in isolation, but every screen constructor takes a different argument list
// and none of them takes a painter. Threading one through all of them would be
// a larger and riskier change than the feature it carries. The sentence used to
// say how many constructors there are, which was one more thing to keep right
// and was already wrong.
//
// The command sets this once, at startup, from flags and environment, before
// any screen is built. Tests drive the real entry point, so they set it the
// same way a user does rather than reaching past it.
var active = NoPaint

// SetPainter installs the painter screens render with.
func SetPainter(p Painter) { active = p }

// Active is the painter screens render with.
func Active() Painter { return active }

// paint is the shorthand the screens use.
func paint(st Style, s string) string { return active.Paint(st, s) }
