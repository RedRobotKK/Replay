package main

import (
	"os"
	"strings"
	"testing"
)

// "1 were read" is what the tool said, and only a reader saw it.
//
// Two messages explain why a cost figure is missing, and both wrote "%d were
// read" regardless of the count. With exactly one unpriced transcript — the
// commonest case for somebody trying a new model, because the first run after
// a release carries one session — the sentence a reader studies most closely
// is the one with broken agreement in it.
//
// Found by running `replay cost` against a corpus on an unpriced model and
// reading the output as a first-time operator would. No test covered the
// singular case, the type checker cannot see it, and a reviewer reading the
// format string sees "%d were" and moves on, because it is correct for every
// count but one.
func TestCW1_TheVerbAgreesWithTheCount(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{0, "0 were read but their models are not in the price table."},
		{1, "1 was read but its model is not in the price table."},
		{2, "2 were read but their models are not in the price table."},
		{11, "11 were read but their models are not in the price table."},
	} {
		if got := unpricedClause(tc.n); got != tc.want {
			t.Errorf("unpricedClause(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestCW2 pins that the clause is built in one place.
//
// The helper is only a fix while both call sites use it. This reads the source,
// which is unusual and deliberate: the defect was a format string correct for
// every count except one, so a value test would have to construct the singular
// corpus at each call site to catch a regression.
//
// It counts CODE occurrences, skipping comment lines, and both exclusions were
// earned. The first version looked for the phrase anywhere and failed on
// unpricedClause's own plural branch — the helper legitimately contains the
// words the call sites must not. The second counted comment lines and found
// three, two of them inside the doc comment that quotes the old string to
// explain the defect.
//
// That second failure is the same hole I had just reported in another PR's
// header check: a source-text scan that cannot tell a comment from the code.
// Finding it in someone else's test did not stop me writing it into mine an
// hour later, which is the argument for running a check against its own
// subject rather than reasoning that it must be right.
func TestCW2_TheClauseIsBuiltInOnePlace(t *testing.T) {
	const phrase = "were read but their model"
	total := 0
	for _, f := range []string{"cost.go", "costusage.go"} {
		for _, line := range strings.Split(readSourceFile(t, f), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			total += strings.Count(line, phrase)
		}
	}
	// Exactly one: the plural branch inside unpricedClause.
	if total != 1 {
		t.Errorf("%q appears %d times in CODE across cost.go and costusage.go, want 1 "+
			"(the plural branch inside unpricedClause).\n"+
			"      More than one means a call site is building the sentence itself "+
			"again, and with a single unpriced transcript it prints \"1 were read but "+
			"their model\" in the line explaining why the reader's number is missing.",
			phrase, total)
	}
}

// readSourceFile reads a file from this package's own directory.
func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}
