package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/facts"

	"github.com/RedRobotKK/Replay/internal/reference"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// surfaceBurn is one agent surface's consumption, on its own terms.
//
// The fields are deliberately not comparable across surfaces and this type
// does not offer a way to add them. A Claude Code token is billed or drawn
// against a subscription, a Codex token is drawn against a rate-limit window,
// and an Ollama token is compute on this machine drawn against nothing. Their
// headline figures do not even agree on whether the cached prefix is inside
// them. A grand total would be a number with no unit.
type surfaceBurn struct {
	name string
	// requests is the number of calls made to a provider.
	//
	// It used to be the number of FILES read, on two of the three surfaces,
	// under a column header that said "requests". Ollama counted requests
	// because its log has no file-per-session shape to confuse them with; the
	// Codex and Claude Code readers incremented once per transcript. So one
	// column held two different quantities and the reader had no way to tell:
	// a machine with 148 Codex rollouts covering thousands of turns reported
	// "148 requests", next to an Ollama row where the same column meant what
	// it said.
	//
	// A session is not a transcript is not a request. This project has
	// retracted a published figure over that conflation once already.
	requests int
	// sessions is how many transcripts or rollouts were read to get there. It
	// is reported separately rather than folded in, because a surface with no
	// per-session file (Ollama) genuinely has none and must say so rather than
	// borrow the request count.
	sessions    int
	hasSessions bool
	// tokens is the surface's own headline figure, and Unit says what it means.
	tokens int
	unit   string
	// cached is the share of the prompt that did not have to be paid for
	// again, where the surface reports enough to say.
	cached    float64
	hasCached bool
	// quota is the live reading, where one exists at all.
	quota    string
	first    time.Time
	last     time.Time
	problems []string
}

// cachedShareLine is the published cache hit rate, for the line under this
// surface's own. It returns "" when there is no local reading to place.
//
// The published half only: the caller has already printed the local share in
// its own precision, and restating it here put "99%" and "98.6% here" on
// consecutive rows — two renderings of one measurement, which is the failure
// this whole comparison exists to help a reader notice.
//
// A hit rate means little alone: 99% reads as excellent to anybody who has not
// seen another machine's and as unremarkable to anybody who has. TraceLab
// measured 4,265 sessions across Claude Code and Codex at 95.7%, which is the
// first thing that makes 99% mean anything.
//
// Zero is treated as no reading rather than as a measured zero, matching the
// caller: burn clears hasCached on Ollama deliberately, because every observed
// request there is a back-off case and averaging the unlabelled ones would look
// like a measurement of the cache instead of a measurement of the logging.
func cachedShareLine(share float64) string {
	if share <= 0 {
		return ""
	}
	return publishedShare("cachedShare", share)
}

// publishedShare is the published half of a comparison for one metric.
//
// The metric is a parameter so the not-found branch can be entered from a
// test. It is unreachable while Compiled() carries cachedShare, and a branch no
// test can enter is one this repository does not keep.
func publishedShare(metric string, share float64) string {
	ref, ok := reference.For(metric)
	if !ok {
		return ""
	}
	return ref.Compare(share).Against()
}

// perHour is the burn rate over the window actually observed, which is the
// only rate the data supports. A corpus spanning an afternoon says nothing
// about a month.
func (s surfaceBurn) perHour() (float64, bool) {
	if s.first.IsZero() || s.last.IsZero() {
		return 0, false
	}
	h := s.last.Sub(s.first).Hours()
	if h <= 0 {
		return 0, false
	}
	return float64(s.tokens) / h, true
}

