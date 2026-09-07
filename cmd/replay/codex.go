package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// codexRoots are the places Codex keeps rollout logs.
//
// Both of them. `codex archive` moves a session from sessions/ to
// archived_sessions/ without changing its format or its relevance to a bill,
// and a reader that knows only the first reports a fraction of the corpus as
// though it were all of it. On the machine this was written against that is
// 27 of 148 files: a total that is wrong and looks right, which is the failure
// this project fixed in its own discovery once already.
func codexRoots(home string) []string {
	if home == "" {
		return nil
	}
	base := filepath.Join(home, ".codex")
	return []string{
		filepath.Join(base, "sessions"),
		filepath.Join(base, "archived_sessions"),
	}
}

func findCodexRollouts(roots []string) []string {
	var out []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil //nolint:nilerr // an unreadable subtree is not fatal to the rest
			}
			b := filepath.Base(p)
			if strings.HasPrefix(b, "rollout-") && strings.HasSuffix(b, ".jsonl") {
				out = append(out, p)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

func runCodex(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("codex", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}

	var files []string
	var searched []string
	if fs.NArg() > 0 {
		for _, d := range fs.Args() {
			searched = append(searched, d)
			files = append(files, findCodexRollouts([]string{d})...)
		}
		// A directory named directly may hold the files without the rollout-
		// prefix convention, which is how fixtures are kept.
		if len(files) == 0 {
			for _, d := range fs.Args() {
				m, _ := filepath.Glob(filepath.Join(d, "*.jsonl"))
				files = append(files, m...)
			}
			sort.Strings(files)
		}
	} else {
		home, _ := os.UserHomeDir()
		searched = codexRoots(home)
		files = findCodexRollouts(searched)
	}

	if len(files) == 0 {
		_, _ = fmt.Fprintf(stdout, "  No Codex sessions found. Searched:\n")
		for _, s := range searched {
			_, _ = fmt.Fprintf(stdout, "    %s\n", s)
		}
		_, _ = fmt.Fprintf(stdout, "\n  Codex writes rollout-*.jsonl under ~/.codex. If it has never\n"+
			"  run on this machine there is nothing to read, which is not an error.\n")
		return nil
	}

	var billed, reported int
	var refused, compacted, quotas int
	var latest *transcript.CodexQuota
	type row struct {
		name             string
		billed, reported int
		rebased          bool
		refused          int
	}
	var rows []row

	for _, f := range files {
		s, err := transcript.ParseCodexFile(f)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "replay: %s: %v\n", filepath.Base(f), err)
			continue
		}
		billed += s.Billed.Total()
		reported += s.Reported.Total()
		refused += s.Skipped
		if s.Rebased {
			compacted++
		}
		if s.Quota != nil {
			quotas++
			latest = s.Quota
		}
		rows = append(rows, row{filepath.Base(f), s.Billed.Total(), s.Reported.Total(), s.Rebased, s.Skipped})
	}

	_, _ = fmt.Fprintf(stdout, "\n  %s tokens billed across %d Codex session(s)\n",
		comma(billed), len(rows))
	_, _ = fmt.Fprintf(stdout, "  Summed from per-turn deltas, which is what was paid for.\n\n")

	if compacted > 0 {
		gap := billed - reported
		pct := 0.0
		if billed > 0 {
			pct = 100 * float64(gap) / float64(billed)
		}
		_, _ = fmt.Fprintf(stdout, "  [NOTE] %s of that (%.0f%%) is invisible to Codex's own counter.\n"+
			"         %d session(s) compacted, and Codex rebases its running total when\n"+
			"         that happens, so it reports %s. The counter is not wrong; it is\n"+
			"         measuring what is in context, not what was paid for.\n\n",
			comma(gap), pct, compacted, comma(reported))
	}
	if refused > 0 {
		_, _ = fmt.Fprintf(stdout, "  [NOTE] %d record(s) refused: a cached or reasoning count larger\n"+
			"         than the total it is a share of cannot be true, so it was not\n"+
			"         added to the figure above.\n\n", refused)
	}

	if latest != nil {
		_, _ = fmt.Fprintf(stdout, "  quota (%s plan, from %d session(s) that recorded it)\n",
			plainOr(latest.PlanType, "unknown"), quotas)
		_, _ = fmt.Fprintf(stdout, "    %-10s %5.0f%% used   window %s\n", "primary",
			latest.PrimaryUsedPercent, minutes(latest.PrimaryWindowMinutes))
		_, _ = fmt.Fprintf(stdout, "    %-10s %5.0f%% used   window %s\n\n", "secondary",
			latest.SecondaryUsedPercent, minutes(latest.SecondaryWindowMinutes))
	} else {
		_, _ = fmt.Fprintf(stdout, "  quota   not recorded in these sessions\n\n")
	}

	_, _ = fmt.Fprintf(stdout, "  ran   replay codex\n")
	return nil
}

func plainOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func minutes(m int) string {
	switch {
	case m <= 0:
		return "unknown"
	case m%(60*24) == 0:
		return fmt.Sprintf("%dd", m/(60*24))
	case m%60 == 0:
		return fmt.Sprintf("%dh", m/60)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

func comma(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
