package facts

import (
	"strings"
	"testing"
)

func set() Set {
	return Set{
		Surface: "ollama", VerifiedAgainst: "0.33.3", VerifiedOn: "2026-09-07",
		Facts: []Fact{{
			Key: "flash_attention_default", Value: "auto",
			Source: "ollama/ollama, server source at the pinned tag",
			Method: "read from the source that implements it",
		}},
	}
}

// TestFA1: the same version is the only case that may be reported as current.
func TestFA1(t *testing.T) {
	if n := set().Note("0.33.3"); n != "" {
		t.Errorf("same version should produce no warning, got %q", n)
	}
}

// TestFA2: a different installed version must be named, both sides.
//
// A generic "these may be stale" is the failure this exists to prevent. The
// reader has to be able to see which pair disagreed without going to look.
func TestFA2(t *testing.T) {
	n := set().Note("0.33.2")
	if n == "" {
		t.Fatal("a version mismatch produced no warning at all")
	}
	for _, want := range []string{"0.33.2", "0.33.3", "ollama"} {
		if !strings.Contains(n, want) {
			t.Errorf("warning must name %q, got: %s", want, n)
		}
	}
}

// TestFA3: an unknown installed version is not the same as a match.
//
// The tempting shortcut is to treat "cannot tell" as "probably fine". That is
// how a stale default ships as current advice.
func TestFA3(t *testing.T) {
	n := set().Note("")
	if n == "" {
		t.Fatal("unknown installed version must not read as agreement")
	}
	if !strings.Contains(strings.ToLower(n), "could not") && !strings.Contains(strings.ToLower(n), "unknown") {
		t.Errorf("warning must say the version could not be read, got: %s", n)
	}
}

// TestFA4: every fact carries where it came from and how it was checked.
//
// A default with no source is a prior wearing a fact's clothes, which is the
// specific way wrong advice shipped before this package existed.
func TestFA4(t *testing.T) {
	for _, f := range Ollama().Facts {
		if strings.TrimSpace(f.Source) == "" || strings.TrimSpace(f.Method) == "" {
			t.Errorf("fact %q has no source or no method: %+v", f.Key, f)
		}
	}
	if len(Ollama().Facts) < 3 {
		t.Fatalf("only %d facts; the table is empty enough that this test proves nothing", len(Ollama().Facts))
	}
}

// TestFA5: a fact set that has never been verified cannot claim to be current.
func TestFA5(t *testing.T) {
	s := set()
	s.VerifiedAgainst = ""
	n := s.Note("0.33.3")
	if n == "" {
		t.Fatal("a set with no recorded verification target must warn, not pass")
	}
	// Not any warning: the one that names the actual problem. Falling through
	// to the version-mismatch message would report a disagreement with the
	// empty string, which is true and useless.
	if !strings.Contains(n, "records no version") {
		t.Errorf("must say the table records no verification target, got: %s", n)
	}
}
