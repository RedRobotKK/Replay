// Package outcome scores one fixed, offline task: whether a run answered the
// pinned question correctly on a working copy it left untouched.
//
// The truth is never taken from the producer, and not from the manifest
// alone. A result tree is accepted only if its git tree id, computed here from
// the bytes on disk, equals the pinned substrate's; the answer is then counted
// from those verified bytes and must also equal the manifest's stated answer.
// Nothing here runs an agent, estimates an effect or claims task quality.
package outcome

import (
	"bufio"
	"bytes"
	"crypto/sha1" //nolint:gosec // git object ids are SHA-1; this reproduces them, it does not rely on SHA-1 for security
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/outputdiscipline"
)

//go:embed manifests/count-b-tests.v1.json
var countBTestsManifest []byte

// Schemas this package reads. A manifest or run record naming any other is
// refused.
const (
	ManifestSchema = "replay.outcome.manifest.v1"
	RunSchema      = "replay.outcome.run.v1"
)

// Verdict is one run's score.
type Verdict string

// The three verdicts. PASS needs every piece of evidence and the right answer
// on an untouched tree; FAIL needs admissible evidence; anything else is
// UNAVAILABLE.
const (
	Pass        Verdict = "PASS"
	Fail        Verdict = "FAIL"
	Unavailable Verdict = "UNAVAILABLE"
)

// Manifest pins the task.
type Manifest struct {
	Schema           string   `json:"schema"`
	TaskID           string   `json:"task_id"`
	Prompt           string   `json:"prompt"`
	SubstrateCommit  string   `json:"substrate_commit"`
	SubstrateTree    string   `json:"substrate_tree"`
	Glob             string   `json:"glob"`
	ExpectedAnswer   int      `json:"expected_answer"`
	AnswerRule       string   `json:"answer_rule"`
	RequiredEvidence []string `json:"required_evidence"`
	Source           string   `json:"source"`
}

// Usage is the run's reported token usage. Both counts are required.
type Usage struct {
	InputTokens  *int64 `json:"input_tokens"`
	OutputTokens *int64 `json:"output_tokens"`
}

// Run is run.json. It has no field for a truth or a verdict, and unknown
// fields are refused, so a producer cannot supply either.
type Run struct {
	Schema          string  `json:"schema"`
	TaskID          string  `json:"task_id"`
	ManifestSHA256  string  `json:"manifest_sha256"`
	SubstrateCommit string  `json:"substrate_commit"`
	Arm             string  `json:"arm"`
	Reply           *string `json:"reply"`
	ExitCode        *int    `json:"exit_code"`
	Usage           *Usage  `json:"usage"`
}

// Result is one run's verdict and why.
type Result struct {
	Verdict Verdict
	Reasons []string
	Arm     string
	Answer  *int
	Truth   *int
	TreeID  string
}

// ComparisonStatus says whether two arms may be compared at all.
type ComparisonStatus string

// The comparison statuses. None of them carries an effect estimate.
const (
	Comparable     ComparisonStatus = "COMPARABLE"
	RefusedCeiling ComparisonStatus = "REFUSED_CEILING"
	Insufficient   ComparisonStatus = "INSUFFICIENT"
)

// ArmCount counts PASS among admissible runs; UNAVAILABLE runs are kept apart.
type ArmCount struct{ Pass, N, Unavailable int }

// Comparison is a validity status and counts. It carries no effect estimate.
type Comparison struct {
	Status    ComparisonStatus
	Control   ArmCount
	Treatment ArmCount
	Note      string
}

var errModeUnobservable = errors.New("file modes are not observable on this platform")

// Pinned returns the embedded manifest and its exact bytes.
func Pinned() (Manifest, []byte, error) {
	m, err := ParseManifest(countBTestsManifest)
	return m, countBTestsManifest, err
}

// ParseManifest decodes and validates a manifest.
func ParseManifest(raw []byte) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, err
	}
	switch {
	case m.Schema != ManifestSchema:
		return Manifest{}, fmt.Errorf("unsupported manifest schema %q", m.Schema)
	case m.TaskID == "" || m.Prompt == "" || m.Glob == "":
		return Manifest{}, errors.New("manifest lacks task id, prompt or glob")
	case !hex40(m.SubstrateCommit) || !hex40(m.SubstrateTree):
		return Manifest{}, errors.New("substrate commit and tree must be full 40-character object ids")
	case m.AnswerRule != "leading_integer":
		return Manifest{}, fmt.Errorf("unsupported answer rule %q", m.AnswerRule)
	}
	return m, nil
}

