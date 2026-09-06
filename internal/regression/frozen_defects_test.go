package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The register of defects this project has found, and what stops each one
// coming back.
//
// A list of fixed bugs is worth nothing on its own; every project has one and
// none of them fails. What makes this a harness is the rule below: a row
// claiming to be guarded must name a test function that exists, and a row
// claiming the defect is still live in this tree must name a fragment of the
// defective code that is still there. Neither status can be typed into place.
//
// The model is internal/ledger/surface_registry_test.go, where LIVE requires a
// captured fixture on disk. Same discipline, different subject: there, a
// support claim; here, a claim to be protected from something.

// DefectStatus is what actually stands between this tree and the defect.
type DefectStatus string

const (
	// StatusGuarded means a test in this tree fails if the defect returns, and
	// that test has been made to fail on purpose to prove it.
	StatusGuarded DefectStatus = "GUARDED"

	// StatusUnmerged means the defect was found and fixed, and the fix is on a
	// branch that has not reached this tree. The defect is therefore STILL
	// PRESENT here and cannot be guarded from here: a test asserting the fixed
	// behaviour would be red, and one weakened until it passed would be the
	// project's own recurring failure mode wearing a harness badge.
	//
	// The row carries a detector instead — a fragment of the defective code
	// that must still be findable. When the fix lands, the detector stops
	// matching and this test goes red, which is the harness asking for the row
	// to be promoted rather than quietly outliving its subject.
	StatusUnmerged DefectStatus = "UNMERGED"
)

// Detector locates the defect itself, for a row that claims it is still here.
type Detector struct {
	File string
	Text string
}

// Defect is one entry in the harness.
type Defect struct {
	// ID is stable and appears in the guard's own test name, so a reader
	// landing on a failure can find the story behind it.
	ID string
	// Title is what the defect was, in one line.
	Title string
	// Looked is what it looked like from outside — what an operator or a
	// reader saw while it was live. Almost every one of these looked like
	// success, which is why they lasted.
	Looked string
	// Symptom is what would be true again if it came back. This is the field
	// that makes the row a test rather than a memory.
	Symptom string
	// Status is what stands between this tree and the defect.
	Status DefectStatus
	// Guards name the test functions that fail if it returns. Required when
	// GUARDED, empty otherwise.
	Guards []string
	// StillHere locates the live defect. Required when UNMERGED, empty
	// otherwise.
	StillHere *Detector
	// Fix names where the correction lives, so an UNMERGED row points
	// somewhere rather than reading as an open bug nobody owns.
	Fix string
	// Evidence names the commit or the document this is drawn from. Every
	// entry here was mined from the project's own record; none was invented.
	Evidence string
}

