package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/advisor"
	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/card"
	"github.com/RedRobotKK/Replay/internal/money"
	"github.com/RedRobotKK/Replay/internal/proxy"
	"github.com/RedRobotKK/Replay/internal/transcript"
	"github.com/RedRobotKK/Replay/internal/tui"
)

// runTUI opens the question-first surface.
//
// The nine questions are the whole point: most people will never type a flag,
// so the tool runs the command for them and every screen prints what it ran.
// See docs/TUI-FLAG-SURFACE.md for the classification and
// docs/DASHBOARD-DESIGN.md for the states.
func runTUI(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	screen := fs.String("screen", "cost", "which question to open on: "+
		"cost, why, context, advise, guards, model, safe, doctor, share")
	once := fs.Bool("once", false,
		"render one frame and exit, for a pipe or a screenshot")
	// auto is the only default that is safe in both directions: colour when a
	// person is looking, clean bytes when the output is a file, a README
	// capture or another agent. NO_COLOR overrides all three values, because
	// it is an accessibility setting and not a preference.
	color := fs.String("color", "auto",
		"when to colour: auto (a terminal only), always, never. NO_COLOR always wins")
	// parseArgs, not fs.Parse: --help is a request, not a parse failure.
	// Calling fs.Parse directly sent usage to stderr with a non-zero exit and
	// a trailing "flag: help requested", which is the defect help.go exists to
	// prevent. It survived here because tui was written last.
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	switch *color {
	case "auto", "always", "never":
	default:
		return fmt.Errorf("-color must be auto, always or never, not %q: %w", *color, errUsage)
	}
	// Resolved once, before any screen is built, from the real stdout rather
	// than from the writer this function was handed: a test writes to a buffer
	// and still needs "auto" to mean what it means for a user.
	tui.SetPainter(tui.NewPainter(*color, tui.IsTerminal(os.Stdout)))
	// The reader's currency, resolved once from their locale, the same way and
	// from the same code as the cost report. Two surfaces showing the same
	// figures must not disagree about what currency they are in.
	fx := money.Detect(os.LookupEnv, time.Now())

	key := rune(0)
	for _, s := range tui.Shortcuts() {
		if s.Label == *screen {
			key = s.Key
		}
	}
	if key == 0 {
		return fmt.Errorf("no screen called %q. The nine are: cost, why, context, "+
			"advise, guards, model, safe, doctor, share: %w", *screen, errUsage)
	}

	// Every screen is rendered from the same source, so the loop has no
	// knowledge of what any question means. Swapping illustrative figures for
	// measured ones later changes this function and nothing else.
	// The doctor screen reads this machine; the rest are example data and say
	// so on screen. Wiring one at a time, and each one that lands moves a
	// screen from Example to Measured in a change that has to say which source
	// it now reads.
	m := machineState()
	m.FX = fx

	// The corpus walk takes seconds and must not hold the first frame.
	//
	// Counting on the render path would hand the delay to the keyboard, which
	// is the one thing the design says never to do. It runs behind a mutex
	// instead and the cost screen shows its fields waiting until the answer
	// lands: the pinwheel is the difference between "nothing" and "not yet",
	// and zero dollars is a number somebody would believe.
	var mu sync.Mutex
	// Only the interactive loop needs the count in the background. --once
	// computes it synchronously below because a single frame that says "still
	// counting" has answered nothing, so starting the goroutine here as well
	// gave two writers to m and only one of them took the mutex: the goroutine
	// at this line, and the --once branch further down, which assigned
	// costState(m) unguarded. The race detector reported it on every run of
	// TestTW1, which drives runTUI through --once.
	//
	// Not starting the goroutine is the fix rather than locking the second
	// write, because in --once mode its work is duplicated and then discarded.
	// Locking would have made the report go quiet while leaving two walks of
	// the corpus racing to write the same answer.
	if !*once {
		go func() {
			counted := costState(m)
			mu.Lock()
			m = counted
			mu.Unlock()
		}()
	}

	// Where the share screen writes, resolved once at startup. A path decided
	// per keystroke could change under the reader between the preview and the
	// file.
	shareDir, _ := os.Getwd()
	ui := shareUI{variant: card.VariantC, tone: card.ToneMeasured}
	shareK := screenKey("share")

	var loop *tui.Loop
	// opened is the session the reader pressed enter on, and what the why
	// screen answers about. Nil until they choose one: a screen that picked a
	// session for them would be answering a question nobody asked.
	var opened *tui.Task
	// adviseOpen is the finding enter was pressed on, and nil when the reader
	// is looking at the list. Esc clears it by returning to the list render.
	var adviseOpen *tui.AdviceRow
	var lastAdviseKey rune

	src := func(k rune, tick int) tui.Frame {
		mu.Lock()
		cur := m
		mu.Unlock()

		// Cleared on every render, so a screen's own keys cannot follow the
		// reader off it. Only the share screen sets one.
		loop.SetLocal(nil)

		switch k {
		case 'd':
			sc := tui.DoctorScreen(cur)
			loop.SetRows(sc.Rows)
			return tui.Frame{Key: k, Lines: sc.Lines}
		case 'x':
			sc := tui.ContextScreen(contextState())
			loop.SetRows(sc.Rows)
			return tui.Frame{Key: k, Lines: sc.Lines}
		case 'g':
			sc := tui.GuardsScreen(guardsState())
			loop.SetRows(sc.Rows)
			return tui.Frame{Key: k, Lines: sc.Lines}
		case 's':
			sc := tui.SafeScreen(safeState())
			loop.SetRows(sc.Rows)
			return tui.Frame{Key: k, Lines: sc.Lines}
		case 'm':
			sc := tui.ModelScreen(modelState())
			loop.SetRows(sc.Rows)
			return tui.Frame{Key: k, Lines: sc.Lines}
		case 'a':
			// Cached first. Recomputing took 7.7s of wall clock per keypress.
			var sc tui.Screen
			if rows, ids, n, at, ok := adviceFromCache(); ok {
				sel := loop.Cursor().At
				// Enter opens the evidence. The help line advertised this
				// before anything was behind it, which is the overpromise
				// pattern unwired-3-branches-and-docs.md catalogues, written
				// into a help string while cataloguing it.
				//
				// Read-and-clear, so one keystroke opens one finding, and the
				// detail is shown for exactly the render after the press.
				if loop.TakeOpened() && sel >= 0 && sel < len(rows) {
					adviseOpen = &rows[sel]
				} else if k != lastAdviseKey {
					adviseOpen = nil
				}
				lastAdviseKey = k
				if adviseOpen != nil {
					sc = tui.AdviceDetail(*adviseOpen)
					loop.SetRows(0)
				} else {
					sc = tui.AdviseScreenAt(rows, n, sel)
				}
				// Local keys, cleared on every render, so `a` and `x` cannot
				// follow the reader onto a screen where they mean something
				// else. This is the whole point of the screen: a finding the
				// reader has judged should stop being offered.
				loop.SetLocal(func(r rune) bool {
					var status advisor.Status
					switch r {
					case 'a':
						status = advisor.Applied
					case 'x':
						status = advisor.Dismissed
					default:
						return false
					}
					i := loop.Cursor().At
					if i < 0 || i >= len(ids) {
						return false
					}
					// A write that fails must not look like one that worked.
					// The next render re-reads the file, so a silent failure
					// would show the old status and the reader would press
					// the key again.
					return markAdvice(ids[i], status) == nil
				})
				// The age is not decoration. Advice read from disk without a
				// date is yesterday's answer wearing today's clothes.
				sc.Lines = append(sc.Lines, fmt.Sprintf(
					"  as of %s; `replay advise` to refresh",
					at.Local().Format("15:04 on 2 Jan")))
			} else {
				sc = tui.AdviseScreen(adviceState())
			}
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
		case 'l':
			loop.SetRows(0)
			return tui.Frame{Key: k, Lines: tui.LiveScreen(liveState(), time.Now()).Lines}
		case 'w':
			loop.SetRows(0)
			return tui.Frame{Key: k, Lines: tui.WhyScreen(opened, blameFor).Lines}
		case shareK:
			loop.SetRows(0)
			// The state that was previewed is the state w writes, captured
			// here rather than rebuilt at keystroke time. The file is then the
			// card that was on screen, which is the whole point of previewing
			// it.
			st := shareState(cur, ui)
			loop.SetLocal(func(k rune) bool { return ui.press(k, st, shareDir) })
			return tui.Frame{Key: k, Lines: tui.ShareScreen(st).Lines}
		}
		loop.SetRows(0)
		return tui.Frame{Key: k, Lines: tui.Outcome(k).Lines}
	}

	if *once {
		// The footer the loop would have added, so a single frame is the same
		// frame either way.
		// --once waits for the count, because a single frame that says
		// "still counting" and exits has answered nothing.
		if key == 'c' || key == shareK {
			m = costState(m)
		}
		loop = &tui.Loop{}
		frame := src(key, 0).Lines
		lines := make([]string, 0, len(frame)+1)
		lines = append(lines, frame...)
		lines = append(lines, tui.Footer(key))
		// One boundary, so prose is fitted once rather than in nine screens.
		//
		// tui.Fit wraps a sentence that overruns the terminal and carries its
		// indent onto the continuation. It leaves column layouts alone: this
		// point sees strings and cannot know which column a table could afford
		// to drop, which is what storyboard.go scene 25 specifies and where
		// that work belongs.
		//
		// At 80 or wider nothing here overruns, so the committed screen images
		// are untouched by this.
		cols := tui.Cols()
		for _, l := range lines {
			for _, fitted := range tui.Fit(l, cols) {
				// Right-trimmed, because this path is the pipe and the
				// screenshot. A live terminal pads a row to the column width so
				// the row it is overwriting disappears; down a pipe that padding
				// is invisible junk that lands in a README code block and in
				// every diff of it afterwards.
				if _, err := fmt.Fprintln(stdout, strings.TrimRight(fitted, " \t")); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return tui.StartWith(stdout, src, &loop, key)
}

// machineState reads what the local filesystem can answer without a proxy.
//
// Everything here comes from the same helpers `replay doctor` uses, so the two
// commands cannot disagree about how much is on the machine. A screen that
// counted transcripts a second, subtly different way would be a second source
// of truth, and the point of the surface is that there is one.
// corpusFiles resolves the transcripts every wired screen reads.
//
// One place, so four screens cannot disagree about what "this machine" means,
// and so the empty case is decided once. Returning no files is what makes a
// screen Unavailable rather than Example.
func corpusFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	roots := defaultTranscriptRoots(home)
	if len(roots) == 0 {
		return nil
	}
	files, err := transcriptFiles(roots)
	if err != nil {
		return nil
	}
	return files
}

// contextState attributes what entered the context, by tool.
func contextState() ([]tui.ContextRow, int) {
	files := corpusFiles()
	if len(files) == 0 {
		return nil, 0
	}
	totals := map[string]*tui.ContextRow{}
	sessions := 0
	_ = forEachSession(files, func(_ string, _ *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || rep == nil {
			return nil
		}
		sessions++
		for _, e := range analysis.EnteredContext(rep.Blame) {
			r, ok := totals[e.Label]
			if !ok {
				r = &tui.ContextRow{Label: e.Label}
				totals[e.Label] = r
			}
			r.Tokens += e.Tokens
			r.Occurrences += e.Occurrences
		}
		return nil
	})

	sum := 0
	for _, r := range totals {
		sum += r.Tokens
	}
	rows := make([]tui.ContextRow, 0, len(totals))
	for _, r := range totals {
		// Share is recomputed over the pooled total rather than averaged from
		// the per-session shares. A mean of shares weights a tiny session the
		// same as a long one, which is how a label that appeared once in a
		// short session outranks one that dominated a long one.
		if sum > 0 {
			r.Share = float64(r.Tokens) / float64(sum)
		}
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Tokens > rows[j].Tokens })
	if len(rows) > 12 {
		rows = rows[:12]
	}
	return rows, sessions
}

// guardsState draws spend caps from this machine's own session spread.
func guardsState() ([]string, int) {
	files := corpusFiles()
	if len(files) == 0 {
		return nil, 0
	}
	var usd, toks []float64
	sessions := 0
	_ = forEachSession(files, func(_ string, _ *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || rep == nil {
			return nil
		}
		sessions++
		if pols := rep.Policies(); len(pols) > 0 {
			usd = append(usd, pols[0].CostUSD)
			toks = append(toks, float64(pols[0].PromptTokens))
		}
		return nil
	})
	return guardAdviceLines(usd, toks), sessions
}

// safeState scores the default byte cap on tool results.
func safeState() (tui.TrimSummary, int) {
	files := corpusFiles()
	if len(files) == 0 {
		return tui.TrimSummary{}, 0
	}
	var total analysis.TrimPlan
	sessions := 0
	_ = forEachSession(files, func(_ string, _ *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || rep == nil {
			return nil
		}
		sessions++
		p := analysis.ScoreTrim(rep.Lane, rep.Fit, defaultTrimCap)
		total.Blocks += p.Blocks
		total.RemovedBytes += p.RemovedBytes
		total.RemovedPromptTokens += p.RemovedPromptTokens
		return nil
	})
	return tui.TrimSummary{CapBytes: defaultTrimCap, Blocks: total.Blocks,
		RemovedBytes: total.RemovedBytes, RemovedPromptTokens: total.RemovedPromptTokens}, sessions
}

// modelState reports what the corpus actually ran on.
//
// No target is passed. Naming a model the reader did not choose and pricing a
// switch to it would invent the question as well as the answer; the screen says
// which command asks it properly.
func modelState() (string, []tui.ModelRow, int) {
	files := corpusFiles()
	if len(files) == 0 {
		return "", nil, 0
	}
	turns := map[string]int{}
	sessions := 0
	_ = forEachSession(files, func(_ string, session *transcript.Session, _ *analysis.LaneReport, err error) error {
		if err != nil || session == nil {
			return nil
		}
		sessions++
		// Lanes then requests: the model is a property of the request, and a
		// session can carry several lanes with different ones when a sub-agent
		// runs on something cheaper.
		for _, ln := range session.Lanes {
			if ln == nil {
				continue
			}
			for _, rq := range ln.Requests {
				if rq != nil && rq.Model != "" {
					turns[rq.Model]++
				}
			}
		}
		return nil
	})
	sum := 0
	for _, n := range turns {
		sum += n
	}
	rows := make([]tui.ModelRow, 0, len(turns))
	for m, n := range turns {
		share := 0.0
		if sum > 0 {
			share = float64(n) / float64(sum)
		}
		rows = append(rows, tui.ModelRow{Model: m, Turns: n, Share: share})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Turns > rows[j].Turns })
	return "", rows, sessions
}

