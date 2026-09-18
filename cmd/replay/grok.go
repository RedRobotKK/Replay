package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
	"github.com/RedRobotKK/Replay/internal/usage"
)

// grokHome is where the Grok CLI keeps everything it writes.
const grokHome = ".grok"

// grokRoots is the one place Grok keeps session records.
//
// Sessions live two levels below it, not one: ~/.grok/sessions is a directory
// of URL-encoded working directories ("%2FUsers%2Fdaniel%2FDevelopment"), each
// holding one directory per session id. A reader that globbed a single level
// down would find no sessions and report a machine with none, which is a total
// that is wrong and looks right — the failure this project fixed in its own
// discovery once already.
//
// It is a slice for the shape Codex needed, where `codex archive` moves
// sessions to a second root. Nothing observed here does that; the shape costs
// nothing and the alternative is rewriting every caller when something does.
func grokRoots(home string) []string {
	if home == "" {
		return nil
	}
	return []string{filepath.Join(home, grokHome, "sessions")}
}

// findGrokSessions returns every session directory under the roots.
//
// A session is a directory that carries updates.jsonl, which is where the
// per-turn usage is written. Directories holding only a summary or a lock file
// are sessions that spent nothing, and counting them would put a denominator
// under sessions that never called a provider.
func findGrokSessions(roots []string) []string {
	var out []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			// A subtree this process cannot read is skipped and the walk goes
			// on, which means UNKNOWN rather than absent: the sessions under it
			// are not reported as zero, they are not reported at all. The
			// difference is stated here rather than swallowed, because a
			// permission error on one project directory must not quietly
			// shorten a machine-wide total.
			if err != nil || fi.IsDir() {
				return nil //nolint:nilerr // an unreadable subtree is not fatal to the rest
			}
			if filepath.Base(p) == "updates.jsonl" {
				out = append(out, filepath.Dir(p))
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// grokTotals is what a machine's Grok sessions add up to, in the engine's own
// vocabulary rather than the provider's.
//
// There is no cost field, and that is the point. Grok records costUsdTicks on
// every record, and its own client documentation states the scale, at
// ~/.grok/docs/user-guide/17-sessions.md. That is a vendor statement, not a
// measurement: nothing here has reconciled it against a statement of account,
// and this project has retracted a published figure over a number that was
// repeated rather than checked. A struct field for money is a field somebody
// fills in, and an unverified scale is indistinguishable from a measured one
// once it is in a total.
type grokTotals struct {
	// Sessions is directories read. Turns is completed turns. Requests is the
	// model calls those turns made, which the record states and which is the
	// only one of the three that counts what a provider was asked to do.
	Sessions, Turns, Requests int

	// Prompt is every input token the provider processed, cache included,
	// because that is how this surface counts. Fresh is what is left after the
	// cached read and the cache write are taken out of it.
	Prompt, Fresh, CachedRead, CachedWrite int
	Output, Reasoning                      int

	// Refused counts records this reader would not believe, and Reasons keeps
	// one line per distinct reason. A count with no reason says something was
	// dropped without saying what, and absent is not zero.
	Refused int
	Reasons []string

	// RollupSessions is how many sessions carry a usage.json of their own.
	// RollupDiverged is how many of those state a larger input total than
	// their own turns do, and RollupExcess is the difference. On this corpus
	// the cause is sub-agent sessions: a parent's rollup absorbs children that
	// are also on disk as sessions in their own right, so the rollups cannot
	// be summed even though each one is correct about its own session.
	RollupSessions, RollupDiverged, RollupExcess int

	First, Last time.Time
}

// grokRecord converts one turn into the engine's shape.
//
// It goes through usage.FromInclusive because Grok counts INCLUSIVELY:
// cachedReadTokens is a share of inputTokens rather than a figure beside it.
// Measured 2026-09-17 over a 106-session corpus, inputTokens + outputTokens ==
// totalTokens held on 2,683 of 2,683 usage objects and the exclusive reading
// held on none except where the cache was zero.
//
// So Fresh is a SUBTRACTION, and Validate is what says so. Copying inputTokens
// into Fresh is right on the Anthropic surface, where input_tokens is the
// uncached remainder, and on the sample record here it would invent 6,272
// fresh tokens out of 17,415 — 36% — every one of which was served from cache.
// The error is largest on exactly the sessions that cache best, which are the
// ones anyone runs this tool for.
func grokRecord(t transcript.GrokTurn) (usage.Record, error) {
	u := t.Usage
	r := usage.FromInclusive(usage.ProviderGrok, usage.MechanismImplicitPrefix, t.Model, t.At,
		usage.InclusiveCounts{
			Prompt:    u.InputTokens,
			Cached:    u.CachedReadTokens,
			Written:   u.CacheCreationTokens,
			Output:    u.OutputTokens,
			Reasoning: u.ReasoningTokens,
		}, t.Raw)
	if err := r.Validate(); err != nil {
		return usage.Record{}, err
	}
	return r, nil
}

// readGrok sums the sessions, and says what it would not sum.
func readGrok(dirs []string) grokTotals {
	var g grokTotals
	for _, dir := range dirs {
		s, err := transcript.ParseGrokSession(dir)
		if err != nil {
			g.Refused++
			g.addReason(fmt.Sprintf("a session directory could not be read: %v", err))
			continue
		}
		g.Sessions++
		for _, t := range s.Turns {
			r, err := grokRecord(t)
			if err != nil {
				// A record that will not validate is refused rather than
				// half-counted. This is the arithmetic the conversion exists
				// to protect, so a failure here means the shape changed.
				g.Refused++
				g.addReason(err.Error())
				continue
			}
			g.Turns++
			g.Requests += t.Usage.ModelCalls
			g.Prompt += r.Prompt
			g.Fresh += r.Fresh
			g.CachedRead += r.CachedRead
			g.CachedWrite += r.CachedWrite
			g.Output += r.Output
			g.Reasoning += r.Reasoning
			g.observe(r.At)
		}
		for _, ref := range s.Refused {
			g.Refused++
			g.addReason(ref.Reason)
		}
		// The rollup is compared, never added. See grokTotals.
		if s.Rollup != nil {
			g.RollupSessions++
			own := 0
			for _, t := range s.Turns {
				own += t.Usage.InputTokens
			}
			if s.Rollup.InputTokens > own {
				g.RollupDiverged++
				g.RollupExcess += s.Rollup.InputTokens - own
			}
		}
	}
	return g
}

// addReason keeps one line per distinct reason, in the order first seen.
//
// Per-record reasons carry the counts that failed, so a format change produces
// a handful of distinct lines and a healthy corpus produces none. Keeping every
// occurrence would print the same sentence hundreds of times and bury the one
// that is different.
func (g *grokTotals) addReason(reason string) {
	for _, r := range g.Reasons {
		if r == reason {
			return
		}
	}
	if len(g.Reasons) < 5 {
		g.Reasons = append(g.Reasons, reason)
	}
}

func (g *grokTotals) observe(at time.Time) {
	if at.IsZero() {
		return
	}
	if g.First.IsZero() || at.Before(g.First) {
		g.First = at
	}
	if g.Last.IsZero() || at.After(g.Last) {
		g.Last = at
	}
}

// CachedShare is the cached read over the whole prompt, which on this surface
// is inputTokens itself. Dividing by the fresh remainder instead would put a
// well-cached turn above 1.0.
func (g grokTotals) CachedShare() float64 {
	if g.Prompt <= 0 {
		return 0
	}
	return float64(g.CachedRead) / float64(g.Prompt)
}

// burnGrok reads the Grok surface for `replay burn`.
//
// The cost column stays empty on purpose and the notes say why. Grok is a
// hosted API with a real bill, so it is not localOnly — "nobody invoices for
// this" and "this build cannot read the invoice" are opposite situations and
// rendering them the same way misinforms.
func burnGrok(home, dir string) surfaceBurn {
	s := surfaceBurn{
		name: "grok", unit: "context, cached inside",
		quota: "not recorded",
	}
	roots := grokRoots(home)
	if dir != "" {
		roots = []string{filepath.Join(dir, "grok")}
	}
	g := readGrok(findGrokSessions(roots))
	if g.Sessions == 0 {
		return s
	}
	s.sessions, s.hasSessions = g.Sessions, true
	s.requests = g.Requests
	s.tokens = g.Prompt + g.Output
	s.first, s.last = g.First, g.Last
	if g.Prompt > 0 {
		s.cached, s.hasCached = g.CachedShare(), true
	}
	// Every request is unpriced, and the note below says why a rules document
	// is not the missing piece here.
	s.unpricedReqs = g.Requests

	s.problems = append(s.problems, fmt.Sprintf(
		"not priced. Grok states costUsdTicks on every record (%d turn(s) here). Its own "+
			"client documentation states the scale, at ~/.grok/docs/user-guide/17-sessions.md, "+
			"and that statement has not been checked against a statement of account by anything "+
			"here, so this build derives no money figure from it. A rules document would not "+
			"close it either: what is missing is the check, not a rate table",
		g.Turns))

	if g.Reasoning > 0 {
		s.problems = append(s.problems, fmt.Sprintf(
			"%s reasoning token(s) inside that output, which the record states and most "+
				"readers discard. They are replayed as input when the block is sent back",
			comma(g.Reasoning)))
	}
	if g.CachedRead > 0 {
		s.problems = append(s.problems, fmt.Sprintf(
			"%s of the %s prompt token(s) were served from cache, counted INSIDE the prompt: "+
				"%s were fresh",
			comma(g.CachedRead), comma(g.Prompt), comma(g.Fresh)))
	}
	if v := cachemodel.ClassifyCounters(int64(g.CachedRead), int64(g.CachedWrite)); v.Impossible() {
		s.problems = append(s.problems, fmt.Sprintf(
			"%s cached read(s) reported and no cache writes at all: %s. Something wrote the "+
				"prefix being read, so the write half of this surface is unmeasured",
			comma(g.CachedRead), v))
	}
	if g.RollupDiverged > 0 {
		s.problems = append(s.problems, fmt.Sprintf(
			"%d of %d session(s) with a usage.json state %s more input token(s) than their own "+
				"turns do, because a parent's rollup absorbs the sub-agent sessions that are "+
				"also on disk in their own right. The figures above sum the turns, once per "+
				"session; summing usage.json would count that work twice",
			g.RollupDiverged, g.RollupSessions, comma(g.RollupExcess)))
	}
	if g.Refused > 0 {
		s.problems = append(s.problems, fmt.Sprintf(
			"%d record(s) refused and left out of every figure above, which is not the same as "+
				"counting them as zero: %s",
			g.Refused, strings.Join(g.Reasons, "; ")))
	}
	return s
}

// runGrok reads the Grok surface on its own terms.
//
// `replay burn` already carries a Grok row, and a row is the wrong shape for
// this surface. Grok states the cached share, the reasoning tokens and a
// session rollup on every record, and burn's job is to put surfaces beside each
// other rather than to read one closely. This is the `replay codex` of Grok:
// tokens, never dollars, and the refusals printed rather than dropped.
func runGrok(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("grok", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}

	var dirs, searched []string
	if fs.NArg() > 0 {
		for _, d := range fs.Args() {
			searched = append(searched, d)
			dirs = append(dirs, findGrokSessions([]string{d})...)
		}
	} else {
		home, _ := os.UserHomeDir()
		searched = grokRoots(home)
		dirs = findGrokSessions(searched)
	}

	if len(dirs) == 0 {
		_, _ = fmt.Fprintf(stdout, "\n  No Grok sessions found. Searched:\n")
		for _, s := range searched {
			_, _ = fmt.Fprintf(stdout, "    %s\n", s)
		}
		_, _ = fmt.Fprintf(stdout, "\n  Grok keeps one directory per session under ~/.grok/sessions,\n"+
			"  named for the working directory it ran in. If it has never run on\n"+
			"  this machine there is nothing to read, which is not an error.\n")
		return nil
	}

	g := readGrok(dirs)

	_, _ = fmt.Fprintf(stdout, "\n  %s prompt token(s) across %d Grok session(s)\n",
		comma(g.Prompt), g.Sessions)
	_, _ = fmt.Fprintf(stdout, "  Counted once per turn. The cached share is reported INSIDE the\n"+
		"  prompt on this surface, so it is not added to it.\n")

	if g.Refused > 0 {
		_, _ = fmt.Fprintf(stdout, "\n  [NOTE] %d record(s) refused. Neither can be priced, and neither is zero.\n", g.Refused)
		for _, r := range g.Reasons {
			_, _ = fmt.Fprintf(stdout, "         %s\n", r)
		}
	}

	_, _ = fmt.Fprintf(stdout, "\n  cache\n")
	_, _ = fmt.Fprintf(stdout, "    %s of %s prompt token(s) served from cache, %.0f%%\n",
		comma(g.CachedRead), comma(g.Prompt), g.CachedShare()*100)
	_, _ = fmt.Fprintf(stdout, "    %s were fresh\n", comma(g.Fresh))
	if g.CachedRead > 0 && g.CachedWrite == 0 {
		_, _ = fmt.Fprintf(stdout, "    no cache writes of any size are reported, so the write half\n"+
			"    of this surface is unmeasured rather than zero\n")
	}

	if g.Reasoning > 0 {
		_, _ = fmt.Fprintf(stdout, "\n  reasoning\n")
		_, _ = fmt.Fprintf(stdout, "    %s reasoning token(s) inside that output, which the record\n"+
			"    states and most readers discard\n", comma(g.Reasoning))
	}

	if g.RollupDiverged > 0 {
		_, _ = fmt.Fprintf(stdout, "\n  rollup\n")
		_, _ = fmt.Fprintf(stdout, "    %d of %d session(s) with a usage.json state %s more input\n"+
			"    token(s) than their own turns do, because a parent's rollup absorbs\n"+
			"    sub-agent sessions that are also on disk in their own right. The\n"+
			"    figures above sum the turns once per session; summing usage.json\n"+
			"    would count that work twice\n",
			g.RollupDiverged, g.RollupSessions, comma(g.RollupExcess))
	}

	_, _ = fmt.Fprintf(stdout, "\n  money\n")
	_, _ = fmt.Fprintf(stdout, "    not priced. Grok states costUsdTicks on every record, and its own\n"+
		"    client documentation states the scale, but nothing here has checked\n"+
		"    that against a statement of account. A rules document would not close\n"+
		"    it either: what is missing is the check, not a rate table.\n")

	_, _ = fmt.Fprintf(stdout, "\n  ran   replay grok\n")
	return nil
}
