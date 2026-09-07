package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	// The corpus walk takes seconds and must not hold the first frame.
	//
	// Counting on the render path would hand the delay to the keyboard, which
	// is the one thing the design says never to do. It runs behind a mutex
	// instead and the cost screen shows its fields waiting until the answer
	// lands: the pinwheel is the difference between "nothing" and "not yet",
	// and zero dollars is a number somebody would believe.
	var mu sync.Mutex
	go func() {
		counted := costState(m)
		mu.Lock()
		m = counted
		mu.Unlock()
	}()

	src := func(k rune, tick int) tui.Frame {
		mu.Lock()
		cur := m
		mu.Unlock()
		switch k {
		case 'd':
			return tui.Frame{Key: k, Lines: tui.DoctorScreen(cur).Lines}
		case 'c':
			return tui.Frame{Key: k, Lines: tui.CostScreen(cur, tick).Lines}
		}
		return tui.Frame{Key: k, Lines: tui.Outcome(k).Lines}
	}

	if *once {
		// The footer the loop would have added, so a single frame is the same
		// frame either way.
		// --once waits for the count, because a single frame that says
		// "still counting" and exits has answered nothing.
		if key == 'c' {
			m = costState(m)
		}
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

// costState adds up the corpus, using the same summary the cost report prints.
//
// Same helpers, so the screen and `replay cost` cannot disagree about the
// total. A screen that computed its own figure a second, subtly different way
// would be a second source of truth, and the point of this surface is that
// there is one.
func costState(m tui.Machine) tui.Machine {
	if !m.Found {
		return m
	}
	// The cost report, run in process and read back as JSON.
	//
	// Not a reimplementation. The unit assembly lives inside runCost's own walk
	// and extracting it would refactor a working, tested command for no gain;
	// calling it means the screen and `replay cost` cannot disagree about the
	// total, which is the property that matters. A second, subtly different
	// walk producing a slightly different figure is exactly how the two counts
	// in `doctor` drifted apart once already.
	var buf bytes.Buffer
	if err := runCost([]string{"--json"}, &buf, io.Discard); err != nil {
		return m
	}
	var out struct {
		Summary struct {
			Tasks           int     `json:"tasks"`
			TotalUSD        float64 `json:"totalUsd"`
			MedianUSD       float64 `json:"medianUsd"`
			P90USD          float64 `json:"p90Usd"`
			AvoidableUSD    float64 `json:"avoidableUsd"`
			AvoidableShare  float64 `json:"avoidableShare"`
			AvoidableTokens int     `json:"avoidableTokens"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || out.Summary.Tasks == 0 {
		return m
	}
	sm := out.Summary
	m.CostReady = true
	m.Tasks = sm.Tasks
	m.TotalUSD, m.MedianUSD, m.P90USD = sm.TotalUSD, sm.MedianUSD, sm.P90USD
	m.AvoidableUSD, m.AvoidableShare = sm.AvoidableUSD, sm.AvoidableShare
	m.AvoidableTokens = sm.AvoidableTokens
	m.PriceDate = cachemodel.PriceTableVersion
	m.CorpusFiles = m.Transcripts
	return m
}
