package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

// RELEASE BLOCKER 5. A published document that OMITS a quantitative key must be
// refused, not read as zero.
//
// The presence guard existed for `rebilledUsd` and `rebilledShare` because
// those two had been renamed. The others were never guarded, so a document
// declaring replay.corpus.v2 and omitting `medianTaskUsd` parsed to 0.0, passed
// Validate, and dragged the PUBLISHED median range to zero. Absence read as a
// measurement, on the document a stranger downloads.
//
// Presence, not value: a genuine measured zero is a real submission and still
// pools.

func corpusWith(t *testing.T, omit string) []byte {
	t.Helper()
	full := map[string]any{
		"schema": CorpusSchema, "takenAt": "2026-10-02T09:00:00Z",
		"tasks": 10, "totalUsd": 12.5, "rebilledUsd": 1.25, "rebilledShare": 0.1,
		"medianTaskUsd": 1.0, "pricedAt": "2026-09-07", "rulesVersion": "x",
		"unpriced": 0, "sourceTag": "tag", "tagBasis": "basis", "digest": "d",
	}
	delete(full, omit)
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestB5_AnOmittedQuantitativeKeyIsRefusedNotReadAsZero(t *testing.T) {
	// POSITIVE CONTROL: the complete document parses, so a refusal below is
	// about the missing key and not about the fixture being malformed.
	var ok Corpus
	if err := json.Unmarshal(corpusWith(t, "nothing-is-omitted"), &ok); err != nil {
		t.Fatalf("the complete fixture must parse, or this test proves nothing: %v", err)
	}

	for _, key := range []string{"rebilledUsd", "rebilledShare", "tasks", "totalUsd", "medianTaskUsd"} {
		var c Corpus
		err := json.Unmarshal(corpusWith(t, key), &c)
		if err == nil {
			t.Errorf("a submission omitting %q was accepted. It parses to zero and pools "+
				"an absence as a measurement, which is the defect the rename guard was "+
				"written to close, reappearing on the keys nobody guarded.", key)
			continue
		}
		if !strings.Contains(err.Error(), key) {
			t.Errorf("omitting %q was refused, but the refusal does not name the key: %v", key, err)
		}
	}
}

// NEGATIVE CONTROL. A genuine measured zero must still pool. A guard that
// refused zeros would have replaced a false measurement with a lost one.
func TestB5_AGenuineMeasuredZeroStillPools(t *testing.T) {
	full := map[string]any{
		"schema": CorpusSchema, "takenAt": "2026-10-02T09:00:00Z",
		"tasks": 10, "totalUsd": 12.5, "rebilledUsd": 0.0, "rebilledShare": 0.0,
		"medianTaskUsd": 0.0, "pricedAt": "2026-09-07", "rulesVersion": "x",
		"unpriced": 0, "sourceTag": "tag", "tagBasis": "basis", "digest": "d",
	}
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	var c Corpus
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("a corpus whose figures are genuinely zero is a real measurement and "+
			"must parse: %v", err)
	}
}
