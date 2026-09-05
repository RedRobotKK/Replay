package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
)

const rulesFileName = "rules.json"

// rulesTransport is the round tripper used to fetch a rules document. Nil, the
// zero value, means http.DefaultTransport, which is what every real run uses.
// Tests point it at a local server so the 402 path can be exercised over TLS
// without reaching the network.
//
// swapTransport, which writes it, lives in rules_seam_test.go. An earlier
// version kept it here on the reasoning that the variable should have exactly
// one writer — which it still does, while no longer shipping a mutation entry
// point in the binary.
var rulesTransport http.RoundTripper

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
		_, _ = fmt.Fprintf(stderr, "replay: ignoring %s: %v\n", path, err)
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
	measure := fs.String("measure", "", "read a ledger directory and emit a rules document carrying what the wire showed about each model's caching floor")
	export := fs.Bool("export", false, "write the compiled rules as an installable JSON document to stdout; this is the free tier, complete")
	x402JSON := fs.Bool("x402-json", false, "with --update, if the feed asks for payment, print its terms as JSON instead of prose; replay never pays")
	checkPrices := fs.Bool("check-prices", false, "compare the compiled price table against an independent published database and report where they differ; never changes anything")
	// Not hoistFlags: --update takes a value, and reordering would separate a
	// flag from its argument. This command has no positional arguments, so
	// there is nothing to hoist past.
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}

	// Mutually exclusive, and said so rather than silently preferring one.
	// `replay rules --update <url> --export` used to exit 0 with a price table
	// on stdout having fetched and installed nothing, so a script or an agent
	// could read success where no update happened.
	chosen := 0
	for _, on := range []bool{*export, *checkPrices, *update != "", *measure != ""} {
		if on {
			chosen++
		}
	}
	if chosen > 1 {
		return fmt.Errorf("--update, --export, --measure and --check-prices each do a different thing; pick one: %w", errUsage)
	}
	if *measure != "" {
		return measureRules(*measure, stdout)
	}
	if *export {
		if *dryRun {
			return fmt.Errorf("--dry-run applies to --update, not --export: %w", errUsage)
		}
		return exportRules(stdout)
	}
	if *checkPrices {
		return runCheckPrices(stdout)
	}
	path, err := rulesPath()
	if err != nil {
		return err
	}

	if *update == "" {
		return describeRules(path, stdout)
	}
	return updateRules(*update, path, stdout, *dryRun, *x402JSON)
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
func updateRules(src, path string, stdout io.Writer, dryRun bool, x402JSON bool) error {
	body, from, err := fetchRules(src)
	if err != nil {
		// A feed that asks to be paid is not a failure of this tool. Report
		// the terms on stdout, where whatever holds the wallet can read them,
		// and exit 2 so a script can tell "pay for this" from "it broke".
		var pay *paymentRequiredError
		if errors.As(err, &pay) {
			if werr := pay.Write(stdout, x402JSON); werr != nil {
				return werr
			}
			return err
		}
		return err
	}

	tmp, err := os.CreateTemp("", "replay-rules-*.json")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
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
		client := &http.Client{
			Timeout:   30 * time.Second,
			Transport: rulesTransport,
			// The scheme check above runs once, on the URL the user typed. Go
			// follows redirects by default and will follow one across schemes,
			// so an https URL that 302s to plain http was fetched in cleartext
			// and installed — while the stored document recorded the original
			// https address, so every later report asserted TLS that never
			// happened. Refusing here makes the promise hold for the whole
			// chain rather than only its first hop.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" {
					return fmt.Errorf("refusing a redirect to plain http: %s", req.URL.Redacted())
				}
				if len(via) >= 10 {
					return errors.New("too many redirects")
				}
				return nil
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("fetch rules: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusPaymentRequired {
			// A paid feed. Read the terms and report them; do not pay. There
			// is no wallet in this binary and no code that can sign — see
			// docs/adr/0013-x402-rules-feed.md.
			body, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			if rerr != nil {
				return nil, "", fmt.Errorf("fetch rules: %s asked for payment, and its terms could not be read: %w", src, rerr)
			}
			terms, perr := cachemodel.ParsePaymentRequired(body)
			if perr != nil {
				return nil, "", fmt.Errorf("fetch rules: %s returned 402 Payment Required but %w", src, perr)
			}
			return nil, "", &paymentRequiredError{Resource: src, Terms: terms}
		}
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("fetch rules: %s returned %s", src, resp.Status)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, "", err
		}
		// The URL the bytes actually came from, not the one that was typed.
		// After a redirect these differ, and provenance that names the wrong
		// origin is worse than none: it is confidently wrong.
		final := src
		if resp.Request != nil && resp.Request.URL != nil {
			final = resp.Request.URL.Redacted()
		}
		return body, final, nil
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return nil, "", fmt.Errorf("read rules: %w", err)
	}
	abs, _ := filepath.Abs(src)
	return body, strings.TrimSpace(abs), nil
}

