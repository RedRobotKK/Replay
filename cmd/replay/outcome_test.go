package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/outcome"
)

// outcomeReport is what `replay outcome --json` prints, decoded with pointers
// so a test can tell an absent answer or truth from a zero.
type outcomeReport struct {
	Schema          string `json:"schema"`
	TaskID          string `json:"task_id"`
	ManifestSHA256  string `json:"manifest_sha256"`
	SubstrateCommit string `json:"substrate_commit"`
	SubstrateTree   string `json:"substrate_tree"`
	Runs            []struct {
		RunDir  string   `json:"run_dir"`
		Verdict string   `json:"verdict"`
		Reasons []string `json:"reasons"`
		Arm     string   `json:"arm"`
		Answer  *int     `json:"answer"`
		Truth   *int     `json:"truth"`
		TreeID  string   `json:"tree_id"`
	} `json:"runs"`
	Summary struct {
		Pass        int `json:"pass"`
		Fail        int `json:"fail"`
		Unavailable int `json:"unavailable"`
	} `json:"summary"`
}

func decodeOutcome(t *testing.T, out string) outcomeReport {
	t.Helper()
	var r outcomeReport
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("outcome --json: %v\n%s", err, out)
	}
	return r
}

// outcomeRunDir writes run.json from fields, so a test can add a field the
// schema lacks, and copies tree into tree/ when tree is not empty.
func outcomeRunDir(t *testing.T, fields map[string]any, tree string) string {
	t.Helper()
	dir := t.TempDir()
	if tree != "" {
		copyOutcomeTree(t, tree, filepath.Join(dir, "tree"))
	}
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func copyOutcomeTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chmod(dst, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

// runFields is an admissible run.json for manifest m.
func runFields(m outcome.Manifest, raw []byte, reply string) map[string]any {
	return map[string]any{
		"schema": outcome.RunSchema, "task_id": m.TaskID, "manifest_sha256": outcome.ManifestDigest(raw),
		"substrate_commit": m.SubstrateCommit, "arm": "control", "reply": reply, "exit_code": 0,
		"usage": map[string]any{"input_tokens": 1200, "output_tokens": 4},
	}
}

func needOutcomeModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("TreeID refuses on Windows, where executable bits are not observable")
	}
}

