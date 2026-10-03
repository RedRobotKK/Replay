package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/proxy"
)

// replay simulate --policy <file> <ledger-dir...>
//
// Replays the requests a ledger recorded through the proxy's own spend guard
// under a policy you supply, and says which of them would have been refused.
// It is the existing replay engine pointed at admission: the guard is
// proxy.SpendGuard, the pricing is proxy.ListCost, the order is the records'
// own timestamps. Nothing is predicted. A result is a statement about requests
// that were recorded, under a cap that was not in force when they ran, and it
// is labelled SIMULATED everywhere it is printed.
//
// What it does not do, on purpose: it does not replay requests the proxy
// refused at the time (their usage was never observed, so they are counted
// and not replayed); it does not call a figure a saving (the list price of a
// refused request is what that request cost when it ran, and what the agent
// would have done instead is unknown); and it does not reach the network or
// write anything.

const simulateSchema = "replay.simulate.v1"

// simulatePolicy is the alternative cap, in the proxy's own terms. The file is
// JSON with exactly these keys; an unknown key is refused rather than ignored,
// because a misspelled cap silently evaluates as "off".
type simulatePolicy struct {
	MaxSessionTokens int     `json:"maxSessionTokens"`
	MaxDayTokens     int     `json:"maxDayTokens"`
	MaxSessionUSD    float64 `json:"maxSessionUsd"`
	MaxDayUSD        float64 `json:"maxDayUsd"`
}

func (p simulatePolicy) limits() proxy.SpendLimits {
	return proxy.SpendLimits{SessionTokens: p.MaxSessionTokens, DayTokens: p.MaxDayTokens, SessionUSD: p.MaxSessionUSD, DayUSD: p.MaxDayUSD}
}

// hash identifies the policy that produced a result. It is the SHA-256 of the
// canonical encoding, so two files that mean the same cap carry the same id
// and nothing needs to register anything.
func (p simulatePolicy) hash() string {
	canonical := fmt.Sprintf("maxSessionTokens=%d\nmaxDayTokens=%d\nmaxSessionUsd=%.6f\nmaxDayUsd=%.6f\n",
		p.MaxSessionTokens, p.MaxDayTokens, p.MaxSessionUSD, p.MaxDayUSD)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])[:12]
}

func loadSimulatePolicy(path string) (simulatePolicy, error) {
	var p simulatePolicy
	f, err := os.Open(path)
	if err != nil {
		return p, fmt.Errorf("policy: %w: %w", err, errUsage)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, fmt.Errorf("policy %s is not a cap document: %v: %w", path, err, errUsage)
	}
	if p.MaxSessionTokens < 0 || p.MaxDayTokens < 0 || p.MaxSessionUSD < 0 || p.MaxDayUSD < 0 {
		return p, fmt.Errorf("policy %s names a negative cap: %w", path, errUsage)
	}
	if !proxy.NewSpendGuard(p.limits()).Enabled() {
		return p, fmt.Errorf("policy %s enables no cap (every value is 0, which the proxy reads as off), so there is nothing to simulate: %w", path, errUsage)
	}
	return p, nil
}

// simulateDecision is one recorded request replayed. Every field is read from
// the record or produced by the guard; none is inferred.
type simulateDecision struct {
	Session    string  `json:"session"`
	RequestID  string  `json:"requestId"`
	TS         string  `json:"ts"`
	Model      string  `json:"model"`
	ListUSD    float64 `json:"listUsd"`
	UpperBound bool    `json:"upperBound"`
	Simulated  string  `json:"simulated"`
	Reason     string  `json:"reason,omitempty"`
}

type simulateReport struct {
	Schema    string `json:"schema"`
	Simulated bool   `json:"simulated"`
	Policy    struct {
		Hash string `json:"hash"`
		simulatePolicy
	} `json:"policy"`
	PricedAt     string   `json:"pricedAt"`
	RulesVersion string   `json:"rulesVersion"`
	Ledgers      []string `json:"ledgers"`
	Population   struct {
		Requests            int `json:"requests"`
		Sessions            int `json:"sessions"`
		HistoricallyRefused int `json:"historicallyRefused"`
		WithoutUsage        int `json:"withoutUsage"`
		Unpriced            int `json:"unpriced"`
		Unreadable          int `json:"unreadable"`
	} `json:"population"`
	Summary struct {
		Admitted         int     `json:"admitted"`
		Refused          int     `json:"refused"`
		SessionsAffected int     `json:"sessionsAffected"`
		RefusedListUSD   float64 `json:"refusedListUsd"`
	} `json:"summary"`
	Decisions []simulateDecision `json:"decisions"`
}

