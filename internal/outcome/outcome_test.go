package outcome

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The outcome scorer judges one fixed task: count the `func Test` lines in
// cmd/replay/b*_test.go at commit 6871022, the substrate the 2026-09-25
// output-discipline runs read (docs/evidence/output-discipline-2026-09-25.md).
// Truth is derived from bytes proven identical to that commit's tree, never
// taken from the producer or from the manifest alone.

func needModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("executable bits are not observable on Windows; TreeID refuses there")
	}
}

type spec struct {
	body string
	exec bool
}

func writeTree(t *testing.T, root string, files map[string]spec) {
	t.Helper()
	for name, s := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if s.exec {
			mode = 0o755
		}
		if err := os.WriteFile(p, []byte(s.body), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
}

// A small substrate shaped like the real one: 3 test functions under the glob,
// one decoy outside it, one indented decoy and one executable file.
func substrate() map[string]spec {
	return map[string]spec{
		"cmd/replay/budget_test.go":  {body: "package main\n\nfunc TestA(t *testing.T) {}\nfunc TestB(t *testing.T) {}\n\t// func TestNotAtLineStart\n"},
		"cmd/replay/blame_test.go":   {body: "package main\nfunc TestC(t *testing.T) {}\n"},
		"cmd/replay/cost_test.go":    {body: "func TestOutsideTheGlob(t *testing.T) {}\n"},
		"cmd/replay/sub/b_x_test.go": {body: "func TestInASubdirectory(t *testing.T) {}\n"},
		"scripts/run.sh":             {body: "#!/bin/sh\n", exec: true},
		"README.md":                  {body: "x\n"},
	}
}

type fixture struct {
	m    Manifest
	raw  []byte
	tree string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	needModes(t)
	tree := filepath.Join(t.TempDir(), "substrate")
	writeTree(t, tree, substrate())
	id, err := TreeID(tree)
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{
		Schema: ManifestSchema, TaskID: "synthetic-v1", Prompt: "count",
		SubstrateCommit: strings.Repeat("a", 40), SubstrateTree: id,
		Glob: "cmd/replay/b*_test.go", ExpectedAnswer: 3, AnswerRule: "leading_integer",
		RequiredEvidence: []string{"run.json", "tree/"}, Source: "synthetic",
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{m: m, raw: raw, tree: tree}
}

func copyTree(t *testing.T, from, to string) {
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

// run writes a run directory: run.json from fields (a map, so a test can add
// fields the schema does not have) and tree/ copied from the substrate.
func (f fixture) run(t *testing.T, edit func(map[string]any)) string {
	t.Helper()
	dir := t.TempDir()
	copyTree(t, f.tree, filepath.Join(dir, "tree"))
	fields := map[string]any{
		"schema": RunSchema, "task_id": f.m.TaskID, "manifest_sha256": ManifestDigest(f.raw),
		"substrate_commit": f.m.SubstrateCommit, "arm": "control", "reply": "3", "exit_code": 0,
		"usage": map[string]any{"input_tokens": 1200, "output_tokens": 4},
	}
	if edit != nil {
		edit(fields)
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

func hasReason(r Result, code string) bool {
	for _, c := range r.Reasons {
		if c == code {
			return true
		}
	}
	return false
}

func TestPinnedManifestIsTheMeasuredTask(t *testing.T) {
	m, raw, err := Pinned()
	if err != nil {
		t.Fatal(err)
	}
	if m.SubstrateCommit != "6871022744cf99e5bd9a8090706f2f369ca29f16" ||
		m.SubstrateTree != "188abd59a95d7c68e7f5ba4ba1fb14ee88039dec" ||
		m.Glob != "cmd/replay/b*_test.go" || m.ExpectedAnswer != 60 || m.TaskID != "count-b-tests-v1" {
		t.Errorf("pinned manifest %+v", m)
	}
	if !strings.Contains(m.Prompt, "lines starting with `func Test`") || !strings.HasSuffix(m.Prompt, "Reply with only the number.") {
		t.Errorf("prompt %q is not the documented task", m.Prompt)
	}
	sum := sha256.Sum256(raw)
	if ManifestDigest(raw) != hex.EncodeToString(sum[:]) || !bytes.Equal(raw, countBTestsManifest) {
		t.Errorf("digest %q does not bind the embedded bytes", ManifestDigest(raw))
	}
}

func TestDeriveAnswerCountsOnlyLineStartsUnderTheGlob(t *testing.T) {
	needModes(t)
	f := newFixture(t)
	n, err := DeriveAnswer(f.tree, f.m.Glob)
	if err != nil || n != 3 {
		t.Fatalf("derived %d, %v; want 3 (2 + 1 under the glob, decoys excluded)", n, err)
	}
}

func TestCorrectAnswerOnTheUntouchedTreePasses(t *testing.T) {
	f := newFixture(t)
	r := Score(f.m, f.raw, f.run(t, nil))
	if r.Verdict != Pass || len(r.Reasons) != 0 || r.Answer == nil || *r.Answer != 3 || r.Truth == nil || *r.Truth != 3 || r.Arm != "control" {
		t.Fatalf("result %+v", r)
	}
}

func TestWrongOrUnreadableAnswerFails(t *testing.T) {
	f := newFixture(t)
	for reply, code := range map[string]string{"4": "wrong_answer", "2 functions": "wrong_answer", "three": "answer_not_a_number", "**3**": "answer_not_a_number"} {
		r := Score(f.m, f.raw, f.run(t, func(m map[string]any) { m["reply"] = reply }))
		if r.Verdict != Fail || !hasReason(r, code) {
			t.Errorf("reply %q: %+v, want FAIL %s", reply, r, code)
		}
	}
}

func TestATamperedTreeFailsEvenWithTheCorrectAnswer(t *testing.T) {
	f := newFixture(t)
	for name, tamper := range map[string]func(string){
		"edited file outside the glob": func(tree string) {
			_ = os.WriteFile(filepath.Join(tree, "README.md"), []byte("changed\n"), 0o644)
		},
		"added file":   func(tree string) { _ = os.WriteFile(filepath.Join(tree, "new.txt"), []byte("x"), 0o644) },
		"removed file": func(tree string) { _ = os.Remove(filepath.Join(tree, "cmd/replay/cost_test.go")) },
		"mode change":  func(tree string) { _ = os.Chmod(filepath.Join(tree, "README.md"), 0o755) },
	} {
		dir := f.run(t, nil)
		tamper(filepath.Join(dir, "tree"))
		r := Score(f.m, f.raw, dir)
		if r.Verdict != Fail || !hasReason(r, "result_tree_modified") {
			t.Errorf("%s: %+v, want FAIL result_tree_modified", name, r)
		}
	}
}

func TestMissingOrInsufficientEvidenceIsUnavailable(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name string
		edit func(map[string]any)
		dir  func(string)
		code string
	}{
		{"no reply", func(m map[string]any) { delete(m, "reply") }, nil, "answer_missing"},
		{"blank reply", func(m map[string]any) { m["reply"] = "  \n" }, nil, "answer_missing"},
		{"no exit code", func(m map[string]any) { delete(m, "exit_code") }, nil, "exit_code_missing"},
		{"failed execution", func(m map[string]any) { m["exit_code"] = 1 }, nil, "execution_failed"},
		{"no usage", func(m map[string]any) { delete(m, "usage") }, nil, "usage_missing"},
		{"no input tokens", func(m map[string]any) { m["usage"] = map[string]any{"output_tokens": 4} }, nil, "usage_missing"},
		{"negative tokens", func(m map[string]any) { m["usage"] = map[string]any{"input_tokens": -1, "output_tokens": 4} }, nil, "usage_invalid"},
		{"wrong task", func(m map[string]any) { m["task_id"] = "other" }, nil, "provenance_task_mismatch"},
		{"no manifest digest", func(m map[string]any) { delete(m, "manifest_sha256") }, nil, "provenance_manifest_mismatch"},
		{"other manifest", func(m map[string]any) { m["manifest_sha256"] = strings.Repeat("0", 64) }, nil, "provenance_manifest_mismatch"},
		{"other substrate", func(m map[string]any) { m["substrate_commit"] = strings.Repeat("b", 40) }, nil, "provenance_substrate_mismatch"},
		{"other schema", func(m map[string]any) { m["schema"] = "replay.outcome.run.v0" }, nil, "run_schema_unsupported"},
		{"no tree", nil, func(dir string) { _ = os.RemoveAll(filepath.Join(dir, "tree")) }, "result_tree_missing"},
		{"no run record", nil, func(dir string) { _ = os.Remove(filepath.Join(dir, "run.json")) }, "run_record_missing"},
	}
	for _, c := range cases {
		dir := f.run(t, c.edit)
		if c.dir != nil {
			c.dir(dir)
		}
		r := Score(f.m, f.raw, dir)
		if r.Verdict != Unavailable || !hasReason(r, c.code) {
			t.Errorf("%s: %+v, want UNAVAILABLE %s", c.name, r, c.code)
		}
	}
}

// The evidence is the run directory's own run.json and tree/. A symlink in
// either place points the scorer at bytes the run did not leave, here a
// pristine copy of the substrate and another run's valid record, so neither is
// followed to establish trust: the run is UNAVAILABLE, never scored from the
// external bytes. A symlink inside tree/ is content, hashed as git hashes a
// link, so replacing a directory with a link to a clean copy is a modified tree.
// These checks are made once, by path; they do not make the run directory
// immune to a change between the check and the read.
func TestSymlinkedEvidenceIsNotFollowed(t *testing.T) {
	f := newFixture(t)
	clean := filepath.Join(t.TempDir(), "clean")
	copyTree(t, f.tree, clean)

	linkedTree := f.run(t, nil)
	if err := os.RemoveAll(filepath.Join(linkedTree, "tree")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(clean, filepath.Join(linkedTree, "tree")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	r := Score(f.m, f.raw, linkedTree)
	if r.Verdict != Unavailable || len(r.Reasons) != 1 || r.Reasons[0] != "result_tree_symlink" || r.Truth != nil || r.TreeID != "" {
		t.Errorf("tree/ linked to a clean copy: %+v, want UNAVAILABLE [result_tree_symlink] with no truth or tree id", r)
	}

	other := f.run(t, nil)
	linkedRecord := f.run(t, nil)
	if err := os.Remove(filepath.Join(linkedRecord, "run.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(other, "run.json"), filepath.Join(linkedRecord, "run.json")); err != nil {
		t.Fatal(err)
	}
	r = Score(f.m, f.raw, linkedRecord)
	if r.Verdict != Unavailable || len(r.Reasons) != 1 || r.Reasons[0] != "run_record_symlink" || r.Truth != nil {
		t.Errorf("run.json linked to another run's record: %+v, want UNAVAILABLE [run_record_symlink]", r)
	}

	nested := f.run(t, nil)
	sub := filepath.Join(nested, "tree", "cmd")
	if err := os.RemoveAll(sub); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(clean, "cmd"), sub); err != nil {
		t.Fatal(err)
	}
	r = Score(f.m, f.raw, nested)
	if r.Verdict != Fail || len(r.Reasons) != 1 || r.Reasons[0] != "result_tree_modified" || r.Truth != nil {
		t.Errorf("tree/cmd linked to a clean copy: %+v, want FAIL [result_tree_modified]", r)
	}

	if r := Score(f.m, f.raw, f.run(t, nil)); r.Verdict != Pass {
		t.Errorf("the control run with no links: %+v, want PASS", r)
	}
}

// A run record cannot carry the truth or the verdict: the scorer derives both.
func TestTheProducerCannotSupplyTruthOrVerdict(t *testing.T) {
	f := newFixture(t)
	for _, field := range []string{"expected_answer", "truth", "verdict"} {
		r := Score(f.m, f.raw, f.run(t, func(m map[string]any) { m[field] = "PASS"; m["reply"] = "999" }))
		if r.Verdict != Unavailable || !hasReason(r, "run_record_invalid") {
			t.Errorf("run with %q: %+v, want UNAVAILABLE run_record_invalid", field, r)
		}
	}
	// Nor can the manifest stand in for the derivation: a manifest whose stated
	// answer disagrees with the verified tree is not scored at all.
	m := f.m
	m.ExpectedAnswer = 4
	raw, _ := json.Marshal(m)
	r := Score(m, raw, f.run(t, func(x map[string]any) { x["manifest_sha256"] = ManifestDigest(raw); x["reply"] = "4" }))
	if r.Verdict != Unavailable || !hasReason(r, "manifest_truth_mismatch") {
		t.Errorf("manifest stating 4 over a tree holding 3: %+v", r)
	}
}

func TestParseManifestRefusesWhatItCannotPin(t *testing.T) {
	_, raw, err := Pinned()
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"unknown field":  func(m map[string]any) { m["truth_override"] = 1 },
		"schema":         func(m map[string]any) { m["schema"] = "v0" },
		"short commit":   func(m map[string]any) { m["substrate_commit"] = "6871022" },
		"short even hex": func(m map[string]any) { m["substrate_tree"] = "188abd59" },
		"tree":           func(m map[string]any) { m["substrate_tree"] = "XYZ" },
		"glob":           func(m map[string]any) { m["glob"] = "" },
		"answer rule":    func(m map[string]any) { m["answer_rule"] = "contains" },
	} {
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		mutate(fields)
		b, _ := json.Marshal(fields)
		if _, err := ParseManifest(b); err == nil {
			t.Errorf("%s: manifest accepted", name)
		}
	}
}

func results(verdicts ...Verdict) []Result {
	var out []Result
	for _, v := range verdicts {
		out = append(out, Result{Verdict: v})
	}
	return out
}

func TestCompareRefusesACeiling(t *testing.T) {
	treat := results(Pass, Fail, Pass)
	for name, c := range map[string]struct {
		control []Result
		want    ComparisonStatus
	}{
		"0/n":                {results(Fail, Fail, Fail, Unavailable), RefusedCeiling},
		"n/n":                {results(Pass, Pass, Pass, Pass), RefusedCeiling},
		"not a ceiling":      {results(Pass, Fail, Pass, Unavailable), Comparable},
		"nothing admissible": {results(Unavailable, Unavailable), Insufficient},
	} {
		got := Compare(c.control, treat)
		if got.Status != c.want {
			t.Errorf("%s: %+v, want %s", name, got, c.want)
		}
		if got.Status == RefusedCeiling && !strings.Contains(got.Note, "not evidence of no effect") {
			t.Errorf("%s: refusal note %q must say a ceiling is not evidence of no effect", name, got.Note)
		}
	}
	got := Compare(results(Pass, Fail, Unavailable), treat)
	if got.Control != (ArmCount{Pass: 1, N: 2, Unavailable: 1}) || got.Treatment != (ArmCount{Pass: 2, N: 3}) {
		t.Errorf("counts %+v / %+v: UNAVAILABLE runs are counted apart, never in n", got.Control, got.Treatment)
	}
	if Compare(results(Pass, Fail), nil).Status != Insufficient {
		t.Error("an empty treatment arm was compared")
	}
}

func TestTreeIDRefusesOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	if _, err := TreeID(t.TempDir()); !errors.Is(err, errModeUnobservable) {
		t.Fatalf("err = %v, want errModeUnobservable", err)
	}
}

// What git would record differently, or cannot record, is refused rather than
// hashed: a nested repository (git stores a gitlink) and special files.
func TestTreeIDRefusesWhatGitWouldNotHashAsContent(t *testing.T) {
	needModes(t)
	root := t.TempDir()
	writeTree(t, root, map[string]spec{"sub/.git/HEAD": {body: "ref: refs/heads/main\n"}, "sub/f": {body: "x"}})
	if _, err := TreeID(root); err == nil || !strings.Contains(err.Error(), "nested git repository") {
		t.Errorf("nested repository hashed: %v", err)
	}
	// And the top-level .git, which is working-copy metadata, is excluded.
	root = t.TempDir()
	writeTree(t, root, map[string]spec{"f": {body: "x"}})
	before, _ := TreeID(root)
	writeTree(t, root, map[string]spec{".git/HEAD": {body: "anything"}})
	if after, err := TreeID(root); err != nil || after != before {
		t.Errorf("top-level .git changed the id: %s -> %s (%v)", before, after, err)
	}
}

// A result tree the scorer cannot hash is UNAVAILABLE, not FAIL: nothing was
// shown to be modified.
func TestAnUnhashableResultTreeIsUnavailable(t *testing.T) {
	f := newFixture(t)
	dir := f.run(t, nil)
	writeTree(t, filepath.Join(dir, "tree"), map[string]spec{"vendor/x/.git/HEAD": {body: "x"}})
	r := Score(f.m, f.raw, dir)
	if r.Verdict != Unavailable || !hasReason(r, "result_tree_unverifiable") {
		t.Errorf("result %+v", r)
	}
}

// Read failures are refused, not hashed or counted around.
func TestUnreadableEntriesAreRefused(t *testing.T) {
	needModes(t)
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not deny reads")
	}
	lock := func(t *testing.T, p string) {
		t.Helper()
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o755) })
	}
	root := t.TempDir()
	writeTree(t, root, map[string]spec{"dir/f": {body: "x"}})
	lock(t, filepath.Join(root, "dir"))
	if _, err := TreeID(root); err == nil {
		t.Error("TreeID hashed around an unreadable directory")
	}
	if _, err := DeriveAnswer(root, "dir/*"); err == nil {
		t.Error("DeriveAnswer counted around an unreadable directory")
	}
	root = t.TempDir()
	writeTree(t, root, map[string]spec{"cmd/replay/b_test.go": {body: "func TestX(t *testing.T) {}\n"}})
	lock(t, filepath.Join(root, "cmd/replay/b_test.go"))
	if _, err := TreeID(root); err == nil {
		t.Error("TreeID hashed an unreadable file")
	}
	if _, err := DeriveAnswer(root, "cmd/replay/b*_test.go"); err == nil {
		t.Error("DeriveAnswer counted an unreadable file as zero")
	}
}
