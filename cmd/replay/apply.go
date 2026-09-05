package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// applyPlan is one setting this tool is willing to change on a user's behalf.
//
// Exactly one setting qualifies today: the prompt cache TTL. It qualifies
// because it is a documented client setting, because changing it alters no
// content, and because the corpus can say what it would have cost. Every other
// suggestion the advisor makes is a change to how somebody works, and a tool
// that rewrites your instruction files because it judged them too long would be
// a worse tool than one that tells you and stops.
type applyPlan struct {
	Setting string
	// Want is the value the corpus supports; Have is what is set today.
	Want string
	Have string
	// Trustworthy gates the write. Numbers this tool would not stand behind
	// must not become changes to somebody's configuration.
	Trustworthy bool
	Reason      string
	// Evidence is the one line a person needs to judge the change themselves.
	Evidence string
}

// write applies the plan, or explains why it will not.
//
// The file belongs to the user and predates this tool, so: refuse on
// untrustworthy input, keep every key we did not set, back up what was there,
// and write owner-only.
func (p applyPlan) write(path string, out io.Writer, commit bool) error {
	if !p.Trustworthy {
		reason := p.Reason
		if reason == "" {
			reason = "the calibration for this corpus is not good enough to act on"
		}
		return fmt.Errorf("refusing to change %s: %s", p.Setting, reason)
	}
	if p.Want == p.Have {
		_, err := fmt.Fprintf(out, "%s is already %s. Nothing to change.\n", p.Setting, p.Want)
		return err
	}

	have := p.Have
	if have == "" {
		have = "(unset)"
	}
	if !commit {
		_, err := fmt.Fprintf(out, "would set %s: %s -> %s in %s\n  %s\nRe-run with --apply --yes to write it.\n",
			p.Setting, have, p.Want, path, p.Evidence)
		return err
	}

	settings := map[string]any{}
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		// A settings file that does not parse is a file to leave alone. The
		// user has something in there we do not understand, and guessing is
		// how a tool destroys a config it was asked to improve.
		if err := json.Unmarshal(existing, &settings); err != nil {
			return fmt.Errorf("%s is not valid JSON, so it will not be modified: %w", path, err)
		}
		backup := fmt.Sprintf("%s.bak-%s", path, time.Now().UTC().Format("20060102T150405Z"))
		if err := os.WriteFile(backup, existing, 0o600); err != nil {
			return fmt.Errorf("write backup: %w", err)
		}
		if _, err := fmt.Fprintf(out, "backed up %s\n", filepath.Base(backup)); err != nil {
			return err
		}
	case !os.IsNotExist(err):
		return err
	default:
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
	}

	settings[p.Setting] = p.Want
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "set %s: %s -> %s in %s\n", p.Setting, have, p.Want, path)
	return err
}

// ttlObservation is one session's cost under each TTL, in effective tokens.
type ttlObservation struct {
	Short float64 // ttl-5m
	Long  float64 // ttl-1h
}

// chooseTTL decides whether one TTL is worth switching to, weighted by cost.
//
// Sessions are the wrong unit. Most are short enough that no idle gap exceeds
// either TTL, so the two tie exactly, and counting those ties as votes let a
// few hundred trivial sessions outvote the handful that actually cost money.
// Summing effective tokens gives every session precisely the weight of its
// bill, which is the thing being optimised.
//
// The margin exists because the offline tier is an estimate with error bars
// wider than a percent. A difference this tool cannot distinguish from noise is
// not a reason to edit somebody's configuration.
func chooseTTL(obs []ttlObservation, have string) applyPlan {
	// With no wider corpus to compare against, what was scored is all there is.
	return chooseTTLWithCoverage(obs, have, 1)
}

