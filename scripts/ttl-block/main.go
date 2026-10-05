package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// defaultSettings is the Claude Code settings file this environment points at.
func defaultSettings() string {
	cfg := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfg == "" {
		home, _ := os.UserHomeDir()
		cfg = filepath.Join(home, ".claude")
	}
	return filepath.Join(cfg, "settings.json")
}

// run is the command line: schedule, start or end, with the paths as flags.
// It returns the exit code and prints the record, when there is one, as JSON.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		usage(stderr)
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	settings := fs.String("settings", defaultSettings(), "the Claude Code settings file")
	register := fs.String("register", "docs/evidence/ttl-register-2026-10-05.md", "the frozen register")
	sched := fs.String("schedule", "docs/evidence/ttl-schedule-2026-10-05.json", "the frozen block order")
	logp := fs.String("log", "docs/evidence/ttl-blocks-2026-10-05.jsonl", "the block log, appended")
	block := fs.Int("block", 0, "block number, from 1")
	arm := fs.String("arm", "", "treatment or control; must match the schedule")
	blocks := fs.Int("blocks", 6, "schedule: how many blocks")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	p := paths{settings: *settings, register: *register, schedule: *sched, log: *logp}
	var rec blockRecord
	var err error
	switch args[0] {
	case "schedule":
		err = writeSchedule(p, *blocks)
	case "start":
		rec, err = startBlock(p, *block, *arm, time.Now())
	case "end":
		rec, err = endBlock(p, *block, time.Now())
	default:
		usage(stderr)
		return 2
	}
	if rec.Schema != "" {
		b, _ := json.Marshal(rec)
		_, _ = fmt.Fprintln(stdout, string(b))
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "ttl-block:", err)
		return 1
	}
	return 0
}

// writeSchedule derives the block order from the register as it is now and
// writes it where every later operation will check it.
func writeSchedule(p paths, blocks int) error {
	regSHA, err := fileSHA256(p.register)
	if err != nil {
		return err
	}
	s, err := makeSchedule(regSHA, blocks)
	if err != nil {
		return err
	}
	b, err := marshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the schedule, so nothing was written: %w", err)
	}
	return os.WriteFile(p.schedule, append(b, '\n'), 0o644)
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: ttl-block schedule|start|end [-block N] [-arm treatment|control] [-settings file] [-register file] [-schedule file] [-log file]")
}
