package observation

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Two things, and the order matters.
//
// Observation's own doc comment promises that "nothing derived from prompts,
// responses, file paths, project names, session ids, SPEND, request volume, or
// any credential appears". Nothing enforced that. It is the payload that leaves
// the machine, and its central guarantee was a sentence.
//
// Adding cost figures to that struct would have broken the promise silently,
// which is how this was nearly built. A corpus contribution is a DIFFERENT
// submission with a different bargain — the contributor is knowingly sending
// aggregate spend — so it gets its own type, its own consent, and its own
// statement of what it carries. Smuggling five money fields into a struct that
// says it carries none would be the defect this repository keeps finding, aimed
// at the one artefact a stranger receives.

// OB1: spend never appears in a probe Observation.
//
// PASS: no field name or JSON tag in Observation looks monetary.
// FAIL: any does — at which point the struct's own comment is false, and the
// contributor who read it has been misled about what they sent.
func TestOB1_ObservationCarriesNoSpend(t *testing.T) {
	banned := []string{"usd", "cost", "spend", "dollar", "price", "bill", "requests", "tokens"}
	rt := reflect.TypeOf(Observation{})
	if rt.NumField() < 10 {
		t.Fatalf("Observation has %d fields; this guard is inspecting the wrong type", rt.NumField())
	}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		hay := strings.ToLower(f.Name + " " + f.Tag.Get("json"))
		for _, b := range banned {
			if strings.Contains(hay, b) {
				t.Errorf("Observation.%s (json %q) looks monetary. The struct's own comment "+
					"promises no spend or request volume leaves in it; a corpus contribution "+
					"is a separate submission with a separate consent.",
					f.Name, f.Tag.Get("json"))
			}
		}
	}
}

// CC1: a corpus contribution carries the five figures and states its basis.
//
// PASS: total, avoidable, share, median and the task count are present, and so
// is what priced them.
// FAIL: a figure without its basis. An aggregate of totals priced on different
// tables is a number nobody can defend, which is the shape of the retraction
// this project already published once.
func TestCC1_TheFiveFiguresTravelWithTheirBasis(t *testing.T) {
	c := Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-10T00:00Z",
		Tasks: 115, TotalUSD: 3382.13, AvoidableUSD: 161.66,
		AvoidableShare: 0.0478, MedianTaskUSD: 0.77,
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		Unpriced: 6, SourceTag: "abc", TagBasis: "random",
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"tasks", "totalUsd", "avoidableUsd", "avoidableShare",
		"medianTaskUsd", "pricedAt", "rulesVersion", "unpriced"} {
		if !strings.Contains(got, want) {
			t.Errorf("the contribution does not carry %q:\n%s", want, got)
		}
	}
}

// CC2: a corpus contribution carries nothing identifying.
//
// PASS: no path, project name, session id or model list.
// FAIL: any of them — the contributor sent aggregate spend, not an inventory of
// their machine.
func TestCC2_NothingIdentifyingTravels(t *testing.T) {
	rt := reflect.TypeOf(Corpus{})
	banned := []string{"path", "project", "session", "dir", "home", "user", "host", "file"}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		hay := strings.ToLower(f.Name + " " + f.Tag.Get("json"))
		for _, b := range banned {
			if strings.Contains(hay, b) {
				t.Errorf("Corpus.%s (json %q) names something about the contributor's machine",
					f.Name, f.Tag.Get("json"))
			}
		}
	}
}

// CC3: an unpriced corpus is refused rather than submitted as free.
//
// PASS: a contribution with no priced tasks does not validate, refused BY the
// zero-tasks guard and by name.
// FAIL: zeroes aggregated as though somebody measured them, which is the
// "nothing measured is not a pass" rule this repository applies everywhere else.
//
// The fixture is the whole test, and the first version had it wrong in the way
// ADR-0014 warns about. It validated `Corpus{Schema: CorpusSchema, Tasks: 0}`
// and asserted only `err == nil`. That value has no Digest, so Validate refused
// at the FIRST case — "the submission has no content digest" — and the
// zero-tasks case three cases below was never reached at all. Measured by
// scripts/refusal-reachability: forcing `case c.Tasks <= 0` false left this
// test green, and forcing `case c.TotalUSD <= 0` false left it green too,
// because it never reached either.
//
// Reaching a case in a `switch { case ... }` means satisfying every case above
// it. So each fixture below is a corpus that is valid in every respect except
// the one under test, and each assertion names the sentence only that case
// produces — never the shared "NOT MEASURED" tier, which the case below it
// carries as well.
func TestCC3_AnEmptyCorpusIsNotAContribution(t *testing.T) {
	// Valid in everything but the field each case names.
	base := Corpus{
		Schema: CorpusSchema, Tasks: 115, TotalUSD: 3382.13,
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01", SourceTag: "a", TagBasis: "b",
	}

	noTasks := base
	noTasks.Tasks = 0
	noTasks.TotalUSD = 0
	err := noTasks.Digested().Validate()
	if err == nil {
		t.Fatal("a corpus with zero tasks validated; aggregating it would add a measured " +
			"zero to somebody's total")
	}
	if strings.Contains(err.Error(), "no content digest") {
		t.Fatalf("the digest case refused first, so this fixture never reaches the "+
			"zero-tasks guard and the test cannot see it: %v", err)
	}
	// The sentence only `case c.Tasks <= 0` produces. The case below it also
	// says NOT MEASURED, so asserting the tier accepts the wrong refusal.
	if !strings.Contains(err.Error(), "priced no tasks") {
		t.Errorf("zero tasks was refused, but not by the zero-tasks guard: %v", err)
	}

	// Tasks priced, all of them to nothing. This is the case below, and it has
	// to be reachable on its own or the guard above is doing both jobs.
	noMoney := base
	noMoney.TotalUSD = 0
	err = noMoney.Digested().Validate()
	if err == nil {
		t.Fatal("a corpus whose tasks all priced to $0.00 validated")
	}
	if strings.Contains(err.Error(), "priced no tasks") {
		t.Fatalf("the zero-tasks guard refused a corpus with %d tasks: %v", noMoney.Tasks, err)
	}
	if !strings.Contains(err.Error(), "priced to $0.00") {
		t.Errorf("a $0.00 total was refused, but not by the guard that names it: %v", err)
	}

	if err := base.Digested().Validate(); err != nil {
		t.Errorf("a real corpus was refused: %v", err)
	}
}
