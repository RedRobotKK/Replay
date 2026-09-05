package cachemodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
)

// Rules are the provider's published numbers, as a dated document rather than
// compiled constants.
//
// Every figure this tool prints is built on these, and they change on a cadence
// faster than release cycles. Compiling them in means a provider changing one
// number requires a binary release, and until that release every report is
// quietly wrong. Loading them from a file means the same correction is a
// download, and the version string on every report says which document produced
// the numbers.
//
// The compiled table stays as the fallback, so a machine that has never run
// `replay rules --update` behaves exactly as before.
type Rules struct {
	Schema    string      `json:"schema"`
	Version   string      `json:"version"`
	Provider  string      `json:"provider,omitempty"`
	Source    string      `json:"source,omitempty"`
	FetchedAt string      `json:"fetchedAt,omitempty"`
	Models    []ModelRule `json:"models"`
	// AccountDiscount is a negotiated rate multiplier the operator states,
	// between 0 and 1 exclusive. Zero means none.
	//
	// Replay cannot observe this. A committed-spend or enterprise rate is
	// private and never appears on the wire, so the only honest options are to
	// ignore it or to let the account holder declare it — and ignoring it
	// overstates every figure for anyone who has one. Declaring it changes the
	// document's price tier to `declared`.
	AccountDiscount float64 `json:"accountDiscount,omitempty"`
}

// ModelRule is one row of the table: what a model id matches, its caching
// floor, and its prices.
type ModelRule struct {
	Match         string  `json:"match"`
	MinPrefix     int     `json:"minPrefix"`
	InputPerMTok  float64 `json:"inputPerMTok,omitempty"`
	OutputPerMTok float64 `json:"outputPerMTok,omitempty"`
	ReadMult      float64 `json:"readMult,omitempty"`
	Priced        bool    `json:"priced,omitempty"`
	// MinPrefixClaim carries what the provider documents about the minimum
	// cacheable prefix alongside what replaying real traffic bounded it to.
	// Optional: a row without one behaves exactly as before.
	MinPrefixClaim *Claim `json:"minPrefixClaim,omitempty"`
	// EffectiveFrom and EffectiveUntil bound when this row applies, as
	// YYYY-MM-DD. Both empty means always, which is every row written before
	// dated pricing existed.
	//
	// A vendor promotion is a pricing event, not a fact about traffic: the
	// ledger records tokens and timings, and those do not change because
	// someone ran a sale. So a promotion is a dated row here, and a request is
	// priced by the rules in effect at ITS OWN timestamp. Without that, a
	// report spanning the end of a promotion prices the whole period at one
	// rate and is wrong on one side of the boundary whichever rate it picks.
	EffectiveFrom  string `json:"effectiveFrom,omitempty"`
	EffectiveUntil string `json:"effectiveUntil,omitempty"`
}

// windowContains reports whether this row applies at t.
//
// The window is inclusive at both ends and interpreted in UTC. A row with no
// dates applies always.
func (m ModelRule) windowContains(t time.Time) bool {
	from, until, err := m.window()
	if err != nil {
		return false
	}
	if from != nil && t.Before(*from) {
		return false
	}
	if until != nil && t.After(*until) {
		return false
	}
	return true
}

// window parses the row's dates. The end date is taken as the last instant of
// that day, so "until 2026-09-30" includes the whole of the 30th — which is
// what a person writing a promotion end date means.
func (m ModelRule) window() (from, until *time.Time, err error) {
	if m.EffectiveFrom != "" {
		t, perr := time.Parse("2006-01-02", m.EffectiveFrom)
		if perr != nil {
			return nil, nil, fmt.Errorf("effectiveFrom %q is not YYYY-MM-DD", m.EffectiveFrom)
		}
		from = &t
	}
	if m.EffectiveUntil != "" {
		t, perr := time.Parse("2006-01-02", m.EffectiveUntil)
		if perr != nil {
			return nil, nil, fmt.Errorf("effectiveUntil %q is not YYYY-MM-DD", m.EffectiveUntil)
		}
		end := t.Add(24*time.Hour - time.Nanosecond)
		until = &end
	}
	return from, until, nil
}

// dated reports whether the row carries any window at all.
func (m ModelRule) dated() bool { return m.EffectiveFrom != "" || m.EffectiveUntil != "" }

