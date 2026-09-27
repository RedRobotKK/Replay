package e005

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// The committed fixtures, pinned by content.
//
// Provenance that lives only in a README drifts silently. These hashes make it
// mechanical: edit a fixture and this fails, so a number in the frozen table
// cannot be changed by quietly changing its input.
const (
	eventsSHA256 = "2de9f6206ea004a3c86350db786a8cb885a98dc8ad9c3ace16b514a7164b81f9"
	breaksSHA256 = "a713793541bc489a5b720d52c45aed0e6b142a7643253612b4eb2445694e0f76"
)

func TestFixtureProvenanceIsPinned(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"testdata/events.json", eventsSHA256},
		{"testdata/breaks.json", breaksSHA256},
	} {
		b, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != tc.want {
			t.Errorf("%s sha256 = %s, want %s\n"+
				"      The committed input changed. Either this is a deliberate "+
				"re-derivation, in which case update the hash AND say so in "+
				"testdata/README.md, or a frozen result is being moved by "+
				"editing its evidence.", tc.path, got, tc.want)
		}
	}
}

// End to end: committed fixtures through the real parser and the real scorer,
// asserting the complete frozen E005 result in one place.
//
// The targeted tests beside this one each check a column. This one checks that
// the whole table comes out of one executable path, which is the thing a
// reviewer actually wants to know.
func TestEndToEndReproducesFrozenE005(t *testing.T) {
	ev, err := LoadEvents("testdata/events.json")
	if err != nil {
		t.Fatal(err)
	}
	br, err := LoadBreaks("testdata/breaks.json")
	if err != nil {
		t.Fatal(err)
	}
	// The parser must actually have parsed something. A silently empty load
	// would make every zero below look like agreement.
	if len(ev) != 2064 {
		t.Fatalf("parsed %d events, want 2064: the fixture did not load", len(ev))
	}
	if len(br) != 866 {
		t.Fatalf("parsed %d break boundaries, want 866", len(br))
	}

	r := Score(ev, br)

	for _, c := range []struct {
		class          string
		n, eco         int
		missed, dw     float64
		renderedMissed string
		lower, upper   int
		loPct, hiPct   string
	}{
		{"tools_changed", 1006, 932, 13482.5, 5005, "13482", 587, 932, "58%", "93%"},
		{"messages_changed", 591, 163, 54125, 0, "54125", 140, 163, "24%", "28%"},
		{"system_changed", 119, 35, 82484, 0, "82484", 29, 35, "24%", "29%"},
		{"model_changed", 45, 13, 243095, 0, "243095", 12, 13, "27%", "29%"},
	} {
		got := r.PerClass[c.class]
		if got.N != c.n {
			t.Errorf("%s n = %d, want %d", c.class, got.N, c.n)
		}
		if got.Economic != c.eco {
			t.Errorf("%s agreement = %d, want %d", c.class, got.Economic, c.eco)
		}
		if got.MedianMissed != c.missed {
			t.Errorf("%s median missed = %v, want %v", c.class, got.MedianMissed, c.missed)
		}
		if RenderMedian(got.MedianMissed) != c.renderedMissed {
			t.Errorf("%s median missed renders %s, want %s",
				c.class, RenderMedian(got.MedianMissed), c.renderedMissed)
		}
		if got.MedianDWrite != c.dw {
			t.Errorf("%s median delta-write = %v, want %v", c.class, got.MedianDWrite, c.dw)
		}
		b := Bounds(ev, br, c.class)
		if b.Lower != c.lower || b.Upper != c.upper {
			t.Errorf("%s bounds = %d/%d, want %d/%d", c.class, b.Lower, b.Upper, c.lower, c.upper)
		}
		if Pct(b.Lower, b.N) != c.loPct || Pct(b.Upper, b.N) != c.hiPct {
			t.Errorf("%s bounds render %s-%s, want %s-%s", c.class,
				Pct(b.Lower, b.N), Pct(b.Upper, b.N), c.loPct, c.hiPct)
		}
	}

	if r.Total != 2064 || r.A != 1143 || r.B != 618 || r.D != 303 {
		t.Errorf("accounting = total %d, A %d, B %d, D %d; want 2064, 1143, 618, 303",
			r.Total, r.A, r.B, r.D)
	}
	for k, v := range map[string]int{
		"tools_changed": 1006, "messages_changed": 591, "previous_message_not_found": 255,
		"system_changed": 119, "unavailable": 48, "model_changed": 45,
	} {
		if r.ClassCounts[k] != v {
			t.Errorf("%s = %d, want %d", k, r.ClassCounts[k], v)
		}
	}
}

// Negative control: the frozen numbers must come from the fixture, not from
// the assertions.
//
// Every test above compares Score's output against constants, which is exactly
// the shape that passes when the pipeline is dead and the constants are right.
// This perturbs the parsed input by one event and requires the result to move.
// If it does not, the scorer is ignoring its input and every assertion above
// is decoration.
func TestFrozenNumbersDependOnTheFixture(t *testing.T) {
	ev, br := load(t)
	base := Score(ev, br)

	// Drop one tools_changed event. Nothing else changes.
	var cut []Event
	dropped := false
	for _, e := range ev {
		if !dropped && e.Oracle == "tools_changed" {
			dropped = true
			continue
		}
		cut = append(cut, e)
	}
	if !dropped {
		t.Fatal("no tools_changed event found to drop; the fixture is not what this test assumes")
	}

	got := Score(cut, br)
	if got.PerClass["tools_changed"].N == base.PerClass["tools_changed"].N {
		t.Error("removing an event did not change the class count: Score is not " +
			"reading its input, and every frozen assertion in this package is " +
			"passing on constants alone")
	}
	if got.Total == base.Total {
		t.Error("removing an event did not change the total")
	}

	// And the breaks input must matter too.
	empty := Score(ev, map[string]int{})
	if empty.A == base.A {
		t.Error("emptying the break index did not change the agreement count: " +
			"the second input is not wired in")
	}
}
