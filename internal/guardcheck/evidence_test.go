package guardcheck

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are written here rather than kept on disk so the tests carry
// what they depend on. Each differs from evBefore in exactly one way, and that
// difference is what the test is about.
const (
	evCollide = `package sample

func f() error {
	obj, err := one()
	if err != nil {
		return err
	}
	id, err := two()
	if err != nil {
		return err
	}
	_, _ = obj, id
	return nil
}
func one() (int, error) { return 0, nil }
func two() (int, error) { return 0, nil }
`
	// An identical guard inserted ahead of the others.
	evAfter = `package sample

func f() error {
	z, err := zero()
	if err != nil {
		return err
	}
	obj, err := one()
	if err != nil {
		return err
	}
	id, err := two()
	if err != nil {
		return err
	}
	_, _, _ = obj, id, z
	return nil
}
func zero() (int, error) { return 0, nil }
func one() (int, error)  { return 0, nil }
func two() (int, error)  { return 0, nil }
`
	// The condition itself changed.
	evCondChanged = `package sample

func f() error {
	obj, err := one()
	if err != nil && obj == 0 {
		return err
	}
	return nil
}
func one() (int, error) { return 0, nil }
`
	// The statement feeding the guard changed.
	evProdChanged = `package sample

func f() error {
	obj, err := oneRenamed()
	if err != nil {
		return err
	}
	_ = obj
	return nil
}
func oneRenamed() (int, error) { return 0, nil }
`
)

// evFixture writes one source into its own directory and returns the path.
func evFixture(t *testing.T, src string) string {
	t.Helper()
	return evWrite(t, t.TempDir(), "sample.go", src)
}

// evPair writes two sources into the SAME directory.
//
// Pkg is part of the address, so two fixtures in different directories can
// never share one however alike their guards are. A comparison across separate
// TempDirs therefore passes for the wrong reason: it reports a difference the
// layout created rather than one the edit did. Both halves of an edit belong in
// one package, because that is what an edit is.
func evPair(t *testing.T, before, after string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	return evWrite(t, dir, "before.go", before), evWrite(t, dir, "after.go", after)
}