// PriceAt returns the price for a model at a moment in time.
//
// A dated row wins over an undated one while it is in effect, so a promotion
// beats the base rate inside its window and the base rate returns after it.
// Ordering is by specificity rather than file position: a figure that depends
// on which line came first is not a figure anyone should act on, and validate
// refuses two windows that cover the same instant for the same model.
//
// AccountDiscount, when the operator has stated one, is applied last. It is a
// negotiated rate that is invisible on the wire, so PriceTier reports
// `declared` and no reader mistakes it for something measured.
func (r *Rules) PriceAt(model string, t time.Time) (Price, bool) {
	if r == nil {
		return Price{}, false
	}
	lower := strings.ToLower(model)
	var fallback *ModelRule
	for i := range r.Models {
		m := r.Models[i]
		if !strings.Contains(lower, strings.ToLower(m.Match)) {
			continue
		}
		if !m.windowContains(t) {
			continue
		}
		if m.dated() {
			return r.applyDiscount(priceOf(m)), m.Priced
		}
		if fallback == nil {
			fallback = &r.Models[i]
		}
	}
	if fallback == nil {
		return Price{}, false
	}
	return r.applyDiscount(priceOf(*fallback)), fallback.Priced
}

func priceOf(m ModelRule) Price {
	return Price{InputPerMTok: m.InputPerMTok, OutputPerMTok: m.OutputPerMTok, ReadMult: m.ReadMult}
}

func (r *Rules) applyDiscount(p Price) Price {
	if r.AccountDiscount <= 0 || r.AccountDiscount >= 1 {
		return p
	}
	p.InputPerMTok *= r.AccountDiscount
	p.OutputPerMTok *= r.AccountDiscount
	return p
}

// PriceTier names how much weight a dollar figure from this document deserves.
//
// `declared` when an account discount has been stated: a negotiated rate is
// private, invisible on the wire, and cannot be checked by anyone but the
// account holder. It is applied because the operator asked for it, and
// labelled because nobody else can verify it.
func (r *Rules) PriceTier() string {
	if r != nil && r.AccountDiscount > 0 && r.AccountDiscount < 1 {
		return "declared"
	}
	return "documented"
}

// RulesSchema is the only document shape this build will load. A file that
// claims a different one is refused rather than interpreted optimistically: a
// rules file is not a place to guess.
const RulesSchema = "replay.rules.v1"

// LoadRules reads a rules document, or returns (nil, nil) when there is none.
//
// A missing file is the normal case and not an error. Anything present but not
// fully valid is refused: a stale rules file announces itself in the version
// string on every report, while a wrong one does not, so the wrong one is the
// more dangerous of the two and does not get the benefit of the doubt.
func LoadRules(path string) (*Rules, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read rules %s: %w", path, err)
	}
	var r Rules
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse rules %s: %w", path, err)
	}
	if err := r.validate(); err != nil {
		return nil, fmt.Errorf("rules %s: %w", path, err)
	}
	return &r, nil
}