// snapshotDir records every entry under root: path, mode, size, modification
// time and content digest. Two equal snapshots mean nothing was written.
func snapshotDir(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		line := fmt.Sprintf("%s %v %d %d", rel, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			line += " " + hex.EncodeToString(sum[:])
		}
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// A small substrate and a manifest pinned to it, installed in place of the
// embedded manifest for the length of one test. The shipped command has no
// flag or environment variable that does this: the embedded manifest is the
// only one a user can score against.
func withSyntheticManifest(t *testing.T) (outcome.Manifest, []byte, string) {
	t.Helper()
	needOutcomeModes(t)
	tree := filepath.Join(t.TempDir(), "substrate")
	files := map[string]string{
		"cmd/replay/budget_test.go": "package main\n\nfunc TestA(t *testing.T) {}\nfunc TestB(t *testing.T) {}\n",
		"cmd/replay/blame_test.go":  "package main\nfunc TestC(t *testing.T) {}\n",
		"cmd/replay/cost_test.go":   "func TestOutsideTheGlob(t *testing.T) {}\n",
		"README.md":                 "x\n",
	}
	for name, body := range files {
		p := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	id, err := outcome.TreeID(tree)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"schema": outcome.ManifestSchema, "task_id": "synthetic-v1", "prompt": "count",
		"substrate_commit": strings.Repeat("a", 40), "substrate_tree": id,
		"glob": "cmd/replay/b*_test.go", "expected_answer": 3, "answer_rule": "leading_integer",
		"required_evidence": []string{"run.json", "tree/"}, "source": "synthetic",
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := outcome.ParseManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	prev := outcomeManifest
	outcomeManifest = func() (outcome.Manifest, []byte, error) { return m, raw, nil }
	t.Cleanup(func() { outcomeManifest = prev })
	return m, raw, tree
}

// TestE2E_Outcome reaches the command through dispatch with the embedded
// manifest, as the binary does. Three run directories: one that does not
// exist, one whose run record is not JSON, and one with admissible evidence
// whose tree is not the pinned substrate. The first two are UNAVAILABLE, with
// no truth, because nothing was verified; only the third is a FAIL.
func TestE2E_Outcome(t *testing.T) {
	needOutcomeModes(t)
	home := t.TempDir()
	isolateHome(t, home)
	m, raw, err := outcome.Pinned()
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(home, "no-such-run")
	malformed := t.TempDir()
	if err := os.WriteFile(filepath.Join(malformed, "run.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "README.md"), []byte("not the substrate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tampered := outcomeRunDir(t, runFields(m, raw, "60"), other)

	out, errb, err := e2e(t, "outcome", "--json", missing, malformed, tampered)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	r := decodeOutcome(t, out)
	sum := sha256.Sum256(raw)
	if r.Schema != "replay.outcome.report.v1" || r.TaskID != "count-b-tests-v1" ||
		r.ManifestSHA256 != hex.EncodeToString(sum[:]) ||
		r.SubstrateCommit != m.SubstrateCommit || r.SubstrateTree != m.SubstrateTree {
		t.Fatalf("report header does not name the embedded manifest: %+v", r)
	}
	if len(r.Runs) != 3 {
		t.Fatalf("got %d runs, want 3", len(r.Runs))
	}
	want := []struct{ verdict, reason string }{
		{"UNAVAILABLE", "run_record_missing"},
		{"UNAVAILABLE", "run_record_invalid"},
		{"FAIL", "result_tree_modified"},
	}
	for i, w := range want {
		got := r.Runs[i]
		if got.Verdict != w.verdict || len(got.Reasons) != 1 || got.Reasons[0] != w.reason {
			t.Errorf("run %d: %s %v, want %s [%s]", i, got.Verdict, got.Reasons, w.verdict, w.reason)
		}
		if got.Truth != nil {
			t.Errorf("run %d: truth %d reported, but no tree was verified, so none was derived", i, *got.Truth)
		}
	}
	if r.Runs[0].RunDir != missing || r.Runs[2].RunDir != tampered {
		t.Errorf("run directories not reported as given: %q %q", r.Runs[0].RunDir, r.Runs[2].RunDir)
	}
	if r.Summary.Pass != 0 || r.Summary.Fail != 1 || r.Summary.Unavailable != 2 {
		t.Errorf("summary %+v, want 0 PASS, 1 FAIL, 2 UNAVAILABLE", r.Summary)
	}

	human, errb, err := e2e(t, "outcome", missing, tampered)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	mustContain(t, "outcome", human,
		"count-b-tests-v1",
		"UNAVAILABLE  "+missing+"  run_record_missing",
		"FAIL         "+tampered+"  result_tree_modified",
		"0 PASS, 1 FAIL, 1 UNAVAILABLE",
		"UNAVAILABLE is missing or unverifiable evidence, not a failed run")
	if strings.Contains(human, "truth 60") {
		t.Errorf("human report states a truth that was never derived:\n%s", human)
	}

	// The same run named twice is refused before anything is scored.
	var dup, duperr bytes.Buffer
	if err := dispatch([]string{"outcome", tampered, tampered}, &dup, &duperr); !errors.Is(err, errUsage) || dup.Len() != 0 {
		t.Errorf("a run directory named twice: %v, stdout %q; want a usage refusal and nothing scored", err, dup.String())
	}
}

// Every verdict, through dispatch, on a substrate small enough to build here.
func TestOutcomeScoresEveryVerdict(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	pass := outcomeRunDir(t, runFields(m, raw, "3"), tree)
	wrong := outcomeRunDir(t, runFields(m, raw, "4"), tree)
	words := outcomeRunDir(t, runFields(m, raw, "three"), tree)
	failedExec := runFields(m, raw, "3")
	failedExec["exit_code"] = 1
	failed := outcomeRunDir(t, failedExec, tree)
	noTree := outcomeRunDir(t, runFields(m, raw, "3"), "")

	out, errb, err := e2e(t, "outcome", pass, wrong, words, failed, noTree, "--json")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	r := decodeOutcome(t, out)
	want := []struct {
		verdict, reason string
		answer, truth   *int
	}{
		{"PASS", "", intp(3), intp(3)},
		{"FAIL", "wrong_answer", intp(4), intp(3)},
		{"FAIL", "answer_not_a_number", nil, intp(3)},
		{"UNAVAILABLE", "execution_failed", nil, nil},
		{"UNAVAILABLE", "result_tree_missing", nil, nil},
	}
	if len(r.Runs) != len(want) {
		t.Fatalf("got %d runs, want %d", len(r.Runs), len(want))
	}
	for i, w := range want {
		got := r.Runs[i]
		if got.Verdict != w.verdict {
			t.Errorf("run %d: verdict %s, want %s", i, got.Verdict, w.verdict)
		}
		if w.reason == "" && len(got.Reasons) != 0 || w.reason != "" && (len(got.Reasons) != 1 || got.Reasons[0] != w.reason) {
			t.Errorf("run %d: reasons %v, want [%s]", i, got.Reasons, w.reason)
		}
		if !sameIntp(got.Answer, w.answer) || !sameIntp(got.Truth, w.truth) {
			t.Errorf("run %d: answer %v truth %v, want %v %v", i, show(got.Answer), show(got.Truth), show(w.answer), show(w.truth))
		}
		if got.Arm != "control" {
			t.Errorf("run %d: arm %q, want control", i, got.Arm)
		}
	}
	if r.Runs[0].TreeID != m.SubstrateTree {
		t.Errorf("PASS run's tree id %s, want the pinned %s", r.Runs[0].TreeID, m.SubstrateTree)
	}
	if r.Summary.Pass != 1 || r.Summary.Fail != 2 || r.Summary.Unavailable != 2 {
		t.Errorf("summary %+v, want 1 PASS, 2 FAIL, 2 UNAVAILABLE", r.Summary)
	}

	human, _, err := e2e(t, "outcome", pass, wrong, words, failed)
	if err != nil {
		t.Fatal(err)
	}
	// A truth was derived for the unreadable answer, and none for the failed
	// execution, which was never scored: each line says which.
	mustContain(t, "outcome", human,
		"PASS         "+pass+"  answer 3, truth 3 derived from the verified tree",
		"FAIL         "+wrong+"  wrong_answer: answer 4, truth 3 derived from the verified tree",
		"FAIL         "+words+"  answer_not_a_number: no answer read, truth 3 derived from the verified tree",
		"UNAVAILABLE  "+failed+"  execution_failed: no truth derived",
		"1 PASS, 2 FAIL, 1 UNAVAILABLE")
}

// A run that edits the working copy fails, whatever it answered, and a run
// record carrying a truth or a verdict is not admitted at all.
func TestOutcomeRefusesATamperedTreeAndProducerVerdicts(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	tampered := outcomeRunDir(t, runFields(m, raw, "3"), tree)
	if err := os.WriteFile(filepath.Join(tampered, "tree", "README.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	added := outcomeRunDir(t, runFields(m, raw, "3"), tree)
	if err := os.WriteFile(filepath.Join(added, "tree", "notes.txt"), []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	claimed := runFields(m, raw, "3")
	claimed["verdict"] = "PASS"
	claimed["truth"] = 3
	producer := outcomeRunDir(t, claimed, tree)

	out, errb, err := e2e(t, "outcome", "--json", tampered, added, producer)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	r := decodeOutcome(t, out)
	for i, w := range []struct{ verdict, reason string }{
		{"FAIL", "result_tree_modified"},
		{"FAIL", "result_tree_modified"},
		{"UNAVAILABLE", "run_record_invalid"},
	} {
		got := r.Runs[i]
		if got.Verdict != w.verdict || len(got.Reasons) != 1 || got.Reasons[0] != w.reason {
			t.Errorf("run %d: %s %v, want %s [%s]", i, got.Verdict, got.Reasons, w.verdict, w.reason)
		}
		if got.Truth != nil || got.Answer != nil {
			t.Errorf("run %d: answer or truth reported from an unverified tree", i)
		}
	}
	if r.Runs[0].TreeID == "" || r.Runs[0].TreeID == m.SubstrateTree {
		t.Errorf("tampered run's tree id %q should be reported and differ from the pinned tree", r.Runs[0].TreeID)
	}
}

// A run whose tree/ is a symlink to a clean copy elsewhere is scored from
// nothing: the command reports it UNAVAILABLE with no truth, rather than a
// PASS taken from bytes the run did not leave.
func TestOutcomeDoesNotFollowASymlinkedTree(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	dir := outcomeRunDir(t, runFields(m, raw, "3"), "")
	if err := os.Symlink(tree, filepath.Join(dir, "tree")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	out, errb, err := e2e(t, "outcome", "--json", dir)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	r := decodeOutcome(t, out)
	got := r.Runs[0]
	if got.Verdict != "UNAVAILABLE" || strings.Join(got.Reasons, ",") != "result_tree_symlink" || got.Truth != nil || got.Answer != nil {
		t.Errorf("symlinked tree/: %s %v truth %s, want UNAVAILABLE [result_tree_symlink] and no truth", got.Verdict, got.Reasons, show(got.Truth))
	}
	if r.Summary.Pass != 0 || r.Summary.Unavailable != 1 {
		t.Errorf("summary %+v, want 0 PASS, 1 UNAVAILABLE", r.Summary)
	}
}

// Every reason is reported, not just the first, and missing evidence never
// becomes a FAIL: each of these runs has the right answer on an untouched tree.
func TestOutcomeMissingEvidenceIsNeverAFailure(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	cases := []struct {
		reason string
		edit   func(map[string]any)
	}{
		{"exit_code_missing", func(f map[string]any) { delete(f, "exit_code") }},
		{"usage_missing", func(f map[string]any) { delete(f, "usage") }},
		{"answer_missing", func(f map[string]any) { f["reply"] = "  " }},
		{"provenance_manifest_mismatch", func(f map[string]any) { f["manifest_sha256"] = strings.Repeat("0", 64) }},
		{"run_schema_unsupported", func(f map[string]any) { f["schema"] = "replay.outcome.run.v0" }},
	}
	var dirs []string
	for _, c := range cases {
		f := runFields(m, raw, "3")
		c.edit(f)
		dirs = append(dirs, outcomeRunDir(t, f, tree))
	}
	several := runFields(m, raw, "3")
	delete(several, "exit_code")
	delete(several, "usage")
	dirs = append(dirs, outcomeRunDir(t, several, tree))

	out, errb, err := e2e(t, append([]string{"outcome", "--json"}, dirs...)...)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	r := decodeOutcome(t, out)
	for i, c := range cases {
		got := r.Runs[i]
		if got.Verdict != "UNAVAILABLE" || len(got.Reasons) != 1 || got.Reasons[0] != c.reason {
			t.Errorf("%s: %s %v, want UNAVAILABLE [%s]", c.reason, got.Verdict, got.Reasons, c.reason)
		}
	}
	last := r.Runs[len(cases)]
	if last.Verdict != "UNAVAILABLE" || strings.Join(last.Reasons, ",") != "exit_code_missing,usage_missing" {
		t.Errorf("two gaps: %s %v, want UNAVAILABLE [exit_code_missing usage_missing]", last.Verdict, last.Reasons)
	}
	if r.Summary.Fail != 0 || r.Summary.Unavailable != len(dirs) {
		t.Errorf("summary %+v: missing evidence was counted as a failure", r.Summary)
	}
}

// Scoring reads and never writes: every byte, mode and modification time
// under each run directory is the same afterwards, the home directory holds
// nothing new, and nothing is created beside a run directory that is absent.
func TestOutcomeWritesNothing(t *testing.T) {
	m, raw, tree := withSyntheticManifest(t)
	home := t.TempDir()
	isolateHome(t, home)
	pass := outcomeRunDir(t, runFields(m, raw, "3"), tree)
	wrong := outcomeRunDir(t, runFields(m, raw, "4"), tree)
	parent := t.TempDir()
	missing := filepath.Join(parent, "absent")

	before := []string{snapshotDir(t, pass), snapshotDir(t, wrong), snapshotDir(t, home), snapshotDir(t, parent)}
	for _, args := range [][]string{{"outcome", pass, wrong, missing}, {"outcome", "--json", pass, wrong, missing}} {
		if _, errb, err := e2e(t, args...); err != nil {
			t.Fatalf("%v\n%s", err, errb)
		}
	}
	after := []string{snapshotDir(t, pass), snapshotDir(t, wrong), snapshotDir(t, home), snapshotDir(t, parent)}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("snapshot %d changed:\nbefore:\n%s\nafter:\n%s", i, before[i], after[i])
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("the absent run directory now exists: %v", err)
	}
}

// An embedded manifest that does not parse scores nothing: the command fails
// rather than judging runs against a task it cannot pin.
func TestOutcomeRefusesAnInvalidManifest(t *testing.T) {
	prev := outcomeManifest
	outcomeManifest = func() (outcome.Manifest, []byte, error) {
		raw := []byte(`{"schema":"replay.outcome.manifest.v0"}`)
		m, err := outcome.ParseManifest(raw)
		return m, raw, err
	}
	t.Cleanup(func() { outcomeManifest = prev })
	out, _, err := e2e(t, "outcome", "--json", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "embedded outcome manifest is invalid") {
		t.Fatalf("err %v, want the invalid-manifest error", err)
	}
	if out != "" {
		t.Errorf("a report was printed against an invalid manifest:\n%s", out)
	}
}

func TestOutcomeUsage(t *testing.T) {
	if _, _, err := e2e(t, "outcome"); err == nil {
		t.Error("no run directory must be a usage error")
	} else if exitCode(err) != exitUsage {
		t.Errorf("exit %d, want %d", exitCode(err), exitUsage)
	}
	if _, _, err := e2e(t, "outcome", "--manifest", "x.json", t.TempDir()); err == nil {
		t.Error("--manifest must be refused: the embedded manifest is the only one")
	}
	out, _, err := e2e(t, "outcome", "--help")
	if err != errHelpShown {
		t.Fatalf("--help: %v", err)
	}
	mustContain(t, "outcome --help", out, "replay outcome", "run.json", "tree/")
}

func intp(v int) *int { return &v }

func sameIntp(a, b *int) bool { return (a == nil) == (b == nil) && (a == nil || *a == *b) }

func show(p *int) string {
	if p == nil {
		return "none"
	}
	return fmt.Sprint(*p)
}

// One run named twice would be scored twice and counted twice: two PASS for
// one piece of evidence, and an inflated n for anything that later counts
// runs. The invocation is refused before anything is scored. Two directories
// that hold copies of one run are not detected; this is identity by path.
func TestOutcomeRefusesARunDirectoryNamedTwice(t *testing.T) {
	_, _, sub := withSyntheticManifest(t)
	parent := t.TempDir()
	runA := filepath.Join(parent, "a")
	copyOutcomeTree(t, sub, filepath.Join(runA, "tree"))
	link := filepath.Join(parent, "link-to-a")
	if err := os.Symlink(runA, link); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(parent, "absent")
	t.Chdir(parent)
	cases := map[string][]string{
		"the same path":           {runA, runA},
		"a trailing separator":    {runA, runA + string(filepath.Separator)},
		"relative and dot":        {"a", "./a"},
		"relative and absolute":   {"a", runA},
		"through a symbolic link": {runA, link},
		"a missing path twice":    {missing, missing},
		"with --json":             {"--json", runA, runA},
		"apart from another run":  {runA, missing, "a"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var out, errb bytes.Buffer
			err := dispatch(append([]string{"outcome"}, args...), &out, &errb)
			if !errors.Is(err, errUsage) {
				t.Fatalf("outcome %v: %v, want a usage refusal", args, err)
			}
			if !strings.Contains(err.Error(), "same run directory") {
				t.Errorf("the refusal does not say why: %v", err)
			}
			if out.Len() != 0 {
				t.Errorf("something was scored before the refusal:\n%s", out.String())
			}
		})
	}
	t.Run("distinct directories are scored", func(t *testing.T) {
		var out, errb bytes.Buffer
		if err := dispatch([]string{"outcome", "--json", runA, missing, link + "-other"}, &out, &errb); err != nil {
			t.Fatalf("distinct directories refused: %v", err)
		}
		if r := decodeOutcome(t, out.String()); len(r.Runs) != 3 {
			t.Fatalf("%d runs scored, want 3", len(r.Runs))
		}
	})
}