func runBurn(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("burn", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "read surfaces from this directory instead of the machine's own")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}

	home, _ := os.UserHomeDir()
	var surfaces []surfaceBurn
	surfaces = append(surfaces, burnCodex(home, *dir))
	surfaces = append(surfaces, burnOllama(home, *dir))
	surfaces = append(surfaces, burnClaudeCode(home, *dir))

	_, _ = fmt.Fprintf(stdout, "\n  %s\n\n", "What your agents are consuming, per surface")
	_, _ = fmt.Fprintf(stdout, "  %-14s %9s %14s  %-22s %s\n",
		"surface", "requests", "tokens", "what that counts", "quota")
	_, _ = fmt.Fprintf(stdout, "  %-14s %9s %14s  %-22s %s\n",
		strings.Repeat("-", 14), strings.Repeat("-", 9), strings.Repeat("-", 14),
		strings.Repeat("-", 22), strings.Repeat("-", 12))
	for _, s := range surfaces {
		tok := "not read"
		if s.requests > 0 {
			tok = comma(s.tokens)
		}
		_, _ = fmt.Fprintf(stdout, "  %-14s %9s %14s  %-22s %s\n",
			s.name, commaOrDash(s.requests), tok, s.unit, s.quota)
	}

	_, _ = fmt.Fprintf(stdout, "\n  These columns are not addable. Anthropic reports the prompt with the\n")
	_, _ = fmt.Fprintf(stdout, "  cached share partitioned out of it, Codex reports it nested inside, and\n")
	_, _ = fmt.Fprintf(stdout, "  Ollama reports the work it performed with the cached prefix excluded\n")
	_, _ = fmt.Fprintf(stdout, "  entirely. Summing them would produce a figure with no unit.\n\n")

	for _, s := range surfaces {
		if s.requests == 0 {
			continue
		}
		_, _ = fmt.Fprintf(stdout, "  %s\n", s.name)
		if s.hasSessions {
			_, _ = fmt.Fprintf(stdout, "    %s request(s) across %s session transcript(s)\n",
				comma(s.requests), comma(s.sessions))
		}
		if r, ok := s.perHour(); ok {
			_, _ = fmt.Fprintf(stdout, "    %s tokens/hour over the %s observed\n",
				comma(int(r)), humanWindow(s.last.Sub(s.first)))
		}
		if s.hasCached {
			_, _ = fmt.Fprintf(stdout, "    %s of the prompt served from cache\n", sharePct(s.cached))
			if line := cachedShareLine(s.cached); line != "" {
				_, _ = fmt.Fprintf(stdout, "    %s\n", line)
			}
		}
		for _, p := range s.problems {
			_, _ = fmt.Fprintf(stdout, "    [NOTE] %s\n", p)
		}
		_, _ = fmt.Fprintf(stdout, "\n")
	}

	_, _ = fmt.Fprintf(stdout, "\n  ran   replay burn\n")
	// The behaviour table this surface's advice rests on is true of a version,
	// not of a date. Say so when the installed one differs.
	if n := facts.Ollama().Note(ollamaVersion()); n != "" {
		_, _ = fmt.Fprintf(stdout, "  %s\n\n", wrapAt(n, 74, "  "))
	}
	return nil
}

func humanWindow(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	case d < 48*time.Hour:
		return fmt.Sprintf("%.0f hours", d.Hours())
	default:
		return fmt.Sprintf("%.0f days", d.Hours()/24)
	}
}

func commaOrDash(n int) string {
	if n == 0 {
		return "-"
	}
	return comma(n)
}

func burnCodex(home, dir string) surfaceBurn {
	s := surfaceBurn{
		name: "codex", unit: "context, cached inside",
		quota: "reported",
	}
	roots := codexRoots(home)
	if dir != "" {
		roots = []string{filepath.Join(dir, "codex")}
	}
	files := findCodexRollouts(roots)
	if dir != "" && len(files) == 0 {
		files, _ = filepath.Glob(filepath.Join(dir, "codex", "*.jsonl"))
	}
	var billed, breaks, rebased int
	var q *transcript.CodexQuota
	for _, f := range files {
		r, err := transcript.ParseCodexFile(f)
		if err != nil {
			continue
		}
		s.sessions++
		s.hasSessions = true
		s.requests += r.Turns
		billed += r.Billed.Total()
		breaks += len(r.Breaks)
		if r.Quota != nil {
			q = r.Quota
		}
		if r.Rebased {
			rebased++
		}
	}
	s.tokens = billed
	if q != nil {
		s.quota = fmt.Sprintf("%.0f%% of %s", q.PrimaryUsedPercent, minutes(q.PrimaryWindowMinutes))
	}
	if rebased > 0 {
		s.problems = append(s.problems, fmt.Sprintf(
			"%d session(s) compacted; Codex rebases its own counter there, so its total is not the bill", rebased))
	}
	if breaks > 0 {
		s.problems = append(s.problems, fmt.Sprintf("%d cache break(s)", breaks))
	}
	return s
}