func (r *Rules) validate() error {
	if r.Schema != RulesSchema {
		return fmt.Errorf("schema is %q, this build reads %q", r.Schema, RulesSchema)
	}
	if strings.TrimSpace(r.Version) == "" {
		return errors.New("version is required: a report has to be able to name the rules that produced it")
	}
	if len(r.Models) == 0 {
		return errors.New("no model rows: a rules file with nothing in it would silently disable pricing")
	}
	if r.AccountDiscount < 0 || r.AccountDiscount >= 1 {
		return fmt.Errorf("accountDiscount is %v; it is a multiplier strictly between 0 and 1, and a value outside that "+
			"is far more likely to be a typo than a deal — a negative one turns spend into savings", r.AccountDiscount)
	}
	// Two dated rows covering the same instant for the same model make the
	// price depend on file order. A figure that depends on which line came
	// first is not one anyone should act on, so it is refused rather than
	// resolved by a rule nobody would guess.
	for i, a := range r.Models {
		if !a.dated() {
			continue
		}
		af, au, err := a.window()
		if err != nil {
			return fmt.Errorf("model %d (%s): %w", i, a.Match, err)
		}
		if af != nil && au != nil && au.Before(*af) {
			return fmt.Errorf("model %d (%s): effectiveUntil %s is before effectiveFrom %s", i, a.Match, a.EffectiveUntil, a.EffectiveFrom)
		}
		for j := i + 1; j < len(r.Models); j++ {
			b := r.Models[j]
			if !b.dated() || !strings.EqualFold(a.Match, b.Match) {
				continue
			}
			bf, bu, berr := b.window()
			if berr != nil {
				return fmt.Errorf("model %d (%s): %w", j, b.Match, berr)
			}
			if windowsOverlap(af, au, bf, bu) {
				return fmt.Errorf("models %d and %d both price %q over the same dates; "+
					"overlapping windows make the price depend on file order", i, j, a.Match)
			}
		}
	}

	for i, m := range r.Models {
		if m.dated() {
			if _, _, err := m.window(); err != nil {
				return fmt.Errorf("model %d (%s): %w", i, m.Match, err)
			}
		}
		switch {
		case strings.TrimSpace(m.Match) == "":
			return fmt.Errorf("model %d: match is empty, so it would match every model", i)
		case m.MinPrefix < 0:
			return fmt.Errorf("model %d (%s): minPrefix is negative", i, m.Match)
		case m.InputPerMTok < 0 || m.OutputPerMTok < 0:
			return fmt.Errorf("model %d (%s): a negative price would make waste look like savings", i, m.Match)
		case m.Priced && m.InputPerMTok == 0 && m.OutputPerMTok == 0:
			return fmt.Errorf("model %d (%s): marked priced with no price, which would report every session as free", i, m.Match)
		case m.Priced && m.ReadMult == 0:
			// PriceFor passes ReadMult straight through, so zero here prices
			// every cache read at nothing, while EffectiveTokens keeps using
			// the compiled multiple. Two figures from one usage record,
			// disagreeing by the whole read term. Refuse rather than let a
			// dollar column quietly become fiction.
			return fmt.Errorf("model %d (%s): priced with readMult 0, which would price every cache read at zero", i, m.Match)
		case m.ReadMult < 0 || m.ReadMult > 1:
			return fmt.Errorf("model %d (%s): readMult %.3f is outside 0..1; a cache read costing more than a fresh one is not a rule, it is a typo", i, m.Match, m.ReadMult)
		}
		if m.MinPrefixClaim != nil {
			if err := m.MinPrefixClaim.validate(fmt.Sprintf("model %d (%s) minPrefixClaim", i, m.Match)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Provenance is the one line a reader needs to judge where the numbers came
// from.
func (r *Rules) Provenance() string {
	if r == nil {
		return RulesVersion + " (compiled in)"
	}
	parts := []string{r.Version}
	if r.Source != "" {
		parts = append(parts, "from "+r.Source)
	}
	if r.FetchedAt != "" {
		parts = append(parts, "fetched "+r.FetchedAt)
	}
	return strings.Join(parts, ", ")
}

var (
	overrideMu sync.RWMutex
	override   *Rules
)

// Override installs a rules document for the process and returns a function
// that removes it again. Tests and `replay rules --dry-run` both need the
// removal half.
func Override(r *Rules) func() {
	overrideMu.Lock()
	prev := override
	override = r
	overrideMu.Unlock()
	return func() {
		overrideMu.Lock()
		override = prev
		overrideMu.Unlock()
	}
}

// RulesVersionInEffect names the rules a report was produced under, which is
// the compiled version unless a document has been loaded.
func RulesVersionInEffect() string {
	overrideMu.RLock()
	defer overrideMu.RUnlock()
	if override == nil {
		return RulesVersion
	}
	return override.Version
}

// activeRow finds a loaded rule for a model id, matched the same way the
// compiled table is: by substring, most specific first, in file order.
func activeRow(model string) (ModelRule, bool) {
	overrideMu.RLock()
	defer overrideMu.RUnlock()
	if override == nil {
		return ModelRule{}, false
	}
	// lookup() lowercases before matching the compiled table; this did not,
	// so an installed feed correcting a price was silently ignored for
	// `Claude-Sonnet-5` or `anthropic/CLAUDE-SONNET-5` while reports still
	// cited the feed's version. A paid correction that applies to some
	// spellings of a model id and not others is worse than none.
	lower := strings.ToLower(model)
	for _, m := range override.Models {
		if strings.Contains(lower, strings.ToLower(m.Match)) {
			return m, true
		}
	}
	return ModelRule{}, false
}

// windowsOverlap reports whether two half-open-ended windows share an instant.
// A nil bound is unbounded on that side.
func windowsOverlap(aFrom, aUntil, bFrom, bUntil *time.Time) bool {
	if aUntil != nil && bFrom != nil && aUntil.Before(*bFrom) {
		return false
	}
	if bUntil != nil && aFrom != nil && bUntil.Before(*aFrom) {
		return false
	}
	return true
}
