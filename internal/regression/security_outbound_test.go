package regression

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// SEC. SECURITY.md said "Replay runs on your machine and sends nothing
// anywhere" from launch until 2026-10-20. It was the one sentence a security
// reader is most likely to quote, and it was false: `replay serve` forwards the
// operator's traffic to a provider by design, and four typed commands reach
// other hosts. The README had been corrected for the same claim twice (FCNET,
// FCWK2, DC1) and nothing read SECURITY.md, so the correction never reached the
// policy file.
//
// These guards tie SECURITY.md to the code rather than to another document.
// outboundSurfaces is the list; every source file in the binary that can build
// or send a request, dial, or run a subprocess must belong to one entry, each
// entry's anchor must still be in its first file, and SECURITY.md must name
// every entry. A new sender fails here until somebody says in SECURITY.md what
// it reaches and when.

type outboundSurface struct {
	id string
	// files are the production sources that implement the surface. The
	// anchor must appear in files[0], so a moved or deleted sender is noticed.
	files  []string
	anchor string
	// names must each appear in SECURITY.md.
	names []string
}

var outboundSurfaces = []outboundSurface{
	{
		id:     "proxy upstream",
		files:  []string{"internal/proxy/server.go", "internal/proxy/retry.go", "cmd/replay/serve.go"},
		anchor: "r.SetURL(cfg.Upstream)",
		names:  []string{"replay serve", "api.anthropic.com", "--upstream", "HTTPS_PROXY"},
	},
	{
		id:     "rules --check-prices",
		files:  []string{"cmd/replay/priceclient.go"},
		anchor: "raw.githubusercontent.com/BerriAI/litellm",
		names:  []string{"rules --check-prices", "raw.githubusercontent.com"},
	},
	{
		id:     "rules --update",
		files:  []string{"cmd/replay/rules.go"},
		anchor: "refusing to fetch rules over plain http",
		names:  []string{"rules --update", "402"},
	},
	{
		id:     "probe --execute",
		files:  []string{"internal/probe/run.go", "internal/probe/vary.go"},
		anchor: "/v1/messages/count_tokens",
		names:  []string{"probe --execute"},
	},
	{
		id:     "upgrade",
		files:  []string{"internal/selfupdate/fetch.go", "internal/selfupdate/verify.go"},
		anchor: "/releases/latest",
		names:  []string{"replay upgrade", "github.com/RedRobotKK/Replay/releases", "cosign"},
	},
	{
		id:     "doctor and tui loopback probe",
		files:  []string{"cmd/replay/doctor.go", "cmd/replay/doctor_guards.go", "cmd/replay/tui.go"},
		anchor: "isLoopbackURL(base)",
		names:  []string{"replay doctor", "replay tui", "loopback"},
	},
	{
		id:     "burn Ollama probe",
		files:  []string{"cmd/replay/ollamaversion.go"},
		anchor: "http://127.0.0.1:11434/api/version",
		names:  []string{"replay burn", "127.0.0.1:11434"},
	},
	{
		id:     "signed rules feed (unwired)",
		files:  []string{"internal/feed/feed.go"},
		anchor: "func Fetch(c *http.Client",
		names:  []string{"internal/feed"},
	},
}

// notEgress are sources that match a sender shape and cannot reach another
// host, each with the reason. Adding one is an argument, not an exemption.
var notEgress = map[string]string{
	"internal/proxy/uds.go": "dials the operator's own Unix socket path to detect a live listener before binding",
}

// senderShapes are the source spellings that build or perform a request, dial,
// or run a subprocess. Text, not AST, on purpose: the aim is to over-match and
// make each hit be accounted for, and cmd/replay/outbound_drift_test.go
// already holds the import-level check that does not depend on spelling.
var senderShapes = []string{
	"http.NewRequest", "http.DefaultClient", "http.Client{", "httputil.ReverseProxy{",
	"http.Get(", "http.Post(", "http.Head(", "net.Dial", "exec.Command",
}

func binarySenderFiles(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for rel, body := range textFiles(t, ".go") {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		// scripts/ is not part of any build of the binary; the drift test in
		// cmd/replay accounts for scripts/x402-e2e by name.
		if !strings.HasPrefix(rel, "cmd/") && !strings.HasPrefix(rel, "internal/") {
			continue
		}
		if _, ok := containsAny(body, senderShapes...); ok {
			out[rel] = true
		}
	}
	return out
}