func hex40(s string) bool {
	if len(s) != 40 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

// ManifestDigest is the SHA-256 of the manifest bytes a run must name.
func ManifestDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// TreeID computes the git tree object id of root, ignoring a top-level .git,
// exactly as `git write-tree` would for that content. Empty directories have
// no entry, as in git. Anything but a regular file, symlink or directory is
// refused. On Windows executable bits cannot be observed, so it refuses.
func TreeID(root string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", errModeUnobservable
	}
	id, empty, err := treeOf(root, true)
	if err != nil {
		return "", err
	}
	if empty {
		id, _ = gitObject("tree", nil)
	}
	return id, nil
}

func treeOf(dir string, top bool) (string, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, err
	}
	type entry struct {
		name, mode, sortKey string
		id                  []byte
	}
	var list []entry
	for _, e := range entries {
		name := e.Name()
		if name == ".git" {
			if top {
				continue
			}
			// git records a nested repository as a gitlink (mode 160000)
			// naming a commit, not as the files under it. Hashing those
			// files would produce an id git never would, so refuse.
			return "", false, fmt.Errorf("nested git repository at %s is unsupported", filepath.Join(dir, name))
		}
		p := filepath.Join(dir, name)
		info, err := os.Lstat(p)
		if err != nil {
			return "", false, err
		}
		var mode, id string
		key := name
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return "", false, err
			}
			mode, id = "120000", mustObject("blob", []byte(target))
		case info.IsDir():
			sub, empty, err := treeOf(p, false)
			if err != nil {
				return "", false, err
			}
			if empty {
				continue
			}
			mode, id, key = "40000", sub, name+"/"
		case info.Mode().IsRegular():
			b, err := os.ReadFile(p)
			if err != nil {
				return "", false, err
			}
			mode = "100644"
			if info.Mode().Perm()&0o100 != 0 {
				mode = "100755"
			}
			id = mustObject("blob", b)
		default:
			return "", false, fmt.Errorf("unsupported entry %s", p)
		}
		raw, _ := hex.DecodeString(id)
		list = append(list, entry{name: name, mode: mode, sortKey: key, id: raw})
	}
	if len(list) == 0 {
		return "", true, nil
	}
	sort.Slice(list, func(i, j int) bool { return list[i].sortKey < list[j].sortKey })
	var buf bytes.Buffer
	for _, e := range list {
		buf.WriteString(e.mode + " " + e.name)
		buf.WriteByte(0)
		buf.Write(e.id)
	}
	id, err := gitObject("tree", buf.Bytes())
	return id, false, err
}

func gitObject(kind string, body []byte) (string, error) {
	h := sha1.New() //nolint:gosec // see import
	_, _ = fmt.Fprintf(h, "%s %d\x00", kind, len(body))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func mustObject(kind string, body []byte) string {
	id, _ := gitObject(kind, body)
	return id
}

// DeriveAnswer counts lines beginning `func Test` in the files under root whose
// slash path matches glob. path.Match does not cross a slash, so the glob
// names one directory, as the task does.
func DeriveAnswer(root, glob string) (int, error) {
	n := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		// A glob anchored at the root never matches under .git, and a
		// directory is not a regular file, so neither needs its own case.
		rel = filepath.ToSlash(rel)
		ok, err := path.Match(glob, rel)
		if err != nil || !ok || !d.Type().IsRegular() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(bytes.NewReader(b))
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "func Test") {
				n++
			}
		}
		return sc.Err()
	})
	return n, err
}

