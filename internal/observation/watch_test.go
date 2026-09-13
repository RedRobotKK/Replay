package observation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/consent"
)

// WA. The Watch record: one session, one repository, counts only.
//
// Replay reads one machine once. Watch is the standing version: a record per
// session per repository, produced where the session happened and carried to
// whoever owns the invoice, so that "spend on this repo went up and nobody can
// say which week or which change" has an answer.
//
// THE BOUNDARY IS THE PRODUCT, so it is a test rather than a paragraph.
// Counts, ratios, cause classes and hour-resolution timestamps travel. Content
// does not: no prompt, no path, no tool name, no file name, no session id. The
// bridge between them is one local command the digest ends with, and that
// command runs on the operator's machine against data that never left it.
//
// This package still cannot send. TestO7_ThisPackageCannotSend walks these
// imports and there is no transport among them; a record is a value and a file,
// and something outside this package posts it. That separation is what lets the
// binary keep saying it makes no network call while a plugin the user
// configured makes one.

func watchFixture() Watch {
	return Watch{
		Schema:          WatchSchema,
		Repo:            "acme/billing",
		SessionTag:      "9f2c4e1a77b0d3e5",
		StartedAt:       "2026-09-07T14:00:00Z",
		EndedAt:         "2026-09-07T16:00:00Z",
		Provider:        "anthropic",
		Model:           "claude-opus-5",
		Lanes:           3,
		Turns:           118,
		TotalUSD:        1204.10,
		AvoidableUSD:    96.30,
		AvoidableTokens: 412_004,
		BreaksByCause:   map[string]int{"toolChange": 7, "ttlExpiry": 2},
		IdleGapsOverTTL: 6,
		Compactions:     3,
		BinaryVersion:   "v0.6.0",
		Commit:          "cccc3f0",
		PricingDigest:   "p02eb9163145c",
		RulesVersion:    "anthropic-2026-09-01",
	}
}

// WA1: nothing in the record can carry content.
//
// The field names are checked against the same banned list the corpus payload
// uses, and the wire form is checked rather than the struct, because a json tag
// can differ from its field name and the wire form is what leaves.
//
// PASS: no field names or carries a path, a prompt, a tool or a file.
// FAIL: the product's entire positioning is gone, and it is gone in a way a
// customer discovers by reading their own outbound traffic.
func TestWA1_NoFieldCanCarryContent(t *testing.T) {
	b, err := json.Marshal(watchFixture().Digested())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}

	banned := []string{"path", "prompt", "tool", "file", "content", "text", "message",
		"project", "dir", "home", "user", "host", "session_id", "sessionId"}
	for k := range m {
		low := strings.ToLower(k)
		for _, bad := range banned {
			if strings.Contains(low, strings.ToLower(bad)) && low != "sessiontag" {
				t.Errorf("the wire form carries a field named %q, which reads as content", k)
			}
		}
	}
}

// WA2: the session tag identifies a session to itself and nobody to anyone.
//
// It de-duplicates re-reads, so the same session read twice does not count
// twice. It must not be reversible to a session id, and it must not be stable
// across repositories, or two customers could be correlated by it.
func TestWA2_TheSessionTagDeduplicatesAndIdentifiesNobody(t *testing.T) {
	same := WatchSessionTag("repo-key-a", "session-1")
	if again := WatchSessionTag("repo-key-a", "session-1"); again != same {
		t.Error("the same session under the same key produced two tags, so re-reads would double count")
	}
	if other := WatchSessionTag("repo-key-a", "session-2"); other == same {
		t.Error("two sessions produced one tag, so distinct work would collapse into one record")
	}
	if crossRepo := WatchSessionTag("repo-key-b", "session-1"); crossRepo == same {
		t.Error("one session tagged under two repository keys produced the same tag, which " +
			"lets two customers be correlated by a value neither of them chose")
	}
	if strings.Contains(same, "session-1") {
		t.Errorf("the tag contains the session id it was derived from: %q", same)
	}
	if len(same) != 16 {
		t.Errorf("the tag is %d characters, want 16", len(same))
	}
}

