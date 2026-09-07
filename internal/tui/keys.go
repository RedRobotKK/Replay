package tui

import "strings"

// Keyboard interaction in four layers.
//
// Adopted from Stefanie Jane, "The Terminal Renaissance: Designing Beautiful
// TUIs in the Age of AI" (hyperbliss.tech, 2026-04-04), which is the closest
// thing this field has to a written design standard. Terminal developers,
// as she puts it, have had vibes.
//
// The first draft of this surface failed two of her principles outright.
//
// It put all eight shortcuts in the footer, which is the wall-of-shortcuts
// problem progressive disclosure exists to prevent: beginners should see a
// floor of three to five keys and find the ceiling through "?" on demand.
//
// And it invented eight mnemonics with nothing underneath them. j/k/h/l, "/",
// "?", g/G and Esc are the terminal lingua franca, and a surface that omits
// them asks its most fluent users to unlearn the vocabulary they already have.
// Those eight mnemonics are L2 keys; they were shipped with no L0 or L1.

// KeyLayer is how far down the vocabulary a binding sits.
type KeyLayer int

const (
	// L0 is universal: arrows, Enter, Esc, q. Anyone can use it, and it is
	// shown in the footer.
	L0 KeyLayer = iota
	// L1 is vim motion: j k h l / ? g G. Terminal natives expect it, and it is
	// shown in the footer.
	L1
	// L2 is single mnemonic actions, discovered through the help overlay
	// rather than the footer.
	L2
	// L3 is composed commands and configuration, documented and not displayed.
	L3
)

// Binding is one key and what it does at its layer.
type Binding struct {
	Keys  string
	Does  string
	Layer KeyLayer
}

// Bindings is the whole vocabulary, floor first.
func Bindings() []Binding {
	b := []Binding{
		{"enter", "open the row", L0},
		{"esc", "back, or cancel", L0},
		{"q", "quit", L0},

		// j/k and g/G are back, because there are rows now. They were removed
		// when nothing had rows to move between: a key listed in the footer is
		// a promise, and four inert ones in the help overlay are worse than
		// their absence.
		//
		// "/" is still absent. Searching a list needs a query line, a match
		// highlight and a way out of it, and none of that exists yet. It
		// returns the same way these did, when it works.
		{"j k", "down and up", L1},
		// H and G, not gg and G.
		//
		// The vim idiom for the top of a list is gg, a two-key prefix, and
		// bare g is already the guards screen. Binding g to "first row" made
		// one keystroke mean two things and the overlay listed both.
		//
		// H is vim's own High, the top of the screen, so this borrows the
		// vocabulary rather than inventing one. G stays as last, which is the
		// same key vim uses.
		{"H G", "first row and last", L1},
		{"?", "every key, including the ones not shown", L1},
	}
	for _, s := range Shortcuts() {
		b = append(b, Binding{string(s.Key), s.Question, L2})
	}
	return append(b,
		Binding{"--json", "the same answer for a machine, no screen at all", L3},
		Binding{"replay <cmd>", "the command each screen prints, run directly", L3},
	)
}

// footerKeys is the floor: what a first-time user sees without asking.
//
// Five, which is the top of the range the principle allows. Four of them are
// L0 or L1 and one is the escape hatch to everything else, because a footer
// that lists actions has already stopped being a floor.
func footerKeys() []Binding {
	// Three, not five.
	//
	// The floor is three to five keys, and it was five by listing "j k move"
	// and "enter open" while neither did anything. A footer that advertises a
	// key it does not handle is lying in the one place a beginner looks, and
	// padding to the top of the allowed range with keys that do nothing is the
	// worst way to reach it.
	//
	// It grows back to five when row selection exists.
	return []Binding{
		{"?", "keys", L1},
		{"esc", "back", L0},
		{"q", "quit", L0},
	}
}

// Footer renders the always-visible strip.
//
// It names the current screen so the user knows where they are without
// counting, and then the floor. The eight questions moved behind "?", which is
// what progressive disclosure asks for and what the first draft got wrong.
func Footer(cur rune) string {
	var b strings.Builder
	for _, s := range Shortcuts() {
		if s.Key == cur {
			b.WriteString("  " + s.Label)
			break
		}
	}
	b.WriteString("   ")
	for i, k := range footerKeys() {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(k.Keys + " " + k.Does)
	}
	return b.String()
}

// Help is the "?" overlay: the full vocabulary, by layer, on demand.
//
// It has to fit the same twenty-four rows as everything else. The first version
// was twenty-eight, which would have meant the screen that explains the keys was
// itself the one screen you had to scroll to read. L0 and L1 sit in two columns
// because they are short and because they are learned together: the universal
// key and its vim equivalent are the same idea twice.
func Help() []string {
	byLayer := map[KeyLayer][]Binding{}
	for _, b := range Bindings() {
		byLayer[b.Layer] = append(byLayer[b.Layer], b)
	}
	out := []string{"  keys" + strings.Repeat(" ", BudgetCols-24) + "esc to close", ""}

	out = append(out, "  anywhere                        moving around")
	l0, l1 := byLayer[L0], byLayer[L1]
	for i := 0; i < len(l0) || i < len(l1); i++ {
		line := "  "
		if i < len(l0) {
			line += cell(l0[i].Keys, 10) + cell(l0[i].Does, 18)
		} else {
			line += strings.Repeat(" ", 28)
		}
		if i < len(l1) {
			line += cell(l1[i].Keys, 8) + l1[i].Does
		}
		out = append(out, strings.TrimRight(line, " "))
	}

	out = append(out, "", "  the questions")
	for _, b := range byLayer[L2] {
		out = append(out, "  "+cell(b.Keys, 8)+b.Does)
	}
	out = append(out, "", "  beyond the screen")
	for _, b := range byLayer[L3] {
		out = append(out, "  "+cell(b.Keys, 14)+b.Does)
	}
	return out
}