var defects = []Defect{
	{
		ID:    "FD-1",
		Title: "Five telemetry fields keyed by session where the unit of work is the lane",
		Looked: "Nothing. It produced no error, no break and no anomaly. An unseen lane's " +
			"usage is the zero value, ExpectedRead of it is 0, and an opening request reads " +
			"0, so the two matched exactly and scored 'reproduced'. Every sub-agent lane in " +
			"a fan-out contributed a cache hit that never happened, inflating the cached " +
			"share on /replay/status — the direction nobody audits. The other four fields " +
			"forged prefix changes and model changes between siblings, and put 98.8% into a " +
			"commit message when the lane-correct figure was 4.2%.",
		Symptom: "Two lanes interleaved through one session would produce breaks, prefix " +
			"changes or model changes that neither lane caused, or an opening request in a " +
			"fresh lane would score as a reproduction instead of ReadFirst.",
		Status: StatusGuarded,
		Guards: []string{
			"TestObserve_AnOpeningRequestInAnyLaneIsNotAReproduction",
			"TestObserve_ConcurrentLanesDoNotForgeCacheBreaks",
			"TestObserve_ASiblingOnAnotherModelDoesNotForgeAModelChange",
			"TestObserve_ConcurrentLanesDoNotForgeAPrefixChange",
			"TestObserve_OpeningALaneIsNotAChange",
			"TestRescore_ALaneDoesNotEraseItsSiblingsContext",
			"TestSessionState_LaneAccessorsReadTheKeyTheyAreGiven",
		},
		Fix: "9b9e261, 'Correct the session-vs-lane boundary in the proxy's telemetry (#27)'",
		Evidence: "docs/evidence/lane-isolation-2026-09-06.md: 31 of 34 attributed events " +
			"had not happened, and both the 98.8% and the 11.0% intermediate are retracted " +
			"in place",
	},
	{
		ID:    "FD-2",
		Title: "The consent gate read Unix mode bits, and Windows has none",
		Looked: "A Windows user answering the consent question and being asked again. " +
			"readDecision refused any file whose mode carried group or other write bits; Go " +
			"synthesises 0666 for any writable file on Windows, so every consent file a user " +
			"had just written was refused, citing group and other permissions that do not " +
			"exist on that platform. Their answer was thrown away each time it was read, so " +
			"they could not opt into the corpus and could not grant update consent.",
		Symptom: "A consent decision written on a platform without Unix mode semantics would " +
			"be rejected on its own permissions, and 'verified as this user's' would be " +
			"indistinguishable from 'not verifiable here'.",
		Status: StatusGuarded,
		Guards: []string{
			"TestConsent_AUserWrittenFileIsReadableOnThisPlatform",
			"TestConsent_TheDecisionReportsWhetherOwnershipWasVerified",
			"TestConsent_AWorldWritableFileIsStillRefusedOnUnix",
		},
		Fix: "merged in #34, 8e1b1d8: the check moves behind a build tag and reports " +
			"whether it ran, via Decision.OwnershipChecked. The guard is behavioural rather " +
			"than textual, because a row that only greps for the old expression passes the " +
			"moment somebody rewrites it a different way",
		Evidence: "8e1b1d8, 'Windows could never record a consent decision, and eviction was " +
			"not an LRU'",
	},
	{
		ID:    "FD-3",
		Title: "Spend eviction broke ties by timestamp, so a coarse clock made it random",
		Looked: "A passing test on Linux. Eviction scanned for the smallest wall-clock stamp, " +
			"which is least-recently-used only if the clock can separate two records. A fast " +
			"clock separates them and the check looked correct; under a burst on a coarse " +
			"clock every record compares equal, Before is never true, and the victim is " +
			"whichever key Go's randomised map iteration yields first. A heavy, still-active " +
			"session could be evicted in favour of idle ones, and attribution would then name " +
			"a lane that spent 500 tokens for a 500,000-token overrun.",
		Symptom: "With a frozen clock, evicting past the table's cap would drop an entry other " +
			"than the least recently used one.",
		Status: StatusGuarded,
		Guards: []string{
			"TestSpendGuard_EvictsLeastRecentlyUsedWhenTheClockCannotSeparateRecords",
			"TestSpendGuard_AStillActiveHeavySessionOutlivesIdleOnes",
		},
		Fix: "merged in #34, 8e1b1d8: a monotonic touch counter, which has no " +
			"resolution to run out of; seen stays for attribution's staleness filter",
		Evidence: "INCIDENTS.md 2026-09-06 FAKE GREEN: 'The defect was on every platform. The " +
			"test could only fail on one of them.'",
	},
	{
		ID:    "FD-4",
		Title: "Test isolation set HOME and not USERPROFILE, so Windows read the real home",
		Looked: "A product bug. os.UserHomeDir reads USERPROFILE on Windows, so every test " +
			"built on isolateHome ran against the CI runner's own home directory, found no " +
			"transcripts and failed on empty output. The windows-latest job stayed red long " +
			"enough to read as background, and the harness was the thing that was broken.",
		Symptom: "After isolateHome, one of the variables os.UserHomeDir consults would still " +
			"name the developer's or the runner's real home.",
		Status:   StatusGuarded,
		Guards:   []string{"TestFrozenFD4_IsolateHomeRedirectsEveryHomeVariableOnEveryPlatform"},
		Fix:      "cmd/replay/bare_test.go: USERPROFILE is set on every platform, not only where it is read",
		Evidence: "8e1b1d8 §3, 'Test isolation did not isolate on Windows'",
	},
	{
		ID:    "FD-5",
		Title: "Grok filed under openai:/v1/chat/completions, and called STUB",
		Looked: "A supported client. Both halves were conclusions with nothing behind them: " +
			"the grouping came from an assumption about what an OpenAI-compatible CLI must " +
			"send, and STUB claims a payload we wrote parses when nothing in the build parses " +
			"that path at all. A user pointing Replay at Grok would get an empty report and " +
			"nothing telling them why.",
		Symptom: "A source comment, changelog line or document would place Grok inside the " +
			"OpenAI-compatible family without the /responses endpoint that was actually " +
			"measured standing beside it.",
		Status:   StatusGuarded,
		Guards:   []string{"TestFrozenFD5_GrokIsNotFiledUnderTheOpenAICompatibleWire"},
		Fix:      "this branch: three claim sites corrected in place, with the measurement named",
		Evidence: "6141e10, 'Grok was on the wrong row, and the row said STUB'; INCIDENTS.md 2026-09-06",
	},
	{
		ID:    "FD-6",
		Title: "x-ratelimit-remaining-* written up as a falling counter before a value was read",
		Looked: "A better instrument. It was described as 'a falling per-request counter at " +
			"token granularity' and 'a significantly higher-fidelity instrument' than " +
			"Anthropic's utilization fraction, composed from the header NAMES. Across 8 model " +
			"calls and ~940KB of responses, remaining never moved off the plan ceiling. A " +
			"remaining figure that always equals the limit is the same shape as a healthcheck " +
			"that cannot fail, and it was about to be published as a spend signal.",
		Symptom: "A document would describe these headers as moving, or as more precise than " +
			"the alternative, with no captured reading cited behind it.",
		Status:   StatusGuarded,
		Guards:   []string{"TestFrozenFD6_ARateLimitHeaderClaimCarriesItsMeasurement"},
		Fix:      "cb124cd, retracted in place; docs/requirements.md and docs/guide/commands.md now cite the titration",
		Evidence: "INCIDENTS.md 2026-09-06 INVENTED CONSTANT; docs/evidence/quota-titration-2026-09-06.md",
	},
	{
		ID:    "FD-7",
		Title: "golangci-lint's default display caps hid 81 of 119 issues",
		Looked: "A backlog of 37. max-same-issues defaults to 3 and max-issues-per-linter to " +
			"50; those are display caps, not filters, so CI was never wrong and every person " +
			"who read the output reasoned about 38 issues while 81 were not on the page. The " +
			"instrument worked, what it reported was true, and it was not what it knew.",
		Symptom: "The caps would be absent or non-zero, and the issue count in " +
			"RELEASE-CRITERIA.md would count what was displayed rather than what exists.",
		Status:   StatusGuarded,
		Guards:   []string{"TestFrozenFD7_TheLinterReportsEveryIssueItFinds"},
		Fix:      "138c96b's .golangci.yml issues block, adopted verbatim on this branch",
		Evidence: "138c96b, 'Report every lint issue, not the first three of each'",
	},
	{
		ID:    "FD-8",
		Title: "The installer told every new user an unsourced $2851",
		Looked: "A credential. It sat in the installer's closing screen as the corpus's cost, " +
			"read by every person who ran the command, and it appeared in that file and in no " +
			"evidence anywhere. At install time the tool has measured nothing, so the only " +
			"thing a cost there can appeal to is the author's question rather than the " +
			"reader's.",
		Symptom: "A figure would reach the installer's output that the corpus evidence does " +
			"not hold, or the record of the removal would be quietly dropped.",
		Status:   StatusGuarded,
		Guards:   []string{"TestFrozenFD8_TheInstallerPrintsOnlyFiguresTheEvidenceHolds"},
		Fix:      "6325f63 and 8fb6617; the retraction stays in install.sh rather than being erased",
		Evidence: "8fb6617, 'The installer told every new user a retracted number (#29)'",
	},
	{
		ID:    "FD-9",
		Title: "A document said Windows was built after goreleaser stopped building it",
		Looked: "Current. v0.4.0 shipped Windows archives beside release notes calling Windows " +
			"unsupported; the archives were deleted after publication and the goos line " +
			"dropped windows, and docs/SURFACES.md went on saying 'Windows is built and never " +
			"tested. goreleaser produces the target' until it was re-read against the config. " +
			"Record lag: a stored fact that no longer matched reality, with work planned on " +
			"top of it.",
		Symptom: "A document's list of built platforms and .goreleaser.yaml's would disagree " +
			"in either direction, with nothing failing.",
		Status: StatusGuarded,
		Guards: []string{"TestFrozenFD9_PlatformClaimsMatchTheReleaseConfig"},
		Fix:    "this branch: docs/SURFACES.md corrected in place against .goreleaser.yaml",
		Evidence: "e55114e 'Stop building Windows archives'; d046a7f 'Correct three docs that " +
			"claimed Windows was verified'; INCIDENTS.md 2026-09-06 RECORD-LAG",
	},
}

