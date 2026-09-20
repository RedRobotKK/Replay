package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// A dollar figure on its own does not say what kind of dollar it is.
//
// `since` filters replay.cost.v2 rather than deriving anything again, so its
// figures are the cost command's figures exactly. The cost command says what
// they are: "at list prices dated <date> (caching rules <version>)". The digest
// printed the same numbers and said none of it, so `$12.34` under "Since you
// last looked" read as an amount charged rather than as list price applied to
// observed token counts.
//
// The disclosure is the whole change. Nothing here recalculates, re-prices, or
// gates a figure, and the basis is read from the same constants the cost
// command uses rather than written down a second time.

// shiftedFixture writes the shared transcript fixture with every timestamp
// moved so the session's first request lands at `first`.
//
// atFixture sets the file's mtime, and the window is not decided by mtime. The
// cost report dates a session from the timestamp inside its first request, so
// the fixture's recorded 2026-09-02 sits outside every window a test can
// establish and the digest reports a quiet one forever. Shifting the content is
// what puts work inside the window; a single constant delta does it without
// disturbing any interval the session actually recorded.
func shiftedFixture(t *testing.T, root, name string, first time.Time) {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	stamp := regexp.MustCompile(`"timestamp":"([^"]+)"`)
	var delta time.Duration
	var anchored bool
	body := stamp.ReplaceAllStringFunc(string(b), func(m string) string {
		ts, err := time.Parse(time.RFC3339Nano, stamp.FindStringSubmatch(m)[1])
		if err != nil {
			return m
		}
		if !anchored {
			delta, anchored = first.Sub(ts), true
		}
		return fmt.Sprintf(`"timestamp":"%s"`, ts.Add(delta).UTC().Format("2006-01-02T15:04:05.000Z"))
	})
	if !anchored {
		t.Fatal("the fixture carried no timestamp to shift, so no window can be built from it")
	}
	proj := filepath.Join(root, name)
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sinceWindow establishes a marker, lands priced work after it, and returns the
// digest that reports on it. A first run has no marker and says so, which is
// SN1's subject, so a populated window needs the second look.
func sinceWindow(t *testing.T) string {
	t.Helper()
	root := sinceEnv(t)

	var a, ae bytes.Buffer
	if err := run([]string{"since"}, &a, &ae); err != nil {
		t.Fatalf("first look: %v\n%s", err, ae.String())
	}
	shiftedFixture(t, root, "proj", time.Now().Add(time.Minute))

	var b, be bytes.Buffer
	if err := run([]string{"since"}, &b, &be); err != nil {
		t.Fatalf("second look: %v\n%s", err, be.String())
	}
	out := b.String() + be.String()
	// Not a skip. A window with no figure in it means this helper stopped
	// building the thing every test below is about, and a test that quietly
	// passes when its subject is absent is not evidence of anything.
	if !strings.Contains(out, "$") {
		t.Fatalf("the window carried no priced session, so there is no figure to qualify:\n%s", out)
	}
	return out
}

// sinceSource reads since.go, so the two structural tests below assert against
// the command rather than against a rendering of it.
func sinceSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("since.go")
	if err != nil {
		t.Fatalf("read since.go: %v", err)
	}
	return string(b)
}

// SB1: a figure comes with its basis.
//
// The price table and the caching rules both decide the number, and a reader
// who cannot see which ones were used cannot tell a stale table from a change
// in their own spend.
func TestSB1_ADollarFigureCarriesItsPriceAndRulesBasis(t *testing.T) {
	out := sinceWindow(t)
	if !strings.Contains(out, cachemodel.PriceTableVersion) {
		t.Errorf("the price-table date (%s) is not in the digest:\n%s",
			cachemodel.PriceTableVersion, out)
	}
	if !strings.Contains(out, cachemodel.RulesVersionInEffect()) {
		t.Errorf("the caching-rules version (%s) is not in the digest:\n%s",
			cachemodel.RulesVersionInEffect(), out)
	}
}

// SB2: the figure is named as list price, not as an amount charged.
//
// The wording is the cost command's, because these are the cost command's
// numbers and a second vocabulary for one quantity is how two surfaces start
// meaning different things.
func TestSB2_TheFigureIsNamedAsListPrice(t *testing.T) {
	out := strings.ToLower(sinceWindow(t))
	if !strings.Contains(out, "list price") {
		t.Errorf("the digest does not say the figures are list prices, so a reader "+
			"cannot tell them from an amount charged:\n%s", out)
	}
	// Nothing may claim the figure is what the provider actually charged.
	for _, banned := range []string{"amount charged", "actual cost", "you were billed", "invoice"} {
		if strings.Contains(out, banned) {
			t.Errorf("the digest claims %q, which these figures do not establish:\n%s", banned, out)
		}
	}
}

// SB3: disclosure changed no figure.
//
// The same window reported before and after must carry the same numbers. This
// is a presentation change and the arithmetic is not this change's business.
func TestSB3_TheFiguresAreUnchangedByDisclosure(t *testing.T) {
	out := sinceWindow(t)
	// The digest's own figures, read back out of the line that carries them.
	var figures []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "session(s)") || strings.Contains(line, "re-billed") {
			figures = append(figures, strings.TrimSpace(line))
		}
	}
	if len(figures) == 0 {
		t.Fatalf("no figure line found to compare:\n%s", out)
	}
	// The basis must be its own statement, not spliced into the figure lines,
	// so the numbers a reader scans stay the shape they were.
	for _, f := range figures {
		if strings.Contains(strings.ToLower(f), "list price") {
			t.Errorf("the basis was written into the figure line, changing the shape of "+
				"the digest rather than qualifying it: %q", f)
		}
	}
}

// SB4: the canonical cost path is still the source.
//
// `since` filters replay.cost.v2 so the two surfaces cannot disagree about what
// a session cost. A disclosure that came with a second calculation would have
// defeated the reason the command was written this way.
func TestSB4_TheCanonicalCostPathIsPreserved(t *testing.T) {
	src := sinceSource(t)
	if !strings.Contains(src, `runCost([]string{"--per-task", "--json"}`) {
		t.Error("since no longer consumes replay.cost.v2 through runCost")
	}
	// Nothing here may price anything itself.
	for _, banned := range []string{"PriceFor(", "CostUSD(", "CostLegsUSD("} {
		if strings.Contains(src, banned) {
			t.Errorf("since calls %s, so it now derives a figure instead of filtering one", banned)
		}
	}
}

// SB5: an unpriced model is still excluded, and disclosure did not price it.
//
// The exclusion note already existed. Naming the price table must not read as a
// claim that everything in the window was priced by it.
func TestSB5_AnUnpricedModelIsStillExcluded(t *testing.T) {
	src := sinceSource(t)
	if !strings.Contains(src, "not in the price table") {
		t.Error("the unpriced-exclusion note is gone")
	}
	if !strings.Contains(src, "Unpriced") {
		t.Error("since no longer reads the unpriced count from the cost document")
	}
}
