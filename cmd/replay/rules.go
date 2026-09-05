package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

const rulesFileName = "rules.json"

// rulesPath is where a loaded rules document lives, beside the ledger.
func rulesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".replay", rulesFileName), nil
}

// LoadInstalledRules applies the rules document if one is installed.
//
// Called once at startup so every command reports under the same rules. A
// missing file is the normal case; a broken one is loud, because silently
// falling back to compiled numbers after someone deliberately installed an
// override is how a person ends up trusting a figure they thought they had
// corrected.
func LoadInstalledRules(stderr io.Writer) {
	path, err := rulesPath()
	if err != nil {
		return
	}
	r, err := cachemodel.LoadRules(path)
	if err != nil {
		fmt.Fprintf(stderr, "replay: ignoring %s: %v\n", path, err)
		return
	}
	if r != nil {
		cachemodel.Override(r)
	}
}

func runRules(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.SetOutput(stderr)
	update := fs.String("update", "", "install a rules document from a file path or https URL")
	dryRun := fs.Bool("dry-run", false, "with --update, validate and describe the change without installing it")
	// Not hoistFlags: --update takes a value, and reordering would separate a
	// flag from its argument. This command has no positional arguments, so
	// there is nothing to hoist past.
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	path, err := rulesPath()
	if err != nil {
		return err
	}

	if *update == "" {
		return describeRules(path, stdout)
	}
	return updateRules(*update, path, stdout, *dryRun)
}

func describeRules(path string, stdout io.Writer) error {
	r, err := cachemodel.LoadRules(path)
	if err != nil {
		return err
	}
	if r == nil {
		_, err := fmt.Fprintf(stdout,
			"rules      %s (compiled in)\nfile       %s (none installed)\n\nThese numbers are published by the provider and change faster than releases do.\nInstall a dated document with:  replay rules --update <file|https URL>\n",
			cachemodel.RulesVersion, path)
		return err
	}
	if _, err = fmt.Fprintf(stdout, "rules      %s\nfile       %s\nmodels     %d\nprovenance %s\n\nclaims     what the provider documents, against what replaying traffic showed\n",
		r.Version, path, len(r.Models), r.Provenance()); err != nil {
		return err
	}
	for _, l := range claimLines(r.Models) {
		if _, err := fmt.Fprintln(stdout, l); err != nil {
			return err
		}
	}
	return nil
}

// updateRules installs a rules document.
//
// A URL is a network request, and this tool's promise is that it makes none
// except to the provider you configured. That promise survives because this
// only happens when a person types the URL: there is no background refresh, no
// check-for-updates on startup, and no default source.
func updateRules(src, path string, stdout io.Writer, dryRun bool) error {
	body, from, err := fetchRules(src)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "replay-rules-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Validate through the same loader every run uses, so nothing can be
	// installed that a later run would refuse.
	r, err := cachemodel.LoadRules(tmp.Name())
	if err != nil {
		return fmt.Errorf("not installing: %w", err)
	}
	if r == nil {
		return fmt.Errorf("not installing: %s is empty", from)
	}
	r.Source, r.FetchedAt = from, time.Now().UTC().Format(time.RFC3339)

	current := cachemodel.RulesVersion
	if existing, _ := cachemodel.LoadRules(path); existing != nil {
		current = existing.Version
	}
	if dryRun {
		_, err := fmt.Fprintf(stdout, "would install %s over %s (%d models) from %s\nNothing was changed.\n",
			r.Version, current, len(r.Models), from)
		return err
	}

	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "installed %s over %s (%d models) from %s\n%s\n",
		r.Version, current, len(r.Models), from, path)
	return err
}

func fetchRules(src string) ([]byte, string, error) {
	u, err := url.Parse(src)
	if err == nil && (u.Scheme == "https" || u.Scheme == "http") {
		if u.Scheme != "https" {
			return nil, "", fmt.Errorf("refusing to fetch rules over plain http: %s", src)
		}
		req, err := http.NewRequest(http.MethodGet, src, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("user-agent", "replay-rules")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("fetch rules: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("fetch rules: %s returned %s", src, resp.Status)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, "", err
		}
		return body, src, nil
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return nil, "", fmt.Errorf("read rules: %w", err)
	}
	abs, _ := filepath.Abs(src)
	return body, strings.TrimSpace(abs), nil
}
