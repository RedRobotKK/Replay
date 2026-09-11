package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// `replay purge` removes ledger records past a retention window.
//
// Nothing in this tool deleted a ledger record before it. That is a gap in the
// product rather than a missing feature request: a proxy that records every
// request indefinitely, with no documented period and no way to remove
// anything, has no answer to an erasure request and fails the Confidentiality
// criterion of a SOC 2 audit on retention alone.
//
// The exposure is genuinely low and that is not the same as none. record.go is
// explicit that the ledger holds "Counts and thresholds only, never content",
// so no prompt text is at stake. Session ids, request ids, paths and timestamps
// are still pseudonymous identifiers, and "it was only metadata" is not a
// retention policy.
//
// Three decisions, all made because this is the one command whose mistake
// cannot be corrected by running it again:
//
//   - --older-than is required. A default window would eventually delete
//     something for a reader who never chose one, and the reader who most needs
//     a retention policy is the one who has not thought about it yet.
//   - --dry-run is the default. It reports and removes nothing until --yes.
//   - Whole files only. A ledger file is one session; rewriting one in place to
//     drop records risks leaving a half-written file where a complete one was.

// windowRe accepts a positive count of days, hours or minutes.
var windowRe = regexp.MustCompile(`^([0-9]+)([dhm])$`)

// parseWindow reads a retention window, and refuses anything it is not certain
// of rather than guessing.
//
// Deliberately narrower than time.ParseDuration, which accepts "0s" and has no
// unit for days. A window this command misreads deletes the wrong set, so the
// grammar is small enough to be obvious: 30d, 12h, 90m.
func parseWindow(s string) (time.Duration, error) {
	m := windowRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("--older-than %q is not a window this command will guess at. "+
			"Use a positive count of days, hours or minutes: 30d, 12h, 90m", s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("--older-than %q is not positive; a zero window would remove "+
			"the whole ledger", s)
	}
	switch m[2] {
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "h":
		return time.Duration(n) * time.Hour, nil
	default:
		return time.Duration(n) * time.Minute, nil
	}
}

func runPurge(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("purge", flag.ContinueOnError)
	fs.SetOutput(stderr)
	olderThan := fs.String("older-than", "", "remove records older than this window: 30d, 12h, 90m")
	session := fs.String("session", "", "remove every record for one session id, whatever its age")
	export := fs.String("export", "", "write what is about to be removed to this file first")
	yes := fs.Bool("yes", false, "actually remove them; without this the command reports and changes nothing")
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, "Usage: replay purge <ledger-dir> --older-than <window> [--yes]\n\n"+
			"Removes ledger session files older than the window. Reports and removes\n"+
			"nothing unless --yes is given. The ledger holds counts and timings, never\n"+
			"message content.\n\n")
		fs.PrintDefaults()
	}
	// Flags may follow the directory, as they do for every other command here.
	if err := fs.Parse(hoistFlagsFor(fs, args)); err != nil {
		return errUsage
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("one ledger directory is required: %w", errUsage)
	}
	// One mode at a time. Combining them would delete a union the reader
	// never described, and the two answer different questions: a window is a
	// retention policy, a session id is an erasure request.
	switch {
	case *olderThan != "" && *session != "":
		return fmt.Errorf("--older-than and --session ask for different deletions and this "+
			"command will not guess which you meant. Run it twice: %w", errUsage)
	case *olderThan == "" && *session == "":
		return fmt.Errorf("one of --older-than or --session is required. There is no default "+
			"retention window, because a default would decide what to delete for somebody "+
			"who never chose one: %w", errUsage)
	case *session != "":
		return purgeSession(fs.Arg(0), *session, *export, *yes, stdout)
	}

	window, err := parseWindow(*olderThan)
	if err != nil {
		return fmt.Errorf("%w: %w", err, errUsage)
	}

	dir := fs.Arg(0)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading the ledger directory: %w", err)
	}

	cutoff := time.Now().Add(-window)
	type doomed struct {
		path string
		age  time.Duration
		size int64
	}
	var found []doomed
	for _, e := range entries {
		// Ledger session files only. Whatever else shares this directory is
		// somebody else's, and deleting a file this command does not own is
		// the worst outcome available to it.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		found = append(found, doomed{
			path: filepath.Join(dir, e.Name()),
			age:  time.Since(info.ModTime()),
			size: info.Size(),
		})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].age > found[j].age })

	if len(found) == 0 {
		_, _ = fmt.Fprintf(stdout, "Nothing to remove: no ledger record in %s is older than %s.\n",
			dir, *olderThan)
		return nil
	}

	var bytes int64
	for _, d := range found {
		bytes += d.size
	}
	verb := "would remove"
	if *yes {
		verb = "removed"
	}

	for _, d := range found {
		if *yes {
			if err := os.Remove(d.path); err != nil {
				return fmt.Errorf("removing %s: %w", d.path, err)
			}
		}
		_, _ = fmt.Fprintf(stdout, "  %s  %s  (%.0f days old, %s)\n",
			verb, filepath.Base(d.path), d.age.Hours()/24, formatBytes(d.size))
	}
	_, _ = fmt.Fprintf(stdout, "\n%s %d session file(s), %s, older than %s.\n",
		verb, len(found), formatBytes(bytes), *olderThan)
	if !*yes {
		_, _ = fmt.Fprintf(stdout, "Nothing was changed. Add --yes to remove them.\n")
	}
	return nil
}