func evWrite(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// Two guards can share an Identity and still be different guards.
//
// Identity is {Pkg, Func, Cond} and is deliberately a grouping key: base-tree
// pairing spends it by count, because withholding an exemption it cannot
// justify is the safe direction. Policy B runs the other way. Granting
// evidence to one guard must never grant it to another that merely shares the
// grouping, so the manifest needs an address Identity cannot provide.
//
// Measured on internal/transcript/jev.go: 26 guards collapse into 18
// identities. jevEvaluation alone holds four `err != nil` conditions and
// jevAttempt three, each propagating a different call.
func TestEV1_GuardsSharingAnIdentityAreAddressedApart(t *testing.T) {
	gs, err := Conditionals(evFixture(t, evCollide), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 2 {
		t.Fatalf("want 2 guards, got %d", len(gs))
	}
	if gs[0].Identity() != gs[1].Identity() {
		t.Fatalf("the fixture no longer produces an identity collision, so it tests nothing")
	}
	a, b := gs[0].EvidenceAddress(), gs[1].EvidenceAddress()
	if a == b {
		t.Errorf("two guards propagating different calls share one evidence address %+v; "+
			"a manifest entry for either would exempt both", a)
	}
	if a.Producer == "" || b.Producer == "" {
		t.Errorf("the producing statement was not captured: %q / %q", a.Producer, b.Producer)
	}
}

// addrAt returns the address of the guard whose producer names the given call.
func addrAt(t *testing.T, file, producerSubstr string) EvidenceAddress {
	t.Helper()
	gs, err := Conditionals(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gs {
		if strings.Contains(g.Producer, producerSubstr) {
			return g.EvidenceAddress()
		}
	}
	t.Fatalf("no guard in %s produced by %q", file, producerSubstr)
	return EvidenceAddress{}
}

// EV2: inserting an identical guard ahead of an evidenced one does not
// retarget the evidence.
//
// This is the invariant the whole address exists for, and it is the one an
// ordinal cannot hold. Measured before the design was chosen: with an ordinal,
// the entry reviewed for the guard on one() matched the guard on zero() after
// the insertion, with every field of the address unchanged.
func TestEV2_InsertingAnIdenticalGuardDoesNotRetargetEvidence(t *testing.T) {
	beforeFile, afterFile := evPair(t, evCollide, evAfter)
	reviewed := addrAt(t, beforeFile, "one()")
	afterOne := addrAt(t, afterFile, "one()")
	afterZero := addrAt(t, afterFile, "zero()")

	if reviewed != afterOne {
		t.Errorf("the reviewed guard changed address when an unrelated guard was inserted "+
			"before it:\n  was %+v\n  now %+v", reviewed, afterOne)
	}
	if reviewed == afterZero {
		t.Fatalf("the entry reviewed for one() now matches the guard on zero(); evidence " +
			"migrated to a guard nobody looked at")
	}
}

// EV3: changing the producing statement invalidates the evidence.
func TestEV3_ChangingTheProducerInvalidatesEvidence(t *testing.T) {
	beforeFile, afterFile := evPair(t, evCollide, evProdChanged)
	if before, after := addrAt(t, beforeFile, "one()"),
		addrAt(t, afterFile, "oneRenamed()"); before == after {
		t.Error("the address survived a change to the call it propagates; the entry would " +
			"still match a guard whose meaning moved")
	}
}

// EV4: changing the condition invalidates the evidence.
func TestEV4_ChangingTheConditionInvalidatesEvidence(t *testing.T) {
	beforeFile, afterFile := evPair(t, evCollide, evCondChanged)
	before := addrAt(t, beforeFile, "one()")
	after := addrAt(t, afterFile, "one()")
	if before == after {
		t.Error("the address survived a change to the condition itself")
	}
}

// EV5: moving the function to another file does not invalidate the evidence
// when the guard is otherwise unchanged.
//
// Pkg is part of the address, so a move across packages does invalidate it and
// should: base-tree pairing draws the same line, because the suite that would
// observe a guard is a different suite one package over. Within a package a
// rename of the file must not cost a re-review.
func TestEV5_AFileRenameWithinAPackageKeepsTheAddress(t *testing.T) {
	a := addrAt(t, evFixture(t, evCollide), "one()")
	b := addrAt(t, evFixture(t, evCollide), "one()")
	// Same func, cond and producer; only Pkg differs because the fixture dirs
	// differ. Compare the parts a within-package rename preserves.
	if a.Func != b.Func || a.Cond != b.Cond || a.Producer != b.Producer {
		t.Errorf("a file rename changed the guard's identity beyond its package:\n  %+v\n  %+v", a, b)
	}
}

// EV6: a manifest entry exempts exactly the guard it names.
func TestEV6_EvidenceExemptsOnlyTheGuardItNames(t *testing.T) {
	gs, err := Conditionals(evFixture(t, evCollide), nil)
	if err != nil {
		t.Fatal(err)
	}
	m := mustManifest(t, Evidence{
		EvidenceAddress: gs[0].EvidenceAddress(),
		Category:        Dominated,
		Justification:   "dominated by the next check",
		Evidence:        "probed against the fixture corpus",
	})
	evidenced, unexplained, stale := ClassifyEvidenced(gs, m)
	if len(evidenced) != 1 || evidenced[0].Guard.Line != gs[0].Line {
		t.Fatalf("evidenced = %d, want exactly the named guard", len(evidenced))
	}
	if len(unexplained) != 1 || unexplained[0].Line != gs[1].Line {
		t.Errorf("the guard with no entry was not left unexplained: %+v", unexplained)
	}
	if len(stale) != 0 {
		t.Errorf("stale = %d, want 0", len(stale))
	}
}

// EV7: an entry matching no surviving introduced guard is stale and fails.
func TestEV7_StaleEvidenceIsReported(t *testing.T) {
	m := mustManifest(t, Evidence{
		EvidenceAddress: EvidenceAddress{Pkg: "./x", Func: "gone", Cond: "err != nil", Producer: "v, err := f()"},
		Category:        Dominated,
		Justification:   "j",
		Evidence:        "e",
	})
	_, _, stale := ClassifyEvidenced(nil, m)
	if len(stale) != 1 {
		t.Fatalf("stale = %d, want 1: an entry nobody can tie to a guard has stopped "+
			"describing the tree", len(stale))
	}
}

// EV8: the manifest refuses what it cannot trust.
func TestEV8_TheManifestFailsClosed(t *testing.T) {
	ok := `[{"pkg":"./p","func":"f","cond":"err != nil","producer":"v, err := g()",` +
		`"category":"dominated","justification":"j","evidence":"e"}]`
	bad := map[string]string{
		"malformed json":          `[`,
		"unknown category":        strings.Replace(ok, `"dominated"`, `"because-i-said-so"`, 1),
		"empty justification":     strings.Replace(ok, `"justification":"j"`, `"justification":""`, 1),
		"empty evidence":          strings.Replace(ok, `"evidence":"e"`, `"evidence":""`, 1),
		"missing func":            strings.Replace(ok, `"func":"f"`, `"func":""`, 1),
		"missing cond":            strings.Replace(ok, `"cond":"err != nil"`, `"cond":""`, 1),
		"duplicate address":       `[` + ok[1:len(ok)-1] + `,` + ok[1:len(ok)-1] + `]`,
		"wildcard is not a thing": strings.Replace(ok, `"cond":"err != nil"`, `"cond":"*"`, 1),
	}
	for name, raw := range bad {
		t.Run(name, func(t *testing.T) {
			m, err := ParseManifest([]byte(raw))
			if name == "wildcard is not a thing" {
				// A literal "*" is simply a condition no guard has. It parses,
				// and it exempts nothing, which is the point: there is no
				// wildcard to reject because the address has no wildcard form.
				if err != nil {
					t.Fatalf("a literal condition should parse: %v", err)
				}
				if _, unexplained, stale := ClassifyEvidenced([]Guard{{Pkg: "./p", Func: "f", Cond: "err != nil"}}, m); len(unexplained) != 1 || len(stale) != 1 {
					t.Errorf("a star matched something; unexplained=%d stale=%d", len(unexplained), len(stale))
				}
				return
			}
			if err == nil {
				t.Errorf("%s was accepted; the manifest must fail closed", name)
			}
		})
	}
}

// EV9: a nil manifest exempts nothing.
func TestEV9_NoManifestExemptsNothing(t *testing.T) {
	gs := []Guard{{Pkg: "./p", Func: "f", Cond: "err != nil", Producer: "v, err := g()"}}
	evidenced, unexplained, _ := ClassifyEvidenced(gs, nil)
	if len(evidenced) != 0 || len(unexplained) != 1 {
		t.Errorf("a nil manifest exempted something: evidenced=%d unexplained=%d",
			len(evidenced), len(unexplained))
	}
}

func mustManifest(t *testing.T, entries ...Evidence) *Manifest {
	t.Helper()
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// EV10: evidenced guards stop failing the run; nothing else does.
func TestEV10_OnlyEvidencedGuardsStopFailingTheRun(t *testing.T) {
	g := Guard{Pkg: "./p", Func: "f", Cond: "err != nil", Producer: "v, err := g()"}
	cases := []struct {
		name                            string
		unexplained, unchecked, unbuilt []Guard
		stale                           []Evidence
		want                            int
	}{
		{"nothing left over", nil, nil, nil, nil, 0},
		{"an unexplained survivor", []Guard{g}, nil, nil, nil, 1},
		{"a mutant the compiler rejected", nil, []Guard{g}, nil, nil, 1},
		{"a file this host does not build", nil, nil, []Guard{g}, nil, 1},
		{"an entry tied to no guard", nil, nil, nil, []Evidence{{Justification: "j", Evidence: "e"}}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExitCodeWithEvidence(c.unexplained, c.unchecked, c.unbuilt, c.stale); got != c.want {
				t.Errorf("exit = %d, want %d", got, c.want)
			}
		})
	}
}

// EV11: the evidence mechanism does not touch base-tree pairing.
//
// Identity is still {Pkg, Func, Cond}. Two guards that differ only in the
// statement feeding them are one identity to PairSurvivors and two addresses to
// the manifest, and that is the whole separation. If Identity ever grew the
// producer, a base tree holding one of them would start exempting the wrong
// count of survivors here.
func TestEV11_PairingIdentityIsUnchangedByTheAddress(t *testing.T) {
	a := Guard{Pkg: "./p", Func: "f", Cond: "err != nil", Producer: "x, err := one()"}
	b := Guard{Pkg: "./p", Func: "f", Cond: "err != nil", Producer: "y, err := two()"}
	if a.Identity() != b.Identity() {
		t.Error("Identity started reading the producer; base-tree pairing would change")
	}
	if a.EvidenceAddress() == b.EvidenceAddress() {
		t.Error("the evidence address stopped telling the two apart")
	}
	// The budget still pairs by identity, one for one, as it always did.
	pre, introduced := PairSurvivors([]Guard{a, b}, map[Identity]int{a.Identity(): 1})
	if len(pre) != 1 || len(introduced) != 1 {
		t.Errorf("pairing changed: pre=%d introduced=%d, want 1 and 1", len(pre), len(introduced))
	}
}

// An if with its own init is addressed by that init, not by what precedes it.
//
// The producing statement is the whole of what separates two guards sharing an
// identity, and `if v, err := f(); err != nil` carries its producer inside
// itself. Reading the preceding statement for those would be right most of the
// time and wrong exactly where it matters: two such guards in one function,
// with the same condition and the same statement above them, would collapse
// onto one address, and a manifest entry for either would exempt both.
//
// The fixture is built for that collision. Both ifs are preceded by the same
// statement and differ only in their init, so a reader that prefers the
// preceding statement cannot tell them apart.
const evInitVsPrev = `package p

func f() error {
	_ = 0
	if a, err := one(); err != nil {
		_ = a
		return err
	}
	_ = 0
	if b, err := two(); err != nil {
		_ = b
		return err
	}
	return nil
}

func one() (int, error) { return 0, nil }
func two() (int, error) { return 0, nil }
`

func TestEV12_AnInitClauseIsThePreferredProducer(t *testing.T) {
	gs, err := Conditionals(evFixture(t, evInitVsPrev), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 2 {
		t.Fatalf("want 2 guards, got %d", len(gs))
	}
	if gs[0].Identity() != gs[1].Identity() {
		t.Fatalf("the fixture no longer produces an identity collision, so it tests nothing")
	}
	for i, want := range []string{"a, err := one()", "b, err := two()"} {
		if got := gs[i].Producer; got != want {
			t.Errorf("guard %d producer = %q, want the init clause %q: an if that carries "+
				"its own producer must be addressed by it", i, got, want)
		}
		if gs[i].Producer == "_ = 0" {
			t.Errorf("guard %d was addressed by the statement above it rather than its own "+
				"init clause", i)
		}
	}
	if a, b := gs[0].EvidenceAddress(), gs[1].EvidenceAddress(); a == b {
		t.Errorf("both guards resolved to one address %+v; an entry for either would "+
			"exempt the other", a)
	}
}

// A manifest that is not there is an empty manifest, not a failure.
//
// Nothing evidenced is the ordinary state of a repository, and most of this
// tree has no manifest at all. A reviewer that refused to run without one would
// fail every change that touches a conditional anywhere, which is the opposite
// of what Policy B is for. Absence and unreadability are told apart here and
// nowhere else, so this is the only place the distinction is observable.
func TestEV13_AnAbsentManifestIsEmptyRatherThanAnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-file.json")
	m, err := LoadManifest(missing)
	if err != nil {
		t.Fatalf("LoadManifest on an absent file returned %v; a repository with nothing "+
			"evidenced is the ordinary case and must not fail on the missing file", err)
	}
	if m == nil {
		t.Fatal("LoadManifest returned a nil manifest and a nil error; the caller would " +
			"exempt nothing and could not say why")
	}
	// An empty manifest exempts nothing, which is what makes the absence safe.
	g := Guard{Pkg: "./p", Func: "f", Cond: "err != nil", Producer: "v, err := g()"}
	evidenced, unexplained, stale := ClassifyEvidenced([]Guard{g}, m)
	if len(evidenced) != 0 || len(unexplained) != 1 || len(stale) != 0 {
		t.Errorf("an absent manifest exempted something: evidenced=%d unexplained=%d stale=%d, "+
			"want 0, 1 and 0", len(evidenced), len(unexplained), len(stale))
	}
}

// A file that will not read is refused, and refused as a read failure.
//
// This is the fail-open the reviewer exists to refuse. A manifest that cannot
// be read is not a manifest that is absent, and reading the first as the second
// would turn an unreadable file into "nothing is evidenced" and let a survivor
// through under a verdict nobody wrote.
//
// The assertion is on what the error carries rather than on its text. Dropping
// this branch does not stop the call failing, because the nil bytes then reach
// the parser and it refuses them too. What is lost is which failure it was: the
// error stops wrapping the OS error and starts claiming the file held malformed
// JSON, so a reader is sent to fix a file they cannot open.
func TestEV14_AnUnreadableManifestIsARefusalNotAnEmptyOne(t *testing.T) {
	// A directory is readable as a path and not as a file, on every platform
	// the suite runs on, and it is not ENOENT.
	dir := t.TempDir()
	m, err := LoadManifest(dir)
	if err == nil {
		t.Fatalf("LoadManifest on an unreadable path returned no error and manifest %+v; "+
			"an unreadable manifest is not an empty one", m)
	}
	if m != nil {
		t.Errorf("a manifest came back alongside the error: %+v", m)
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("error %q does not carry the underlying read failure; the run would "+
			"report a parse problem over a file it never opened", err)
	}
}

// Stale entries are ordered by the guard they name, not by the statement above
// it.
//
// The order is what a reader sees when a run fails on entries that no longer
// resolve, and entries are grouped by function so the reader can work through
// one function at a time. The producer is the tiebreak within a function and
// not the sort. The two entries below disagree: ordering by function puts the
// first one first, and ordering by producer reverses them.
func TestEV15_StaleEntriesSortByFunctionBeforeProducer(t *testing.T) {
	raw := `[
	  {"pkg":"./p","func":"aaa","cond":"err != nil","producer":"zzz, err := z()",
	   "category":"dominated","justification":"j","evidence":"e"},
	  {"pkg":"./p","func":"bbb","cond":"err != nil","producer":"aaa, err := a()",
	   "category":"dominated","justification":"j","evidence":"e"}
	]`
	m, err := ParseManifest([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	// No introduced guards, so both entries are stale and the whole list is
	// what the comparator ordered.
	_, _, stale := ClassifyEvidenced(nil, m)
	if len(stale) != 2 {
		t.Fatalf("want 2 stale entries, got %d", len(stale))
	}
	// The fixture checks are on the set rather than on the order, so the only
	// thing a wrong comparator can trip is the assertion below it.
	producerOfFunc := map[string]string{}
	for _, e := range stale {
		producerOfFunc[e.Func] = e.Producer
	}
	if len(producerOfFunc) != 2 {
		t.Fatal("the fixture no longer differs by function, so it tests nothing")
	}
	if producerOfFunc["aaa"] <= producerOfFunc["bbb"] {
		t.Fatal("the fixture's producers no longer disagree with its functions, so " +
			"ordering by either would give the same answer and the test proves nothing")
	}
	if got := []string{stale[0].Func, stale[1].Func}; got[0] != "aaa" || got[1] != "bbb" {
		t.Errorf("stale order = %v, want [aaa bbb]: the entries were ordered by the "+
			"statement above the guard rather than by the guard's own function", got)
	}
}