// paymentRequiredError reports that a rules feed asked to be paid.
//
// It is an error because no rules were installed, not because anything went
// wrong. `replay rules --update` exits 2 for it, distinct from 1, so an agent
// with a funded wallet can tell "this costs money" from "this is broken"
// without parsing prose.
type paymentRequiredError struct {
	Resource string
	Terms    cachemodel.PaymentRequired
}

func (e *paymentRequiredError) Error() string {
	return fmt.Sprintf("%s asks to be paid before it will serve rules; replay holds no wallet and will not pay", e.Resource)
}

// Write renders the demand: prose for a person, JSON for a spending policy.
//
// The JSON is the server's own terms plus the resource they apply to, and
// nothing this tool inferred. `paid: false` is stated rather than implied,
// because a machine reading this needs to know the transfer has not happened.
func (e *paymentRequiredError) Write(out io.Writer, asJSON bool) error {
	if !asJSON {
		_, err := io.WriteString(out, e.Terms.Explain(e.Resource))
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Resource string                     `json:"resource"`
		Paid     bool                       `json:"paid"`
		PaidBy   string                     `json:"paid_by"`
		Terms    cachemodel.PaymentRequired `json:"payment_required"`
	}{
		Resource: e.Resource,
		Paid:     false,
		PaidBy:   "nothing: replay holds no wallet and has no code that can sign a transaction",
		Terms:    e.Terms,
	})
}

// exportRules writes the compiled table as a document that `--update` accepts.
//
// This is what makes "the free tier is complete" a checkable statement instead
// of a claim: the published free feed is this output, so anyone can diff it
// against the paid one and see exactly what the money buys.
func exportRules(stdout io.Writer) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(cachemodel.ExportRules())
}

// measureRules reads a ledger and emits a rules document carrying claims.
//
// This is the content of the maintained feed, and the reason it is worth
// paying for. The compiled table records what a provider documents; running
// this over real traffic records what the wire showed. Those answer different
// questions, and only the second one works on a model that did not exist when
// the binary shipped.
//
// The document it writes is installable by `--update` like any other, and its
// claims are derived rather than declared: the loader refuses a file that
// tries to state a verdict, so a `status` field written by hand is rejected.
func measureRules(dir string, stdout io.Writer) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read ledger directory: %w", err)
	}

	// The machine identity is what makes breadth meaningful: one machine
	// agreeing with a documented figure says much less than several do, and
	// the claim records both. A hostname is not needed and not collected — a
	// stable local identifier is enough to count distinct sources, and it
	// never leaves unless the operator publishes the document.
	machine := machineTag()

	var evidence []cachemodel.PrefixEvidence
	var files, skipped int
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() || !ledger.IsLedgerFile(path) {
			continue
		}
		records, dropped, rerr := ledger.ReadRecords(path)
		if rerr != nil {
			// One unreadable file must not lose the rest of the evidence.
			skipped++
			continue
		}
		files++
		skipped += dropped
		evidence = append(evidence, ledger.EvidenceFrom(records, machine)...)
	}

	claims := cachemodel.MeasureClaims(evidence)
	if len(claims) == 0 {
		return fmt.Errorf("no cache writes in %d ledger file(s) under %s, so there is nothing measured to publish. "+
			"Run traffic through `replay serve` first: transcripts cannot answer this, because only the proxy sees what the provider cached", files, dir)
	}

	doc := cachemodel.ExportRules()
	doc.Version = doc.Version + "+measured-" + time.Now().UTC().Format("2006-01-02")
	doc.Source = "measured from " + dir
	doc.FetchedAt = time.Now().UTC().Format(time.RFC3339)

	// Attach a claim to every row the evidence covers. Rows with no evidence
	// keep no claim rather than an empty one: "untested" is the honest state
	// and it is what the absence already says.
	for i := range doc.Models {
		for _, model := range cachemodel.MeasuredModels(claims) {
			if !strings.Contains(strings.ToLower(model), strings.ToLower(doc.Models[i].Match)) {
				continue
			}
			c := claims[model]
			doc.Models[i].MinPrefixClaim = &c
			break
		}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}

	// The summary goes to stderr in the JSON case elsewhere in this file; here
	// the document is the whole point of stdout, so the count is a comment the
	// caller can ignore. Written to stderr so `--measure > rules.json` stays
	// valid JSON.
	fmt.Fprintf(os.Stderr, "measured %d model(s) from %d ledger file(s); %d record(s) skipped\n",
		len(claims), files, skipped)
	return nil
}

// machineTag is a stable per-machine identifier used only to count distinct
// sources of evidence. It is derived from the ledger key path rather than from
// a hostname, so it identifies the installation and not the computer.
func machineTag() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "unknown"
	}
	return fmt.Sprintf("%08x", fnvString(home))
}

func fnvString(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
