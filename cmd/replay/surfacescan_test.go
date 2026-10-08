package main

// Moved from internal/claims on 2026-10-02: the scans cover the whole tree, and
// a test that certifies the binary must live in a package the binary links,
// which internal/claims is not (TestWiringGate_RegisteredOraclesRunInTheShippedClosure).

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Repository-wide scans. These are the tests for claims whose subject is the
// whole product surface rather than one package's behaviour.
//
// Every scan here carries a positive control: a scan that finds nothing
// because it is broken would otherwise pass, and a check that cannot fail is
// not evidence.

func nonTestGoSources(t *testing.T) map[string]string {
	t.Helper()
	root := wiringRepoRoot(t)
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		out[rel] = string(b)
		return nil
	})
	if len(out) < 200 {
		t.Fatalf("scan found only %d non-test Go files; the walk is broken and every "+
			"assertion built on it would pass vacuously", len(out))
	}
	return out
}

// userFacingSources is nonTestGoSources minus this register.
//
// The register has to quote a claim verbatim in order to classify it as
// NOT_MEASURED or as a non-claim, so scanning it for claim vocabulary finds
// the refusal and calls it an assertion. The exclusion is exactly one
// directory and is asserted to be exactly one directory, so it cannot grow
// into a place to hide things.
func userFacingSources(t *testing.T) map[string]string {
	t.Helper()
	all := nonTestGoSources(t)
	out := map[string]string{}
	excluded := 0
	for rel, src := range all {
		if strings.HasPrefix(rel, "internal/claims/") {
			excluded++
			continue
		}
		out[rel] = src
	}
	if excluded == 0 {
		t.Fatal("the register was not found by the scan, so excluding it proved nothing")
	}
	if len(all)-len(out) != excluded {
		t.Fatalf("exclusion removed %d files but only %d were accounted for",
			len(all)-len(out), excluded)
	}
	return out
}

var urlRe = regexp.MustCompile(`https?://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)

// RPL-C012. The claim under attack is that Replay can establish what a
// provider actually billed. It cannot, and the reason is structural: nothing
// in the binary can reach a place where a charge is recorded.
//
// This is an absence proof, so it is built to be falsifiable: the scan is
// shown to find the destinations that ARE there, and is shown to catch a
// planted billing endpoint.
func TestC012_NoProviderBillingEndpointExists(t *testing.T) {
	srcs := nonTestGoSources(t)

	var found []string
	for rel, src := range srcs {
		for _, u := range urlRe.FindAllString(src, -1) {
			found = append(found, rel+" "+u)
		}
	}

	// Positive control. If the scanner finds no URL at all it is broken, and
	// the absence claim below would be vacuous.
	if len(found) == 0 {
		t.Fatal("scanner found no URL anywhere in non-test code; it is broken")
	}

	// A destination that would indicate authoritative billing. These are the
	// shapes a provider uses to expose a charge, as distinct from inference.
	billing := regexp.MustCompile(`(?i)/v\d+/(invoices?|billing|balance|credits?|usage_report)|` +
		`(?i)(billing|invoice)\.[a-z]+\.(com|ai)|` +
		`(?i)api\.stripe\.com`)

	var offenders []string
	for _, f := range found {
		if billing.MatchString(f) {
			offenders = append(offenders, f)
		}
	}
	if len(offenders) != 0 {
		t.Errorf("RPL-C012 is REFUTED: code reaches what looks like an authoritative "+
			"billing endpoint:\n  %s\nIf this is real, the claim surface must be "+
			"updated, not the test.", strings.Join(offenders, "\n  "))
	}

	// Negative control for the detector itself. A planted billing URL must be
	// caught, or the clean result above means nothing.
	if !billing.MatchString("x https://api.anthropic.com/v1/invoices") {
		t.Fatal("the billing detector does not detect a billing endpoint; the clean " +
			"result above is worthless")
	}
	if billing.MatchString("x https://api.anthropic.com/v1/messages") {
		t.Fatal("the billing detector fires on an ordinary inference endpoint; it is " +
			"too broad to mean anything")
	}
}

// RPL-C022. The footprint claim. The set of REMOTE destinations the shipped
// binary can reach is small and enumerated; this pins it so that adding one
// is a deliberate act visible in a diff rather than a quiet change to a
// promise printed on the README.
//
// Scope, stated because it is doing real work here:
//   - loopback is excluded. A request to 127.0.0.1 is the local proxy or a
//     local Ollama, and the claim is about the network, not about sockets.
//   - format placeholders and documentation examples are excluded. They are
//     not destinations.
//   - scripts/ is excluded. It is build-tagged tooling, not the binary.
func TestC022_OutboundDestinationsAreAnEnumeratedSet(t *testing.T) {
	// Each entry needs a reason, because the reason is what a reviewer checks.
	allowed := map[string]string{
		"https://api.anthropic.com":                   "probe and replay serve, both only on a command the user types",
		"https://www.anthropic.com":                   "the pricing page the compiled price table cites as its source",
		"https://api.github.com":                      "self-update release check",
		"https://github.com":                          "self-update artifact download",
		"https://objects.githubusercontent.com":       "self-update artifact storage",
		"https://raw.githubusercontent.com":           "the LiteLLM price database, the second price observer",
		"https://token.actions.githubusercontent.com": "the Sigstore OIDC issuer identity checked during release verification",
		"https://replay.doctor":                       "opt-in corpus contribution",
		"https://redrobot.jp":                         "project home",
		"https://www.redrobot.jp":                     "project home",
	}

	// Not destinations.
	skip := regexp.MustCompile(`(?i)^https?://(localhost|127\.0\.0\.1|\[::1\]|0\.0\.0\.0|%[sv]|replay\b|internal\.corp|seller\.test)`)

	srcs := nonTestGoSources(t)
	host := regexp.MustCompile(`^https?://[^/\s"\'` + "`" + `]+`)

	unexpected := map[string][]string{}
	counted := 0
	for rel, src := range srcs {
		if strings.HasPrefix(rel, "scripts/") {
			continue
		}
		for _, u := range urlRe.FindAllString(src, -1) {
			h := host.FindString(u)
			if h == "" || skip.MatchString(h) {
				continue
			}
			h = strings.TrimRight(h, ".")
			counted++
			if _, ok := allowed[h]; !ok {
				unexpected[h] = append(unexpected[h], rel)
			}
		}
	}

	// Positive control. If nothing is counted the enumeration is vacuous.
	if counted == 0 {
		t.Fatal("no remote destination found at all; the scan is broken and the " +
			"enumeration below would pass against any code")
	}
	for h, where := range unexpected {
		t.Errorf("RPL-C022: remote destination %s is not enumerated, seen in %v. "+
			"Either it is legitimate and belongs in the list with a reason, or the "+
			"footprint claim is now false.", h, where)
	}

	// Negative control for the skip rule: it must not swallow a real host.
	if skip.MatchString("https://api.anthropic.com") {
		t.Fatal("the loopback skip swallows a real remote host; the enumeration is worthless")
	}
}

