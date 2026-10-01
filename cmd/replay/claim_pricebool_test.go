package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// EC-00. The shared compression point.
//
// `cachemodel.PriceFor` and `PriceForAt` return (Price, bool). That one
// boolean is the single place where "this model has a price" and "it does
// not" are told apart, and seventeen non-test call sites each decide for
// themselves what to do with it.
//
// The contract is not in doubt. docs/TOKEN-PRICES.md:53 says:
//
//	"An unpriced model must never count as zero. Two defensible options, and
//	 today's behaviour is neither: refuse the request and say the model is
//	 unpriced, or price it at the most expensive known row and label the
//	 figure an upper bound. Fail conservative, and say which."
//
// So this is not a question of whether the distinction matters. The document
// says it matters, names zero as indefensible, and names the two acceptable
// answers. This test measures how many call sites take one of them.

// A call site that handles the false branch acceptably does one of three
// things: counts it, refuses with a reason, or substitutes a labelled bound.
var acceptableFalseBranch = regexp.MustCompile(
	`[Uu]npriced\+\+|unpricedReqs|e\.Unpriced|` + // counts it
		`not in the compiled table|Excluded, not free|` + // refuses and says so
		`return 0, false|return Topology\{|` + // propagates the refusal
		`dearest row|UPPER BOUND`) // labelled bound

func TestEC00_ThePriceBooleanIsTheSharedCompressionPoint(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}

	call := regexp.MustCompile(`cachemodel\.PriceFor(At)?\(|[^A-Za-z]PriceFor(At)?\(`)
	type site struct {
		file   string
		line   int
		window string
	}
	var sites []site

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				switch d.Name() {
				case ".git", "vendor", "bin", "testdata", "claims":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			if !call.MatchString(l) || strings.Contains(l, "func PriceFor") {
				continue
			}
			hi := i + 8
			if hi > len(lines) {
				hi = len(lines)
			}
			rel, _ := filepath.Rel(root, path)
			sites = append(sites, site{rel, i + 1, strings.Join(lines[i:hi], " ")})
		}
		return nil
	})

	// POSITIVE CONTROL: the scan must find the call sites, or the count below
	// describes nothing.
	if len(sites) < 10 {
		t.Fatalf("found only %d call sites; the scan is broken", len(sites))
	}

	var silent []string
	for _, s := range sites {
		if !acceptableFalseBranch.MatchString(s.window) {
			silent = append(silent, s.file+":"+itoaEC(s.line))
		}
	}

	// NEGATIVE CONTROL for the detector: it must recognise a site that DOES
	// handle the branch, or every site would read as silent.
	if !acceptableFalseBranch.MatchString("if !ok { e.Unpriced++; return }") {
		t.Fatal("the detector does not recognise an explicit unpriced counter; the " +
			"count below would be meaningless")
	}
	if acceptableFalseBranch.MatchString("if p, ok := PriceForAt(m, at); ok { total += cost }") {
		t.Fatal("the detector accepts a site with no false branch at all; it cannot " +
			"tell the two apart")
	}

	t.Logf("RECORDED: %d non-test call sites of the price boolean. %d do not count, "+
		"refuse, or substitute a labelled bound within 8 lines:\n  %s\n\n"+
		"docs/TOKEN-PRICES.md:53 states the contract: \"An unpriced model must never "+
		"count as zero... refuse the request and say the model is unpriced, or price "+
		"it at the most expensive known row and label the figure an upper bound.\"",
		len(sites), len(silent), strings.Join(silent, "\n  "))
}

func itoaEC(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
