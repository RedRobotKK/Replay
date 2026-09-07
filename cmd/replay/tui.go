package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
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
	// The doctor screen reads this machine; the rest are example data and say
	// so on screen. Wiring one at a time, and each one that lands moves a
	// screen from Example to Measured in a change that has to say which source
	// it now reads.
	m := machineState()
	src := func(k rune, _ int) tui.Frame {
		if k == 'd' {
			return tui.Frame{Key: k, Lines: tui.DoctorScreen(m).Lines}
		}
		return tui.Frame{Key: k, Lines: tui.Outcome(k).Lines}
	}

	if *once {
		// The footer the loop would have added, so a single frame is the same
		// frame either way.
		frame := src(key, 0).Lines
		lines := make([]string, 0, len(frame)+1)
		lines = append(lines, frame...)
		lines = append(lines, tui.Footer(key))
		for _, l := range lines {
			if _, err := fmt.Fprintln(stdout, l); err != nil {
				return err
			}
		}
		return nil
	}
	return tui.Start(stdout, src)
}

// machineState reads what the local filesystem can answer without a proxy.
//
// Everything here comes from the same helpers `replay doctor` uses, so the two
// commands cannot disagree about how much is on the machine. A screen that
// counted transcripts a second, subtly different way would be a second source
// of truth, and the point of the surface is that there is one.
func machineState() tui.Machine {
	m := tui.Machine{}

	home, err := os.UserHomeDir()
	if err != nil {
		return m
	}
	projects := filepath.Join(claudeConfigDir(home), "projects")
	if roots := defaultTranscriptRoots(home); len(roots) > 0 {
		projects = roots[0]
	}
	m.ProjectsDir = projects

	c := countTranscripts(projects)
	m.Transcripts, m.Lanes, m.Projects = c.sessions+c.lanes, c.lanes, c.projects
	m.Found = m.Transcripts > 0

	if dir, err := defaultLedgerDir(); err == nil {
		m.LedgerDir = dir
		m.LedgerWritable = writable(dir)
	}

	m.PriceTableDate = cachemodel.PriceTableVersion
	m.PriceAgeDays = daysSince(cachemodel.PriceTableVersion)

	m.Readings, m.Models = readingCounts(home)
	return m
}

// writable reports whether a directory can be written to, by asking rather
// than by checking mode bits: the permission that matters is the one the
// operating system will actually apply.
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".replay-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// daysSince ages a YYYY-MM-DD table version.
func daysSince(date string) int {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0
	}
	return int(time.Since(t).Hours() / 24)
}

// readingCounts reports how many probe readings exist and how many models they
// cover. One reading per model means no within-model variance at all, which the
// screen says rather than implies.
func readingCounts(home string) (readings, models int) {
	b, err := os.ReadFile(filepath.Join(home, ".replay", "measurements.jsonl"))
	if err != nil {
		return 0, 0
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		readings++
		var r struct {
			Model string `json:"model"`
		}
		if json.Unmarshal([]byte(line), &r) == nil && r.Model != "" {
			seen[r.Model] = true
		}
	}
	return readings, len(seen)
}
