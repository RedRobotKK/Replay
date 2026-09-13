package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// pkg.go.dev shows the package documentation that go/doc assembles from
// every non-test file's package comment, concatenated in filename order.
// Two files each carrying an opening sentence would render as two openings,
// and the reader would not know which one the maintainers meant. Exactly one
// file owns the comment, and it points at the docs site.
func TestPackageDocIsCarriedByOneFile(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		t.Fatalf("parsing cmd/replay: %v", err)
	}
	pkg, ok := pkgs["main"]
	if !ok {
		t.Fatalf("no package main parsed in cmd/replay; found %v", pkgs)
	}

	var carriers []string
	var doc string
	for name, f := range pkg.Files {
		if f.Doc != nil {
			carriers = append(carriers, name)
			doc = f.Doc.Text()
		}
	}
	sort.Strings(carriers)
	if len(carriers) != 1 {
		t.Fatalf("exactly one file may carry the package comment; go/doc concatenates "+
			"them in filename order and pkg.go.dev would show every opening. Carriers: %v",
			carriers)
	}
	if carriers[0] != "doc.go" {
		t.Errorf("the package comment lives in %s; doc.go is where a reader looks for it", carriers[0])
	}
	if !strings.Contains(doc, "https://replay.doctor") {
		t.Errorf("the package comment does not name https://replay.doctor, so pkg.go.dev "+
			"readers have no route to the documentation. Comment:\n%s", doc)
	}
}
