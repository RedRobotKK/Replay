package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/RedRobotKK/Replay/internal/observation"
)

// runPool aggregates corpus submissions into one figure whose parts a reader
// can fetch.
//
// `cost --contribute` has written submissions since it shipped and nothing read
// one back: internal/observation.Pool was complete — totals as methods over the
// roster so no drifted figure can be stored, digests re-derived on Add, a
// RulesVersion refusal — and had no caller in the binary. The unwired-packages
// guard could not see it, because it asks whether a PACKAGE is reachable and
// internal/observation is: contribute.go imports Corpus, and that one live type
// vouched for the dead one beside it.
//
// Pooling happens where the submissions were collected — a pull request, a
// directory of checked-in files — never over a wire. This command reads local
// paths and nothing else, which is why it needs no allowlist of its own.
// pooledAtOr resolves the pool date: the caller's, or today in UTC.
//
// Split out so the resolution can be tested against the clock seam, and so the
// flag's printed default stays a constant. An explicit value is passed through
// untouched, because somebody regenerating a published document supplies the
// date it was published on.
func pooledAtOr(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return timeNow().UTC().Format("2006-01-02")
}

func runPool(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pool", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the pooled document rather than the table")
	// pooledAt records a date rather than a clock read, for the reason NewPool
	// states: a document reporting a different timestamp on every regeneration
	// cannot be diffed against the one that was published.
	//
	// The default is EMPTY and resolved at use, not rendered into the flag.
	// flag.PrintDefaults writes the default into --help, docs/CLI.md is
	// generated from that help and committed, and cli-blueprint regenerates and
	// diffs — so a default carrying today's date made a published surface go
	// stale at every UTC midnight. It failed on a tree nobody had touched, 83
	// seconds after the rollover.
	pooledAt := fs.String("pooled-at", "",
		"the date this pool was assembled, recorded in the document (default: today, UTC)")
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, "Usage: replay pool <submission.json...> [--json] [--pooled-at <date>]\n\n"+
			"Aggregate corpus submissions written by `replay cost --contribute`.\n"+
			"Reads local files. Sends nothing.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()

	p := observation.NewPool(pooledAtOr(*pooledAt))
	admitted := 0
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "replay pool: %s: %v\n", f, err)
			continue
		}
		var c observation.Corpus
		if err := json.Unmarshal(body, &c); err != nil {
			_, _ = fmt.Fprintf(stderr, "replay pool: %s: not a submission: %v\n", f, err)
			continue
		}
		// A refusal is reported and the run continues. Failing the whole pool
		// on one bad file would make a reader re-run it without that file and
		// never learn what was wrong with it; swallowing the refusal would
		// publish a figure covering fewer submissions than the operator handed
		// over, which is the failure the roster exists to prevent.
		if err := p.Add(c, filepath.Base(f)); err != nil {
			_, _ = fmt.Fprintf(stderr, "replay pool: %s: %v\n", f, err)
			continue
		}
		admitted++
	}

	if *asJSON {
		// No Totals pre-check here. Pool.MarshalJSON calls Totals itself and
		// propagates its refusal, so asking first was a second guard shadowing
		// the one below: neutralising the error check changed nothing, because
		// the pre-check had already returned. One reachable guard beats two
		// where only the first can fire.
		b, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "%s\n", b)
		return nil
	}

	out, err := p.Render()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(stdout, out)
	return nil
}
