package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
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

	var loop *tui.Loop
	// opened is the session the reader pressed enter on, and what the why
	// screen answers about. Nil until they choose one: a screen that picked a
	// session for them would be answering a question nobody asked.
	var opened *tui.Task

	src := func(k rune, tick int) tui.Frame {
		mu.Lock()
		cur := m
		mu.Unlock()

		switch k {
		case 'd':
			sc := tui.DoctorScreen(cur)
			loop.SetRows(sc.Rows)
			return tui.Frame{Key: k, Lines: sc.Lines}
		case 'c':
			sc := tui.CostScreen(cur, tick, loop.Cursor())
			loop.SetRows(sc.Rows)
			if loop.TakeOpened() {
				if t := taskAt(cur.TaskRows, loop.Cursor().At); t != nil && t.Path != "" {
					opened = t
				}
			}
			return tui.Frame{Key: k, Lines: sc.Lines}
		case 'w':
			loop.SetRows(0)
			return tui.Frame{Key: k, Lines: tui.WhyScreen(opened, blameFor).Lines}
		}
		loop.SetRows(0)
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
		loop = &tui.Loop{}
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
	return tui.StartWith(stdout, src, &loop)
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
	if err := runCost([]string{"--per-task", "--json"}, &buf, io.Discard); err != nil {
		return m
	}
	var out struct {
		Tasks []struct {
			Session      string  `json:"session"`
			Model        string  `json:"model"`
			Requests     int     `json:"requests"`
			CostUSD      float64 `json:"costUsd"`
			AvoidableUSD float64 `json:"avoidableUsd"`
			Breaks       int     `json:"breaks"`
		} `json:"tasks"`
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

	// Most expensive first: the question the list answers is "where did the
	// money go", and the answer is almost never the most recent task.
	sort.Slice(out.Tasks, func(i, j int) bool {
		return out.Tasks[i].CostUSD > out.Tasks[j].CostUSD
	})
	index := transcriptIndex(m.ProjectsDir)
	for _, t := range out.Tasks {
		m.TaskRows = append(m.TaskRows, tui.Task{
			Session: t.Session, Model: t.Model, CostUSD: t.CostUSD,
			Breaks: t.Breaks, Requests: t.Requests, Avoidable: t.AvoidableUSD,
			Path: index[t.Session],
		})
	}
	return m
}

// transcriptIndex maps an eight-character session prefix to its transcript.
//
// The cost report identifies a task by prefix, which is enough for a human to
// recognise and not enough to open. Building the map once beats globbing per
// row: a corpus of 1,614 files against 1,599 tasks would otherwise be one walk
// per keystroke.
//
// A prefix that matches two files is dropped rather than guessed. Opening the
// wrong session and saying nothing is worse than saying it cannot be opened,
// and the screen already renders a row that has no path as one that cannot be
// opened.
func transcriptIndex(projects string) map[string]string {
	index := map[string]string{}
	ambiguous := map[string]bool{}
	_ = filepath.WalkDir(projects, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		name := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		if len(name) < 8 {
			return nil
		}
		key := name[:8]
		if _, seen := index[key]; seen {
			ambiguous[key] = true
			return nil
		}
		index[key] = path
		return nil
	})
	for k := range ambiguous {
		delete(index, k)
	}
	return index
}

// taskAt returns the task under the cursor, or nil.
func taskAt(tasks []tui.Task, at int) *tui.Task {
	if at < 0 || at >= len(tasks) {
		return nil
	}
	return &tasks[at]
}

// blameFor runs the real blame report for one session.
//
// The same command the screen names, run in process, so what the screen shows
// and what `replay blame <path>` prints cannot differ.
func blameFor(path string) (string, error) {
	var buf bytes.Buffer
	// The same call main.go makes for `replay blame`, so what the screen shows
	// and what the command prints cannot differ.
	err := runReport("blame", []string{path}, &buf, io.Discard,
		func(r *analysis.LaneReport, w io.Writer) error {
			return r.WriteBlame(w, defaultBlameLimit)
		})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
