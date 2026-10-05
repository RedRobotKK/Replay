package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	home, _ := os.UserHomeDir()
	cfg := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfg == "" {
		cfg = filepath.Join(home, ".claude")
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	settings := fs.String("settings", filepath.Join(cfg, "settings.json"), "the Claude Code settings file")
	register := fs.String("register", "docs/evidence/ttl-register-2026-10-05.md", "the frozen register")
	sched := fs.String("schedule", "docs/evidence/ttl-schedule-2026-10-05.json", "the frozen block order")
	logp := fs.String("log", "docs/evidence/ttl-blocks-2026-10-05.jsonl", "the block log, appended")
	block := fs.Int("block", 0, "block number, from 1")
	arm := fs.String("arm", "", "treatment or control; must match the schedule")
	blocks := fs.Int("blocks", 6, "schedule: how many blocks")
	_ = fs.Parse(os.Args[2:])
	p := paths{settings: *settings, register: *register, schedule: *sched, log: *logp}
	var err error
	switch os.Args[1] {
	case "schedule":
		var regSHA string
		regSHA, err = fileSHA256(p.register)
		if err == nil {
			var s schedule
			s, err = makeSchedule(regSHA, *blocks)
			if err == nil {
				var b []byte
				b, err = json.MarshalIndent(s, "", "  ")
				if err == nil {
					err = os.WriteFile(p.schedule, append(b, '\n'), 0o644)
				}
			}
		}
	case "start":
		var rec blockRecord
		rec, err = startBlock(p, *block, *arm, time.Now())
		printRecord(rec)
	case "end":
		var rec blockRecord
		rec, err = endBlock(p, *block, time.Now())
		printRecord(rec)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ttl-block:", err)
		os.Exit(1)
	}
}

func printRecord(r blockRecord) {
	if r.Schema == "" {
		return
	}
	b, _ := json.Marshal(r)
	fmt.Println(string(b))
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ttl-block schedule|start|end [-block N] [-arm treatment|control] [-settings file] [-register file] [-schedule file] [-log file]")
}
