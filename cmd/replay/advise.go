package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/RedRobotKK/Replay/internal/advisor"
	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// adviceFileName is where advise records its suggestions under ~/.replay;
// adviceFileMode keeps it owner-only like the ledger.
const (
	adviceFileName = "advice.json"
	adviceFileMode = 0o600
)

// adviceFile is the on-disk record: every suggestion with its status.
type adviceFile struct {
	Schema      int                  `json:"schema"`
	Generated   time.Time            `json:"generated"`
	Sessions    int                  `json:"sessions"`
	Suggestions []advisor.Suggestion `json:"suggestions"`
}

// runAdvise turns the largest token sources across all sessions into
// suggestions and prints them best first with their tracking status.
func runAdvise(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("advise", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", "", "advice file to write (default ~/.replay/advice.json; \"-\" for none)")
	apply := fs.Bool("apply", false, "propose the one setting this can change for you, and show the diff")
	yes := fs.Bool("yes", false, "with --apply, write the change instead of only describing it")
	asJSON := fs.Bool("json", false, "with --apply, emit the plan as JSON for an agent to act on")
	// Print-only on purpose, and so not accepted alongside --apply: a spend
	// cap the tool wrote for you is a refusal you did not choose.
	guards := fs.Bool("guards", false, "suggest spend caps from your own session spread (print-only, never written)")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("one or more transcript or ledger directories are required: %w", errUsage)
	}
	if *guards && *apply {
		return fmt.Errorf("--guards is print-only and cannot be combined with --apply: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}
	var obs []advisor.Observation
	var reports []*analysis.LaneReport
	var sessionUSD, sessionTokens []float64
	// The visitor never fails, so the walk cannot either.
	_ = forEachSession(files, func(_ string, session *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil {
			return nil
		}
		if ob, ok := advisor.Observe(session); ok {
			obs = append(obs, ob)
		}
		if rep != nil {
			reports = append(reports, rep)
			// policies[0] is the session as it actually ran, which is the
			// same as-run row the proxy scores live, so the cap suggested
			// here is measured against what the user really spent.
			if pols := rep.Policies(); len(pols) > 0 {
				sessionUSD = append(sessionUSD, pols[0].CostUSD)
				sessionTokens = append(sessionTokens, float64(pols[0].PromptTokens))
			}
		}
		return nil
	})

	if *guards {
		p := analysis.NewPrinter(stdout)
		for _, l := range guardAdviceLines(sessionUSD, sessionTokens) {
			p.Printf("%s\n", l)
		}
		return p.Err()
	}
	suggestions := advisor.Suggest(obs)

	// With --json, stdout belongs to the machine. The human report still gets
	// written — it is useful beside the JSON — but on stderr, so that
	// `advise --apply --json --out - | jq` works. Emitting 43KB of prose ahead
	// of the document on the same stream made --json unusable for the one
	// audience it exists for.
	prose := stdout
	if *asJSON {
		prose = stderr
	}
	p := analysis.NewPrinter(prose)
	p.Printf("Sessions: %d found, %d calibrated. Predictions assume the target is halved; shares are of prompt tokens, the scale-free metric.\n\n", len(files), len(obs))
	if len(suggestions) == 0 {
		p.Printf("No token source above %.0f%% of prompt tokens in any session.\n", advisor.MinShare*100)
	}
	for i, s := range suggestions {
		tier := ""
		if s.Estimated {
			tier = " *"
		}
		p.Printf("%d. [%s] %s\n", i+1, s.Status, s.Title)
		p.Printf("   %s\n", s.Action)
		p.Printf("   evidence: %d session(s), %s tokens in prompts%s; predicted saving %.0f%% of prompt tokens per session (%s tokens across the corpus)", s.Sessions, formatCount(s.PromptTokens), tier, s.PredictedShare*100, formatCount(s.PredictedTokens))
		if s.Status == advisor.Verified || s.Status == advisor.NotVerified {
			p.Printf("; realized %.0f%%", s.RealizedShare*100)
		}
		p.Printf("\n\n")
	}
	if len(suggestions) > 0 {
		p.Printf("* = estimated via the byte-to-token fit. Statuses: pending, applied, verified, not verified, advice only.\n")
	}
	if err := p.Err(); err != nil {
		return err
	}
	// "-" means do not write a file. It does not mean skip the work: this
	// return used to sit above the --apply dispatch below, so
	// `advise --apply --json --out -` printed prose, emitted no JSON and
	// exited 0. That is the obvious agent-safe invocation — do not touch my
	// disk, give me JSON — and it was the one combination that silently did
	// nothing.
	if *out == "-" {
		if *apply {
			return applySettings(reports, stdout, *yes, *asJSON)
		}
		return nil
	}
	path := *out
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home directory for the advice file: %w", err)
		}
		path = filepath.Join(home, ".replay", adviceFileName)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create advice directory: %w", err)
	}
	data, err := json.MarshalIndent(adviceFile{Schema: advisor.AdviceFileSchema, Generated: time.Now().UTC(), Sessions: len(obs), Suggestions: suggestions}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode advice file: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), adviceFileMode); err != nil {
		return fmt.Errorf("write advice file: %w", err)
	}
	if _, err := fmt.Fprintf(prose, "Advice file: %s\n", path); err != nil {
		return err
	}
	if *apply {
		return applySettings(reports, stdout, *yes, *asJSON)
	}
	return nil
}