// adviceState runs the same analysis `replay advise` runs, for the screen.
//
// Here rather than in internal/tui because that package does no I/O: the
// boundary that keeps every frame testable without a disk is the reason this
// walk lives on the command side and hands over a flat slice.
//
// The session count is returned separately and is not len(rows). Zero rows from
// six sessions is a finding — nothing crossed a threshold — and zero rows from
// zero sessions is an absence. Collapsing them would put "example data" back on
// a screen that simply had nothing to rank.
// adviceFromCache reads the advice `replay advise` last wrote.
//
// The screen re-ran the whole corpus analysis on every keypress: 7.7 seconds of
// wall clock and 24.5 of CPU across 1,738 transcripts, to draw four rows. A TUI
// that takes eight seconds to answer a keystroke is not slow, it is broken —
// and the answer was already on disk, because `replay advise` writes
// advice.json every time it runs.
//
// Freshness is the whole risk, so the caller shows the timestamp rather than
// presenting yesterday's advice as today's. A cache that cannot say how old it
// is would be worse than the delay it saves.
func adviceFromCache() ([]tui.AdviceRow, []string, int, time.Time, bool) {
	b, err := os.ReadFile(filepath.Join(tipStateDir(), adviceFileName))
	if err != nil {
		return nil, nil, 0, time.Time{}, false
	}
	var f adviceFile
	if json.Unmarshal(b, &f) != nil || f.Schema != advisor.AdviceFileSchema {
		// A file this build does not understand is not advice. Recompute
		// rather than render fields that may have moved.
		return nil, nil, 0, time.Time{}, false
	}
	if len(f.Suggestions) == 0 {
		return nil, nil, 0, time.Time{}, false
	}
	// Does this advice describe the corpus that is here now? Counting files is
	// cheap; analysing them is the 7.7 seconds. So the check costs nothing and
	// stops the screen being instant and wrong, which is how it shipped an hour
	// ago: three findings over one transcript, from a fixture run, against a
	// corpus of 1,729.
	if home, err := os.UserHomeDir(); err == nil {
		if files, ferr := transcriptFiles(defaultTranscriptRoots(home)); ferr == nil {
			if !cacheCoversCorpus(f.Transcripts, len(files)) {
				return nil, nil, 0, time.Time{}, false
			}
		}
	}
	ids := make([]string, 0, len(f.Suggestions))
	for _, sg := range f.Suggestions {
		ids = append(ids, sg.ID)
	}
	// Transcripts, not Sessions. The screen's header says "transcript(s)" and
	// f.Sessions is the CALIBRATED count — 1,246 against 1,738 files here.
	// Passing it labelled the smaller number with the larger one's noun, which
	// is the third time the same category error has landed in this one screen
	// today: the header said sessions while counting files, the cache check
	// compared calibrated against files, and then this.
	return adviceRows(f.Suggestions), ids, f.Transcripts, f.Generated, true
}

