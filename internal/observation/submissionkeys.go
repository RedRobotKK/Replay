package observation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// The contribution boundary: the payload's keys are an allowlist.
//
// WHY THIS FILE EXISTS. encoding/json drops a key it does not recognise.
// Nothing here asked it not to, so a document carrying "prompt", "transcript"
// or "apiKey" parsed cleanly, validated, pooled, and carried the field into a
// public repository, where a merged submission lives forever. The field was in
// the payload the contributor published, and deleting it after decoding would
// be too late in exactly the way that matters.
//
// Every other guard in this package is about fields that ARE in a type: CC2
// checks that no field NAME describes the contributor's machine, the
// retired-spelling checks refuse names that used to mean something else, and
// Validate checks ranges. None of them can see a key that is simply not in the
// struct, which is the shape both an accident and a deliberate submission take.
//
// THE RULE IS AN ALLOWLIST, NOT A LIST OF BAD WORDS, because a list of bad
// words only ever catches the words somebody listed. TestSB3 pins that by
// feeding it a key nobody would think to ban.
//
// THE LISTS ARE WRITTEN OUT RATHER THAN REFLECTED. This package may not import
// reflect: TestO7 holds an import allowlist whose whole value is that nobody
// widens it for a convenience, and it fired on the first attempt at this. So
// each list is explicit below and TestSB5 proves each equals its struct's own
// tags, in a test, where reflect is free. That check fails in both directions,
// so neither a field added to a type nor one removed from it can drift.
//
// WHICH TYPES. Every artifact a contributor can publish, at every level a hand
// edit can reach. `replay cost --contribute` writes a corpus AND a calibration;
// `replay pool` turns corpora into a public roster whose rows are read back
// when a pooler appends. A calibration's `models` array is the case a
// top-level-only guard would miss, and it is the likeliest place for somebody
// to paste a figure that came with something attached.

// unknownKeys returns the keys of a decoded document that its type does not
// declare, quoted and sorted so a refusal reads the same on every run.
func unknownKeys(probe map[string]json.RawMessage, allowed map[string]bool) []string {
	var out []string
	for k := range probe {
		if !allowed[k] {
			out = append(out, fmt.Sprintf("%q", k))
		}
	}
	sort.Strings(out)
	return out
}

// refuseUnknown builds the refusal a contributor sees, or nil if there is none.
func refuseUnknown(what string, probe map[string]json.RawMessage, allowed map[string]bool) error {
	unknown := unknownKeys(probe, allowed)
	if len(unknown) == 0 {
		return nil
	}
	return fmt.Errorf("this %s carries %s, which it does not have. A contribution is "+
		"the aggregate figures and their basis, and nothing else; remove the key rather "+
		"than leaving it for a reader to find in the published file. If the field is a "+
		"genuine addition, add it to the type first",
		what, strings.Join(unknown, ", "))
}

// corpusKeys is every JSON key a Corpus may carry.
var corpusKeys = map[string]bool{
	"schema": true, "takenAt": true,
	"tasks": true, "totalUsd": true, "rebilledUsd": true,
	"rebilledShare": true, "medianTaskUsd": true,
	"pricedAt": true, "rulesVersion": true, "unpriced": true,
	"cacheBreaks": true, "reReads": true, "errorShare": true,
	"sourceTag": true, "tagBasis": true,
	"binaryVersion": true, "commit": true, "pricingDigest": true,
	"digest": true,
}

// calibrationKeys is every JSON key a Calibration may carry.
var calibrationKeys = map[string]bool{
	"schema": true, "takenAt": true, "rulesVersion": true,
	"clientVersions": true, "models": true, "breakCauses": true,
	"sourceTag": true, "tagBasis": true, "digest": true,
}

// calibrationRowKeys is every JSON key one model row may carry. Nested, and the
// level a guard written only for the outer object would leave open.
var calibrationRowKeys = map[string]bool{
	"model": true, "sessions": true, "compared": true, "matched": true,
	"exact": true, "ruleMinPrefix": true, "largestUncached": true,
	"smallestCached": true, "stale": true,
	"fitTokensPerByte": true, "fitErrorPct": true,
}

// poolEntryKeys is every JSON key a published roster row may carry.
var poolEntryKeys = map[string]bool{
	"sourceTag": true, "tagBasis": true, "digest": true, "file": true,
	"takenAt": true, "tasks": true, "totalUsd": true, "rebilledUsd": true,
	"rebilledShare": true, "medianTaskUsd": true, "pricedAt": true,
	"rulesVersion": true, "unpriced": true, "cacheBreaks": true,
	"reReads": true, "binaryVersion": true, "pricingDigest": true,
}

// UnmarshalJSON refuses a calibration carrying a key a calibration does not
// have. See the file comment; `replay cost --contribute` publishes this
// artifact alongside the corpus and it had no decoder of its own at all.
func (c *Calibration) UnmarshalJSON(b []byte) error {
	var probe map[string]json.RawMessage
	// A document that does not parse as a map does not parse as the struct
	// either, and the struct decode below reports it. Two spellings of one
	// refusal, one untestable, is the shape ADR-0014 rules out.
	_ = json.Unmarshal(b, &probe)
	if err := refuseUnknown("calibration", probe, calibrationKeys); err != nil {
		return err
	}
	type plain Calibration
	var out plain
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*c = Calibration(out)
	return nil
}

// UnmarshalJSON refuses a model row carrying a key a model row does not have.
func (r *ModelCalibrationRow) UnmarshalJSON(b []byte) error {
	var probe map[string]json.RawMessage
	_ = json.Unmarshal(b, &probe)
	if err := refuseUnknown("model row", probe, calibrationRowKeys); err != nil {
		return err
	}
	type plain ModelCalibrationRow
	var out plain
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	*r = ModelCalibrationRow(out)
	return nil
}