// WA3: timestamps are hour resolution.
//
// The contribution-payload evidence already records why: a stable tag over
// repeated submissions is a spend time series, and a minute is closer to a
// keystroke than to a measurement. An hour orders the week, which is the job,
// and does not say when somebody was at their desk.
func TestWA3_TimestampsAreHourResolution(t *testing.T) {
	w := watchFixture()
	for _, ts := range []string{w.StartedAt, w.EndedAt} {
		if !strings.HasSuffix(ts, ":00:00Z") {
			t.Errorf("%q is finer than hour resolution, which turns a weekly figure into "+
				"a record of when a person was working", ts)
		}
	}
	if err := w.Digested().Validate(); err != nil {
		t.Fatalf("the fixture does not validate: %v", err)
	}
}

// WA4: a record says which build priced it.
//
// Same reason as the corpus submission, and the reason is measured: two builds
// priced one corpus at $4,088.49 and $11,969.37 under one rules label. A weekly
// timeline that silently mixes them would report a step change in spend that
// was a step change in arithmetic, which is the worst failure this product has
// available to it.
func TestWA4_ARecordNamesTheBuildThatPricedIt(t *testing.T) {
	b, _ := json.Marshal(watchFixture().Digested())
	for _, key := range []string{"binaryVersion", "pricingDigest", "rulesVersion"} {
		if !strings.Contains(string(b), key) {
			t.Errorf("a Watch record has no %q. A timeline that mixes two builds shows an "+
				"arithmetic change as a spend change.", key)
		}
	}
}

// WA5: a record with nothing measured is refused.
//
// Same rule as the corpus: nothing measured is not a pass. A week of empty
// records would draw a flat line that a reader takes for a quiet week.
func TestWA5_NothingMeasuredIsRefused(t *testing.T) {
	empty := watchFixture()
	empty.Turns = 0
	if err := empty.Digested().Validate(); err == nil {
		t.Error("a record with no turns validated. Aggregated into a week it draws a flat " +
			"line that reads as a quiet week rather than as an absent one.")
	}

	noRepo := watchFixture()
	noRepo.Repo = ""
	if err := noRepo.Digested().Validate(); err == nil {
		t.Error("a record with no repository validated, and the repository is the unit")
	}
}

// WA6: the digest covers the record, so a page that lists one cannot restate it.
func TestWA6_TheDigestCoversTheRecord(t *testing.T) {
	base := watchFixture().Digested()
	moved := watchFixture()
	moved.AvoidableUSD++
	if moved.Digested().Digest == base.Digest {
		t.Error("changing the avoidable figure did not change the digest, so a published " +
			"record could be restated after the fact")
	}
}

// WA7: a machine that has not opted in produces no record at all.
//
// Not an empty record, not a record the sender is trusted to drop: no value.
// The gate is at construction rather than at the sending edge because an edge
// check is something a second code path gets added around, and the second code
// path is always written by somebody who did not read this comment.
//
// Unset is refused exactly as Declined is. Silence is not a grant anywhere in
// this repository, and this is where that rule earns its keep: Watch is the
// first thing here that a machine would send on a schedule with nobody present
// for the individual send.
func TestWA7_NoRecordWithoutConsent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		state   consent.State
		allowed bool
	}{
		{"nobody has been asked", consent.Unset, false},
		{"the operator said no", consent.Declined, false},
		{"the operator said yes", consent.Granted, true},
	} {
		got, err := BuildWatch(consent.Decision{State: tc.state}, watchFixture())
		switch {
		case tc.allowed && err != nil:
			t.Errorf("%s: BuildWatch refused a consented machine: %v", tc.name, err)
		case tc.allowed && got.Digest == "":
			t.Errorf("%s: BuildWatch returned an undigested record", tc.name)
		case !tc.allowed && err == nil:
			t.Errorf("%s: BuildWatch produced a record anyway. The opt-in is a comment, "+
				"not a mechanism.", tc.name)
		case !tc.allowed && !errors.Is(err, ErrWatchNotConsented):
			t.Errorf("%s: refused with %v, which a caller cannot tell from a measurement "+
				"failure", tc.name, err)
		case !tc.allowed && (got.Repo != "" || got.Turns != 0 || got.TotalUSD != 0 ||
			got.SessionTag != "" || got.Digest != "" || len(got.BreaksByCause) != 0):
			t.Errorf("%s: a refusal still returned a populated record: %+v", tc.name, got)
		}
	}
}

