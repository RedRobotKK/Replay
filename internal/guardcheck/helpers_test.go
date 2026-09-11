package guardcheck

import (
	"go/parser"
	"go/token"
	"os"
)

// pos builds a token.Position for a line and column, for tests that describe a
// guard body without parsing one.
func pos(line, col int) token.Position {
	return token.Position{Line: line, Column: col}
}

// goParses reports whether a file is syntactically valid Go.
//
// It is the difference between a mutant the suite judged and a mutant the
// compiler threw out, which is the difference between evidence and a number.
func goParses(path string) bool {
	_, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	return err == nil
}

// readFile is a test helper that reads a file or fails the test.
func readFile(t interface{ Fatal(...any) }, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
