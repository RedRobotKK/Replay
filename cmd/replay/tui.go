package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// runTUI opens the question-first surface.
//
// The eight questions are the whole point: most people will never type a flag,
// so the tool runs the command for them and every screen prints what it ran.
// See docs/TUI-FLAG-SURFACE.md for the classification and
// docs/DASHBOARD-DESIGN.md for the states.
func runTUI(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	screen := fs.String("screen", "cost", "which question to open on: "+
		"cost, why, context, advise, guards, model, safe, doctor")
	once := fs.Bool("once", false,
		"render one frame and exit, for a pipe or a screenshot")
	if err := fs.Parse(args); err != nil {
		return err
	}

	key := rune(0)
	for _, s := range tui.Shortcuts() {
		if s.Label == *screen {
			key = s.Key
		}
	}
	if key == 0 {
		return fmt.Errorf("no screen called %q. The eight are: cost, why, context, "+
			"advise, guards, model, safe, doctor: %w", *screen, errUsage)
	}

	// Every screen is rendered from the same source, so the loop has no
	// knowledge of what any question means. Swapping illustrative figures for
	// measured ones later changes this function and nothing else.
	src := func(k rune, _ int) tui.Frame {
		return tui.Frame{Key: k, Lines: tui.Outcome(k).Lines}
	}

	if *once {
		for _, l := range src(key, 0).Lines {
			if _, err := fmt.Fprintln(stdout, l); err != nil {
				return err
			}
		}
		return nil
	}
	return tui.Start(stdout, src)
}
