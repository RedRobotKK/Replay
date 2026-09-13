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
// PASS: total, re-billed, share, median and the task count are present, and so
// is what priced them.
// FAIL: a figure without its basis. An aggregate of totals priced on different
// tables is a number nobody can defend, which is the shape of the retraction
// this project already published once.
func TestCC1_TheFiveFiguresTravelWithTheirBasis(t *testing.T) {
	c := Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-10T00:00Z",
		Tasks: 115, TotalUSD: 3382.13, RebilledUSD: 161.66,
		RebilledShare: 0.0478, MedianTaskUSD: 0.77,
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		Unpriced: 6, SourceTag: "abc", TagBasis: "random",
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"tasks", "totalUsd", "rebilledUsd", "rebilledShare",
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

// CR-OLD. A pre-rename submission must be refused, not read as zero.
//
// On 2026-09-13 `avoidableUsd` became `rebilledUsd` across the payload. The
// rename was mechanical and complete, and it introduced a defect no test
// caught: a submission written before it parses cleanly into the new struct
// with RebilledUSD = 0, and Validate() returns nil, because a missing JSON key
// is indistinguishable from a zero value in Go.
//
// The one contributed corpus in existence was written under the old spelling.
// Pooled by the new build it would have added $0.00 to the total and reported
// success. That is ADR-0018 exactly: absence read as zero, silently, in the
// payload whose entire job is to carry a figure somebody can check.
//
// REFUSED RATHER THAN TRANSLATED, and the reason is measured. The same corpus
// reads 4.99% on v0.5.4 and 2.75% on the build that renamed these fields
// (two-builds-one-corpus-2026-09-13.md). Silently mapping the old key onto the
// new field would pool two instruments as one number, which is the thing that
// file exists to prevent. A refusal makes somebody decide.
func TestCROLD_APreRenameSubmissionIsRefusedRatherThanReadAsZero(t *testing.T) {
	// One document per retired field, so removing any single entry from the
	// map fails here. A single fixture carrying all three passed even when two
	// of the three checks were deleted.
	for _, tc := range []struct{ field, replacement, doc string }{
		{"avoidableUsd", "rebilledUsd", `"avoidableUsd":211.42`},
		{"avoidableShare", "rebilledShare", `"avoidableShare":0.0499`},
		{"avoidableTokens", "rebilledTokens", `"avoidableTokens":44214854`},
	} {
		doc := []byte(`{"schema":"replay.corpus.v1","takenAt":"2026-09-12T00:00:00Z",
		 "tasks":119,"totalUsd":4236.17,` + tc.doc + `,
		 "medianTaskUsd":1.2,"pricedAt":"2026-09-12","rulesVersion":"anthropic-2026-09-01",
		 "unpriced":6,"sourceTag":"abc","tagBasis":"machine","digest":"65abc02f"}`)

		var c Corpus
		err := json.Unmarshal(doc, &c)
		if err == nil {
			t.Errorf("a submission carrying %s parsed cleanly. RebilledUSD=%v and Validate "+
				"says %v, so it would add $0.00 to a pooled total and report success.",
				tc.field, c.RebilledUSD, c.Validate())
			continue
		}
		for _, want := range []string{tc.field, tc.replacement} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal for %s does not name %q, so the operator cannot tell "+
					"what to do about it: %v", tc.field, want, err)
			}
		}
	}

	// A current submission is unaffected.
	fresh, err := json.Marshal(Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-13T00:00:00Z", Tasks: 121,
		TotalUSD: 12630.61, RebilledUSD: 347.53, RebilledShare: 0.0275,
		MedianTaskUSD: 1.2, PricedAt: "2026-09-13", RulesVersion: "anthropic-2026-09-01",
		SourceTag: "abc", TagBasis: "machine",
	}.Digested())
	if err != nil {
		t.Fatal(err)
	}
	var round Corpus
	if err := json.Unmarshal(fresh, &round); err != nil {
		t.Fatalf("a current submission was refused: %v", err)
	}
	if round.RebilledUSD != 347.53 {
		t.Errorf("round trip lost the figure: %v", round.RebilledUSD)
	}
}

// CR-MALFORMED: a document that is not JSON, and one whose types are wrong.
//
// Corpus gained an UnmarshalJSON on 2026-09-13 to refuse pre-rename
// submissions. It carries two error branches of its own, and `guard
// reachability` reported both UNREACHED: the probe pass and the real pass.
//
// They matter because this is the entry point for a file a stranger sends. A
// panic or a silent zero here is reached by anybody who can put a file in front
// of the pooler, which is the whole point of the contribution path.
func TestCRMALFORMED_UnparseableAndMistypedDocumentsAreRefused(t *testing.T) {
	for _, tc := range []struct{ name, doc string }{
		{"not JSON at all", `this is not json`},
		{"truncated", `{"schema":"replay.corpus.v1","tasks":`},
		{"an array, not an object", `[1,2,3]`},
		{"a field of the wrong type", `{"schema":"replay.corpus.v1","tasks":"one hundred"}`},
		{"totalUsd is a string", `{"schema":"replay.corpus.v1","totalUsd":"4236.17"}`},
		// Every required key PRESENT and one of them the wrong type. Without
		// this the real decode's error branch is unreachable: encoding/json
		// rejects malformed bytes before UnmarshalJSON is ever called, and the
		// other cases here all fail the presence check first, so nothing
		// reached the type error at all.
		{"rebilledUsd is a string", `{"schema":"replay.corpus.v2","rebilledUsd":"lots",` +
			`"rebilledShare":0.0275,"tasks":121,"totalUsd":12630.61}`},
		{"rebilledShare is an object", `{"schema":"replay.corpus.v2","rebilledUsd":347.53,` +
			`"rebilledShare":{"x":1},"tasks":121}`},
	} {
		var c Corpus
		if err := json.Unmarshal([]byte(tc.doc), &c); err == nil {
			t.Errorf("%s was accepted, and it parsed to %+v", tc.name, c)
		}
	}
}

