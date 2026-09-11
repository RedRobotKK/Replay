package observation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CalibrationSchema versions the calibration report independently of the money
// submission, because they answer different questions and one can change
// without the other.
const CalibrationSchema = "replay.calibration.v1"

// Calibration is what ADR-0007 specified as the unit of contribution, and what
// did not ship.
//
// That record says it plainly: "replay corpus emits session id prefixes, client
// version, request counts, match rates, fit parameters, prefix bounds and break
// causes... A contribution is that report and nothing else." What shipped was
// five money figures — ADR-0008's credibility claim — so the data this project
// exists to accumulate is computed on every run and thrown away.
//
// The distinction that makes it worth pooling: this measures the PROVIDER, not
// the contributor. A prefix bound is a fact about Anthropic's cache floor that
// the operator merely happened to observe. Money is a fact about the operator.
// Both are content-free, only one is about them.
//
// **One deliberate departure from ADR-0007: no session id prefixes.** That
// record listed them so a maintainer could debug their own submissions. A
// session id is a linkable identifier that appears in other artifacts — this
// repository's own memory files carry them — and the aggregate answers every
// question the prefixes were there for. Rows are per model.
//
// Client versions are kept, and they carry a residual risk worth stating: a
// rare build narrows the set of people running it. They are here because
// separating a provider change from a client change is the entire analytical
// point, and that cannot be done without them.
type Calibration struct {
	Schema  string `json:"schema"`
	TakenAt string `json:"takenAt"`
	// RulesVersion is the table these bounds were measured against. Bounds from
	// two different tables are not poolable: a floor observed under one set of
	// documented minimums says nothing about a corpus scored under another.
	RulesVersion   string   `json:"rulesVersion"`
	ClientVersions []string `json:"clientVersions,omitempty"`

	Models []ModelCalibrationRow `json:"models"`
	// BreakCauses counts why caches stopped holding, by the cause Replay named.
	// Content-free: the causes are a closed set this build writes, not text from
	// anyone's session.
	BreakCauses map[string]int `json:"breakCauses,omitempty"`

	SourceTag string `json:"sourceTag"`
	TagBasis  string `json:"tagBasis"`
	Digest    string `json:"digest"`
}

// ModelCalibrationRow is one model's observed behaviour.
//
// LargestUncached and SmallestCached are the interval the true minimum
// cacheable prefix lies in. One machine reports a wide one; ADR-0007's
// arithmetic is that "one more machine narrows that interval. A hundred closes
// it." That is the whole reason this type exists.
type ModelCalibrationRow struct {
	Model    string `json:"model"`
	Sessions int    `json:"sessions"`
	Compared int    `json:"compared"`
	Matched  int    `json:"matched"`
	// Exact is the subset of Matched whose read was reproduced EXACTLY.
	//
	// Matched also counts turns where the provider served more cached prefix
	// than the model predicted, which is a prediction that was wrong in the
	// other direction. Without this field a pooled reading would rebuild the
	// same conflated headline across every contributor and no reader of the
	// pool could decompose it either. Added 2026-09-11; see
	// analysis.Calibration.ExactRate.
	Exact int `json:"exact"`

	// RuleMinPrefix is what the table claims, carried beside what was observed
	// so a pooled reading can say the two disagree without re-deriving either.
	RuleMinPrefix   int `json:"ruleMinPrefix"`
	LargestUncached int `json:"largestUncached"`
	SmallestCached  int `json:"smallestCached"`

	// Stale is this build's own verdict that the provider's behaviour moved.
	// Carried as the contributor's reading, not as a fact: one machine cannot
	// tell a provider change from an operator change, and pooling is what
	// supplies the control group that can.
	Stale bool `json:"stale,omitempty"`

	// FitTokensPerByte and FitErrorPct are absent when this build did not fit
	// one, which is not the same as fitting zero.
	FitTokensPerByte *float64 `json:"fitTokensPerByte,omitempty"`
	FitErrorPct      *float64 `json:"fitErrorPct,omitempty"`
}

// Digested returns the report with its content digest filled in, computed over
// the payload with the field empty so a reader can recompute it from the file.
func (c Calibration) Digested() Calibration {
	c.Digest = ""
	body, _ := json.Marshal(c)
	sum := sha256.Sum256(body)
	c.Digest = hex.EncodeToString(sum[:])
	return c
}

// Validate refuses a report that cannot be pooled.
//
// Every refusal is a case where admitting the submission would move a
// population denominator without moving its numerator.
func (c Calibration) Validate() error {
	switch {
	case c.Schema != CalibrationSchema:
		return fmt.Errorf("schema is %q, want %q", c.Schema, CalibrationSchema)
	case c.Digest == "":
		return fmt.Errorf("the report has no content digest, so a pooled reading could not " +
			"name it or let a reader check it")
	case c.RulesVersion == "":
		return fmt.Errorf("bounds measured against an unnamed table cannot be pooled with " +
			"bounds measured against another")
	case c.SourceTag == "" || c.TagBasis == "":
		return fmt.Errorf("a contribution needs its source tag and the basis of that tag")
	case len(c.Models) == 0:
		return fmt.Errorf("NOT MEASURED: the report has no model rows, so there is nothing " +
			"in it to pool")
	}
	compared := 0
	for _, m := range c.Models {
		if m.Model == "" {
			return fmt.Errorf("a row with no model id cannot be pooled with anything")
		}
		if m.Compared < 0 || m.Matched < 0 || m.Sessions < 0 || m.Exact < 0 {
			return fmt.Errorf("%s: negative counters", m.Model)
		}
		if m.Matched > m.Compared {
			return fmt.Errorf("%s: matched %d of %d compared, which describes a state that "+
				"cannot occur", m.Model, m.Matched, m.Compared)
		}
		if m.Exact > m.Matched {
			return fmt.Errorf("%s: exact %d of %d matched, which describes a state that "+
				"cannot occur: an exactly reproduced turn is a matched turn by construction",
				m.Model, m.Exact, m.Matched)
		}
		compared += m.Compared
	}
	if compared == 0 {
		return fmt.Errorf("NOT MEASURED: no row compared a single turn; a match rate over no " +
			"comparisons is a division rather than a measurement")
	}
	return nil
}

// WriteCalibration writes a report, refusing the same ways WriteCorpus does.
//
// Named distinctly from a money submission on purpose. They answer different
// questions, disclose different things, and a reader who has decided about one
// has not decided about the other.
func WriteCalibration(dir string, c Calibration) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	name := fmt.Sprintf("replay-calibration-%s-%s.json", c.SourceTag, shortHex(c.Digest))
	path := filepath.Join(dir, name)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s is a symlink; refusing to write a report through a redirected path", path)
		}
		return "", fmt.Errorf("%s already exists; move or delete it rather than replacing a report that has not been sent", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("could not establish whether %s is free: %w", path, err)
	}
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func shortHex(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
