package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// canonicalInstallHost is the one host the documentation may tell a reader to
// pipe into a shell.
const canonicalInstallHost = "replay.doctor"

// otherInstallHost still serves a byte-identical copy of install.sh, which is
// exactly why this needs a test rather than a habit: nothing breaks when a
// document names it, so nothing tells you.
const otherInstallHost = "redrobot.jp"

// FC-IH. Every documented install command names one host.
//
// On 2026-09-12, four days before launch, the README's install line, the
// getting-started guide, the first-run journey and the launch post itself all
// said `curl -fsSL https://redrobot.jp/replay.sh | sh`, while the product site
// at replay.doctor published its own. The site's Getting Started page is
// generated from docs/guide/getting-started.md in this repository, so
// replay.doctor was telling its own visitors to install from somewhere else.
//
// Both hosts served a byte-identical script, so nothing was broken and nothing
// was going to tell anybody. That is the shape of defect this repository
// exists to find in other people's numbers: not a failure, a divergence that
// no check observes.
//
// WHY IT MATTERS MORE THAN IT LOOKS. A `curl | sh` line is the highest-trust
// sentence a project prints. A reader deciding whether to run it checks that
// the domain is the project's own. Two domains for one tool, at the moment
// when every single visitor is new and has no way to know which is legitimate,
// spends the credibility that the signing, the checksums and the refusal to
// install unverified were built to earn.
//
// PASS: no tracked document names the other host in an install URL.
// FAIL: the list of files below is where a reader is sent somewhere else.
//
// SCOPE. This checks what the documents SAY. It does not fetch either host, so
// it cannot notice the two scripts diverging; `installer drift` in ci.yml owns
// that, and it compares the hosted script against install.sh in this tree.
//
// WHAT IS EXEMPT, AND WHY THAT IS NOT A LOOPHOLE. The files in recordExempt
// exist to preserve things that were wrong. This repository's rule is that a
// correction keeps the mistake legible: the 98.8% that became 4.2% is still on
// the README, and the dated evidence files take amendments rather than
// rewrites. A changelog entry recording that the install host was wrong has to
// be able to quote the host that was wrong, and a test that forbade it would be
// forcing the project to launder its own history to stay green.
//
// The distinction is instruction against record. Every other document tells a
// reader what to run. These say what was once said. The exemption is by
// filename rather than by pattern, because "is this sentence an instruction"
// is not a thing a regular expression can decide, and a clever heuristic here
// would fail open on exactly the file that matters.
func TestFCIH_EveryDocumentedInstallCommandNamesOneHost(t *testing.T) {
	root := repoRoot(t)
	needle := otherInstallHost + "/replay.sh"

	// Files whose job is to record what was previously said. See the note above.
	recordExempt := map[string]bool{
		"CHANGELOG.md": true,
	}

	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Worktrees hold other branches, and vendor and node_modules hold
			// other people's files. None of them are what this repository
			// publishes.
			switch info.Name() {
			case ".git", "node_modules", "vendor", ".claude", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md", ".mdx", ".cff", ".yml", ".yaml":
		default:
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if recordExempt[filepath.ToSlash(rel)] {
			return nil
		}
		if strings.Contains(string(b), needle) {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("%d document(s) publish an install command pointing at %s:\n  %s\n\n"+
			"The canonical install host is %s. Both serve the same script today, which is "+
			"why this drifts silently: nothing breaks, so nothing reports it. Change the URL, "+
			"or change canonicalInstallHost here and say why in the commit.",
			len(offenders), otherInstallHost, strings.Join(offenders, "\n  "), canonicalInstallHost)
	}
}

// FC-IH2: the canonical host is actually the one the README tells people to run.
//
// The check above only proves a wrong host is absent. Absence is not presence:
// deleting the install line entirely would satisfy it. This one requires the
// README to carry a working install command naming the canonical host, so the
// pair cannot both pass on an empty file.
func TestFCIH2_TheReadmeCarriesTheCanonicalInstallCommand(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	want := "https://" + canonicalInstallHost + "/replay.sh"
	if !strings.Contains(string(b), want) {
		t.Errorf("README.md does not contain %s.\nThe companion test refuses the wrong host; "+
			"this one refuses no host at all, so that deleting the install line cannot be "+
			"mistaken for fixing the problem.", want)
	}
}