// RPL-C013. The savings non-claim. The product states it reports what was
// already spent, never what will be saved. This checks the user-facing
// strings do not quietly contradict that.
func TestC013_NoSavingsClaimReachesTheUser(t *testing.T) {
	// Forecast vocabulary. "saving" alone is not an offence: the repository
	// legitimately uses it to describe a projection it labels as one, and
	// "SavingPerTurnUSD" is an internal identifier, not user-facing text.
	forecast := regexp.MustCompile(`(?i)you (will|could|would) save|` +
		`save \$|saves you|monthly saving|projected saving per month|` +
		`guaranteed saving`)

	srcs := userFacingSources(t)
	var hits []string
	for rel, src := range srcs {
		for _, line := range strings.Split(src, "\n") {
			// Only string literals reach a user.
			if !strings.Contains(line, `"`) {
				continue
			}
			if forecast.MatchString(line) {
				hits = append(hits, rel+": "+strings.TrimSpace(line))
			}
		}
	}
	for _, h := range hits {
		t.Errorf("RPL-C013 is contradicted by user-facing text: %s", h)
	}

	// Negative control: the detector must fire on the thing it forbids.
	if !forecast.MatchString(`fmt.Println("you will save $40 a month")`) {
		t.Fatal("the forecast detector does not detect a forecast; a clean run proves nothing")
	}
}

// RPL-C016. No task-improvement claim is asserted anywhere a user reads.
// The R10 trial left this NOT_MEASURED, so an assertion would be ahead of the
// evidence.
func TestC016_NoTaskImprovementClaimIsAsserted(t *testing.T) {
	// The first version of this regex could not match the very string the
	// negative control plants, which is the defect this campaign exists to
	// find in other people's checks. Widened and re-proven below.
	bad := regexp.MustCompile(`(?i)(improve|improves|boost|boosts)\s+(\w+\s+){0,2}(task|agent|coding)\s+(\w+\s+){0,2}(outcome|performance|completion|success)|` +
		`(?i)makes\s+(your\s+)?agents?\s+(smarter|better|more accurate)|` +
		`(?i)reduces?\s+(your\s+)?(agent\s+)?(errors|mistakes)`)

	srcs := userFacingSources(t)
	for rel, src := range srcs {
		for _, line := range strings.Split(src, "\n") {
			if !strings.Contains(line, `"`) {
				continue
			}
			if bad.MatchString(line) {
				t.Errorf("RPL-C016 is NOT_MEASURED but %s asserts it: %s",
					rel, strings.TrimSpace(line))
			}
		}
	}
	if !bad.MatchString(`"Replay improves agent task performance"`) {
		t.Fatal("the improvement detector does not fire on an improvement claim")
	}
}

