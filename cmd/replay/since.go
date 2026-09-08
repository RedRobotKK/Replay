package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// What happened while you were away.
//
// This is the cheapest useful version of a mobile tether, and it is deliberately
// first. A push notification needs a server, a phone number, an account and a
// pairing flow, and every one of those costs something this tool has promised
// not to spend. A digest on return needs none of them and answers the same
// question: what did I miss.
//
// It is also the honest way to find out whether the notification is wanted. If
// nobody reads this, nobody wanted the buzz either, and that has been learned
// for the price of one screen rather than a service with a PII store attached.
//
// It reads `replay cost --per-task --json` rather than re-deriving anything.
// That is a deliberate coupling: the two can never disagree about what a session
// cost, because one is a filter over the other. The schema is versioned and the
// cost path is already cached, so the second read is cheap.

// seenMarker records when the digest was last consumed.
type seenMarker struct {
	At time.Time `json:"at"`
}

func seenPath() string { return filepath.Join(tipStateDir(), "seen.json") }

func loadSeen() (time.Time, bool) {
	b, err := os.ReadFile(seenPath())
	if err != nil {
		return time.Time{}, false
	}
	var m seenMarker
	if json.Unmarshal(b, &m) != nil || m.At.IsZero() {
		return time.Time{}, false
	}
	return m.At, true
}

func saveSeen(at time.Time) error {
	dir := tipStateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(seenMarker{At: at})
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".seen-*.tmp")
	f, err := os.CreateTemp(dir, filepath.Base(tmp))
	if err != nil {
		return err
	}
	name := f.Name()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, seenPath())
}

// costReport is the part of replay.cost.v2 this command consumes.
type costReport struct {
	Tasks []struct {
		Session         string    `json:"session"`
		Model           string    `json:"model"`
		Requests        int       `json:"requests"`
		CostUSD         float64   `json:"costUsd"`
		AvoidableUSD    float64   `json:"avoidableUsd"`
		AvoidableTokens int       `json:"avoidableTokens"`
		Breaks          int       `json:"breaks"`
		At              time.Time `json:"at"`
	} `json:"tasks"`
	Unpriced int `json:"unpriced"`
}

func runSince(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("since", flag.ContinueOnError)
	fs.SetOutput(stderr)
	peek := fs.Bool("peek", false, "report without consuming the window, so it can be read twice")
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, "Usage: replay since [--peek]\n\n"+
			"What ran, and what it cost, since you last looked. Reading it advances the\n"+
			"marker; --peek leaves it where it is.\n\n"+
			"Needs no account, no network and no pairing: the answer is already on disk.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Ask cost for the rows rather than deriving them again.
	var buf bytes.Buffer
	if err := runCost([]string{"--per-task", "--json"}, &buf, io.Discard); err != nil {
		return err
	}
	var rep costReport
	// An empty body is not a malformed one. With no transcripts at all, cost
	// explains itself in prose on stderr and writes no JSON, so there is
	// nothing to parse and nothing wrong. Treating that as a parse error would
	// turn "you have not run an agent here" into a fault in this tool, which is
	// the wrong thing to tell someone on their first day.
	if len(bytes.TrimSpace(buf.Bytes())) > 0 {
		if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
			return fmt.Errorf("could not read the cost report this is built on: %w", err)
		}
	}

	mark, had := loadSeen()
	now := time.Now()

	var window []int
	for i, t := range rep.Tasks {
		if !had || t.At.After(mark) {
			window = append(window, i)
		}
	}
	sort.Slice(window, func(a, b int) bool {
		return rep.Tasks[window[a]].At.Before(rep.Tasks[window[b]].At)
	})

	if !had {
		// A first run has nothing to measure from. Reporting "0 sessions since
		// ..." would be an absence dressed as a finding, and the window would
		// be a fiction: there is no earlier look for it to start at.
		_, _ = fmt.Fprintf(stdout, "\n  No previous look recorded, so there is no window to report yet.\n"+
			"  This run sets the marker. Come back after some work and this will say what changed.\n\n")
		if !*peek {
			if err := saveSeen(now); err != nil {
				return err
			}
		}
		return nil
	}

	if len(window) == 0 {
		// Not a zero. A quiet window and no spend are different claims, and
		// "$0.00" states the second while meaning the first.
		_, _ = fmt.Fprintf(stdout, "\n  Nothing since you last looked, %s ago.\n\n", roughly(now.Sub(mark)))
		if !*peek {
			if err := saveSeen(now); err != nil {
				return err
			}
		}
		return nil
	}

	var total, avoidable float64
	var breaks, reqs, avoidTok int
	worst := -1
	for _, i := range window {
		t := rep.Tasks[i]
		total += t.CostUSD
		avoidable += t.AvoidableUSD
		avoidTok += t.AvoidableTokens
		breaks += t.Breaks
		reqs += t.Requests
		if worst < 0 || t.AvoidableTokens > rep.Tasks[worst].AvoidableTokens {
			worst = i
		}
	}

	_, _ = fmt.Fprintf(stdout, "\n  Since you last looked, %s ago\n\n", roughly(now.Sub(mark)))
	_, _ = fmt.Fprintf(stdout, "    %d session(s)   %d request(s)   $%.2f\n", len(window), reqs, total)
	if breaks > 0 {
		_, _ = fmt.Fprintf(stdout, "    %d cache break(s)   $%.2f re-billed, %s tokens\n",
			breaks, avoidable, thousands(avoidTok))
	}
	if worst >= 0 && rep.Tasks[worst].Breaks > 0 {
		w := rep.Tasks[worst]
		_, _ = fmt.Fprintf(stdout, "\n    largest: session %s at %s, %s tokens re-billed\n",
			w.Session, w.At.Local().Format("15:04"), thousands(w.AvoidableTokens))
		// The cause lives per turn, and this row does not carry it. Pointing at
		// the command that does beats guessing at one here.
		_, _ = fmt.Fprintf(stdout, "    for the cause:  replay diff <transcript>\n")
	}
	if rep.Unpriced > 0 {
		_, _ = fmt.Fprintf(stdout, "\n    %d transcript(s) excluded: their model is not in the price table.\n"+
			"    They are left out rather than counted as free.\n", rep.Unpriced)
	}
	_, _ = fmt.Fprintln(stdout)

	if !*peek {
		if err := saveSeen(now); err != nil {
			return err
		}
	}
	return nil
}

// roughly renders a gap the way a person would say it.
func roughly(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "moments"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}

// thousands groups digits so a token count reads at a glance.
func thousands(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}