// SEC1: every sender in the binary belongs to a listed surface, and every
// listed surface still exists in the code.
func TestSEC1_EveryOutboundSenderBelongsToAListedSurface(t *testing.T) {
	root := repoRoot(t)
	senders := binarySenderFiles(t)
	// Anti-vacuity: eleven sender files existed when this was written.
	if len(senders) < 8 {
		t.Fatalf("found only %d sender files (%v); the walk or the shapes are broken", len(senders), keys(senders))
	}
	owned := map[string]string{}
	for _, s := range outboundSurfaces {
		for _, f := range s.files {
			owned[f] = s.id
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(s.files[0])))
		if err != nil {
			t.Errorf("surface %q: %s is unreadable: %v", s.id, s.files[0], err)
			continue
		}
		if !strings.Contains(string(body), s.anchor) {
			t.Errorf("surface %q: %s no longer contains %q. The sender moved or went away; "+
				"update the list and SECURITY.md together", s.id, s.files[0], s.anchor)
		}
	}
	var unlisted []string
	for f := range senders {
		if _, ok := owned[f]; ok {
			continue
		}
		if _, ok := notEgress[f]; ok {
			continue
		}
		unlisted = append(unlisted, f)
	}
	sort.Strings(unlisted)
	if len(unlisted) > 0 {
		t.Errorf("these files can send, dial or exec and belong to no surface SECURITY.md is held to:\n  %s\n\n"+
			"Add the file to an entry in outboundSurfaces, or a new entry with the names SECURITY.md "+
			"must carry, and say in SECURITY.md what it reaches and when", strings.Join(unlisted, "\n  "))
	}
	for f := range notEgress {
		if !senders[f] {
			t.Errorf("notEgress lists %s, which no longer matches a sender shape; remove the entry", f)
		}
	}
}

// SEC2: SECURITY.md names every surface.
func TestSEC2_SecurityPolicyNamesEveryOutboundSurface(t *testing.T) {
	doc := readDoc(t, "SECURITY.md")
	for _, s := range outboundSurfaces {
		for _, n := range s.names {
			if !strings.Contains(doc, n) {
				t.Errorf("SECURITY.md does not name %q, part of the %q outbound surface. "+
					"A security policy that omits a network path is the document a reviewer "+
					"trusts first and checks last", n, s.id)
			}
		}
	}
}

// trustDocs are the pages a reader consults to decide what leaves the machine.
var trustDocs = []string{
	"SECURITY.md",
	"README.md",
	"docs/SURFACES.md",
	"docs/guide/getting-started.md",
	"packaging/npm/README.md",
}

// blanketNoEgress are the shapes the false claim has taken or would take. A
// quoted occurrence is a document talking ABOUT the claim and is allowed, the
// same rule FCNET applies to the README.
var blanketNoEgress = []string{
	"sends nothing anywhere",
	"nothing leaves your machine",
	"nothing leaves the machine",
	"never sends anything",
	"makes no network request at all",
	"no network access at all",
}

// SEC3: no trust page makes a blanket no-egress claim.
func TestSEC3_NoTrustPageClaimsNothingLeavesTheMachine(t *testing.T) {
	checked := 0
	for _, rel := range trustDocs {
		for _, p := range paragraphs(readDoc(t, rel)) {
			checked++
			low := strings.ToLower(strings.Join(strings.Fields(p), " "))
			for _, claim := range blanketNoEgress {
				idx := 0
				for {
					i := strings.Index(low[idx:], claim)
					if i < 0 {
						break
					}
					at := idx + i
					idx = at + len(claim)
					// Inside a quotation in this paragraph: an odd number of
					// double quotes before it. That is a correction citing the
					// retired sentence, not the sentence.
					if strings.Count(low[:at], `"`)%2 == 1 {
						continue
					}
					t.Errorf("%s asserts %q. `replay serve` forwards the operator's traffic to the "+
						"provider and four typed commands reach other hosts (SEC1's list). Say what "+
						"never happens automatically and what each typed command contacts instead:\n\n%s",
						rel, claim, p)
				}
			}
		}
	}
	if checked < len(trustDocs) {
		t.Fatalf("read %d paragraphs across %d trust pages; the split is broken", checked, len(trustDocs))
	}
}
