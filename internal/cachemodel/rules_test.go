package cachemodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rules are published numbers that change faster than release cycles do, so
// they have to be correctable without shipping a binary. They are also the
// numbers every figure in every report is built on, which means a bad rules
// file is worse than a stale one: stale is visible in the version string, wrong
// is not. Loading is therefore strict, and anything it will not vouch for is
// refused rather than merged.

func writeRules(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodRules = `{
  "schema": "replay.rules.v1",
  "version": "anthropic-2026-09-15",
  "provider": "anthropic",
  "source": "https://example.invalid/pricing",
  "models": [
    {"match": "opus-5", "minPrefix": 4096, "inputPerMTok": 5, "outputPerMTok": 25, "readMult": 0.1, "priced": true}
  ]
}`

func TestLoadRulesAcceptsAWellFormedFile(t *testing.T) {
	r, err := LoadRules(writeRules(t, goodRules))
	if err != nil {
		t.Fatal(err)
	}
	if r.Version != "anthropic-2026-09-15" {
		t.Fatalf("version: %q", r.Version)
	}
	if len(r.Models) != 1 || r.Models[0].Match != "opus-5" {
		t.Fatalf("models: %+v", r.Models)
	}
}

func TestLoadRulesRefusesWhatItCannotVouchFor(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"no schema", `{"version":"v","models":[{"match":"m","minPrefix":1}]}`, "schema"},
		{"unknown schema", `{"schema":"replay.rules.v99","version":"v","models":[{"match":"m","minPrefix":1}]}`, "schema"},
		{"no version", `{"schema":"replay.rules.v1","models":[{"match":"m","minPrefix":1}]}`, "version"},
		{"no models", `{"schema":"replay.rules.v1","version":"v","models":[]}`, "model"},
		{"empty match", `{"schema":"replay.rules.v1","version":"v","models":[{"match":"","minPrefix":1}]}`, "match"},
		{"negative prefix", `{"schema":"replay.rules.v1","version":"v","models":[{"match":"m","minPrefix":-1}]}`, "minPrefix"},
		{"negative price", `{"schema":"replay.rules.v1","version":"v","models":[{"match":"m","minPrefix":1,"inputPerMTok":-5,"priced":true}]}`, "price"},
		{"priced with no price", `{"schema":"replay.rules.v1","version":"v","models":[{"match":"m","minPrefix":1,"priced":true}]}`, "price"},
		{"read multiple above one", `{"schema":"replay.rules.v1","version":"v","models":[{"match":"m","minPrefix":1,"readMult":3}]}`, "readMult"},
		{"not json", `{`, "parse"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadRules(writeRules(t, c.body))
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.want)) {
				t.Fatalf("the error must say what is wrong; got %q, want mention of %q", err, c.want)
			}
		})
	}
}

// A missing file is not an error: the compiled defaults are the fallback, and a
// user who has never run --update must not see a failure.
func TestLoadRulesTreatsAMissingFileAsNoOverride(t *testing.T) {
	r, err := LoadRules(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("a missing rules file must not be an error: %v", err)
	}
	if r != nil {
		t.Fatalf("want no override, got %+v", r)
	}
}

// The whole point is that a report can say which rules produced it.
func TestRulesDescribeTheirOwnProvenance(t *testing.T) {
	r, err := LoadRules(writeRules(t, goodRules))
	if err != nil {
		t.Fatal(err)
	}
	d := r.Provenance()
	for _, want := range []string{"anthropic-2026-09-15", "example.invalid"} {
		if !strings.Contains(d, want) {
			t.Fatalf("provenance %q is missing %q", d, want)
		}
	}
}

// An override replaces the table it overrides, and leaves the compiled defaults
// reachable for anything it does not mention.
func TestOverrideAppliesToLookupsAndIsReversible(t *testing.T) {
	before := MinCacheablePrefix("opus-5")
	r, err := LoadRules(writeRules(t, goodRules))
	if err != nil {
		t.Fatal(err)
	}
	restore := Override(r)
	if got := MinCacheablePrefix("opus-5"); got != 4096 {
		t.Fatalf("override did not apply: minPrefix=%d", got)
	}
	if got := RulesVersionInEffect(); got != "anthropic-2026-09-15" {
		t.Fatalf("version in effect: %q", got)
	}
	restore()
	if got := MinCacheablePrefix("opus-5"); got != before {
		t.Fatalf("restore did not put the compiled rules back: %d != %d", got, before)
	}
	if got := RulesVersionInEffect(); got != RulesVersion {
		t.Fatalf("version not restored: %q", got)
	}
}
