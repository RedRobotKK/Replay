package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FD-5, frozen. Grok was filed under a wire it does not speak.
//
// The registry and two source comments listed Grok beside Cursor, DeepSeek and
// OpenAI on openai:/v1/chat/completions, and the registry called it STUB. Both
// halves were conclusions with nothing behind them. It was grouped by an
// assumption about what an OpenAI-compatible CLI must send, and STUB claims a
// payload we wrote parses — nothing in this build parses that path at all.
// Captured from a live authenticated session, Grok posts to /responses at
// cli-chat-proxy.grok.com.
//
// The cost of the shape is a user pointing Replay at Grok, getting an empty
// report, and having nothing in the tool tell them why.
//
// What would be true again if it returned: a paragraph would name Grok inside
// the OpenAI-compatible family without the /responses endpoint that was actually
// measured standing next to it.
//
// PASS: no paragraph puts Grok on that wire without naming /responses.
// FAIL: the assumption came back, in prose or in a comment.
func TestFrozenFD5_GrokIsNotFiledUnderTheOpenAICompatibleWire(t *testing.T) {
	family := []string{
		"openai-compatible",
		"openai compatible",
		"/v1/chat/completions",
		"chat/completions",
		"chat completions",
	}
	for path, body := range textFiles(t, ".go", ".md") {
		for _, p := range paragraphs(body) {
			if _, ok := containsAny(p, "grok"); !ok {
				continue
			}
			marker, ok := containsAny(p, family...)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(p), "/responses") {
				continue
			}
			t.Errorf("%s names Grok alongside %q without naming /responses:\n\n%s\n\n"+
				"Grok posts to /responses at cli-chat-proxy.grok.com. Grouping it with "+
				"the chat-completions family is the assumption this project already made "+
				"once and had to retract; if the grouping is deliberate now, the "+
				"measurement that supports it belongs in the same paragraph",
				path, marker, p)
		}
	}
}

// FD-6, frozen. The rate-limit headers were described from their names.
//
// `x-ratelimit-remaining-*` was written up as "a falling per-request counter at
// token granularity" and "a significantly higher-fidelity instrument" than
// Anthropic's utilization fraction. That was composed from the header NAMES,
// before a single value had been read. Across 8 model calls and ~940KB of
// responses, remaining never moved off the plan ceiling; the titration moved
// 3.09M tokens through matched arms and shifted the utilisation counter zero
// steps.
//
// A remaining figure that always equals the limit is the same shape as a
// healthcheck that cannot fail, and it was about to be published as a spend
// signal.
//
// What would be true again if it returned: a document would describe these
// headers as moving, or as more precise than the alternative, with no captured
// reading behind it.
//
// PASS: any movement or fidelity claim about them cites an evidence file, and
// that file exists.
// FAIL: the claim came back naked, or its citation points at nothing.
func TestFrozenFD6_ARateLimitHeaderClaimCarriesItsMeasurement(t *testing.T) {
	root := repoRoot(t)
	headers := []string{"x-ratelimit", "anthropic-ratelimit", "ratelimit-remaining"}
	// Claims that the remaining figure MOVES. The retracted write-up said it
	// fell per request; across 8 calls it never left the ceiling. Scoped to
	// paragraphs that actually discuss "remaining", because a utilization
	// fraction really does fall and observing that is not this defect.
	movement := []string{
		"falling",
		"falls",
		"counts down",
		"ticks down",
		"decrement",
		"drains",
		"per-request counter",
		"token granularity",
	}
	// Claims that they are a BETTER instrument than the alternative. This half
	// needs no further trigger: it is a comparison, and a comparison drawn
	// before either side was read is the whole of what was retracted.
	fidelity := []string{
		"higher-fidelity",
		"higher fidelity",
		"more precise than",
		"better instrument",
	}
	cite := regexp.MustCompile(`evidence/([A-Za-z0-9._-]+\.md)`)

	for path, body := range textFiles(t, ".go", ".md") {
		if strings.HasPrefix(path, filepath.Join("internal", "regression")) {
			continue // this file names the retracted phrasings on purpose
		}
		for _, p := range paragraphs(body) {
			if _, ok := containsAny(p, headers...); !ok {
				continue
			}
			claim, ok := containsAny(p, fidelity...)
			if !ok {
				if _, saysRemaining := containsAny(p, "remaining"); !saysRemaining {
					continue
				}
				if claim, ok = containsAny(p, movement...); !ok {
					continue
				}
			}
			m := cite.FindStringSubmatch(p)
			if m == nil {
				t.Errorf("%s says the rate-limit headers %q with no evidence file cited:\n\n%s\n\n"+
					"This claim was made once from the header names alone and retracted: "+
					"remaining never left the ceiling across 8 calls. A behavioural claim "+
					"about these headers needs a reading behind it", path, claim, p)
				continue
			}
			ev := filepath.Join(root, "docs", "evidence", m[1])
			if _, err := os.Stat(ev); err != nil {
				t.Errorf("%s cites docs/evidence/%s for a %q claim, and that file does not "+
					"exist: %v. A citation nobody can follow is the claim without the "+
					"evidence, wearing its clothes", path, m[1], claim, err)
			}
		}
	}
}

