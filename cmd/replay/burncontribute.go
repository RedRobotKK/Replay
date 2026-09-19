package main

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// codexContribution builds a corpus submission from a Codex corpus.
//
// Until this existed only a Claude Code corpus could contribute, so the pool
// could not answer what Astra costs against what Fable costs however many
// people took part: `replay cost --contribute` reads Claude Code transcripts
// and nothing else, and every pooled row therefore came from one vendor.
//
// THE ARITHMETIC IS NOT THE ARITHMETIC ON THE CLAUDE PATH, deliberately.
//
// That path sets RebilledShare to RebilledUSD over TotalUSD where the numerator
// prices a break's deficit at the full base input rate and the denominator is
// what was actually spent, which on a well-cached session is mostly cache reads
// at a tenth of that rate. Two price scales in one ratio, and it exceeds 1 on a
// long session that cached well and broke badly: measured at 1.504 on a real
// four-day session, 2026-09-17.
//
// Here the re-billed figure is the cold tokens priced at the same model's input
// rate, and those tokens are ALREADY INSIDE the total, because a cold read is
// billed as fresh input and CostUSD charges it at exactly that rate. The share
// is therefore a fraction of spend by construction. contributeCorpus still
// guards the range, and this path is built so the guard has nothing to catch
// rather than relying on it.
//
// A task is a session. That is the unit this surface counts in, and the
// submission says so through TagBasis rather than pretending it is the same
// unit a Claude Code lane is.
func codexContribution(home, dir string) (corpusFigures, error) {
	roots := codexRoots(home)
	if dir != "" {
		roots = []string{filepath.Join(dir, "codex")}
	}
	files := findCodexRollouts(roots)
	if dir != "" && len(files) == 0 {
		// The error is discarded because the only one Glob returns is
		// ErrBadPattern, and the pattern here is a constant that is not a bad
		// one. A directory that does not exist or cannot be read yields no
		// matches and no error, which the emptiness check below already
		// answers. Documented rather than handled: there is no branch to write.
		files, _ = filepath.Glob(filepath.Join(dir, "codex", "*.jsonl")) //nolint:errcheck // constant pattern, see above
	}
	if len(files) == 0 {
		return corpusFigures{}, fmt.Errorf(
			"no Codex rollouts found. Looked in %v. A corpus with no sessions is nothing to pool: %w",
			roots, errUsage)
	}

	var f corpusFigures
	sessionsRead := 0
	models := map[string]int{}
	costs := make([]float64, 0, len(files))
	unpricedSessions := 0

	for _, p := range files {
		r, err := transcript.ParseCodexFile(p)
		if err != nil {
			// Counted, never silently skipped. TestED1 enforces this on the
			// contribution path by name, and the reason it gives is this exact
			// failure: a skipped transcript becomes a published figure with no
			// note that anything was skipped. A pool cannot tell a corpus of
			// 170 from a corpus of 176 where 6 were unreadable.
			f.Unreadable++
			continue
		}
		// A file that parses is not the same as a file that was fully read.
		// ParseCodexFile returns no error for a record it refused; it counts
		// them, and a corpus with refused records is a weaker measurement than
		// one without. Both states reach the contributor rather than one of
		// them being rounded into the other.
		f.Unreadable += r.Skipped
		sessionsRead++

		price, ok := cachemodel.PriceFor(r.Model)
		if !ok || r.Model == "" {
			// A session with no price is counted as unpriced rather than
			// priced against a default. A guessed model is a wrong figure with
			// no way for a reader to see it is wrong, which is the rule
			// burnCodex already follows.
			unpricedSessions++
			f.Unpriced += r.Turns
			if r.Model != "" {
				models[r.Model] += r.Turns
			}
			continue
		}

		// Tasks counts the sessions the money covers. TotalUSD is the sum over
		// priced sessions, so counting the unpriced ones here would give a pool
		// a per-task cost lower than anything that happened, and the error
		// grows with how much of the corpus cannot be priced. On a real Codex
		// machine that is most of it.
		// A session whose billing basis is not established is excluded for the
		// same reason an unpriced one is, and the paragraph above gives it:
		// counting it in Tasks while contributing nothing to TotalUSD hands the
		// pool a per-task cost lower than anything that happened. Here the
		// figure is not merely absent, it is one nobody can defend, and a pool
		// is the last place to put one.
		if !r.BasisEstablished {
			f.UnmeasuredSessions++
			continue
		}
		f.Tasks++
		cost := cachemodel.CostUSD(r.Billed, price)
		f.TotalUSD += cost
		costs = append(costs, cost)
		models[r.Model] += r.Turns

		// Cold tokens are the ones a break made the provider re-read at the
		// fresh input rate. Priced at that same rate, which is what they were
		// actually billed at and what CostUSD already charged for them above.
		for _, b := range r.Breaks {
			f.RebilledUSD += float64(b.ColdTokens) / 1_000_000 * price.InputPerMTok
		}
	}

	// An all-refused corpus is NOT an unpriced one, and must not be sent to the
	// rules document. Installing one cannot establish a billing basis: the gap
	// is the token counts, not the rates. Collapsing the two would print the
	// one piece of advice guaranteed not to work.
	if f.TotalUSD <= 0 && f.UnmeasuredSessions > 0 && unpricedSessions == 0 {
		return corpusFigures{}, fmt.Errorf(
			"no session in this Codex corpus has an establishable billing basis, so there is "+
				"nothing to pool. %d session(s) read, %d of them NOT MEASURED. Codex broadcasts "+
				"its usage state, and a broadcast following no response repeats the previous one, "+
				"so a reader summing that field counts the response twice; where this build cannot "+
				"tell a repeat from a second identical response it declines the session rather than "+
				"guessing. A rules document does not change this: %w",
			sessionsRead, f.UnmeasuredSessions, errUsage)
	}
	if f.TotalUSD <= 0 {
		return corpusFigures{}, fmt.Errorf(
			"nothing in this Codex corpus can be priced, so there is no total to pool. "+
				"%d session(s) read, %d of them naming a model no table carries.\n"+
				"Install a rules document and run this again:\n"+
				"  replay rules --update docs/rules/openai-2026-09-15.json\n"+
				"Installing one REPLACES the table in effect rather than merging into it, so a document "+
				"naming only OpenAI leaves every Anthropic model unpriced. Note also that OpenAI publishes "+
				"no rate for gpt-5.1-codex-mini, which is the most common model on a Codex machine, and no "+
				"document can price what nobody publishes: %w",
			sessionsRead, unpricedSessions, errUsage)
	}

	sort.Float64s(costs)
	f.MedianTaskUSD = percentile(costs, 0.5)
	f.RebilledShare = f.RebilledUSD / f.TotalUSD
	f.Surfaces = []string{"codex"}
	if len(models) > 0 {
		f.Models = capModels(models)
	}
	return f, nil
}