// CR-V2. The schema string moved with the field names, and Validate enforces it.
//
// Daniel ruled on 2026-09-13: bump rather than re-derive. The rename changed
// what the wire form looks like, so a document under the old version string and
// the new spelling, or the reverse, is a document nobody wrote and nobody
// should read.
//
// UnmarshalJSON already refuses the old SPELLING. This is the other half: a
// document that claims a version this build does not write. The two catch
// different lies. A hand-edited file carrying the new spelling under
// "replay.corpus.v1" passes the spelling check and is still not a thing this
// project ever produced.
func TestCRV2_TheSchemaStringMovedAndIsChecked(t *testing.T) {
	if CorpusSchema != "replay.corpus.v2" {
		t.Errorf("CorpusSchema is %q. The rename changed the wire form, so the version "+
			"string had to move with it", CorpusSchema)
	}
	// Watch must NOT move: it has never shipped, so there is no older reader to
	// protect and v1 has never meant anything else.
	if WatchSchema != "replay.watch.v1" {
		t.Errorf("WatchSchema is %q. No watch record has ever been written, so there is "+
			"nothing a bump would protect and the first version should be v1", WatchSchema)
	}
	if PoolSchema != "replay.pool.v2" {
		t.Errorf("PoolSchema is %q. The pooled document carries rebilledUsd and "+
			"rebilledShare in its own entries, so its wire form changed too", PoolSchema)
	}

	base := Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-13T00:00:00Z", Tasks: 121,
		TotalUSD: 12630.61, RebilledUSD: 347.53, RebilledShare: 0.0275,
		MedianTaskUSD: 1.2, PricedAt: "2026-09-13", RulesVersion: "anthropic-2026-09-01",
		SourceTag: "abc", TagBasis: "machine",
	}.Digested()
	if err := base.Validate(); err != nil {
		t.Fatalf("a current submission was refused: %v", err)
	}

	for _, bad := range []string{"replay.corpus.v1", "replay.corpus.v3", "", "replay.watch.v1"} {
		wrong := base
		wrong.Schema = bad
		wrong = wrong.Digested()
		if err := wrong.Validate(); err == nil {
			t.Errorf("a document claiming schema %q validated. It is not a shape this "+
				"build writes, so reading it means guessing what its fields mean.", bad)
		}
	}
}

// CR-ABSENT: a v2 document that simply omits the figure is refused.
//
// The retired-spelling refusal was only half the guard, and a reviewer proved
// the other half was missing on 2026-09-13: a document declaring
// replay.corpus.v2 with no rebilledUsd key parsed to zero and PASSED Validate,
// because Go cannot tell an absent key from a zero value and Validate guards
// tasks, totals, tags and the digest but never the re-billed figure.
//
// That is the identical defect the spelling refusal was written to close,
// surviving inside the fix for it.
func TestCRABSENT_AMissingFigureIsNotAMeasuredZero(t *testing.T) {
	for _, missing := range []string{"rebilledUsd", "rebilledShare"} {
		doc := map[string]any{
			"schema": CorpusSchema, "takenAt": "2026-09-13T00:00:00Z", "tasks": 121,
			"totalUsd": 12630.61, "rebilledUsd": 347.53, "rebilledShare": 0.0275,
			"medianTaskUsd": 1.2, "pricedAt": "2026-09-13",
			"rulesVersion": "anthropic-2026-09-01", "unpriced": 0,
			"sourceTag": "abc", "tagBasis": "machine", "digest": "x",
		}
		delete(doc, missing)
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		var c Corpus
		err = json.Unmarshal(b, &c)
		if err == nil {
			t.Errorf("a document with no %q parsed cleanly to %v and Validate says %v. "+
				"A missing figure and a measured zero are different things.",
				missing, c.RebilledUSD, c.Validate())
			continue
		}
		if !strings.Contains(err.Error(), missing) {
			t.Errorf("the refusal does not name %q: %v", missing, err)
		}
	}

	// A genuine measured zero is still accepted. A perfect cache is a
	// submission worth having, and refusing it would be the opposite error.
	zero := Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-13T00:00:00Z", Tasks: 121,
		TotalUSD: 12630.61, RebilledUSD: 0, RebilledShare: 0,
		MedianTaskUSD: 1.2, PricedAt: "2026-09-13", RulesVersion: "anthropic-2026-09-01",
		SourceTag: "abc", TagBasis: "machine",
	}.Digested()
	b, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	var back Corpus
	if err := json.Unmarshal(b, &back); err != nil {
		t.Errorf("a measured zero was refused, which is the opposite error: %v", err)
	}
}