// Score judges one run directory: run.json and tree/.
//
// Admissibility comes first: a run missing required evidence, or whose
// provenance does not bind this manifest, is UNAVAILABLE with every reason,
// and is not scored. An admissible run FAILs on a modified tree or a wrong or
// unreadable answer, and PASSes only with the derived answer on an untouched
// tree.
func Score(m Manifest, manifestRaw []byte, runDir string) Result {
	r := Result{}
	unavailable := func(codes ...string) Result {
		r.Verdict, r.Reasons = Unavailable, append(r.Reasons, codes...)
		return r
	}
	// The evidence is the run directory's own run.json and tree/. A symlink
	// in either place would have the scorer judge bytes from somewhere else,
	// so it is refused by Lstat and never followed. This is one check by
	// path, not isolation: it does not stop a change made after it.
	record := filepath.Join(runDir, "run.json")
	if info, err := os.Lstat(record); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return unavailable("run_record_symlink")
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		return unavailable("run_record_missing")
	}
	var run Run
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&run); err != nil {
		return unavailable("run_record_invalid")
	}
	r.Arm = run.Arm
	var reasons []string
	if run.Schema != RunSchema {
		reasons = append(reasons, "run_schema_unsupported")
	}
	if run.TaskID != m.TaskID {
		reasons = append(reasons, "provenance_task_mismatch")
	}
	if run.ManifestSHA256 != ManifestDigest(manifestRaw) {
		reasons = append(reasons, "provenance_manifest_mismatch")
	}
	if run.SubstrateCommit != m.SubstrateCommit {
		reasons = append(reasons, "provenance_substrate_mismatch")
	}
	if run.ExitCode == nil {
		reasons = append(reasons, "exit_code_missing")
	} else if *run.ExitCode != 0 {
		reasons = append(reasons, "execution_failed")
	}
	if run.Usage == nil || run.Usage.InputTokens == nil || run.Usage.OutputTokens == nil {
		reasons = append(reasons, "usage_missing")
	} else if *run.Usage.InputTokens < 0 || *run.Usage.OutputTokens < 0 {
		reasons = append(reasons, "usage_invalid")
	}
	if run.Reply == nil || strings.TrimSpace(*run.Reply) == "" {
		reasons = append(reasons, "answer_missing")
	}
	tree := filepath.Join(runDir, "tree")
	if info, err := os.Lstat(tree); err == nil && info.Mode()&os.ModeSymlink != 0 {
		reasons = append(reasons, "result_tree_symlink")
	} else if err != nil || !info.IsDir() {
		reasons = append(reasons, "result_tree_missing")
	}
	if len(reasons) > 0 {
		return unavailable(reasons...)
	}
	id, err := TreeID(tree)
	if err != nil {
		return unavailable("result_tree_unverifiable")
	}
	r.TreeID = id
	if id != m.SubstrateTree {
		r.Verdict, r.Reasons = Fail, []string{"result_tree_modified"}
		return r
	}
	truth, err := DeriveAnswer(tree, m.Glob)
	if err != nil {
		return unavailable("truth_underivable")
	}
	r.Truth = &truth
	if truth != m.ExpectedAnswer {
		return unavailable("manifest_truth_mismatch")
	}
	answer, ok := outputdiscipline.Run{Result: *run.Reply}.Answer()
	if !ok {
		r.Verdict, r.Reasons = Fail, []string{"answer_not_a_number"}
		return r
	}
	r.Answer = &answer
	if answer != truth {
		r.Verdict, r.Reasons = Fail, []string{"wrong_answer"}
		return r
	}
	r.Verdict = Pass
	return r
}

// Compare reports whether two arms may be compared, under the preregistered
// R10 stop rule: a control arm at 0/n or n/n is a ceiling, which cannot show
// an effect in either direction. It never estimates an effect.
func Compare(control, treatment []Result) Comparison {
	c := Comparison{Control: count(control), Treatment: count(treatment)}
	switch {
	case c.Control.N == 0 || c.Treatment.N == 0:
		c.Status, c.Note = Insufficient, "an arm has no admissible run"
	case c.Control.Pass == 0 || c.Control.Pass == c.Control.N:
		c.Status = RefusedCeiling
		c.Note = "control is at a ceiling (0/n or n/n); no comparison is valid, and this is not evidence of no effect"
	default:
		c.Status, c.Note = Comparable, "counts only; no effect is estimated"
	}
	return c
}

func count(rs []Result) ArmCount {
	var a ArmCount
	for _, r := range rs {
		switch r.Verdict {
		case Pass:
			a.Pass++
			a.N++
		case Fail:
			a.N++
		default:
			a.Unavailable++
		}
	}
	return a
}