// capModels holds a histogram to the 24 entries the receiver accepts.
//
// Truncated to the busiest, and the tail dropped rather than folded into an
// "other" bucket: a bucket named "other" is a model id that is not one, and the
// pool would have to special-case it forever. Ordered by count then name, so
// two machines with the same corpus truncate identically rather than however
// the map happened to yield.
func capModels(counts map[string]int) map[string]int {
	const maxModels = 24
	if len(counts) <= maxModels {
		return counts
	}
	type row struct {
		model string
		n     int
	}
	rows := make([]row, 0, len(counts))
	for m, n := range counts {
		rows = append(rows, row{m, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].model < rows[j].model
	})
	out := make(map[string]int, maxModels)
	for _, r := range rows[:maxModels] {
		out[r.model] = r.n
	}
	return out
}

// writeCodexContributionNote says what was written, what it holds, and that
// nothing was sent.
//
// The same shape as the note on the Claude path, because a contributor who has
// used one should not have to read the other carefully to see what changed.
// What differs is the unit and the surface, and both are stated rather than
// left for the reader to infer from a filename.
func writeCodexContributionNote(w io.Writer, path string, supersedes []string, f corpusFigures) {
	_, _ = fmt.Fprintf(w, "\n  Wrote %s\n", path)
	_, _ = fmt.Fprintf(w, "    surface   codex\n")
	_, _ = fmt.Fprintf(w, "    sessions  %d priced, and %d request(s) left out on a model no table carries\n", f.Tasks, f.Unpriced)
	_, _ = fmt.Fprintf(w, "    spend     %s at list, %s of it re-billed (%.1f%%)\n",
		usdText(f.TotalUSD), usdText(f.RebilledUSD), f.RebilledShare*100)
	if len(f.Models) > 0 {
		_, _ = fmt.Fprintf(w, "    models    %d named, with a record count each\n", len(f.Models))
	}
	if f.Unreadable > 0 {
		_, _ = fmt.Fprintf(w, "    UNREAD    %d record(s) refused or unparseable, in no figure above\n", f.Unreadable)
	}
	if f.UnmeasuredSessions > 0 {
		// Its own line, beside UNREAD rather than folded into the priced or
		// unpriced counts. A session here is not cheap and not free; its token
		// basis could not be established, so it carries no figure at all.
		_, _ = fmt.Fprintf(w, "    UNMEASURED %d session(s) left out: their billing basis could not be\n"+
			"               established, so they are in no figure above and no price table\n"+
			"               changes that\n", f.UnmeasuredSessions)
	}
	for _, old := range supersedes {
		_, _ = fmt.Fprintf(w, "    supersedes %s\n", old)
	}
	_, _ = fmt.Fprintf(w, "\n  A task here is a SESSION, which is not the unit `replay cost --contribute`\n"+
		"  counts. That command counts agent lanes on a Claude Code corpus, and a\n"+
		"  pool that added the two would be summing two different things.\n")
	_, _ = fmt.Fprintf(w, "\n  Nothing was sent. Read it, and if you are happy with it, post it yourself.\n")
}

// usdText prints a dollar figure at the precision a small one needs.
//
// A contributed Codex corpus can be worth a few cents, and "$0.00" for a real
// reading is a figure that reads as nothing measured.
func usdText(v float64) string {
	if v > 0 && v < 0.01 {
		return fmt.Sprintf("$%.4f", v)
	}
	return fmt.Sprintf("$%.2f", v)
}