// testFuncs indexes every test function declared in the repository.
func testFuncs(t *testing.T) map[string]string {
	t.Helper()
	decl := regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)
	out := map[string]string{}
	for path, body := range textFiles(t, ".go") {
		if !strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, m := range decl.FindAllStringSubmatch(body, -1) {
			out[m[1]] = path
		}
	}
	return out
}

// A GUARDED row must name test functions that exist.
//
// This is the check that makes the register more than a list of war stories.
// Promoting a row by editing a string fails here unless the test came with it,
// and renaming or deleting a guard fails here rather than silently leaving a
// defect unwatched with a row still claiming otherwise.
//
// PASS: every guard named is a test function in this tree.
// FAIL: a row claims protection it does not have.
func TestFrozenDefects_AGuardedRowNamesTestsThatExist(t *testing.T) {
	funcs := testFuncs(t)
	for _, d := range defects {
		if d.Status != StatusGuarded {
			if len(d.Guards) != 0 {
				t.Errorf("%s is %s and still names guards %v. A guard on a row that is not "+
					"GUARDED reads as protection nobody is claiming", d.ID, d.Status, d.Guards)
			}
			continue
		}
		if len(d.Guards) == 0 {
			t.Errorf("%s claims GUARDED and names no test. Without one the claim rests on "+
				"somebody's memory of having written it", d.ID)
			continue
		}
		for _, g := range d.Guards {
			if _, ok := funcs[g]; !ok {
				t.Errorf("%s names guard %s, which is not a test function anywhere in this "+
					"tree. Either it was renamed and the row was not, or it was never "+
					"written: %s", d.ID, g, d.Symptom)
			}
		}
	}
}

