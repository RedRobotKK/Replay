package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// What a commit can do to a cached prefix, before it is merged.
//
// A prefix change is the rarest break cause in the corpus and the most
// expensive per event: 5 breaks, 1,807,000 tokens, a mean of 361,400 — higher
// than a TTL expiry. Rare and enormous is the shape a gate is for. Nobody
// catches this by watching, because it happens five times and each time it
// happens to everyone at once.
//
// It watches the tool set, not the system prompt, and that is a measured choice
// rather than an obvious one. internal/proxy/causedetail.go records that across
// the 30-lane trial of 2026-09-06 "system_bytes never moved once: every real
// prefix change was the tool SET changing." A gate pointed at system prompts
// would be pointed at the half that did not move.
//
// It reads only files it is handed by name. docs/WHAT-YOU-GET.md commits to not
// reading settings.json, CLAUDE.md, .mcp.json or any other configuration on a
// default invocation, because a tool that reads your config to advise you on
// your config has to be trusted with it. This is the `advise --apply` shape: a
// typed request naming one file, which discovers nothing on its own. And it
// refuses settings.json outright, because that is where credentials live and no
// prefix question needs it.

// prefixExit is returned when a change invalidates the cached prefix. It is an
// error so that a shell gates on it without parsing anything.
var errPrefixInvalidated = errors.New("the cached prefix is invalidated by this change")

// toolSetFrom reads a file's tool set, and reports whether the file
// participates in the cached prefix at all.
//
// Only the server names are taken. A server's command, arguments and
// environment are none of this command's business, and reading them would put
// credentials in a code path that prints things.
func toolSetFrom(path string) (names []string, participates bool, err error) {
	base := strings.ToLower(filepath.Base(path))
	if base == "settings.json" {
		return nil, false, fmt.Errorf("refusing to read %s: it can hold environment variables and "+
			"credentials, and no prefix question needs it. Pass the file that defines your tool "+
			"servers instead", filepath.Base(path))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(b, &doc) != nil || doc.MCPServers == nil {
		// Not a tool-server document. Ordinary source, tests and documentation
		// do not enter the cached prefix, and saying so is the difference
		// between a gate and an alarm.
		return nil, false, nil
	}
	for name := range doc.MCPServers {
		names = append(names, name)
	}
	// Object order is not semantic in JSON, and a formatter or a merge can
	// reorder keys, so a reordered file must not read as a changed tool set.
	//
	// This sort is not what guarantees that, which is worth stating because the
	// first version of this comment claimed it was. diffNames compares by set
	// membership, so it is already order-independent, and disabling either
	// mechanism alone leaves TestPX3 passing. Only removing both fails it.
	//
	// The sort is kept for the output: added and removed are printed, and a
	// list whose order comes from Go's randomised map iteration would differ
	// between runs on identical input.
	sort.Strings(names)
	return names, true, nil
}

func diffNames(before, after []string) (added, removed []string) {
	in := func(s []string, v string) bool {
		for _, x := range s {
			if x == v {
				return true
			}
		}
		return false
	}
	for _, a := range after {
		if !in(before, a) {
			added = append(added, a)
		}
	}
	for _, b := range before {
		if !in(after, b) {
			removed = append(removed, b)
		}
	}
	return added, removed
}

func runPrefix(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("prefix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	before := fs.String("before", "", "the file as it is on the base branch")
	after := fs.String("after", "", "the file as this change would leave it")
	asJSON := fs.Bool("json", false, "emit the finding as JSON for a CI step to act on")
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: replay prefix --before <file> --after <file>\n\n"+
			"Reports whether a change to a tool-server document invalidates the cached\n"+
			"prefix, which re-bills the whole prefix on every affected session's next\n"+
			"request. Exits non-zero when it does, so CI can gate on it.\n\n"+
			"Reads only the two files named. It never opens settings.json.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *before == "" || *after == "" {
		fs.Usage()
		return errors.New("both --before and --after are required")
	}

	beforeNames, beforeParticipates, err := toolSetFrom(*before)
	if err != nil {
		return err
	}
	afterNames, afterParticipates, err := toolSetFrom(*after)
	if err != nil {
		return err
	}

	if !beforeParticipates && !afterParticipates {
		if *asJSON {
			return writeJSON(stdout, map[string]any{
				"schema": "replay.prefix.v1", "invalidated": false, "participates": false,
			})
		}
		fmt.Fprintf(stdout, "  not a prefix input: neither file defines tool servers, so this change\n"+
			"  cannot void a cached prefix.\n")
		return nil
	}

	added, removed := diffNames(beforeNames, afterNames)
	invalidated := len(added) > 0 || len(removed) > 0

	if *asJSON {
		if err := writeJSON(stdout, map[string]any{
			"schema": "replay.prefix.v1", "invalidated": invalidated, "participates": true,
			"added": added, "removed": removed,
			"serversBefore": len(beforeNames), "serversAfter": len(afterNames),
			"affectedSessions": nil, // NOT MEASURED. See below.
		}); err != nil {
			return err
		}
		if invalidated {
			return errPrefixInvalidated
		}
		return nil
	}

	if !invalidated {
		fmt.Fprintf(stdout, "  tool set unchanged: %d server(s), same names. The cached prefix survives.\n",
			len(afterNames))
		return nil
	}

	fmt.Fprintf(stdout, "  The cached prefix is invalidated by this change.\n\n")
	if len(added) > 0 {
		fmt.Fprintf(stdout, "    added    %s\n", strings.Join(added, ", "))
	}
	if len(removed) > 0 {
		fmt.Fprintf(stdout, "    removed  %s\n", strings.Join(removed, ", "))
	}
	fmt.Fprintf(stdout, "    servers  %d before, %d after\n\n", len(beforeNames), len(afterNames))

	// The honest stopping point, and the reason this command prints no total.
	//
	// A tool server appearing appends its whole tool block to the prefix, so
	// every session holding a warm prefix cold-writes on its next request. How
	// many sessions that is cannot be derived from a diff: it depends on who is
	// working, on what, and when they next send. Multiplying the per-session
	// figure by a guessed headcount would manufacture exactly the defect this
	// repository keeps finding — a number standing in for one nobody measured.
	fmt.Fprintf(stdout,
		"  Every session holding a warm prefix re-bills it in full on its next request.\n"+
			"  How many sessions that is: NOT MEASURED. It cannot be read from a diff, and a\n"+
			"  headcount multiplied by a per-session figure would be a guess wearing a total.\n\n"+
			"  For the size of one such break on your own history:  replay diff <transcript>\n")
	return errPrefixInvalidated
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
