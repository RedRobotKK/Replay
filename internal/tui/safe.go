package tui

import (
	"fmt"
	"sort"
	"strings"
)

// Privacy is everything Replay has written to this machine.
//
// Populated by the caller from the same store registry `replay privacy` walks,
// so a store added there appears here without anybody remembering to add it
// twice. Err carries a read failure, because an unreadable ~/.replay is not an
// empty one and "Replay has written nothing to this machine" is the most
// reassuring possible rendering of "I could not look".
type Privacy struct {
	Root   string
	Stores []Store
	Err    string
}

// Store is one thing on disk, and enough to decide what to do about it.
type Store struct {
	Name  string
	Bytes int64
	Files int
	// Unmeasured is how many entries under this store could not be walked.
	// Bytes is a total over what was readable, so without this a store that
	// nobody could open renders as the size of the part that was — or as
	// "0 B", which reads as a store known to be empty.
	Unmeasured int
	Sensitive  bool
	Purgeable  bool
}

// storeCols is this screen's table. Narrow enough for a 60-column terminal,
// for the reason armedCols is: docCols lays a row at 65 cells and a third
// table at that width would make the narrow layout worse.
var storeCols = []Column{{"store", 22}, {"size", 32}}

// maxStoreRows is how many stores fit above the notes. Twelve exist on a
// working machine and the screen holds seven, so the list is capped rather
// than trimmed by pad() — a row silently cut by the budget is a store the
// reader does not know is there.
const maxStoreRows = 7

// SafeScreen answers "Is my setup safe?"
//
// It used to render a trim byte-cap summary: what capping tool output would
// have saved. That is a token-savings question, and it was sitting under a key
// whose declared command is `serve --mask --mask-patterns`. Meanwhile `replay
// privacy` — which lists everything Replay has written here, marks what holds
// secrets, says what a retention window will not reach, and hands off to
// `purge` — had no screen at all.
//
// What it does NOT claim is masking coverage. The status endpoint does not
// publish masked or unmasked counts, so a screen reporting them would be
// inventing them. What is here is what the filesystem can answer.
func SafeScreen(p Privacy) Screen {
	if p.Err != "" {
		return unavailable("safe", "could not read "+shortPath(p.Root)+": "+p.Err,
			"An unreadable directory is not an empty one, so this screen will not "+
				"tell you the disk is clean.")
	}
	sc := Screen{Key: 's', Title: "safe", From: Measured}
	lines := screenHead("safe")

	if len(p.Stores) == 0 {
		lines = append(lines,
			"  "+paint(Good, "Replay has written nothing to this machine."), "",
			"  "+cell("looked in", 22)+shortPath(p.Root), "",
			"  notes",
			note(false, "nothing to purge, and nothing to disclose."))
		sc.Lines = padSafe(lines)
		return sc
	}

	// Sensitive first, then largest. Size alone put the vault last — it is 32
	// bytes — so on a real machine the one store that holds secrets rather
	// than counts about them fell past the row cap and the screen said "1
	// store(s) hold your secrets" without showing which. On a screen asking
	// whether the setup is safe, what a store holds outranks how big it is.
	rows := append([]Store(nil), p.Stores...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Sensitive != rows[j].Sensitive {
			return rows[i].Sensitive
		}
		return rows[i].Bytes > rows[j].Bytes
	})

	sensitive, unreachable := 0, 0
	for _, s := range p.Stores {
		if s.Sensitive {
			sensitive++
		}
		if !s.Purgeable {
			unreachable++
		}
	}

	lines = append(lines,
		"  Everything Replay holds under "+shortPath(p.Root), "",
		Row(storeCols, "store", "size"),
		paint(Faint, Row(storeCols, strings.Repeat("-", 22), strings.Repeat("-", 32))))

	shown := rows
	if len(shown) > maxStoreRows {
		shown = shown[:maxStoreRows]
	}
	for _, s := range shown {
		if s.Sensitive {
			// The mark goes in the name column rather than a column of its
			// own: a fourth column for a flag that is set once is a column of
			// blanks. Painted after the cell is padded, which is the rule the
			// whole colour layer turns on — an escape occupies no cells, and a
			// value coloured before padding shifts every column after it.
			lines = append(lines, StyledRow(storeCols, []Style{Alarm}, "! "+s.Name, storeSize(s)))
			continue
		}
		lines = append(lines, Row(storeCols, s.Name, storeSize(s)))
	}
	if n := len(rows) - len(shown); n > 0 {
		lines = append(lines, paint(Faint, Row(storeCols, "", fmt.Sprintf("+%d more in replay privacy", n))))
	}

	lines = append(lines, "", "  notes")
	if sensitive > 0 {
		lines = append(lines,
			note(true, fmt.Sprintf("%d store(s) hold your secrets rather than counts", sensitive)),
			"      about them. Nothing here leaves this machine.")
	}
	if unreachable > 0 {
		lines = append(lines,
			note(false, fmt.Sprintf("%d store(s) are not removed by a retention window;", unreachable)),
			"      deleting one is a decision, not a cleanup.")
	}
	lines = append(lines,
		note(false, "next: replay purge --older-than 30d, or --session <id>"))
	sc.Lines = padSafe(lines)
	return sc
}

// storeSize renders a store's footprint, and says nothing it did not measure.
//
// Bytes totals the entries the walk could read. A store with unreadable ones is
// therefore reported smaller than it is, and the gap is the reader's to know
// about: "0 B" for a directory nobody could open is the same false absence
// `replay privacy` was printing, one surface along.
func storeSize(s Store) string {
	size := humanBytes(s.Bytes)
	if s.Files > 1 {
		size = fmt.Sprintf("%s, %d files", size, s.Files)
	}
	if s.Unmeasured > 0 {
		size = fmt.Sprintf("%s, %d unread", size, s.Unmeasured)
	}
	return size
}

// humanBytes formats a size in the units `replay privacy` uses, so the screen
// and the command do not disagree about how big a thing is.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func padSafe(lines []string) []string {
	for len(lines) < bodyRows {
		lines = append(lines, "")
	}
	if len(lines) > bodyRows {
		lines = lines[:bodyRows]
	}
	return lines
}
