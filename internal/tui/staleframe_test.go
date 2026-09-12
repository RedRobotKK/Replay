package tui

import (
	"strings"
	"testing"
)

// Shrinking the terminal must erase the rows the frame no longer reaches.
//
// paint walks the NEW frame's lines and rewrites the ones that differ. Rows
// past the end of it are never visited. bodyRows() reads the terminal on every
// paint and there is no resize handler, so making the window shorter mid-run
// produces a shorter frame and leaves the bottom of the previous one on screen,
// underneath the new one, reading as part of it.
//
// This was unreachable while the frame was BudgetRows: every paint produced the
// same height, so len(lines) never shrank. #282 made the frame follow the
// terminal, which turned a latent branch into a live defect. It is the failure
// the per-line clear in paint already guards against, one dimension up.
func TestLoop_ShrinkingTheTerminalErasesWhatTheFrameNoLongerCovers(t *testing.T) {
	var out strings.Builder
	src := func(rune, int) Frame {
		return Frame{Lines: []string{"  the answer"}}
	}
	l := &Loop{Out: &out, Source: src, Addressable: true}

	t.Setenv("LINES", "50")
	l.paint()
	tall := bodyRows() + 1

	out.Reset()
	t.Setenv("LINES", "24")
	l.paint()
	short := bodyRows() + 1

	if short >= tall {
		t.Fatalf("the frame did not shrink: %d rows then %d", tall, short)
	}
	got := out.String()
	for row := short + 1; row <= tall; row++ {
		want := "\x1b[" + itoa(row) + ";1H\x1b[2K"
		if !strings.Contains(got, want) {
			t.Fatalf("row %d was left untouched when the frame shrank from %d rows to %d. "+
				"The reader keeps the bottom of the previous answer below the new one, "+
				"with nothing on screen to say it is stale.", row, tall, short)
		}
	}
}