// FD-7, frozen. The linter reported the first three of each and nobody asked.
//
// golangci-lint defaults to max-same-issues: 3 and max-issues-per-linter: 50.
// Those are display caps, not filters: the run still fails on everything it
// found. But the backlog was tracked as "37 golangci issues" because that is
// what the page said, and run uncapped main had 119. Eighty-one issues were
// real, were failing CI, and were not on anybody's screen.
//
// The instrument worked. What it reported was true. It was not what it knew.
//
// What would be true again if it returned: the caps would be absent or
// non-zero, and the issue count in the release criteria would be a count of
// what was displayed rather than of what exists.
//
// PASS: both caps are explicitly 0.
// FAIL: either is missing or restored to a truncating value.
func TestFrozenFD7_TheLinterReportsEveryIssueItFinds(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".golangci.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading .golangci.yml: %v", err)
	}
	for _, key := range []string{"max-same-issues", "max-issues-per-linter"} {
		re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `:\s*(\S+)\s*$`)
		m := re.FindSubmatch(body)
		if m == nil {
			t.Errorf(".golangci.yml does not set %s. Left unset it defaults to a display "+
				"cap, and a reader of the output sees a fraction of what the run found: "+
				"38 of 119 on this repo, and a backlog tracked against the 38", key)
			continue
		}
		if got := string(m[1]); got != "0" {
			t.Errorf(".golangci.yml sets %s: %s. Anything but 0 truncates the report, and "+
				"the number in RELEASE-CRITERIA.md stops being the number that exists",
				key, got)
		}
	}
}

// FD-8, frozen. The installer told every new user a retracted number.
//
// install.sh printed a corpus cost of $2851. It appeared in that file and in no
// evidence anywhere — the only number in the installer's closing screen with no
// source under it, shown to every person who ran the command. It was removed on
// 2026-09-06, and the removal is recorded in the file rather than erased.
//
// What would be true again if it returned: a figure would reach the installer's
// output that no evidence document supports.
//
// PASS: the corpus figures the installer prints appear in the corpus evidence,
// no dollar amount reaches its output, and the retraction is still on record.
// FAIL: a number outran its source again.
func TestFrozenFD8_TheInstallerPrintsOnlyFiguresTheEvidenceHolds(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "install.sh"))
	if err != nil {
		t.Fatalf("reading install.sh: %v", err)
	}
	script := string(body)

	corpus := filepath.Join(root, "docs", "evidence", "calibration-corpus-2026-09-06.md")
	ev, err := os.ReadFile(corpus)
	if err != nil {
		t.Fatalf("reading the corpus evidence the installer's figures come from: %v", err)
	}
	evidence := string(ev)

	// The two figures the closing screen states, checked against the document
	// they claim to summarise rather than against a remembered value.
	for _, claim := range []struct{ label, pattern string }{
		{"sessions", `Calibrated against ([0-9,]+) sessions`},
		{"transcripts", `across ([0-9,]+) transcripts`},
	} {
		m := regexp.MustCompile(claim.pattern).FindStringSubmatch(script)
		if m == nil {
			t.Errorf("install.sh no longer states a %s figure matching %q. If the closing "+
				"screen changed, this guard has to change with it — silently losing the "+
				"cross-check is how the last unsourced number survived", claim.label, claim.pattern)
			continue
		}
		// The figure must appear in the evidence AS A COUNT OF THIS THING. A
		// bare substring search passes on any document holding a large table
		// of numbers, which is a check that cannot fail dressed as one that
		// can — and it survived a mutation that raised the session count to a
		// figure the corpus never measured.
		n := strings.ReplaceAll(m[1], ",", "")
		counted := regexp.MustCompile(`\b` + regexp.QuoteMeta(n) + `\b[^\n]{0,40}?` + claim.label)
		if !counted.MatchString(strings.ReplaceAll(evidence, ",", "")) {
			t.Errorf("install.sh tells every new user %s %s, and %s never counts %s of them. "+
				"This is exactly the shape of the $2851 the installer carried: a number in "+
				"the installer and in no evidence anywhere",
				m[1], claim.label, filepath.Base(corpus), m[1])
		}
	}

	// No money in the output. The removed figure was a dollar amount, and the
	// argument for removing it was that at install time the tool has measured
	// nothing, so any cost it quotes is the author's and not the reader's.
	money := regexp.MustCompile(`\$[0-9]`)
	for i, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "printf") {
			continue
		}
		if money.MatchString(trimmed) {
			t.Errorf("install.sh:%d prints a dollar figure: %s\n"+
				"At install time nothing has been measured, so a cost here answers the "+
				"author's question and not the reader's", i+1, trimmed)
		}
	}

	// And the retraction stays readable. A correction quietly deleted is a
	// correction that has to be made again.
	if !strings.Contains(script, "2851") {
		t.Error("install.sh no longer records that it once printed an unsourced $2851. " +
			"Corrections stay in place in this repository: dropping the note is how the " +
			"number comes back, because nothing in the file says it was ever wrong")
	}
}