// chooseTTLWithCoverage is chooseTTL plus the size of the bill it did not see.
//
// The engine refuses to score alternatives for a model whose behaviour has
// drifted, which is right, and which silently removes those sessions from this
// decision. On a real corpus that removed every one of the largest sessions,
// and the remainder recommended the opposite policy. So a recommendation is
// only offered when the sessions it could price account for most of the spend.
// coverage is the share of the corpus's as-run spend that could be scored at
// all, between 0 and 1.
func chooseTTLWithCoverage(obs []ttlObservation, have string, coverage float64) applyPlan {
	const minCoverage = 0.5
	const minMargin = 0.01

	plan := applyPlan{Setting: "promptCacheTtl", Have: have}
	if len(obs) == 0 {
		plan.Reason = "no session in this corpus reproduced well enough to act on"
		return plan
	}

	var short, long float64
	differing := 0
	for _, o := range obs {
		short += o.Short
		long += o.Long
		if o.Short != o.Long {
			differing++
		}
	}
	if differing == 0 {
		plan.Reason = fmt.Sprintf("5m and 1h cost the same across all %d sessions: no idle gap in this corpus outlives either", len(obs))
		return plan
	}

	want, winner, loser := "1h", long, short
	if short < long {
		want, winner, loser = "5m", short, long
	}
	if loser == 0 {
		plan.Reason = "no priced TTL policy in this corpus"
		return plan
	}
	margin := (loser - winner) / loser
	if margin < minMargin {
		plan.Reason = fmt.Sprintf("%s leads by only %.2f%% across %d sessions, which is inside the fit's own error bars", want, margin*100, len(obs))
		return plan
	}

	// The sessions that dominate a bill can prefer the opposite policy to the
	// many small ones, and a token-weighted total hides that rather than
	// resolving it. If the top decile by cost disagrees with the total, the
	// honest output is the disagreement.
	if big := heaviestDecile(obs); big != "" && big != want {
		plan.Reason = fmt.Sprintf("no single setting is right for this corpus: %s is cheaper in total, but %s is cheaper on the largest sessions, which is where the spend is. Set %s if your next sessions are long with idle gaps, %s if they are short and bursty",
			want, big, big, want)
		return plan
	}

	if coverage < minCoverage {
		plan.Reason = fmt.Sprintf("only %.0f%% of this corpus's effective tokens could be scored, so the sessions that cost the most are not represented; %s led on what was scored, which is not a finding",
			coverage*100, want)
		return plan
	}

	plan.Trustworthy = true
	plan.Want = want
	plan.Evidence = fmt.Sprintf("%s costs %.1f%% fewer effective tokens across %d sessions that reproduced at or above 95%%, covering %.0f%% of the corpus's scored spend (%d sessions differ between the two TTLs)",
		want, margin*100, len(obs), coverage*100, differing)
	return plan
}

// ttlPlan turns scored reports into observations and asks chooseTTL.
//
// A session only counts when the engine reproduced it well enough to believe.
// Sessions it could not follow are excluded rather than averaged in, because a
// confident recommendation built on turns we could not reproduce is exactly the
// failure this tool exists to point at.
func ttlPlan(reports []*analysis.LaneReport, have string) applyPlan {
	const minMatchRate = 0.95
	var obs []ttlObservation
	// Coverage is a share of as-run spend: what could be scored over what was
	// trusted. Leaving the unscoreable sessions out of the denominator is how a
	// recommendation ends up describing only the cheap tail of a corpus.
	var trustedTokens, scoredTokens float64
	for _, r := range reports {
		if r == nil || r.Calibration == nil || r.Calibration.Compared() == 0 {
			continue
		}
		if float64(r.Calibration.Reproduced)/float64(r.Calibration.Compared()) < minMatchRate {
			continue
		}
		var o ttlObservation
		var asRun float64
		for _, pol := range r.Policies() {
			switch pol.Name {
			case "as-run":
				asRun = pol.EffectiveTokens
			case "ttl-5m0s":
				o.Short = pol.EffectiveTokens
			case "ttl-1h0m0s":
				o.Long = pol.EffectiveTokens
			}
		}
		trustedTokens += asRun
		if o.Short > 0 && o.Long > 0 {
			scoredTokens += asRun
			obs = append(obs, o)
		}
	}
	coverage := 1.0
	if trustedTokens > 0 {
		coverage = scoredTokens / trustedTokens
	}
	return chooseTTLWithCoverage(obs, have, coverage)
}

// heaviestDecile reports which TTL wins across the costliest tenth of sessions,
// or "" when that tenth is too small to mean anything.
func heaviestDecile(obs []ttlObservation) string {
	if len(obs) < 10 {
		return ""
	}
	sorted := append([]ttlObservation(nil), obs...)
	sort.Slice(sorted, func(i, j int) bool {
		return math.Min(sorted[i].Short, sorted[i].Long) > math.Min(sorted[j].Short, sorted[j].Long)
	})
	var short, long float64
	for _, o := range sorted[:len(sorted)/10] {
		short += o.Short
		long += o.Long
	}
	if short == long {
		return ""
	}
	if short < long {
		return "5m"
	}
	return "1h"
}

func ttlFromPolicyName(name string) (string, bool) {
	switch name {
	case "ttl-5m0s":
		return "5m", true
	case "ttl-1h0m0s":
		return "1h", true
	}
	return "", false
}