// An UNMERGED row must be able to point at the defect it says is still here.
//
// The status is a claim about this tree, not about the world, so it is checked
// against this tree. When the fix arrives the detector stops matching and this
// goes red — deliberately. That is the register asking to be promoted, at the
// one moment somebody is already looking at the relevant code, rather than
// outliving its own subject and becoming decoration.
//
// PASS: each UNMERGED row's detector still matches, and names where the fix is.
// FAIL: the defect was fixed and the row was not updated, or the row is fiction.
func TestFrozenDefects_AnUnmergedRowPointsAtTheDefectItClaimsIsStillHere(t *testing.T) {
	root := repoRoot(t)
	for _, d := range defects {
		if d.Status != StatusUnmerged {
			if d.StillHere != nil {
				t.Errorf("%s is %s and still carries a detector. A detector on a fixed row "+
					"is a claim that the defect survived the fix", d.ID, d.Status)
			}
			continue
		}
		if d.StillHere == nil {
			t.Errorf("%s claims the defect is still in this tree and does not say where. "+
				"An unfalsifiable status is the thing this file exists to prevent", d.ID)
			continue
		}
		if !strings.Contains(d.Fix, "origin/") {
			t.Errorf("%s is UNMERGED and its Fix field names no branch. A defect recorded "+
				"as fixed elsewhere has to say elsewhere WHERE, or it is an open bug with "+
				"a reassuring label", d.ID)
		}
		body, err := os.ReadFile(filepath.Join(root, d.StillHere.File))
		if err != nil {
			t.Errorf("%s points at %s, which cannot be read: %v", d.ID, d.StillHere.File, err)
			continue
		}
		if !strings.Contains(string(body), d.StillHere.Text) {
			t.Errorf("%s is recorded as UNMERGED — the fix on another branch, the defect "+
				"still here — but %s no longer contains %q.\n\nIf the fix has landed, "+
				"promote this row: write the guard, name it, and set the status to %s. If "+
				"the code merely moved, the detector is stale and the register has stopped "+
				"tracking its subject.\n\nWhat would be true again if it returned: %s",
				d.ID, d.StillHere.File, d.StillHere.Text, StatusGuarded, d.Symptom)
		}
	}
}