func burnOllama(home, dir string) surfaceBurn {
	s := surfaceBurn{
		name: "ollama", unit: "work done, no bill",
		quota: "none exists",
	}
	pat := filepath.Join(home, ".ollama", "logs", "server*.log")
	if dir != "" {
		pat = filepath.Join(dir, "ollama", "server*.log")
	}
	logs, _ := filepath.Glob(pat)
	var ctx, cached, unmeasured int
	for _, p := range logs {
		rs, err := transcript.ParseOllamaLogFile(p)
		if err != nil {
			continue
		}
		for _, r := range rs {
			s.requests++
			s.tokens += r.Total
			// Only requests whose reuse was actually observed reach the
			// aggregate. Ollama's prompt_eval_count excludes the reused
			// prefix, so the n_past line is the only place it appears, and a
			// block without one has an unknown prefix rather than a zero.
			//
			// Averaging the unknown ones in as zero would drag the cached
			// share toward a full miss in proportion to how much of the log
			// went unlabelled, and the resulting figure would look like a
			// measurement of the cache instead of a measurement of the
			// logging.
			c, ok := r.ContextTokens()
			if !ok {
				unmeasured++
				continue
			}
			ctx += c
			cached += r.CachedPrefix
		}
	}
	if ctx > 0 {
		s.cached, s.hasCached = float64(cached)/float64(ctx), true
	}
	// Say what the share is a share of, and on this surface that turns out to
	// disqualify the share entirely.
	//
	// n_past appears in an Ollama log ONLY when the whole prompt was already
	// cached: on this machine's corpus all 586 requests carrying one are
	// back-off cases, where llama.cpp finds every token resident and then
	// re-evaluates one because it must evaluate at least one per active slot.
	// So a cached share computed over those requests is not an estimate of
	// cache performance. It is a measurement of a population defined by having
	// been fully cached, pinned near (n-1)/n by the back-off rule.
	//
	// The honest report is two counts and no average. See
	// docs/evidence/ollama-cache-ceiling-2026-09-08.md.
	if unmeasured > 0 {
		s.hasCached = false
		s.problems = append(s.problems, fmt.Sprintf(
			"%d of %d requests were served entirely from cache apart from the one token the server re-evaluates by rule; the other %d log no reuse figure, so no cache hit rate is reported",
			s.requests-unmeasured, s.requests, unmeasured))
	}
	return s
}

func burnClaudeCode(home, dir string) surfaceBurn {
	s := surfaceBurn{
		name: "claude-code", unit: "prompt, cache separate",
		quota: "not reported",
	}
	if dir != "" {
		return s // the fixture carries no Claude Code corpus
	}
	roots := defaultTranscriptRoots(home)
	var input, cacheRead, cacheWrite int
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".jsonl") {
				return nil //nolint:nilerr // an unreadable subtree is not fatal
			}
			sess, err := transcript.ParseClaudeCodeFile(p)
			if err != nil || sess == nil {
				return nil //nolint:nilerr // a file that will not parse is counted nowhere
			}
			s.sessions++
			s.hasSessions = true
			s.requests += sess.RequestCount()
			for _, lane := range sess.Lanes {
				for _, r := range lane.Requests {
					input += r.Usage.Input
					cacheRead += r.Usage.CacheRead
					cacheWrite += r.Usage.CacheCreation
					if r.Timestamp.After(s.last) {
						s.last = r.Timestamp
					}
					if s.first.IsZero() || r.Timestamp.Before(s.first) {
						s.first = r.Timestamp
					}
				}
			}
			return nil
		})
	}
	s.tokens = input + cacheRead + cacheWrite
	if total := input + cacheRead + cacheWrite; total > 0 {
		s.cached, s.hasCached = float64(cacheRead)/float64(total), true
	}
	return s
}

// sharePct formats a cached share without rounding it into a claim.
//
// %.0f printed the operator's real Ollama corpus as "100% of the prompt served
// from cache" when the measured share was 99.7318%. Those are different
// statements: 100% says nothing was ever recomputed, and 586 tokens were.
// A whole number is fine everywhere else, so the rule is narrow: a share only
// prints as 100% when it IS 100%, and only as 0% when it is 0%. Everything
// that merely rounds to an extreme gains a decimal place instead.
func sharePct(f float64) string {
	pct := 100 * f
	switch {
	case pct >= 100:
		return "100%"
	case pct > 99.5:
		return fmt.Sprintf("%.1f%%", pct)
	case pct <= 0:
		return "0%"
	case pct < 0.5:
		return fmt.Sprintf("%.1f%%", pct)
	default:
		return fmt.Sprintf("%.0f%%", pct)
	}
}