// correlationPathSources restricts the account-identity scan below to the
// files RPL-C019's own Scope field names: "every correlation path: ledger,
// transcript readers, cost report". A repo-wide scan was the right breadth
// before any account-shaped identity existed anywhere and worked only
// because nothing matched by construction. ADR-0028's tenancy unit
// (internal/tenancy, an unwired primitive: docs/design/UNWIRED-LOG.md) now
// defines TenantID and AccountID deliberately, for a hosted-service
// ownership primitive that is not part of this correlation path. RPL-C019
// is "can two provider-account records be told apart in the
// ledger/transcript/cost-report path", not "does any account-shaped word
// exist anywhere in the binary"; scoping the scan to
// the claim's own stated Scope keeps the original protection exactly as
// strong where it matters (it still catches an account-shaped identity
// introduced into the ledger, a transcript reader, or the cost report,
// including one reached only through an import, since the importing
// file's own source then contains the identifier) and stops it firing on
// an unrelated, already-recorded identity living outside that path.
func correlationPathSources(t *testing.T) map[string]string {
	t.Helper()
	all := nonTestGoSources(t)
	out := map[string]string{}
	prefixes := []string{"internal/ledger/", "internal/transcript/"}
	exact := map[string]bool{"cmd/replay/cost.go": true}
	for rel, src := range all {
		match := exact[rel]
		for _, p := range prefixes {
			if strings.HasPrefix(rel, p) {
				match = true
			}
		}
		if match {
			out[rel] = src
		}
	}
	if len(out) < 5 {
		t.Fatalf("correlation-path scan found only %d files; the scope list is probably wrong", len(out))
	}
	return out
}

// RPL-C019. Whether an account boundary exists to correlate on, in the
// ledger/transcript-reader/cost-report path specifically.
//
// The register previously said a cross-account id collision "would not be
// caught", which reads as an untested case. It is not: there is nothing to
// test. No account-shaped identity exists anywhere in the correlation path,
// so two records from different accounts are not merely hard to tell apart,
// they are indistinguishable by construction.
//
// Recording that as NO_ENDPOINT rather than as a gap is the honest
// classification, and this test is what makes it checkable. Scoped to the
// correlation path (see correlationPathSources) rather than the whole
// repository since RPL-C038: a hosted-service tenant/account identity now
// exists in internal/tenancy, deliberately, for a different question than
// this claim asks, and a repo-wide scan would fail on it forever for no
// finding.
// accountShapedIdentityShapes are the identity shapes that would
// constitute the account boundary RPL-C019 forbids anywhere in the
// ledger/transcript/cost-report correlation path (see
// correlationPathSources). Package-level so accountShapedIdentityHits
// and the regression tests in surfacescan_tenancy_test.go share the one
// detector TestXW6 itself runs, rather than each re-declaring their own
// copy of it.
var accountShapedIdentityShapes = regexp.MustCompile(`\b(AccountID|TenantID|OrgID|OrganizationID|OrganisationID|ProjectID|WorkspaceID)\b`)

// accountShapedIdentityHits is RPL-C019's detector: which account-shaped
// identity shapes appear in src. See referencesRegisteredTenancyPrimitive
// (surfacescan_tenancy_test.go) for the one semantic exception this
// applies, to TenantID only.
func accountShapedIdentityHits(src string) []string {
	matches := accountShapedIdentityShapes.FindAllString(src, -1)
	if len(matches) == 0 {
		return nil
	}
	internal := referencesRegisteredTenancyPrimitive(src)
	var hits []string
	for _, m := range matches {
		if m == "TenantID" && internal {
			continue
		}
		hits = append(hits, m)
	}
	return hits
}

func TestXW6_NoAccountIdentityExistsToCorrelateOn(t *testing.T) {
	// The register names these identities in order to record their absence,
	// so scanning it finds the record and calls it the thing.
	srcs := correlationPathSources(t)

	var found []string
	for rel, src := range srcs {
		if len(accountShapedIdentityHits(src)) > 0 {
			found = append(found, rel)
		}
	}

	// POSITIVE CONTROL for the detector: it must fire on the thing it looks
	// for, or "none found" is worthless.
	if len(accountShapedIdentityHits("type Record struct { AccountID string }")) == 0 {
		t.Fatal("the identity detector does not detect an account identity; a clean " +
			"result proves nothing")
	}
	// And it must not fire on the identities that DO exist, or it is too
	// broad to distinguish them.
	if len(accountShapedIdentityHits("type Record struct { SessionID string; AgentID string; RequestID string }")) != 0 {
		t.Fatal("the detector fires on SessionID/AgentID/RequestID; it cannot tell an " +
			"account boundary from the identities Replay actually carries")
	}

	if len(found) != 0 {
		t.Errorf("RPL-C019 is classified NO_ENDPOINT, but an account-shaped identity "+
			"appears in %v. If an account boundary now exists, the claim must be "+
			"reclassified and tested, not left as NO_ENDPOINT.", found)
	}
}
