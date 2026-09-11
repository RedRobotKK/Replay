package cachemodel

import (
	"testing"
	"time"
)

// Three branches of the version guard were entered by no test.
//
// guard-reachability reported them UNREACHED on the PR that introduced them,
// which is the worst place for it: this code exists because substring matching
// priced an unknown model as a known one, and an untested arm of the fix is
// how the same defect comes back wearing the fix's name.
//
// Each case below is an id shape a rules document or a provider can actually
// produce, not a synthetic string chosen to enter a branch.
func TestME1_TheVersionGuardsEdges(t *testing.T) {
	for _, tc := range []struct {
		name, id, match string
		want            bool
		why             string
	}{{
		name: "an empty match pattern matches nothing",
		id:   "claude-opus-5", match: "", want: false,
		why: "A rules row with an empty Match would otherwise match EVERY id, " +
			"pricing every model at that row. One malformed row silently reprices " +
			"the whole table.",
	}, {
		name: "a trailing hyphen with no digits is not a version",
		id:   "claude-opus-4-", match: "claude-opus-4", want: true,
		why: "The remainder is a bare hyphen: fewer than two characters, so there " +
			"is no version number after it. A trailing separator is not a new model.",
	}, {
		name: "a hyphen followed by a non-digit is not a version",
		id:   "claude-opus-4-preview", match: "claude-opus-4", want: true,
		why: "`-preview` is a channel, not a version bump. The comment on " +
			"matchesModel says plainly that this case is NOT fixed and still " +
			"prices as Opus 4; the test records that it is deliberate.",
	}, {
		name: "a hyphen and digits IS a version, and refuses",
		id:   "claude-opus-4-9", match: "claude-opus-4", want: false,
		why: "The defect this file exists for: Opus 4.9 priced as Opus 4 at $15/$75.",
	}, {
		name: "a digit continuing the number is a version, and refuses",
		id:   "claude-opus-45", match: "claude-opus-4", want: false,
		why: "No separator at all. `opus-45` carries the number on and is not Opus 4.",
	}, {
		name: "a non-hyphen separator before digits is not a version",
		id:   "claude-opus-4x5", match: "claude-opus-4", want: true,
		why: "The rule is a digit carrying the number on, or a hyphen then digits. " +
			"`x5` is neither, so this is the Opus 4 row, the same answer `-preview` " +
			"gets. Without the `rest[0] != '-'` arm the scan starts at index 1, " +
			"finds the 5, and reads a version that is not there — which would " +
			"refuse to price an id the table does cover. This is the input that " +
			"separates the arm from the loop below it; a trailing bare hyphen and " +
			"`-preview` both come out false either way, which is why the arm read " +
			"as inert until this case existed.",
	}, {
		name: "an eight-digit build date is still the same row",
		id:   "claude-opus-4-20250514", match: "claude-opus-4", want: true,
		why: "A dated build id is the same model. Refusing it would leave every " +
			"real Anthropic id unpriced, which is the opposite failure.",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesModel(tc.id, tc.match); got != tc.want {
				t.Errorf("matchesModel(%q, %q) = %v, want %v\n      %s",
					tc.id, tc.match, got, tc.want, tc.why)
			}
		})
	}
}

// TestME2 covers the dated lookup, which had the same hole as the undated one.
//
// PriceAt walks the rules rows applying a time window. Its `!matchesModel`
// skip was UNREACHED: every test called it with an id that matched a row, so
// nothing established that an id matching NO row comes back unpriced rather
// than falling into a neighbouring one.
//
// It matters more here than on the undated path, not less. PriceAt is what a
// report uses to price a request against the table as it stood on the day the
// request was made, so a wrong match produces a historical figure that looks
// carefully reconstructed.
func TestME2_TheDatedLookupRefusesAnUnknownVersionToo(t *testing.T) {
	// A table with two rows, built the way window_test builds one, so the
	// refusals below are about matching and not about a missing table.
	r := rules(
		ModelRule{Match: "claude-opus-4", MinPrefix: 1024, InputPerMTok: 15, OutputPerMTok: 75, ReadMult: 0.1, Priced: true},
		ModelRule{Match: "claude-fable-5", MinPrefix: 1024, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true},
	)
	when := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	// Known ids still price, or the refusals below prove nothing.
	for _, id := range []string{"claude-opus-4", "claude-opus-4-20250514", "claude-fable-5"} {
		if _, ok := r.PriceAt(id, when); !ok {
			t.Fatalf("PriceAt(%q) is unpriced; the fixture is wrong and the refusal "+
				"test below would pass for the wrong reason", id)
		}
	}

	for _, id := range []string{
		"claude-opus-4-9",    // a version the table does not carry
		"claude-fable-5-2",   // a tier whose read multiple differs 4x
		"not-a-model-at-all", // matches nothing by any rule
	} {
		if p, ok := r.PriceAt(id, when); ok {
			t.Errorf("PriceAt(%q) = $%.2f/$%.2f priced=true. An id the table does not "+
				"carry was priced from a neighbouring row, and a dated report would "+
				"present that as a reconstruction of what the day actually cost",
				id, p.InputPerMTok, p.OutputPerMTok)
		}
	}
}

// TestME3 pins order-independence, which is a property this fix created.
//
// Raised in review: "the opus-4-1 row sitting before opus-4 is load-bearing;
// if table order flips, does 4.1 still price as 4.1?" It was load-bearing
// before this change — lookup is first-hit-wins and `claude-opus-4-1` contains
// `claude-opus-4`, so a general row placed first swallowed the specific one.
//
// It is not load-bearing now, and that is worth a test rather than a reply.
// The general row REFUSES the specific id: `-1` is a hyphen and one digit, so
// continuesWithVersion says this is a different version and matchesModel
// declines. Both orders therefore price both ids correctly.
//
// The fragility was real and is gone. Without this test it comes back the
// first time someone sorts the rules document alphabetically.
func TestME3_PricingDoesNotDependOnRowOrder(t *testing.T) {
	general := ModelRule{Match: "claude-opus-4", MinPrefix: 1024, InputPerMTok: 15, OutputPerMTok: 75, ReadMult: 0.1, Priced: true}
	specific := ModelRule{Match: "claude-opus-4-1", MinPrefix: 1024, InputPerMTok: 99, OutputPerMTok: 999, ReadMult: 0.1, Priced: true}
	when := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		r    *Rules
	}{
		{"specific row first", rules(specific, general)},
		{"general row first", rules(general, specific)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range []struct {
				id    string
				input float64
			}{
				{"claude-opus-4-1", 99},
				{"claude-opus-4", 15},
			} {
				p, ok := tc.r.PriceAt(want.id, when)
				if !ok || p.InputPerMTok != want.input {
					t.Errorf("PriceAt(%q) = $%.0f priced=%v, want $%.0f. Row order changed "+
						"the price, so the rules document cannot be reordered or sorted "+
						"without repricing somebody's invoice",
						want.id, p.InputPerMTok, ok, want.input)
				}
			}
		})
	}
}
