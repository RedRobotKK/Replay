package tui

import (
	"strings"

	"github.com/RedRobotKK/Replay/internal/card"
)

// The share screen.
//
// It answers a question none of the others do: is any of this worth telling
// anyone. `replay cost` says what the corpus cost; this says what a card of it
// would carry, in the words it would carry them, before anything is written to
// disk and long before anything is posted.
//
// One property is load-bearing and everything here is arranged around it. The
// preview and the file come from one card.Data and one card.Say, so the screen
// cannot show a figure the file does not have. A preview that can disagree with
// the thing it previews is worse than no preview: it is a second source of
// truth for something the reader is about to post under their own name.
//
// It is also the one screen with a key that touches the filesystem, so it says
// the absolute path afterwards. A side effect nobody can see is a side effect
// nobody can undo.

// ShareState is everything the screen draws from.
//
// Populated by the caller, because this package does no I/O. The write happens
// on the other side of that boundary and comes back as Wrote or Failed.
type ShareState struct {
	// Data is the card's figures. The same value is handed to the renderer
	// when w is pressed, which is what makes the preview binding.
	Data card.Data
	// Ready is false when nothing was measured well enough to stand behind. It
	// is the same condition `replay cost --share` refuses on, decided by the
	// same function, so the screen and the command cannot disagree about what
	// is postable.
	Ready bool
	// Tasks is the priced row count, said out loud when Ready is false: a
	// refusal that does not name its own reason is a refusal nobody can act on.
	Tasks int

	Variant card.Variant
	Tone    card.Tone

	// Wrote is the absolute path of the last file written, and Failed what
	// went wrong instead. Both empty until w is pressed.
	Wrote  string
	Failed string
}

// previewWidth is the room inside the frame drawn around the card's words.
//
// Seventy-two, so the frame and its two-space indent land on 78 of the eighty
// columns. The longest thing a card carries is the install line at 39, so
// nothing real is truncated; the cap is there for a corpus with a ten-digit
// figure in it, where a silent overrun would be the screen shearing rather
// than the card being wrong.
const previewWidth = 72

// ShareScreen renders the preview, or says why there is nothing to preview.
func ShareScreen(s ShareState) Screen {
	head := screenHead("share")
	lines := make([]string, 0, BudgetRows)
	lines = append(lines, head...)

	if !s.Ready {
		lines = append(lines,
			"  "+commas(s.Tasks)+" priced sessions. Nothing measured enough to stand behind.",
			"  There is no card to preview, and w will not write one.", "",
			"  "+cell("the same guard", 22)+"replay cost --share refuses here too",
			"  "+cell("why", 22)+"a card of zeros reads as a finding",
			"", "  notes",
			note(false, "point it at a corpus with priced traffic in it, or run the"),
			"      agent for a while and come back.")
		lines = WithBanner(lines, Unavailable, "nothing priced well enough to post")
		return Screen{Key: shareKey, Title: "share", Lines: padShare(lines, s), BodyRows: len(lines), From: Unavailable}
	}

	said := card.Say(s.Variant, s.Tone, s.Data)
	lines = append(lines,
		"  Design "+string(s.Variant)+", the "+string(s.Tone)+" register.",
		"  "+commas(s.Data.Tasks)+" sessions, "+commas(s.Data.Breaks)+
			" cache breaks. No paths, no spend total.",
		"")
	lines = append(lines, framed(said.Lines)...)
	lines = append(lines, "",
		"  t tone   d design   w write the png",
		"  "+Dim("d and w belong to this screen. c first, then d or w, for the others."))
	return Screen{Key: shareKey, Title: "share", Lines: padShare(lines, s), BodyRows: len(lines), From: Measured}
}

// framed draws the card's words inside an ASCII box.
//
// ASCII, like the rest of the surface: the obvious frame is box drawing, and
// box drawing is East Asian Ambiguous, one cell in a Latin locale and two in
// the ja_JP terminal this is read in.
func framed(body []string) []string {
	edge := "  +" + strings.Repeat("-", previewWidth+2) + "+"
	out := make([]string, 0, len(body)+2)
	out = append(out, edge)
	for _, l := range body {
		if len(l) > previewWidth {
			l = l[:previewWidth-1] + string(truncationMark)
		}
		out = append(out, "  | "+l+spaces(previewWidth-len(l))+" |")
	}
	return append(out, edge)
}

// tailOf keeps the end of a path when the whole of it will not fit.
//
// The end, not the beginning. Every other truncation on this surface cuts the
// tail, because the tail is usually the least of it; here the tail is the file
// name, which is the one thing a reader needs in order to go and find what the
// keystroke just wrote. A directory they can infer.
func tailOf(p string, w int) string {
	if w <= 1 || len(p) <= w {
		return p
	}
	return string(truncationMark) + p[len(p)-(w-1):]
}

// padShare fills to the budget, states what the last write did, and names the
// command that would produce the same file from a shell.
func padShare(lines []string, s ShareState) []string {
	switch {
	case s.Failed != "":
		lines = append(lines, "  "+truncate("could not write it: "+s.Failed, Cols()-2))
	case s.Wrote != "":
		lines = append(lines, "  wrote "+tailOf(shortPath(s.Wrote), Cols()-8))
	}
	for len(lines) < bodyRows()-3 {
		lines = append(lines, "")
	}
	if len(lines) > bodyRows()-3 {
		lines = lines[:bodyRows()-3]
	}
	ran := "  ran   replay cost --share --png <path>"
	if s.Ready {
		ran += " --card " + string(s.Variant) + " --tone " + string(s.Tone)
	}
	return append(lines, "", ran,
		"  "+Dim("copy it and you never need this screen again."))
}