// applySettings prints what can be changed automatically and what cannot.
//
// The split is the honest part. One setting can be applied from evidence. The
// rest are changes to how somebody works, and the tool's job there is to name
// the largest one and get out of the way.
func applySettings(reports []*analysis.LaneReport, stdout io.Writer, commit, asJSON bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	settings := filepath.Join(claudeConfigDir(home), "settings.json")

	have := ""
	if b, err := os.ReadFile(settings); err == nil {
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			if v, ok := m["promptCacheTtl"].(string); ok {
				have = v
			}
		}
	}

	plan := ttlPlan(reports, have)

	// An agent asked to "optimise my settings" needs the decision, not prose it
	// has to parse back. Applicable and manual are separate lists on purpose: a
	// caller that applies the manual ones is doing something this tool refused
	// to do.
	if asJSON {
		out := map[string]any{
			"schema":       "replay.apply.v1",
			"settingsPath": settings,
			"applicable":   []any{},
			"manual":       manualSteps,
		}
		if plan.Trustworthy {
			out["applicable"] = []any{map[string]any{
				"setting":  plan.Setting,
				"current":  plan.Have,
				"proposed": plan.Want,
				"evidence": plan.Evidence,
				"applied":  commit,
			}}
		} else {
			out["refused"] = map[string]any{"setting": plan.Setting, "reason": plan.Reason}
		}
		if commit && plan.Trustworthy {
			if err := plan.write(settings, io.Discard, true); err != nil {
				return err
			}
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s\n", b)
		return err
	}

	p := analysis.NewPrinter(stdout)
	p.Printf("\nWhat can be applied from evidence\n\n")
	if err := p.Err(); err != nil {
		return err
	}
	if err := plan.write(settings, stdout, commit); err != nil {
		// A refusal is the feature working, not the command failing.
		if _, e := fmt.Fprintf(stdout, "%v\n", err); e != nil {
			return e
		}
	}

	p.Printf("\nWhat only you can change\n\n")
	p.Printf("These are the 2026 habits that move prompt tokens most, in the order this corpus\n")
	p.Printf("supports. Replay will not edit them for you: they change what your agent reads,\n")
	p.Printf("and a tool that rewrites your instructions because it judged them long would be\n")
	p.Printf("worse than one that tells you.\n\n")
	for i, m := range manualSteps {
		p.Printf("  %d. %s\n     %s\n", i+1, m["title"], m["why"])
	}
	p.Printf("\nThe ranked list above this section is the same advice, measured against your own\nsessions rather than asserted.\n")
	return p.Err()
}

// formatCount renders a token count the way the reports do.
func formatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fk", float64(n)/1_000)
	}
	return fmt.Sprint(n)
}

// manualSteps are the habits that move prompt tokens most, which Replay reports
// and refuses to perform. They change what the agent reads, and a tool that
// rewrote somebody's instruction files because it judged them long would be
// worse than one that told them.
var manualSteps = []map[string]string{
	{"id": "truncate-tool-output", "title": "Truncate tool output before it enters the conversation.",
		"why": "head, tail, grep with limits, or a wrapper that summarises. Tool results are the largest single source of prompt tokens in most sessions."},
	{"id": "split-instructions", "title": "Split the always-on instruction file.",
		"why": "Keep what every turn needs; move the rest into skills the agent loads when they are relevant."},
	{"id": "prune-connectors", "title": "Turn off connectors this task does not use.",
		"why": "Every connected MCP server puts its tool definitions in the prompt whether or not you call them."},
	{"id": "paths-not-contents", "title": "Pass paths, not contents.",
		"why": "Run scripts from files instead of inline heredocs."},
	{"id": "delegate-long-work", "title": "Give long work to subagents.",
		"why": "Their transcripts do not become your prefix."},
}
