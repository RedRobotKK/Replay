package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The generated CLI reference must cover every command the binary dispatches.
//
// scripts/cli-blueprint/gen.py keeps its command list by hand, deliberately, so
// that a command being deleted breaks the generator loudly instead of vanishing
// quietly from the reference. That choice has an opposite hole, and this test
// exists because the hole was found the first time a command was added after the
// generator was written.
//
// `prefix` dispatched. It appeared in --help, because TestCT1 requires that. It
// appeared in docs/guide/commands.md, because TestCT2 requires that. And it was
// absent from docs/CLI.md, silently, because nothing compared the generator's
// list to the dispatch table — so the reference that advertises itself as
// generated from the binary was describing a binary with one fewer command than
// the one in the tree.
//
// The failure mode matters more than the miss. A hand-written reference that
// goes stale is expected to be stale and gets re-read. One labelled "generated
// from the binary, checked in CI" is trusted, and a gap in it is invisible
// precisely because of the label.

var pyCommandsBlock = regexp.MustCompile(`(?s)COMMANDS = \[(.*?)\]`)
var pyString = regexp.MustCompile(`"([a-z0-9-]+)"`)

// blueprintCommands reads the command list out of the generator.
func blueprintCommands(t *testing.T) []string {
	t.Helper()
	const path = "../../scripts/cli-blueprint/gen.py"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	m := pyCommandsBlock.FindSubmatch(b)
	if m == nil {
		t.Fatalf("no COMMANDS list found in %s; the generator changed shape and this test is "+
			"now asserting nothing", path)
	}
	var out []string
	for _, s := range pyString.FindAllSubmatch(m[1], -1) {
		out = append(out, string(s[1]))
	}
	if len(out) < 15 {
		t.Fatalf("found only %d commands in the generator (%v); the parser is wrong, not the code",
			len(out), out)
	}
	return out
}

// TestCLIBlueprintCoversEveryCommand compares the two lists in both directions.
func TestCLIBlueprintCoversEveryCommand(t *testing.T) {
	dispatched := dispatchedCommands(t)
	documented := blueprintCommands(t)

	has := func(list []string, v string) bool {
		for _, x := range list {
			if x == v {
				return true
			}
		}
		return false
	}

	var missing, extra []string
	for _, c := range dispatched {
		if !has(documented, c) {
			missing = append(missing, c)
		}
	}
	for _, c := range documented {
		if !has(dispatched, c) {
			extra = append(extra, c)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 {
		t.Errorf("dispatched but absent from the generated CLI reference: %v\n"+
			"Add them to COMMANDS and META in scripts/cli-blueprint/gen.py, then regenerate:\n"+
			"  go build -o /tmp/replay ./cmd/replay && python3 scripts/cli-blueprint/gen.py --bin /tmp/replay > docs/CLI.md",
			missing)
	}
	if len(extra) > 0 {
		t.Errorf("named in the generator but no longer dispatched: %v\n"+
			"Remove them from gen.py, or the reference documents commands that do not exist.", extra)
	}
}

// TestCLIBlueprintMetaIsComplete stops a command being listed with no description.
//
// COMMANDS drives the tables and META supplies the prose and the two safety
// answers. A command in one and not the other renders a row with an empty
// purpose and, worse, an empty network and writes column — which reads as "this
// does nothing and touches nothing" rather than as a gap.
func TestCLIBlueprintMetaIsComplete(t *testing.T) {
	b, err := os.ReadFile("../../scripts/cli-blueprint/gen.py")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "META = {")
	if start < 0 {
		t.Fatal("no META block found in the generator; this test is asserting nothing")
	}
	meta := src[start:]
	for _, c := range blueprintCommands(t) {
		if !strings.Contains(meta, `"`+c+`":`) {
			t.Errorf("%q is in COMMANDS but has no META entry, so its row would print an empty "+
				"purpose and an empty answer to whether it reaches the network", c)
		}
	}
}
