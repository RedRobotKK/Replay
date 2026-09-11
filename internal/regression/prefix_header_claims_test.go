package regression

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// FC-PX, frozen. A doc comment carried corpus counts, and they aged twice in
// five days.
//
// cmd/replay/prefix.go opened with the argument for why the command exists:
//
//	A prefix change is the rarest break cause in the corpus and the most
//	expensive per event: 5 breaks, 1,807,000 tokens, a mean of 361,400 —
//	higher than a TTL expiry. Rare and enormous is the shape a gate is for.
//	Nobody catches this by watching, because it happens five times and each
//	time it happens to everyone at once.
//
// Re-measured on 2026-09-11 over 1,820 transcripts — 808 breaks, ~40.2M
// tokens — three of those four specifics were wrong and one was flatly false:
//
// That total is written to three figures on purpose, and the reason belongs
// here rather than in a commit message. Four of the five per-cause token
// figures it sums are exact multiples of a thousand and only one is not, so
// they were rounded before they reached me; the run that produced them said
// so. Adding them gives an exact sum of inexact parts. Writing 40,178,269
// would assert nine significant figures on inputs carrying four — false
// precision, in the doc comment of the test that exists to stop false
// precision rotting in doc comments. Review caught it. The break count is
// exact and the orderings below survive the rounding by three orders of
// magnitude.
//
// WHICH ROW, because two causes both say "prefix" and only one of them
// makes the sentence above true. The 11 breaks are CausePrefixChange,
// "system prompt or tool definitions changed" — the cause `replay prefix`
// exists for, since it watches the tool set. They are NOT CauseUnknown,
// "prefix diverged inside the message history at an unknown block", which
// is the 110-break row: 2,646,269 tokens, mean 24,057. A reader who maps
// the word "prefix" to that row gets a mean an order of magnitude BELOW
// the TTL row's 203,667 and every ordinal here inverts. The mapping is
// load-bearing, so it is written down rather than assumed.
//
//	rarest break cause          FALSE. Model change is rarer, 6 against 11.
//	most expensive per event    still true, ~234,000 is the top mean.
//	higher than a TTL expiry    still true, 15% higher rather than 78%.
//	5 / 1,807,000 / 361,400     now 11 breaks at ~2.58M, mean ~234,000.
//	"it happens five times"     stale for the same reason.
//
// The argument survived. Rare and enormous is still why a gate is the right
// shape. Only the numbers rotted, and they rotted silently: nothing in the
// build compares a comment against a corpus, so the figures stayed quotable
// long after they stopped being true.
//
// The repair was not fresher numbers. Fresher numbers age too, and these had
// already been replaced once. The header states the shape and names the
// command that recomputes it, so a reader who wants a figure runs for one.
//
// SCOPE, stated because the next reader will otherwise overestimate this.
// The signature is a comma-grouped number. It would NOT have caught the
// original header's "5 breaks" or "it happens five times" — only 1,807,000 and
// 361,400. Ungrouped counts and ordinals ("the rarest", "the highest mean")
// are out of scope and no check reads them. That is a deliberate trade:
// matching every digit false-positives on version numbers and on this comment,
// and matching wordings freezes the wording rather than the defect. The
// ordinals were removed from prefix.go by hand, for the reason its comment
// gives, and nothing here stops them coming back.
//
// PASS: no comment in prefix.go carries a comma-grouped count — every comment,
// line-leading or trailing or block, because the file is parsed rather than
// scanned.
// FAIL: a measurement came back into the prose, where nothing can check it.
func TestFCPX_ThePrefixHeaderCarriesNoCorpusCounts(t *testing.T) {
	root := repoRootFor(t)
	path := filepath.Join(root, "cmd", "replay", "prefix.go")

	// Parsed, not scanned line by line. The first version of this test matched
	// lines BEGINNING with "//", while its own header promised that no COMMENT
	// carried a count — a stated claim wider than the behaviour, which is the
	// defect this test exists to catch, committed inside the test. Review found
	// both holes by mutation: a trailing `// measured: 1,807,000` on a func
	// declaration, and a `/* ... */` block. Walking ast.File.Comments closes
	// them and makes the promise true. internal/guardcheck already parses for
	// the same reason.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("cannot parse %s: %v", path, err)
	}

	// A comma-grouped number is the signature of a measurement: 1,807,000 and
	// 361,400 both carry one, and no ordinary prose figure in this file does.
	grouped := regexp.MustCompile(`\b\d{1,3}(,\d{3})+\b`)

	var found []string
	groups := 0
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			groups++
			if m := grouped.FindString(c.Text); m != "" {
				found = append(found, formatLine(fset.Position(c.Pos()).Line, m, c.Text))
			}
		}
	}

	// A walk that matched nothing because it read nothing would pass while
	// checking no comment at all.
	if groups == 0 {
		t.Fatal("prefix.go parsed with no comments at all, so this test is not looking at anything")
	}

	if len(found) > 0 {
		t.Errorf("a corpus count is back in prefix.go's prose:\n  %s\n\n"+
			"These figures aged twice in five days and nothing in the build noticed, "+
			"because no test compares a comment against a corpus. State the shape and "+
			"name the command that recomputes it — `replay diff` prints each break and "+
			"its cause — rather than quoting a number that will be wrong by the time "+
			"somebody reads it.", strings.Join(found, "\n  "))
	}
}

func formatLine(n int, match, line string) string {
	line = strings.ReplaceAll(line, "\n", " ")
	if len(line) > 90 {
		line = line[:87] + "..."
	}
	return "prefix.go:" + strconv.Itoa(n) + ": " + match + "   in: " + line
}

// repoRootFor walks up to the module root.
func repoRootFor(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("no go.mod above the test directory")
	return ""
}
