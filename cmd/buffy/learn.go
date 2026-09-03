package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/learn"
	"github.com/RedRobotKK/Buffy/internal/ledger"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// policyFileName is where learn writes its result under ~/.buffy.
const policyFileName = "policy.json"

// policyFileMode keeps the file owner-only like the ledger; it holds no
// content, but it decides what the proxy does.
const policyFileMode = 0o600

// runLearn re-scores the policy catalog over every session under the given
// paths, applies the selection rules, prints the verdicts, and writes the
// policy file. It reads files only.
func runLearn(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("learn", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "policy file to write (default ~/.buffy/policy.json; \"-\" for none)")
	minSessions := fs.Int("min-sessions", learn.DefaultMinSessions, "sessions with evidence a candidate needs before it can be selected")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("one or more transcript or ledger directories are required: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}
	candidates := learn.Catalog()
	// The visitor never fails, so the walk cannot either.
	stores := map[string]*ledger.Store{}
	var sessions []*transcript.Session
	var reports []*analysis.LaneReport
	_ = forEachSession(files, func(path string, session *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil {
			return nil
		}
		if session.Source == transcript.SourceLedger {
			markControl(stores, filepath.Dir(path), session)
		}
		sessions = append(sessions, session)
		reports = append(reports, rep)
		return nil
	})
	// A model whose newest sessions stopped calibrating has no alternatives
	// scored (ST-1); its sessions are left out before selection.
	models := analysis.ModelCalibrations(reports)
	stale := analysis.StaleModels(models)
	var scores []learn.SessionScore
	for i, session := range sessions {
		if stale[reports[i].Lane.Requests[0].Model] {
			continue
		}
		if sc, ok := learn.Score(session, candidates); ok {
			scores = append(scores, sc)
		}
	}
	res := learn.Select(candidates, scores, len(files), learn.Options{MinSessions: *minSessions}, time.Now())
	learn.SortVerdicts(res.Verdicts)

	p := analysis.NewPrinter(stdout)
	for _, m := range models {
		if m.Stale {
			p.Printf("Provider behavior changed for %s: %s.\n  %s\n\n", m.Model, m.Reason, m.MinPrefix)
		}
	}
	p.Printf("Sessions: %d found, %d calibrated, %d held out. Rules %s.\n\n", res.Sessions.Found, res.Sessions.Calibrated, res.Sessions.Holdout, res.Rules)
	p.Printf("  %-36s %8s %18s %9s %s\n", "candidate", "sessions", "saving (interval)", "held-out", "decision")
	for _, v := range res.Verdicts {
		name := v.Name
		if v.Estimated {
			name += " *"
		}
		p.Printf("  %-36s %8d %7.1f%% (%+.1f..%+.1f) %8.1f%% %s\n", name, v.Sessions, v.Mean*100, v.Interval[0]*100, v.Interval[1]*100, v.HoldoutMean*100, v.Decision)
	}
	p.Printf("  saving is the share of as-run effective tokens avoided; * = estimated via the fit\n\n")
	if res.Selected == nil {
		p.Printf("Selected: none (%s)\n", res.Reason)
	} else {
		p.Printf("Selected: %s\n  live: %s\n", res.Selected.Name, res.Selected.Live)
	}
	if tr := res.Trial; tr != nil {
		p.Printf("\nLive trial of %s: %d treated, %d control sessions; cost per new token %.2f (%.2f..%.2f) vs %.2f (%.2f..%.2f); realized saving %.0f%%, predicted %.0f%%\n  %s\n", tr.Policy, tr.Treated, tr.Control, tr.TreatedCost, tr.TreatedInterval[0], tr.TreatedInterval[1], tr.ControlCost, tr.ControlInterval[0], tr.ControlInterval[1], tr.Realized*100, tr.Predicted*100, tr.Reason)
	}
	if len(res.Types) > 0 {
		p.Printf("\nBy session type (model family and first-prompt size, both known at a session's first request):\n")
		for _, tr := range res.Types {
			if tr.Selected == nil {
				p.Printf("  %-24s %3d sessions  none (%s)\n", tr.Type, tr.Sessions, tr.Reason)
			} else {
				p.Printf("  %-24s %3d sessions  %s\n", tr.Type, tr.Sessions, tr.Selected.Name)
			}
		}
	}
	if err := p.Err(); err != nil {
		return err
	}
	if *out == "-" {
		return nil
	}
	path := *out
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home directory for the policy file: %w", err)
		}
		path = filepath.Join(home, ".buffy", policyFileName)
	}
	if err := writePolicyFile(path, res); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Policy file: %s\n", path)
	return err
}

// markControl consults a ledger directory's pins for the session's trial
// arm. A directory that cannot be opened contributes no arm; the rest of
// learning does not depend on it.
func markControl(stores map[string]*ledger.Store, dir string, session *transcript.Session) {
	store, ok := stores[dir]
	if !ok {
		var err error
		store, err = ledger.Open(dir)
		if err != nil {
			store = nil
		}
		stores[dir] = store
	}
	if store != nil {
		store.MarkControl(session)
	}
}

// writePolicyFile renders the result as indented JSON, owner-only.
func writePolicyFile(path string, res learn.Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create policy directory: %w", err)
	}
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("encode policy file: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), policyFileMode); err != nil {
		return fmt.Errorf("write policy file: %w", err)
	}
	return nil
}
