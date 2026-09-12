package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// `replay route` answers whether switching models is worth it, and is careful
// about which half of that answer it is entitled to give.
//
// The structural half is dimensionless. Break-even trim thresholds and the
// cache-read inversion boundary are built from read multiples, write penalties,
// price ratios and the measured hit rate. No token count enters, so a model
// that tokenises the same text differently cannot move them. That half prints
// against a rate card alone.
//
// The dollar half needs sigma, the ratio between two tokenizers on the same
// content, and sigma is measured from this ledger or the figure is suppressed.
// It is not a constant on a rate card. Routing to a 3.3x cheaper model with a
// worse read multiple at a 99% cached share breaks even at sigma = 1.0627, so
// a plausible-looking 1.15 would not be a safety margin, it would be the
// deciding vote cast by a number nobody measured.
//
// It reads the ledger and nothing else: no proxy code, no network, no request
// ever rewritten. ADR-0003 admits only parameters the client left unset, and
// `model` is always client-set, so a proxy that rerouted one would be
// overriding the client. This command tells you what to change. You change it.
func runRoute(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("route", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the topology as JSON")
	to := fs.String("to", "", "the model to consider switching to")
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if *to == "" {
		return fmt.Errorf("--to <model> is required: %w", errUsage)
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("a transcript file or directory is required: %w", errUsage)
	}
	files, err := transcriptFiles(fs.Args())
	if err != nil {
		return err
	}

	corpus, err := gatherByModel(files)
	if err != nil {
		return err
	}
	if len(corpus.fits) == 0 {
		return fmt.Errorf("no sessions with usable token fits were found")
	}

	from := corpus.busiest()
	report := buildRoute(from, *to, corpus)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	if err := report.write(stdout); err != nil {
		return err
	}
	// The ask, on the same terms as the five other commands that hand a reader
	// an analysis: after the human-readable report, and never on --json, where
	// a funding line is corruption rather than persuasion.
	//
	// route shipped without it. It was the one value command whose funding line
	// no test could see, because the subtest that would have caught it called
	// route without --to, took the usage error, and skipped.
	//
	// Gated on the report having been written, not merely attempted. A failed
	// write means the reader got a truncated report or none, and asking for
	// money on a stream that has already broken is the refusal case arriving
	// through the io layer.
	_, err = io.WriteString(stdout, supportLine(describeResult("route"), stdout))
	return err
}

// usageAt is one request's provider-reported usage at the time it ran.
// Dollar figures on this command price each turn; the summed usage map
// remains the TTL-split and prefix arithmetic.
type usageAt struct {
	Usage transcript.Usage
	At    time.Time
}

// modelCorpus is the per-model evidence this command is allowed to use: a
// tokens-per-byte fit measured from provider-reported counts, and the cache
// hit rate the thresholds are computed at.
type modelCorpus struct {
	fits  map[string]analysis.TokenFit
	turns map[string]int
	// usage is each model's summed provider-reported usage. It is what the
	// projection is applied to, so it is measured on both counts: the tokens
	// came off the wire and so did sigma.
	usage map[string]transcript.Usage
	// byTurn is each request at the time it ran. Empty in fixtures that only
	// set usage: those price the aggregate at an unknown time (PriceFor).
	byTurn map[string][]usageAt
	hits   int
	total  int
}

func (c modelCorpus) hitRate() float64 {
	if c.total == 0 {
		return 0
	}
	return float64(c.hits) / float64(c.total)
}

// busiest names the model this corpus is mostly made of, which is the one a
// switch would be switching away from.
func (c modelCorpus) busiest() string {
	names := make([]string, 0, len(c.turns))
	for m := range c.turns {
		names = append(names, m)
	}
	// Sorted first so a tie resolves the same way on every run rather than
	// following map iteration order.
	sort.Strings(names)
	best := ""
	for _, m := range names {
		if best == "" || c.turns[m] > c.turns[best] {
			best = m
		}
	}
	return best
}

// gatherByModel pools each model's fit across sessions, weighted by the turns
// behind it.
//
// Pooling means of pooled ratios is coarser than refitting over every turn at
// once, and it is reported as an estimate for that reason. What makes sigma
// trustworthy is that both sides came off the wire.
//
// The uncertainty used to be a turn-weighted mean of each session's own
// relative error. That is an average of within-session noise and does not fall
// as sessions accumulate, so the pooled figure carried the same band over two
// sessions as over two hundred. analysis.PoolFits reports the standard error of
// the pooled ratio instead: how much the sessions disagree with one another,
// over how many independent sessions there effectively are.
func gatherByModel(files []string) (modelCorpus, error) {
	c := modelCorpus{
		fits:   map[string]analysis.TokenFit{},
		turns:  map[string]int{},
		usage:  map[string]transcript.Usage{},
		byTurn: map[string][]usageAt{},
	}
	samples := map[string][]analysis.FitSample{}

	err := forEachSession(files, func(_ string, _ *transcript.Session, rep *analysis.LaneReport, err error) error {
		if err != nil || rep == nil || rep.Lane == nil || len(rep.Lane.Requests) == 0 {
			return nil
		}
		model := rep.Lane.Requests[0].Model
		if model == "" || rep.Fit.Turns == 0 {
			return nil
		}
		samples[model] = append(samples[model], analysis.FitSample{
			TokensPerByte: rep.Fit.TokensPerByte,
			Turns:         rep.Fit.Turns,
			RelativeError: rep.Fit.RelativeError,
		})
		c.turns[model] += rep.Fit.Turns

		u := c.usage[model]
		for _, req := range rep.Lane.Requests {
			c.total++
			if req.Usage.CacheRead > 0 {
				c.hits++
			}
			u.Input += req.Usage.Input
			u.CacheCreation += req.Usage.CacheCreation
			u.CacheRead += req.Usage.CacheRead
			u.Output += req.Usage.Output
			c.byTurn[model] = append(c.byTurn[model], usageAt{Usage: req.Usage, At: req.Timestamp})
		}
		c.usage[model] = u
		return nil
	})
	if err != nil {
		return c, err
	}
	for m := range c.turns {
		c.fits[m] = analysis.PoolFits(samples[m])
	}
	return c, nil
}

// routeReport is what the command prints, in both shapes.
type routeReport struct {
	Schema    string            `json:"schema"`
	From      analysis.Topology `json:"from"`
	To        analysis.Topology `json:"to"`
	HitRate   float64           `json:"hit_rate"`
	Turns     int               `json:"turns"`
	Inversion *float64          `json:"inversion_share,omitempty"`
	// WinsAbove names the model that is cheaper per turn above the boundary.
	// A crossover without a direction reads equally well both ways round, and
	// the two ways round are opposite advice.
	WinsAbove string            `json:"wins_above,omitempty"`
	Dilation  analysis.Dilation `json:"dilation"`
	// Observed is what the source model actually cost over these turns at
	// list price, and Dollars is what the destination is projected to cost
	// for the same work. Both are nil whenever sigma is unmeasured: there is
	// no estimate and no default of 1.0, because an absolute cross-family
	// figure without a measured sigma is a guess with a currency symbol in
	// front of it. A projection is also meaningless alone, so the observed
	// figure it is measured against is always carried beside it.
	Observed *float64 `json:"observed_usd,omitempty"`
	Dollars  *float64 `json:"projected_usd,omitempty"`
	// Switch is what the move itself costs. Everything above is steady
	// state and charges nothing for arriving: a cache entry is keyed per
	// model, so the destination starts cold and rewrites the whole shared
	// prefix at the write multiple before one read discount applies. Nil
	// under the same rule as the dollar figures - no measured sigma, no
	// number.
	Switch *analysis.Switch `json:"switch,omitempty"`
	Notes  []string         `json:"notes,omitempty"`
}

func buildRoute(from, to string, c modelCorpus) routeReport {
	h := c.hitRate()
	r := routeReport{
		Schema:   "replay.route.v1",
		From:     analysis.TopologyOf(from, h),
		To:       analysis.TopologyOf(to, h),
		HitRate:  h,
		Turns:    c.total,
		Dilation: analysis.MeasureDilation(from, to, c.fits),
	}
	if r.From.Known && r.To.Known && r.From.InputPerMTok > 0 {
		ratio := r.To.InputPerMTok / r.From.InputPerMTok
		if share, ok := analysis.InversionShare(r.From.ReadMult, r.To.ReadMult, ratio); ok {
			r.Inversion = &share
			// Evaluated rather than inferred from which alpha is smaller:
			// the sign depends on the price ratio too, and reasoning about
			// it in prose is exactly how the direction gets written
			// backwards.
			r.WinsAbove = from
			if analysis.CrossRatio(r.From.ReadMult, r.To.ReadMult, ratio, math.Min(share+0.01, 0.999), 1) < 1 {
				r.WinsAbove = to
			}
		}
	}
	if !r.From.Known {
		r.Notes = append(r.Notes, "the rules do not price "+from+", so it has no topology")
	}
	if !r.To.Known {
		r.Notes = append(r.Notes, "the rules do not price "+to+", so it has no topology")
	}
	// The unmeasured case already prints its own reason in full; repeating
	// it as a note said the same sentence twice.
	if r.Dilation.Measured && r.From.Known && r.To.Known {
		u := c.usage[from]
		turns := c.byTurn[from]
		if len(turns) == 0 {
			// Fixtures that only set the summed usage have no per-request
			// clock. PriceForAt at zero time is PriceFor.
			turns = []usageAt{{Usage: u}}
		}
		var observed, projected float64
		var lastTo cachemodel.Price
		priced := false
		for _, turn := range turns {
			pFrom, okF := cachemodel.PriceForAt(from, turn.At)
			pTo, okT := cachemodel.PriceForAt(to, turn.At)
			if !okF || !okT {
				continue
			}
			// Every token count is scaled by the measured sigma: the same
			// work, counted by the destination's tokenizer.
			//
			// The TTL split is carried, not dropped. Without it CostUSD
			// cannot tell a one-hour write from a five-minute one and falls
			// back to the short multiplier, so this side of the comparison
			// priced writes at 1.25x while `observed` above priced the same
			// writes at 2x. Two multipliers, one comparison, and the error
			// ran in the direction that flatters the destination.
			//
			// The total is recomputed from the parts rather than scaled
			// separately, because the wire holds total == 5m + 1h on every
			// creation-bearing record and rounding each independently would
			// break that.
			observed += cachemodel.CostUSD(turn.Usage, pFrom)
			projected += cachemodel.CostUSD(scaleUsage(turn.Usage, r.Dilation.Sigma), pTo)
			lastTo = pTo
			priced = true
		}
		if priced {
			r.Observed, r.Dollars = &observed, &projected

			// The prefix the destination has to write once before it reads
			// anything: what the source was being served warm, per turn,
			// counted by the destination's tokenizer.
			if c.total > 0 {
				prefix := scaleTokens(u.CacheRead/c.total, r.Dilation.Sigma)
				// At the TTL this session actually wrote. A bare
				// CacheCreation carries no TTL, so it was priced at the
				// short multiplier regardless of what the source was doing
				// — which made the payback read faster than it is on any
				// session writing one-hour entries. The observed proportion
				// is kept rather than a dominant bucket chosen, so a mixed
				// session is not rounded to whichever tier happens to lead.
				write := transcript.Usage{CacheCreation: prefix}
				if tot := u.Create5m + u.Create1h; tot > 0 {
					write.Create1h = prefix * u.Create1h / tot
					write.Create5m = prefix - write.Create1h
				}
				cost := cachemodel.CostUSD(write, lastTo)
				sw := analysis.Payback(cost, observed, projected, c.total)
				r.Switch = &sw
			}
		}
	}
	return r
}

func (r routeReport) write(w io.Writer) error {
	p := &printer{w: w}
	p.printf("Hit rate %.2f%% over %d turns, from this ledger.\n\n", r.HitRate*100, r.Turns)

	p.printf("%-22s %16s %16s\n", "", short12(r.From.Model), short12(r.To.Model))
	if r.From.Known && r.To.Known {
		p.printf("%-22s %16.3f %16.3f\n", "cache read multiple", r.From.ReadMult, r.To.ReadMult)
		p.printf("%-22s %15.1f%% %15.1f%%\n", "break-even trim, 5m", r.From.BreakEvenShort*100, r.To.BreakEvenShort*100)
		p.printf("%-22s %15.1f%% %15.1f%%\n", "break-even trim, 1h", r.From.BreakEvenLong*100, r.To.BreakEvenLong*100)
	}
	p.printf("\nWrite penalty is %.2fx at 5m and %.2fx at 1h. It is a property of the\n", r.From.WriteShort, r.From.WriteLong)
	p.printf("request, not the model: the client chooses the TTL.\n")

	if r.Inversion != nil {
		loser := r.From.Model
		if r.WinsAbove == r.From.Model {
			loser = r.To.Model
		}
		p.printf("\nCache-read inversion at a %.2f%% cached share: %s is cheaper per turn\n", *r.Inversion*100, r.WinsAbove)
		p.printf("above it, %s below. Long, heavily cached sessions sit above; short\n", loser)
		p.printf("ones sit below, so a single verdict for both would be wrong.\n")
	}

	p.printf("\nsigma (tokenizer dilation, %s -> %s): ", r.Dilation.From, r.Dilation.To)
	if r.Dilation.Measured {
		p.printf("%.4f +/-%.0f%% from %d and %d turns\n", r.Dilation.Sigma, r.Dilation.RelativeError*100, r.Dilation.FromTurns, r.Dilation.ToTurns)
		if r.Observed != nil && r.Dollars != nil {
			delta := *r.Dollars - *r.Observed
			verb := "more"
			if delta < 0 {
				delta, verb = -delta, "less"
			}
			p.printf("\nOver these turns %s cost $%.2f at list price. The same work on %s\n", r.From.Model, *r.Observed, r.To.Model)
			p.printf("projects to $%.2f, which is $%.2f %s. Carrying sigma's +/-%.0f%%, so the\n", *r.Dollars, delta, verb, r.Dilation.RelativeError*100)
			p.printf("figure is a bound to argue with, not an invoice.\n")
		}
		if sw := r.Switch; sw != nil {
			p.printf("\nThe move itself costs $%.2f: %s starts with a cold cache and writes the\n", sw.CostUSD, r.To.Model)
			p.printf("shared prefix again before it reads any of it.\n")
			switch {
			case !sw.PaysBack:
				p.printf("It never earns that back, because %s is not cheaper per turn here.\n", r.To.Model)
			case sw.WithinObserved:
				p.printf("At $%.4f saved per turn it repays on turn %d, inside the %d turns\n",
					sw.SavingPerTurnUSD, sw.PaybackTurns, r.Turns)
				p.printf("measured. Switching is worth it for work this long or longer.\n")
			default:
				p.printf("At $%.4f saved per turn it repays on turn %d - beyond the %d turns\n",
					sw.SavingPerTurnUSD, sw.PaybackTurns, r.Turns)
				p.printf("measured here. Cheaper per turn and still the wrong move for sessions\n")
				p.printf("this short, which is what a price-only comparison hides.\n")
			}
		}
	} else {
		p.printf("unmeasured\n")
		p.printf("Dollar figures are suppressed. %s\n", r.Dilation.Why)
		p.printf("Run both models over comparable work and this fills itself in.\n")
	}
	for _, n := range r.Notes {
		p.printf("\nnote: %s\n", n)
	}
	return p.err
}

type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

// Model ids run to about sixteen characters, so the columns are cut to fit
// one rather than truncating every name in the table.
// scaleTokens applies sigma to one count. Rounded rather than truncated so a
// projection does not drift systematically low across four fields.
func scaleTokens(n int, sigma float64) int {
	return int(math.Round(float64(n) * sigma))
}

// scaleUsage counts the same work with the destination's tokenizer.
//
// The TTL split is carried rather than dropped. Without it CostUSD cannot
// tell a one-hour write from a five-minute one and falls back to the short
// multiplier, so the projected side of a route comparison priced writes at
// 1.25x while the observed side priced the same writes at 2x. Two
// multipliers, one comparison, and the error ran in the direction that
// flatters the destination model.
//
// CacheCreation is recomputed from the parts when a split exists, because
// the wire holds total == 5m + 1h on every creation-bearing record — all
// 59,348 of them on the machine this was measured on — and scaling three
// numbers independently breaks that at any sigma that is not a whole
// number.
func scaleUsage(u transcript.Usage, sigma float64) transcript.Usage {
	out := transcript.Usage{
		Input:          scaleTokens(u.Input, sigma),
		CacheCreation:  scaleTokens(u.CacheCreation, sigma),
		CacheRead:      scaleTokens(u.CacheRead, sigma),
		Output:         scaleTokens(u.Output, sigma),
		Create5m:       scaleTokens(u.Create5m, sigma),
		Create1h:       scaleTokens(u.Create1h, sigma),
		ThinkingTokens: scaleTokens(u.ThinkingTokens, sigma),
	}
	if out.Create5m+out.Create1h > 0 {
		out.CacheCreation = out.Create5m + out.Create1h
	}
	return out
}

func short12(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:15] + "…"
}