// formatBytes is a size a person reads, not a number they convert.
func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// purgeSession removes one session's records, wherever they are.
//
// The erasure request, as distinct from the retention window above. A window
// asks "what is old"; this asks "what is about them", and the answer has to be
// removed from inside files that hold other sessions too. Taking the whole file
// would be the easy implementation and the wrong one: it erases the requester
// and everybody who shared the file with them.
//
// Records are matched on the session id the ledger already stores. Lines that
// do not parse are kept rather than dropped — an unreadable line is not
// evidence about anyone, and a purge is not the place to tidy a corpus.
// purgeWriteFile and purgeRename are indirected once so the two failure branches below
// can be entered from a test.
//
// Neither is reachable otherwise. Making the parent unwritable stops the write
// and so hides the rename; making the temp path a directory stops the write on
// every platform and hides the rename again. The seam is the only way to reach
// the second, and a branch no test can enter is one this repository does not
// keep — guard-reachability reported both as UNREACHED the moment the AST
// neutraliser made them checkable at all.
var (
	purgeWriteFile = os.WriteFile
	purgeRename    = os.Rename
)

func purgeSession(dir, id, export string, yes bool, stdout io.Writer) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("--session needs an id: %w", errUsage)
	}
	var matched, scanned int
	var removedFrom []string
	var exported [][]byte
	// Rewrites the walk found and did not perform. Held until the export is
	// safely on disk, so a failure there costs nothing.
	type rewrite struct {
		path string
		kept []string
	}
	var pending []rewrite

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		lines := strings.Split(string(body), "\n")
		kept := make([]string, 0, len(lines))
		var hit int
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			scanned++
			var rec struct {
				SessionID string `json:"session_id"`
			}
			// An unparseable line is kept. It says nothing about anyone, and a
			// deletion command is not a corpus cleaner.
			if json.Unmarshal([]byte(line), &rec) == nil && rec.SessionID == id {
				hit++
				exported = append(exported, []byte(line))
				continue
			}
			kept = append(kept, line)
		}
		if hit == 0 {
			return nil
		}
		matched += hit
		removedFrom = append(removedFrom, filepath.Base(path))
		if !yes {
			return nil
		}
		// The rewrite is DEFERRED, not done here, because the export has to be
		// on disk before a single record is removed.
		//
		// It used to happen inline and the export was written after the walk.
		// With --export pointing somewhere unwritable that erased every matched
		// record and then failed, leaving no copy — the exact outcome --export
		// exists to prevent. Reproduced 2026-09-10: a two-record ledger came
		// back holding one, the export never existed, and the command exited 1
		// having already destroyed what it promised to save first.
		pending = append(pending, rewrite{path: path, kept: kept})
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking the ledger: %w", err)
	}

	if matched == 0 {
		_, _ = fmt.Fprintf(stdout, "Nothing to remove: no record in %s carries session %q "+
			"(%d record(s) read).\n", dir, id, scanned)
		return nil
	}

	if export != "" {
		var buf strings.Builder
		for _, line := range exported {
			buf.Write(line)
			buf.WriteString("\n")
		}
		if werr := os.WriteFile(export, []byte(buf.String()), 0o600); werr != nil {
			return fmt.Errorf("writing the export: %w", werr)
		}
		_, _ = fmt.Fprintf(stdout, "Wrote %d record(s) to %s before removing them.\n", matched, export)
	}

	// Only now, with the export written or not requested, is anything removed.
	for _, r := range pending {
		out := strings.Join(r.kept, "\n")
		if out != "" {
			out += "\n"
		}
		// Through a sibling and renamed: a half-written ledger read on the next
		// keystroke would lose every session in the file, not just this one.
		tmp := r.path + ".tmp"
		if werr := purgeWriteFile(tmp, []byte(out), 0o600); werr != nil {
			return fmt.Errorf("rewriting %s: %w", r.path, werr)
		}
		if rerr := purgeRename(tmp, r.path); rerr != nil {
			return fmt.Errorf("replacing %s: %w", r.path, rerr)
		}
	}

	verb := "would remove"
	if yes {
		verb = "removed"
	}
	sort.Strings(removedFrom)
	_, _ = fmt.Fprintf(stdout, "%s %d record(s) for session %q from %d file(s): %s\n",
		verb, matched, id, len(removedFrom), strings.Join(removedFrom, ", "))
	if !yes {
		_, _ = fmt.Fprintf(stdout, "Nothing was changed. Add --yes to remove them.\n")
	}
	return nil
}
