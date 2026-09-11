package regression

import (
	"os"
	"path/filepath"
	"regexp"
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
// Re-measured on 2026-09-11 over 1,820 transcripts — 808 breaks, 40,178,269
// tokens — three of those four specifics were wrong and one was flatly false:
//
//	rarest break cause          FALSE. Model change is rarer, 6 against 11.
//	most expensive per event    still true, 234,182 is the top mean.
//	higher than a TTL expiry    still true, 15% higher rather than 78%.
//	5 / 1,807,000 / 361,400     now 11 / 2,576,000 / 234,182.
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
// PASS: no comment in prefix.go carries a comma-grouped count.
// FAIL: a measurement came back into the prose, where nothing can check it.
func TestFCPX_ThePrefixHeaderCarriesNoCorpusCounts(t *testing.T) {
	root := repoRootFor(t)
	path := filepath.Join(root, "cmd", "replay", "prefix.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}

	// A comma-grouped number is the signature of a measurement: 1,807,000 and
	// 361,400 both carry one, and no ordinary prose figure in this file does.
	grouped := regexp.MustCompile(`\b\d{1,3}(,\d{3})+\b`)

	var found []string
	for i, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}
		if m := grouped.FindString(trimmed); m != "" {
			found = append(found, formatLine(i+1, m, trimmed))
		}
	}

	// A scan that matched nothing because it read nothing would pass while
	// checking no comment at all.
	if !strings.Contains(string(body), "//") {
		t.Fatal("prefix.go has no comments at all, so this test is not looking at anything")
	}

	if len(found) > 0 {
		t.Errorf("a corpus count is back in prefix.go's prose:\n  %s\n\n"+
			"These figures aged twice in five days and nothing in the build noticed, "+
			"because no test compares a comment against a corpus. State the shape and "+
			"name the command that recomputes it — `replay diff` prints the cause "+
			"table — rather than quoting a number that will be wrong by the time "+
			"somebody reads it.", strings.Join(found, "\n  "))
	}
}

func formatLine(n int, match, line string) string {
	if len(line) > 90 {
		line = line[:87] + "..."
	}
	return "prefix.go:" + itoaFC(n) + ": " + match + "   in: " + line
}

func itoaFC(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
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