// WA8: the refusal is distinguishable from a session that measured nothing.
//
// Both arrive at the far end as no record. The operator standing at the
// terminal has to be told which one happened, because "watch is off" is fixed
// by a command and "nothing measured" is not fixed by anything.
func TestWA8_RefusalIsNotAMeasurementFailure(t *testing.T) {
	_, off := BuildWatch(consent.Decision{State: consent.Unset}, watchFixture())
	empty := watchFixture()
	empty.Turns = 0
	_, nothing := BuildWatch(consent.Decision{State: consent.Granted}, empty)

	if off == nil || nothing == nil {
		t.Fatal("one of the two refusals did not happen")
	}
	if errors.Is(nothing, ErrWatchNotConsented) {
		t.Error("an empty measurement reports itself as a consent refusal, so an operator " +
			"would go looking for a setting that is already correct")
	}
}

// WA9: Watch consent is its own grant and does not ride on the corpus one.
//
// The three files in internal/consent answer three different questions. Corpus
// consent permits BUILDING a submission that a human then moves by hand, and
// its own file says nothing in the release transmits it. Watch consent permits
// a machine to send on a schedule. Somebody who agreed to the first has not
// agreed to the second, and a design that read one file for both would have
// silently upgraded their answer.
func TestWA9_WatchConsentIsItsOwnFile(t *testing.T) {
	if consent.WatchFileName == consent.CorpusFileName {
		t.Fatal("watch and corpus read the same consent file, so opting into a file a " +
			"person carries also opts into a machine that sends on a schedule")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "replay"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replay", consent.CorpusFileName),
		[]byte("corpus_opt_in = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := consent.ReadWatchConsent(dir)
	if err != nil {
		t.Fatalf("reading watch consent on a corpus-consented machine: %v", err)
	}
	if d.Allowed() {
		t.Error("a corpus opt-in granted watch as well")
	}
	if !d.ShouldAsk() {
		t.Error("a machine that has never been asked about watch reports that it has been")
	}
}

// WA10: every guard in Validate refuses something.
//
// ADR-0014: a check that cannot fail is not evidence. WA3 through WA5 cover
// four of these; the rest were written in the same commit and would otherwise
// ship as unexercised branches that are wrong the first time they are taken.
// One table so that adding a guard without a case for it is visible in review.
func TestWA10_EveryValidateGuardRefusesSomething(t *testing.T) {
	for _, tc := range []struct {
		guard  string
		damage func(*Watch)
	}{
		{"schema", func(w *Watch) { w.Schema = "replay.watch.v0" }},
		{"repo", func(w *Watch) { w.Repo = "" }},
		{"sessionTag", func(w *Watch) { w.SessionTag = "" }},
		{"turns", func(w *Watch) { w.Turns = 0 }},
		{"startedAt absent", func(w *Watch) { w.StartedAt = "" }},
		{"endedAt absent", func(w *Watch) { w.EndedAt = "" }},
		{"startedAt to the minute", func(w *Watch) { w.StartedAt = "2026-09-07T14:37:00Z" }},
		{"endedAt to the second", func(w *Watch) { w.EndedAt = "2026-09-07T16:00:41Z" }},
		{"binaryVersion", func(w *Watch) { w.BinaryVersion = "" }},
		{"pricingDigest", func(w *Watch) { w.PricingDigest = "" }},
		{"rulesVersion", func(w *Watch) { w.RulesVersion = "" }},
	} {
		w := watchFixture()
		tc.damage(&w)
		w = w.Digested()
		if err := w.Validate(); err == nil {
			t.Errorf("Validate accepted a record with a broken %s, so that guard cannot "+
				"fail and is not evidence", tc.guard)
		}
	}

	// And the digest guard, which cannot be reached through Digested because
	// Digested is what fills it in.
	undigested := watchFixture()
	if err := undigested.Validate(); err == nil {
		t.Error("Validate accepted a record with no digest, so nothing names it and a " +
			"published figure could be restated after the fact")
	}
}

// WA11: the watch file is read for the watch answer, not the corpus one.
//
// WA9 proves the two grants are different FILES. This proves they are different
// QUESTIONS, and the mutation that motivated it survived WA9 cleanly: point
// ReadWatchConsent at the key "corpus_opt_in" and a machine whose watch file
// says watch_opt_in = true reads back as Unset. Nothing sends, so nothing looks
// broken, and the operator's answer is discarded in silence. That is worse than
// the reverse failure, because the reverse one is visible.
func TestWA11_WatchConsentReadsTheWatchAnswer(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "replay"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replay", consent.WatchFileName),
		[]byte("watch_opt_in = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := consent.ReadWatchConsent(dir)
	if err != nil {
		t.Fatalf("reading a watch file that says yes: %v", err)
	}
	if !d.Allowed() {
		t.Errorf("a watch file saying watch_opt_in = true read back as %s. The operator "+
			"answered and the answer was discarded without an error.", d.State)
	}
}

// WA12: a cause class nobody defined is refused.
//
// BreaksByCause is a map, so it is the one place in this record where a caller
// chooses a key rather than filling in a field. A class nobody defined is a
// number nobody can check, and on a dashboard it is worse than absent: it gets
// a row, a colour and a share of the total.
func TestWA12_AnUndefinedCauseClassIsRefused(t *testing.T) {
	w := watchFixture()
	w.BreaksByCause = map[string]int{"toolChange": 3, "becauseTheModelFeltLikeIt": 9}
	if err := w.Digested().Validate(); err == nil {
		t.Error("a record carrying a cause class nothing defines validated. On a page it " +
			"gets a row and a share of the total, which is worse than being absent.")
	}
	for _, c := range WatchCauses() {
		ok := watchFixture()
		ok.BreaksByCause = map[string]int{c: 1}
		if err := ok.Digested().Validate(); err != nil {
			t.Errorf("the admitted class %q was refused: %v", c, err)
		}
	}
}

// WA13: the repository key is the one string a person types, so it is the one
// place a path could get in.
//
// Every other field is a number, a date, a digest or a value this binary
// computed. WA1 proves no FIELD can carry content; this proves the single field
// that carries free text cannot either. A key is a label the customer chose,
// not a directory the tool discovered, and the charset is what makes those two
// different things rather than the same string arriving by different routes.
func TestWA13_TheRepositoryKeyCannotCarryAPath(t *testing.T) {
	for _, bad := range []string{
		"/Users/daniel/Development/Replay-clean",
		"../../etc/passwd",
		"C:\\Users\\daniel\\secret",
		"acme/billing/../../home",
		"~/work/acme",
		"acme billing",
		"ACME/Billing",
		"acme/billing/deep/nesting",
		"acme/billing/deep",
		"acme/..",
		"../acme",
		"acme/.",
		strings.Repeat("a", 65),
		"",
	} {
		w := watchFixture()
		w.Repo = bad
		if err := w.Digested().Validate(); err == nil {
			t.Errorf("the repository key %q was accepted. The one typed field is where a "+
				"path gets in, and this record is posted somewhere.", bad)
		}
	}
	for _, good := range []string{"acme/billing", "billing", "acme-corp/web.api", "a_b/c-d"} {
		w := watchFixture()
		w.Repo = good
		if err := w.Digested().Validate(); err != nil {
			t.Errorf("the ordinary key %q was refused: %v", good, err)
		}
	}
}

// WA14: writing a record does not replace one that has not been sent, and does
// not write through a redirected path.
//
// Same two refusals as the corpus writer and the same reason. A record is the
// artifact a person or a hook is about to post; silently replacing one loses a
// session nobody knows is missing, and following a symlink means somebody other
// than the operator chose where this operator's data lands.
func TestWA14_WritingARecordRefusesToReplaceOrRedirect(t *testing.T) {
	dir := t.TempDir()
	w, err := BuildWatch(consent.Decision{State: consent.Granted}, watchFixture())
	if err != nil {
		t.Fatal(err)
	}
	path, err := WriteWatch(dir, w)
	if err != nil {
		t.Fatalf("writing the first record: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("the record is mode %o. It is a record of what one account spent, and the "+
			"default umask would publish it to everyone with a login", perm)
	}
	if _, err := WriteWatch(dir, w); err == nil {
		t.Error("writing the same record twice replaced the first, so a session that was " +
			"never posted is gone and nothing says so")
	}

	// A redirected path. The name is derived from the record, so the link has to
	// be made at the name WriteWatch will choose.
	other := t.TempDir()
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.Symlink(target, filepath.Join(other, filepath.Base(path))); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	_, symErr := WriteWatch(other, w)
	if symErr == nil {
		t.Fatal("a record was written through a symlink, so somebody other than the " +
			"operator chose where their spend lands")
	}
	// The MESSAGE, not just the refusal. Lstat succeeds on a symlink, so the
	// already-exists branch would refuse this too and the write is safe either
	// way; what the symlink branch changes is what the operator is told. Those
	// are two different things to go and do, and neutralising the branch leaves
	// a refusal that sends them to delete a file that is not the problem.
	if !strings.Contains(symErr.Error(), "symlink") {
		t.Errorf("the refusal does not say the path is redirected, so the operator is "+
			"told to move a file when the problem is where it points: %v", symErr)
	}

	// An unvalidatable record never reaches the disk at all.
	bad := watchFixture()
	bad.Turns = 0
	empty := t.TempDir()
	if _, err := WriteWatch(empty, bad.Digested()); err == nil {
		t.Error("a record that does not validate was written to disk anyway")
	}
	if entries, _ := os.ReadDir(empty); len(entries) != 0 {
		t.Errorf("a refused write still left %d file(s) behind", len(entries))
	}
}

// WA15: a populated record stays small, and the number is pinned.
//
// Same reason as the corpus payload's size pin. This is a record a hook posts
// at the end of every session, so its size is a standing cost on somebody
// else's network and a standing claim in the documentation. A field added
// without thought is how "roughly 600 bytes" quietly becomes six kilobytes,
// and the person who finds out is the customer reading their own egress.
func TestWA15_APopulatedRecordStaysSmall(t *testing.T) {
	w, err := BuildWatch(consent.Decision{State: consent.Granted}, watchFixture())
	if err != nil {
		t.Fatal(err)
	}
	// Every cause class present, which is the largest this record gets.
	full := w
	full.BreaksByCause = map[string]int{}
	for _, c := range WatchCauses() {
		full.BreaksByCause[c] = 999
	}
	full = full.Digested()

	body, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	const limit = 1024
	if len(body) > limit {
		t.Errorf("a fully populated record is %d bytes, over the %d this project documents. "+
			"It is posted at the end of every session, so the size is a standing cost on "+
			"somebody else's network and a claim in the docs", len(body), limit)
	}
	t.Logf("fully populated record: %d bytes of %d", len(body), limit)

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	const fields = 20
	if len(m) != fields {
		t.Errorf("the wire form has %d fields, not %d. If that is deliberate, change this "+
			"number and docs/design/replay-watch.md in the same commit, because the two "+
			"are a promise about what travels", len(m), fields)
	}
}