// FD-9, frozen. A document said Windows was built after it stopped being built.
//
// v0.4.0 shipped Windows archives beside release notes calling Windows
// unsupported. The archives were deleted after publication and goreleaser's
// goos line dropped windows — and docs/SURFACES.md went on saying "Windows is
// built and never tested. goreleaser produces the target" for as long as nobody
// re-read it against the config.
//
// It is the record-lag class: a decision built on a stored fact that no longer
// matches reality. Both directions hurt. A document claiming a platform is
// built when it is not sends someone looking for an archive that was never
// published; a document claiming one is not built when it is leaves a binary
// shipping with nobody's promises behind it, which is what v0.4.0 did.
//
// What would be true again if it returned: a document's list of built platforms
// and the release config's would disagree, with nothing failing.
//
// PASS: every "X is built" and "X is not built" claim matches goreleaser.
// FAIL: the document and the config drifted apart.
func TestFrozenFD9_PlatformClaimsMatchTheReleaseConfig(t *testing.T) {
	root := repoRoot(t)
	cfg, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("reading .goreleaser.yaml: %v", err)
	}
	m := regexp.MustCompile(`(?m)^\s*goos:\s*\[([^\]]*)\]`).FindSubmatch(cfg)
	if m == nil {
		t.Fatal(".goreleaser.yaml has no `goos: [...]` build line. Without it this guard " +
			"cannot tell what is built, and a guard that cannot tell is not a guard")
	}
	built := map[string]bool{}
	for _, p := range strings.Split(string(m[1]), ",") {
		if p = strings.TrimSpace(p); p != "" {
			built[p] = true
		}
	}

	names := map[string]string{"windows": "Windows", "linux": "Linux", "darwin": "macOS"}
	docs := textFiles(t, ".md")
	for goos, pretty := range names {
		isBuilt := regexp.MustCompile(`(?i)(^|[^"\x60])\*{0,2}` + pretty + ` is built`)
		notBuilt := regexp.MustCompile(`(?i)(^|[^"\x60])\*{0,2}` + pretty + ` is not built`)
		for path, body := range docs {
			if strings.HasPrefix(path, filepath.Join("internal", "regression")) {
				continue
			}
			if isBuilt.MatchString(body) && !built[goos] {
				t.Errorf("%s says %q, and .goreleaser.yaml builds %v. The archive that "+
					"sentence promises does not exist", path, pretty+" is built", keys(built))
			}
			if notBuilt.MatchString(body) && built[goos] {
				t.Errorf("%s says %q, and .goreleaser.yaml builds it. A platform shipping "+
					"while the documents disown it is what v0.4.0 did", path, pretty+" is not built")
			}
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
