package tenancy

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// This package defines identity and ownership primitives for a hosted
// service that does not exist yet (ADR-0028: "a separate, opt-in surface").
// It must not be able to reach a network or a process, or "not wired into
// anything" stops being true the day someone adds one import. Mirrors
// internal/observation's TestO7_ThisPackageCannotSend.
//
// PASS: every import is on the allowlist.
// FAIL: net, net/http, net/url, os/exec, database/sql, or anything else
// that could reach outward or touch disk.
func TestTenancy_ImportAllowlist(t *testing.T) {
	allowed := map[string]bool{
		"encoding/json": true, "errors": true, "fmt": true,
		"reflect": true, "regexp": true, "sync": true,
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if !allowed[path] {
					t.Errorf("%s imports %q, which is not on the allowlist. This package is a "+
						"local, unwired identity primitive; it must not be able to reach a "+
						"network or a process.", name, path)
				}
			}
		}
	}
	for _, banned := range []string{"net", "net/http", "net/url", "os/exec", "database/sql", "os"} {
		if allowed[banned] {
			t.Errorf("%q is on the allowlist; the allowlist is not a guarantee", banned)
		}
	}
}