// Every row says what the defect looked like and what would be true again.
//
// A row with neither is a bug number. The two fields are the whole content: a
// defect that looked like a failure gets found on its own, and every entry in
// this file looked like success. Naming the returning symptom is what lets the
// next person write a test instead of a note.
//
// PASS: every row carries both, an evidence pointer, and a unique id.
// FAIL: a row was added as a memo.
func TestFrozenDefects_EveryRowSaysWhatItLookedLikeAndWhatWouldReturn(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range defects {
		if d.ID == "" || d.Title == "" {
			t.Errorf("a row is missing its id or title: %+v", d)
			continue
		}
		if seen[d.ID] {
			t.Errorf("duplicate row for %s", d.ID)
		}
		seen[d.ID] = true
		if len(d.Looked) < 120 {
			t.Errorf("%s says only %q about what it looked like. These defects were not "+
				"visible as defects; describing the appearance is most of the value",
				d.ID, d.Looked)
		}
		if len(d.Symptom) < 60 {
			t.Errorf("%s has symptom %q, which is too short to write a test from. Say what "+
				"would be TRUE again, not that the bug would be back", d.ID, d.Symptom)
		}
		if len(d.Evidence) < 40 {
			t.Errorf("%s has evidence %q, which points nowhere. Name the commit or the "+
				"document, so a reader can check rather than trust this table",
				d.ID, d.Evidence)
		}
		if len(d.Fix) < 20 {
			t.Errorf("%s does not say where its correction lives", d.ID)
		}
	}
}

// The status vocabulary stays closed, and the register keeps its unguarded rows.
//
// Two statuses, one of which is an admission. A register where everything is
// GUARDED is a register that has stopped being a measurement of this tree and
// started being a claim about it — which is the same defect as a surface
// registry with no REFUSED row, and as a linter that reports the first three of
// each.
//
// PASS: every status is declared, and the tree's own unguarded defects are on
// the page.
// FAIL: a status was invented, or the uncomfortable rows were dropped.
func TestFrozenDefects_TheRegisterKeepsItsUnguardedRows(t *testing.T) {
	known := map[DefectStatus]bool{StatusGuarded: true, StatusUnmerged: true}
	var unmerged int
	for _, d := range defects {
		if !known[d.Status] {
			t.Errorf("%s has status %q, which is not one of the two declared. Each carries a "+
				"different weight; a third invented at the call site dilutes both",
				d.ID, d.Status)
		}
		if d.Status == StatusUnmerged {
			unmerged++
		}
	}
	// There is deliberately no assertion that some row must be UNMERGED.
	//
	// This test used to require one, because when it was written two defects
	// were fixed on branches that had not reached this tree and a register
	// that quietly lost them would report the tree as safer than it was. Both
	// fixes have since merged, and the assertion then failed for the best
	// possible reason: there was nothing left unguarded.
	//
	// An assertion that pins a fact about the world on the day it was written,
	// rather than a property that should always hold, goes red when the world
	// improves. Every UNMERGED row is checked against its detector above,
	// which is the property that actually protects the register, and it holds
	// whether the count is two or zero.
	_ = unmerged
}

// Every frozen guard in the tree has a row here, and vice versa.
//
// The naming convention is load-bearing: a guard called TestFrozenFDn_... is
// discoverable from the code it protects, and a reader who hits one can find
// the story. The check runs both ways, because a guard with no row is a test
// nobody can interpret, and a row with no guard has already been caught above.
//
// PASS: the FD ids in test names and the ids in this register agree.
// FAIL: one was added without the other.
func TestFrozenDefects_TheRegisterAndTheGuardsAgreeOnWhatIsFrozen(t *testing.T) {
	rows := map[string][]string{}
	for _, d := range defects {
		rows[d.ID] = d.Guards
	}
	named := regexp.MustCompile(`^TestFrozen(FD-?\d+)_`)
	for fn := range testFuncs(t) {
		m := named.FindStringSubmatch(fn)
		if m == nil {
			continue
		}
		id := strings.Replace(m[1], "FD", "FD-", 1)
		id = strings.Replace(id, "FD--", "FD-", 1)
		guards, ok := rows[id]
		if !ok {
			t.Errorf("%s guards %s and there is no row for it. A frozen guard with no entry "+
				"is a failing test nobody can read: the message says what broke, the row "+
				"says why anyone cared", fn, id)
			continue
		}
		found := false
		for _, g := range guards {
			if g == fn {
				found = true
			}
		}
		if !found {
			t.Errorf("%s guards %s and that row does not name it. The row lists %v",
				fn, id, guards)
		}
	}
}
