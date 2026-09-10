package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The store registry has to be complete, and ST1 could not tell whether it was.
//
// ST1 asserts that nine names it lists appear in the registry. That catches a
// store being REMOVED and cannot catch one being ADDED, because a name nobody
// typed into the test is a name the test does not look for. It is a
// completeness check with a hard-coded idea of completeness, which is the shape
// of check this repository keeps finding: it passes for as long as nothing
// changes, and the thing it exists to notice is a change.
//
// It missed two stores. `contributor-secret` is a persistent per-machine
// identifier written by `replay cost --contribute`, and `rules.json` is the
// price-rules cache; both sit under ~/.replay and neither was disclosed by
// `replay privacy` or removable by `replay purge`. The contributor secret is
// the one that matters: anyone who can read it can compute this machine's tag
// for any campaign, and a privacy command that does not name it tells the
// reader something untrue by omission.
//
// This derives the names from the source instead. Every constant in cmd/replay
// whose name follows the repository's own convention for a store filename must
// be registered, or exempted here with a reason a reader can check.

// notAHomeStore lists store-shaped constants that are deliberately not in the
// registry, and why. An entry here is a decision on the record; the point is
// that adding a store cannot be silent, not that every constant is a store.
var notAHomeStore = map[string]string{
	"replay": "the MCP server's advertised name, not a path",
}

// storeNameConstants reads the source for constants that name a file or
// directory under the reader's home.
//
// The convention is the repository's own: adviceFileName, ledgerDirName,
// vaultDirName, contributorSecretName. Anything matching it is treated as a
// store until someone says otherwise in notAHomeStore.
func storeNameConstants(t *testing.T) map[string]string {
	t.Helper()
	found := map[string]string{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if !isStoreNameIdent(id.Name) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					v, err := strconv.Unquote(lit.Value)
					if err != nil {
						continue
					}
					found[v] = name + ": " + id.Name
				}
			}
		}
	}
	return found
}

func isStoreNameIdent(name string) bool {
	for _, suffix := range []string{"FileName", "DirName", "SecretName", "StoreName", "ServerName"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// SC1: every store-shaped constant is registered or exempted.
//
// PASS: the registry names every one, or notAHomeStore explains it.
// FAIL: the tool writes somewhere no disclosure or retention command covers —
// found by reading the source rather than by remembering to update a list.
func TestSC1_EveryStoreConstantIsRegisteredOrExempted(t *testing.T) {
	registered := map[string]bool{}
	for _, s := range homeStores() {
		registered[s.Name] = true
	}
	consts := storeNameConstants(t)
	// The scan must find something, or it is passing by looking at nothing.
	if len(consts) < 5 {
		t.Fatalf("found %d store-name constants in cmd/replay; the convention was used at "+
			"least six times when this was written, so this scan is not reading what it "+
			"thinks it is", len(consts))
	}
	for value, where := range consts {
		if registered[value] {
			continue
		}
		if why, ok := notAHomeStore[value]; ok {
			if strings.TrimSpace(why) == "" {
				t.Errorf("%q is exempted from the store registry with no reason given", value)
			}
			continue
		}
		t.Errorf("%s = %q is written under the reader's home and is not in the store "+
			"registry, so `replay privacy` does not disclose it and `replay purge` cannot "+
			"remove it. Register it in homeStores, or add it to notAHomeStore with a reason.",
			where, value)
	}
}

// SC2: the contributor secret is registered, sensitive, and not window-purgeable.
//
// It is a stable per-machine identifier. Anyone holding it can compute this
// machine's contributor tag for any campaign, which is the one property the
// per-campaign tag exists to prevent — so it belongs beside the vault rather
// than beside a cache.
//
// Not purgeable by a retention WINDOW, for the same reason the vault is not:
// the tag must be stable or one contributor looks like many, which destroys the
// only thing the tag is for. Deleting it is a decision, not a schedule.
func TestSC2_TheContributorSecretIsRegisteredAndSensitive(t *testing.T) {
	var found bool
	for _, s := range homeStores() {
		if s.Name != contributorSecretName {
			continue
		}
		found = true
		if !s.Sensitive {
			t.Error("the contributor secret is not marked sensitive. Anyone who can read it " +
				"can compute this machine's tag for any campaign.")
		}
		if s.Purgeable {
			t.Error("the contributor secret is marked purgeable by a retention window. A tag " +
				"that changes makes one contributor look like many, which is the one thing " +
				"the tag exists to prevent; removing it is a decision, not a schedule.")
		}
		if !strings.Contains(strings.ToLower(s.Holds), "tag") {
			t.Errorf("the entry does not tell the reader what the secret is for: %q", s.Holds)
		}
	}
	if !found {
		t.Errorf("%q is not in the store registry", contributorSecretName)
	}
}

// SC3: `replay privacy` actually names it.
//
// The registry is only worth having if the disclosure command walks it. This is
// the end-to-end check: the reader runs one command and the identifier appears.
func TestSC3_PrivacyDisclosesTheContributorSecret(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".replay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, contributorSecretName), []byte(strings.Repeat("a", 64)), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if err := run([]string{"privacy"}, &stdout, &stderr); err != nil {
		t.Fatalf("replay privacy: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), contributorSecretName) {
		t.Errorf("`replay privacy` does not name %s, so a reader asking what this tool keeps "+
			"about them is told something untrue by omission:\n%s",
			contributorSecretName, stdout.String())
	}
}
