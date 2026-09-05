package cachemodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
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
	for i, m := range r.Models {
		switch {
		case strings.TrimSpace(m.Match) == "":
			return fmt.Errorf("model %d: match is empty, so it would match every model", i)
		case m.MinPrefix < 0:
			return fmt.Errorf("model %d (%s): minPrefix is negative", i, m.Match)
		case m.InputPerMTok < 0 || m.OutputPerMTok < 0:
			return fmt.Errorf("model %d (%s): a negative price would make waste look like savings", i, m.Match)
		case m.Priced && m.InputPerMTok == 0 && m.OutputPerMTok == 0:
			return fmt.Errorf("model %d (%s): marked priced with no price, which would report every session as free", i, m.Match)
		case m.ReadMult < 0 || m.ReadMult > 1:
			return fmt.Errorf("model %d (%s): readMult %.3f is outside 0..1; a cache read costing more than a fresh one is not a rule, it is a typo", i, m.Match, m.ReadMult)
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
	for _, m := range override.Models {
		if strings.Contains(model, m.Match) {
			return m, true
		}
	}
	return ModelRule{}, false
}