func runSimulate(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("simulate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	policyPath := fs.String("policy", "", "a JSON cap document: {\"maxSessionUsd\": 5} with any of maxSessionTokens, maxDayTokens, maxSessionUsd, maxDayUsd")
	asJSON := fs.Bool("json", false, "print every simulated decision as JSON instead of the summary")
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, "Usage: replay simulate --policy <file> <ledger-dir...> [--json]\n\n"+
			"Replays the requests a ledger recorded through the proxy's spend guard under\n"+
			"the cap in <file>, and says which of them would have been refused. The result\n"+
			"is SIMULATED: a re-run of the recorded requests, not a statement about future ones.\n")
	}
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	if *policyPath == "" {
		return fmt.Errorf("--policy is required: %w", errUsage)
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("one or more ledger directories are required: %w", errUsage)
	}
	policy, err := loadSimulatePolicy(*policyPath)
	if err != nil {
		return err
	}

	var rep simulateReport
	rep.Schema, rep.Simulated = simulateSchema, true
	rep.Policy.Hash, rep.Policy.simulatePolicy = policy.hash(), policy
	rep.PricedAt, rep.RulesVersion = cachemodel.PriceTableVersion, cachemodel.RulesVersionInEffect()
	rep.Ledgers = fs.Args()

	// The population: every request the proxy forwarded and priced, in the
	// order it happened. A request the proxy refused at the time has no usage
	// and is counted, not replayed.
	type member struct {
		rec ledger.Record
		usd float64
		ub  bool
	}
	var members []member
	for _, dir := range fs.Args() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("ledger %s: %w", dir, err)
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			if e.IsDir() || !ledger.IsLedgerFile(path) {
				continue
			}
			records, _, _, _, err := ledger.ReadRecords(path)
			if err != nil {
				rep.Population.Unreadable++
				continue
			}
			for _, rec := range records {
				switch {
				case rec.Refusal != "":
					rep.Population.HistoricallyRefused++
				case rec.Response.Usage == nil:
					rep.Population.WithoutUsage++
				default:
					usd, ub := proxy.ListCost(*rec.Response.Usage, rec.Model)
					if ub {
						rep.Population.Unpriced++
					}
					members = append(members, member{rec: rec, usd: usd, ub: ub})
				}
			}
		}
	}
	sort.SliceStable(members, func(i, j int) bool {
		a, b := members[i].rec, members[j].rec
		if !a.Timestamp.Equal(b.Timestamp) {
			return a.Timestamp.Before(b.Timestamp)
		}
		if a.SessionID != b.SessionID {
			return a.SessionID < b.SessionID
		}
		return a.RequestID < b.RequestID
	})

	// The guard the proxy runs, on the records' own clock, with nothing loaded
	// from disk: the day total starts at zero on the first record's day.
	guard := proxy.NewSpendGuard(policy.limits())
	clock := time.Time{}
	guard.SetClock(func() time.Time { return clock })
	sessions := map[string]bool{}
	affected := map[string]bool{}
	rep.Decisions = []simulateDecision{}
	for _, m := range members {
		clock = m.rec.Timestamp
		sessions[m.rec.SessionID] = true
		d := simulateDecision{Session: m.rec.SessionID, RequestID: m.rec.RequestID, TS: m.rec.Timestamp.UTC().Format(time.RFC3339Nano),
			Model: m.rec.Model, ListUSD: m.usd, UpperBound: m.ub}
		if reason := guard.Check(m.rec.SessionID); reason != "" {
			d.Simulated, d.Reason = "refused", reason
			rep.Summary.Refused++
			rep.Summary.RefusedListUSD += m.usd
			affected[m.rec.SessionID] = true
		} else {
			d.Simulated = "admitted"
			rep.Summary.Admitted++
			u := m.rec.Response.Usage
			guard.Record(m.rec.SessionID, u.Input+u.CacheCreation+u.CacheRead+u.Output, m.usd, m.ub)
		}
		rep.Decisions = append(rep.Decisions, d)
	}
	rep.Population.Requests = len(members)
	rep.Population.Sessions = len(sessions)
	rep.Summary.SessionsAffected = len(affected)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	return writeSimulateReport(stdout, rep)
}

func writeSimulateReport(w io.Writer, rep simulateReport) error {
	caps := []string{}
	if rep.Policy.MaxSessionUSD > 0 {
		caps = append(caps, fmt.Sprintf("session $%.2f", rep.Policy.MaxSessionUSD))
	}
	if rep.Policy.MaxDayUSD > 0 {
		caps = append(caps, fmt.Sprintf("day $%.2f", rep.Policy.MaxDayUSD))
	}
	if rep.Policy.MaxSessionTokens > 0 {
		caps = append(caps, fmt.Sprintf("session %s tokens", comma(rep.Policy.MaxSessionTokens)))
	}
	if rep.Policy.MaxDayTokens > 0 {
		caps = append(caps, fmt.Sprintf("day %s tokens", comma(rep.Policy.MaxDayTokens)))
	}
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
	p("SIMULATED: policy %s (%s) replayed over %d requests in %d sessions from %s,\n",
		rep.Policy.Hash, strings.Join(caps, ", "), rep.Population.Requests, rep.Population.Sessions, strings.Join(rep.Ledgers, " "))
	p("at list prices dated %s (caching rules %s).\n\n", rep.PricedAt, rep.RulesVersion)
	p("  admitted   %d\n", rep.Summary.Admitted)
	p("  refused    %d   in %d session(s)\n", rep.Summary.Refused, rep.Summary.SessionsAffected)
	if rep.Summary.Refused > 0 {
		p("  list price of the refused requests: $%.4f. That is what they cost at list when\n", rep.Summary.RefusedListUSD)
		p("  they ran; what the agent would have done instead is not recorded.\n")
	}
	p("  not replayed: %d refused by the proxy at the time (their usage was never observed),\n", rep.Population.HistoricallyRefused)
	p("  %d without usage, %d unreadable file(s). %d request(s) ran on a model the price\n", rep.Population.WithoutUsage, rep.Population.Unreadable, rep.Population.Unpriced)
	p("  table does not carry and are counted at the dearest known rate, as the proxy counts them.\n")
	first := map[string]bool{}
	for _, d := range rep.Decisions {
		if d.Simulated != "refused" || first[d.Session] {
			continue
		}
		first[d.Session] = true
		if len(first) == 1 {
			p("\n  first refusal per session:\n")
		}
		p("    %-14s %s  %s  %s\n", d.Session, d.TS, d.RequestID, d.Reason)
	}
	p("\nA re-run of the recorded requests under the cap you gave. It says nothing about requests\n")
	p("that have not happened.\n")
	return nil
}

var _ = errors.New