// adviceRows converts suggestions to screen rows. Shared so the cached path
// and the computed path cannot drift into rendering different fields.
func adviceRows(sgs []advisor.Suggestion) []tui.AdviceRow {
	rows := make([]tui.AdviceRow, 0, len(sgs))
	for _, sg := range sgs {
		rows = append(rows, tui.AdviceRow{
			Title: sg.Title, Action: sg.Action, Sessions: sg.Sessions,
			Share: sg.Share, PromptTokens: sg.PromptTokens,
			PredictedShare: sg.PredictedShare, Estimated: sg.Estimated,
			Status: string(sg.Status),
		})
	}
	return rows
}

func adviceState() ([]tui.AdviceRow, int) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, 0
	}
	roots := defaultTranscriptRoots(home)
	if len(roots) == 0 {
		return nil, 0
	}
	files, err := transcriptFiles(roots)
	if err != nil || len(files) == 0 {
		return nil, 0
	}

	var obs []advisor.Observation
	sessions := 0
	_ = forEachSession(files, func(_ string, session *transcript.Session, _ *analysis.LaneReport, err error) error {
		if err != nil || session == nil {
			return nil
		}
		sessions++
		if ob, ok := advisor.Observe(session); ok {
			obs = append(obs, ob)
		}
		return nil
	})

	return adviceRows(advisor.Suggest(obs, appliedIDs())), sessions
}

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
	//
	// No --per-lane, so every row here is one session and m.Tasks is a session
	// count. It was not: `cost` priced one transcript FILE at a time, and a
	// session that fanned out to sub-agents writes one file per lane, so this
	// screen said "$3,129 across 1,614 tasks" over 114 sessions and listed the
	// same session id on a thousand consecutive rows.
	var buf bytes.Buffer
	if err := runCost([]string{"--per-task", "--json"}, &buf, io.Discard); err != nil {
		return m
	}
	// The summary is read back into the same type the report writes, so the
	// screen cannot pick up a field under a name the report stopped using.
	var out struct {
		Tasks []struct {
			Session         string  `json:"session"`
			Lanes           int     `json:"lanes"`
			Model           string  `json:"model"`
			Requests        int     `json:"requests"`
			CostUSD         float64 `json:"costUsd"`
			AvoidableUSD    float64 `json:"avoidableUsd"`
			AvoidableTokens int     `json:"avoidableTokens"`
			Breaks          int     `json:"breaks"`
		} `json:"tasks"`
		Summary costSummary `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil || out.Summary.Tasks == 0 {
		return m
	}
	// The rows must be sessions. This screen labels them "tasks" and its rows
	// open one transcript each, so being handed agent lanes here is the defect
	// rather than a degraded view — and drawing it anyway is how it went
	// unnoticed. Refusing leaves the screen Unavailable, which says so.
	if out.Summary.Unit != "" && out.Summary.Unit != unitSession {
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

	// What a share card of this corpus would carry.
	//
	// Built by cardData, the one function that turns a report into a picture,
	// and gated by shareCard, the one function that decides whether there is
	// anything worth posting. Both are the command's, called rather than
	// copied: a screen that offered a card `replay cost --share` would have
	// refused to produce is offering something nobody stands behind.
	breaks, peak := 0, 0
	for _, t := range out.Tasks {
		breaks += t.Breaks
		if t.AvoidableTokens > peak {
			peak = t.AvoidableTokens
		}
	}
	m.Card, m.ShareOK = shareFrom(sm, breaks, peak)

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
// row: a corpus of 1,614 files against 114 session rows would otherwise be one
// walk per keystroke.
//
// Sub-agent lanes are in this walk and cannot win a key from a session. They
// are named agent-<id>.jsonl, so they all share the prefix "agent-a" or
// similar and drop out as ambiguous, and no session id begins "agent-". Before
// the rows became sessions this map was asked for the same parent transcript
// on a thousand consecutive lane rows; now one row asks once.
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

// liveState asks a running proxy what it is seeing.
//
// The screen it feeds exists for one state above all others: a proxy that has
// been up for hours and recorded nothing, which is not a quiet day but an agent
// that was never pointed at it. That is invisible from the transcripts, which
// is why reading it needs a request rather than a file.
//
// Loopback only, and that restriction is the same one `replay doctor` carries
// for the same reason. ANTHROPIC_BASE_URL is whatever the environment says, so
// without this a screen whose job is "what is my proxy doing" becomes a request
// generator pointed at somebody else's network the moment that variable names
// one.
//
// Every failure is the unreachable state rather than an error. There is nothing
// a reader can do about a malformed status body that they would not also do
// about a refused connection, and the screen already says the useful thing.
func liveState() tui.Live {
	base := os.Getenv("ANTHROPIC_BASE_URL")
	if base == "" {
		base = "http://127.0.0.1:4000"
	}
	l := tui.Live{Addr: strings.TrimPrefix(strings.TrimPrefix(base, "http://"), "https://")}
	if !isLoopbackURL(base) {
		return l
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+proxy.StatusPath, nil)
	if err != nil {
		return l
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return l
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return l
	}
	var st proxy.Status
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&st); err != nil {
		return l
	}
	l.Reachable = true
	l.UptimeSeconds = st.UptimeSeconds
	l.PriceTable = st.PriceTable
	for _, x := range st.Sessions {
		l.Sessions = append(l.Sessions, tui.LiveSession{
			ID:           x.Session,
			Model:        x.Model,
			Requests:     x.Requests,
			PromptTokens: x.PromptTokens,
			CachedShare:  x.CachedShare,
			Breaks:       x.Breaks,
			CostUSD:      x.ListCostUSD,
			LastSeen:     x.LastSeen,
		})
	}
	return l
}
