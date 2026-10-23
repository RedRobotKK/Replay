package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/outcome"
	"github.com/RedRobotKK/Replay/internal/version"
)

// `replay outcome-record` writes a run directory `replay outcome` can score,
// from values the operator supplies (docs/design/outcome-record.md).
//
// Every value is the operator's word, and provenance.json says so. The
// recorder reads no transcript, runs no agent, calls no provider and changes
// nothing it reads. It never writes a verdict or a truth: the scorer derives
// the truth from tree/, and the recorder only copies the tree it is given.
//
// run.json is written last, by rename, so a recording that stops anywhere
// before it leaves nothing Score can admit, and every refusal after the
// output directory exists removes it.

const outcomeRecordLimitation = "A PASS from supplied inputs means only that the supplied answer is correct " +
	"under the scorer's comparison conditions. It does not prove that an agent executed the task, that the " +
	"supplied answer came from that execution, or that the captured post-run tree was the execution tree."

const maxRecordedReply = 1 << 20

var recordArm = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// recordRefusal is a refusal with a stable code; the detail is for a reader.
type recordRefusal struct{ Code, Detail string }

func (e *recordRefusal) Error() string { return "outcome-record refused: " + e.Code + ": " + e.Detail }

func refuse(code, format string, a ...any) error {
	return &recordRefusal{Code: code, Detail: fmt.Sprintf(format, a...)}
}

// outcomeRecordPlatformOK is false on Windows, where TreeID cannot observe
// executable bits. A variable so the refusal can be tested on any host.
var outcomeRecordPlatformOK = runtime.GOOS != "windows"

// outcomeRecordAfterCopy runs between the copy and the second hash of the
// source. Tests change the source there; nothing in the binary does.
var outcomeRecordAfterCopy = func(string) {}

// recordOS holds the filesystem calls a write can fail in, so a test can make
// each one fail and watch the refusal clean up.
var recordOS = struct {
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	readlink  func(string) (string, error)
	symlink   func(string, string) error
	mkdirAll  func(string, os.FileMode) error
	rename    func(string, string) error
	resolve   func(string) (string, error)
}{os.ReadFile, os.WriteFile, os.Readlink, os.Symlink, os.MkdirAll, os.Rename, resolvePath}

// resolvePath is p made absolute with every symbolic link in it resolved.
func resolvePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	return abs, err
}

type recordedTree struct {
	Capture             string `json:"capture"`
	ExecutionTree       string `json:"execution_tree"`
	SourceTreeIDBefore  string `json:"source_tree_id_before"`
	CopyTreeID          string `json:"copy_tree_id"`
	SourceTreeIDAfter   string `json:"source_tree_id_after"`
	MatchesSubstrate    bool   `json:"matches_substrate"`
	ModifiedTreeAllowed bool   `json:"modified_tree_allowed"`
	StabilityCheck      string `json:"stability_check"`
}

type recordProvenance struct {
	Schema      string            `json:"schema"`
	Recorder    string            `json:"recorder"`
	Limitation  string            `json:"limitation"`
	Fields      map[string]string `json:"fields"`
	ReplySHA256 string            `json:"reply_sha256"`
	Tree        recordedTree      `json:"tree"`
}

func runOutcomeRecord(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("outcome-record", flag.ContinueOnError)
	fs.SetOutput(stderr)
	exitCode := fs.String("exit-code", "", "the exit status of the run, 0 to 255; never assumed")
	vals := map[string]*string{
		"task":       fs.String("task", "", "the task id of the manifest embedded in this binary"),
		"reply-file": fs.String("reply-file", "", "a file holding the agent's reply, verbatim"),
		"usage-in":   fs.String("usage-in", "", "input tokens the run reported"),
		"usage-out":  fs.String("usage-out", "", "output tokens the run reported"),
		"arm":        fs.String("arm", "", "a label for the arm this run belongs to: 1 to 64 of A-Z a-z 0-9 . _ -"),
		"tree":       fs.String("tree", "", "the working copy the run left"),
		"out":        fs.String("out", "", "the run directory to create; it must not exist"),
		"exit-code":  exitCode,
	}
	allowModified := fs.Bool("allow-modified-tree", false,
		"record a tree that is not the pinned substrate, which the scorer will FAIL; it says nothing about who changed it or why")
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), "Usage: replay outcome-record --task <id> --reply-file <file> --exit-code <n>\n"+
			"    --usage-in <n> --usage-out <n> --arm <label> --tree <dir> --out <new-dir> [--allow-modified-tree]\n\n"+
			"Writes a run directory `replay outcome` can score, from values you supply:\n"+
			"run.json, provenance.json, and tree/, a copy of --tree. Every value is\n"+
			"recorded as operator-attested. Reads no transcript and runs nothing.\n\n"+
			outcomeRecordLimitation+"\n\n")
		fs.PrintDefaults()
	}
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	for _, name := range []string{"task", "reply-file", "exit-code", "usage-in", "usage-out", "arm", "tree", "out"} {
		if *vals[name] == "" {
			return refuse("flag_missing:"+name, "--%s is required and has no default", name)
		}
	}
	if fs.NArg() > 0 {
		return refuse("unexpected_argument", "%q: every input is a flag", fs.Arg(0))
	}
	if !outcomeRecordPlatformOK {
		return refuse("platform_unsupported", "tree ids cannot be computed on %s, where executable bits are not observable", runtime.GOOS)
	}
	m, raw, err := outcomeManifest()
	if err != nil {
		return fmt.Errorf("the embedded outcome manifest is invalid: %w", err)
	}
	if *vals["task"] != m.TaskID {
		return refuse("task_unknown", "%q is not %s, the task this binary pins", *vals["task"], m.TaskID)
	}
	code, err := strconv.Atoi(*exitCode)
	if err != nil || code < 0 || code > 255 {
		return refuse("exit_code_invalid", "%q is not an exit status from 0 to 255", *exitCode)
	}
	var usage [2]int64
	for i, name := range []string{"usage-in", "usage-out"} {
		usage[i], err = strconv.ParseInt(*vals[name], 10, 64)
		if err != nil || usage[i] < 0 {
			return refuse("usage_invalid", "--%s %q is not a non-negative integer", name, *vals[name])
		}
	}
	if !recordArm.MatchString(*vals["arm"]) {
		return refuse("arm_invalid", "%q is not 1 to 64 of A-Z a-z 0-9 . _ -", *vals["arm"])
	}
	reply, err := readRecordedReply(*vals["reply-file"])
	if err != nil {
		return err
	}
	tree, out := filepath.Clean(*vals["tree"]), filepath.Clean(*vals["out"])
	if err := checkRecordPaths(tree, out); err != nil {
		return err
	}
	if err := checkRecordTree(tree); err != nil {
		return err
	}
	before, err := outcome.TreeID(tree)
	if err != nil {
		return refuse("tree_unverifiable", "%v", err)
	}
	if before != m.SubstrateTree && !*allowModified {
		return refuse("tree_differs_from_substrate", "tree %s is not the substrate %s; --allow-modified-tree records it anyway, and the scorer will FAIL it",
			before, m.SubstrateTree)
	}
	if err := os.Mkdir(out, 0o755); err != nil {
		return refuse("out_create_failed", "%v", err)
	}
	done := false
	defer func() {
		if !done {
			_ = os.RemoveAll(out)
		}
	}()
	copied := filepath.Join(out, "tree")
	if err := copyRecordTree(tree, copied); err != nil {
		return err
	}
	outcomeRecordAfterCopy(tree)
	copyID, e1 := outcome.TreeID(copied)
	after, e2 := outcome.TreeID(tree)
	if e1 != nil || e2 != nil || copyID != before || after != before {
		return refuse("tree_unstable_during_copy", "source %s before, copy %s, source %s after (%v)", before, copyID, after, errors.Join(e1, e2))
	}
	sum := sha256.Sum256([]byte(reply))
	prov, _ := json.MarshalIndent(recordProvenance{
		Schema: "replay.outcome.provenance.v1", Recorder: "replay outcome-record " + version.String(),
		Limitation: outcomeRecordLimitation,
		Fields: map[string]string{
			"reply": "operator_attested", "exit_code": "operator_attested", "usage": "operator_attested",
			"arm": "operator_attested", "task_id": "reconstructed_from_manifest",
			"manifest_sha256": "reconstructed_from_manifest", "substrate_commit": "reconstructed_from_manifest",
		},
		ReplySHA256: hex.EncodeToString(sum[:]),
		Tree: recordedTree{
			Capture: "post_run_copy", ExecutionTree: "not_established",
			SourceTreeIDBefore: before, CopyTreeID: copyID, SourceTreeIDAfter: after,
			MatchesSubstrate: before == m.SubstrateTree, ModifiedTreeAllowed: *allowModified,
			StabilityCheck: "three tree ids compared; detects a change that lands between them, not isolation",
		},
	}, "", "  ")
	run, _ := json.MarshalIndent(outcome.Run{
		Schema: outcome.RunSchema, TaskID: m.TaskID, ManifestSHA256: outcome.ManifestDigest(raw),
		SubstrateCommit: m.SubstrateCommit, Arm: *vals["arm"], Reply: &reply, ExitCode: &code,
		Usage: &outcome.Usage{InputTokens: &usage[0], OutputTokens: &usage[1]},
	}, "", "  ")
	tmp := filepath.Join(out, ".run.json.tmp")
	err = recordOS.writeFile(filepath.Join(out, "provenance.json"), append(prov, '\n'), 0o644)
	if err == nil {
		err = recordOS.writeFile(tmp, append(run, '\n'), 0o644)
	}
	if err == nil {
		err = recordOS.rename(tmp, filepath.Join(out, "run.json"))
	}
	if err != nil {
		return refuse("write_failed", "%v", err)
	}
	done = true
	match := "differs from the substrate"
	if before == m.SubstrateTree {
		match = "the substrate"
	}
	p := analysis.NewPrinter(stdout)
	p.Printf("recorded %s\n", out)
	p.Printf("  run.json, provenance.json, tree/ (tree %s, %s)\n", before[:12], match)
	p.Printf("Every value was supplied by the operator. %s\n", outcomeRecordLimitation)
	p.Printf("Score it: replay outcome %s\n", out)
	return p.Err()
}

// readRecordedReply reads the reply file, refusing a link, anything but a
// regular file, a blank or oversized reply, and bytes JSON would rewrite.
func readRecordedReply(p string) (string, error) {
	info, err := os.Lstat(p)
	switch {
	case err != nil:
		return "", refuse("reply_missing", "%v", err)
	case info.Mode()&os.ModeSymlink != 0:
		return "", refuse("reply_symlink", "%s is a symbolic link; pass the file itself", p)
	case !info.Mode().IsRegular():
		return "", refuse("reply_not_regular", "%s is not a regular file", p)
	}
	var b []byte
	f, err := os.Open(p)
	if err == nil {
		b, err = io.ReadAll(io.LimitReader(f, maxRecordedReply+1))
		_ = f.Close()
	}
	switch {
	case err != nil:
		return "", refuse("reply_unreadable", "%v", err)
	case len(b) > maxRecordedReply:
		return "", refuse("reply_too_large", "over %d bytes", maxRecordedReply)
	case !utf8.Valid(b):
		return "", refuse("reply_not_utf8", "the reply is not valid UTF-8, which JSON would silently rewrite")
	case strings.TrimSpace(string(b)) == "":
		return "", refuse("reply_empty", "the reply is blank")
	}
	return string(b), nil
}

// checkRecordPaths refuses a tree that is not a plain directory, an output
// that exists or has no parent, and an output inside the tree once links in
// either path are resolved. One check by path, not isolation.
func checkRecordPaths(tree, out string) error {
	info, err := os.Lstat(tree)
	switch {
	case err != nil:
		return refuse("tree_missing", "%v", err)
	case info.Mode()&os.ModeSymlink != 0:
		return refuse("tree_symlink_root", "%s is a symbolic link; pass the directory itself", tree)
	case !info.IsDir():
		return refuse("tree_not_dir", "%s is not a directory", tree)
	}
	if _, err := os.Lstat(out); !errors.Is(err, fs.ErrNotExist) {
		return refuse("out_exists", "%s exists or cannot be checked (%v); a recording never writes into an existing path", out, err)
	}
	parent := filepath.Dir(out)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return refuse("out_parent_missing", "%s is not an existing directory", parent)
	}
	realTree, e1 := recordOS.resolve(tree)
	realParent, e2 := recordOS.resolve(parent)
	if err := errors.Join(e1, e2); err != nil {
		return refuse("path_unresolvable", "%v", err)
	}
	if rel, err := filepath.Rel(realTree, filepath.Join(realParent, filepath.Base(out))); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return refuse("out_inside_tree", "%s is inside the tree being recorded", out)
	}
	return nil
}

// checkRecordTree refuses what TreeID would refuse, and names it: a .git below
// the top, and anything that is not a file, a link or a directory. A root
// that is itself named .git is a repository's store, not a working copy, and
// is refused as a nested repository by the same case.
func checkRecordTree(root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return refuse("tree_unreadable", "%v", err)
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case rel == ".git":
			return skipEntry(d)
		case d.Name() == ".git":
			return refuse("tree_nested_repo", "%s: a nested repository cannot be recorded", p)
		case d.IsDir(), d.Type()&fs.ModeSymlink != 0, d.Type().IsRegular():
		default:
			return refuse("tree_unsupported_object", "%s is %v, not a file, link or directory", p, d.Type())
		}
		return nil
	})
}

// copyRecordTree copies root into dst: file bytes and permission bits, links
// as links with the same target, never followed, and no top-level .git.
func copyRecordTree(root, dst string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		rel, _ := filepath.Rel(root, p)
		if err == nil && rel == ".git" {
			return skipEntry(d)
		}
		// A walk error, and any error copying this entry, reaches the one
		// refusal below.
		if err == nil {
			err = copyRecordEntry(p, filepath.Join(dst, rel), d)
		}
		if err != nil {
			return refuse("tree_copy_failed", "%s: %v", rel, err)
		}
		return nil
	})
}

// copyRecordEntry copies one entry: a directory as a directory, a link as a
// link with the same target, and a file as its bytes and permission bits.
func copyRecordEntry(from, to string, d fs.DirEntry) error {
	switch {
	case d.IsDir():
		return recordOS.mkdirAll(to, 0o755)
	case d.Type()&fs.ModeSymlink != 0:
		target, err := recordOS.readlink(from)
		if err == nil {
			err = recordOS.symlink(target, to)
		}
		return err
	}
	info, err := d.Info()
	var b []byte
	if err == nil {
		b, err = recordOS.readFile(from)
	}
	if err == nil {
		err = recordOS.writeFile(to, b, info.Mode().Perm())
	}
	return err
}

// skipEntry leaves out one entry: a directory's walk stops there, and a file
// is passed over. SkipDir on a file would skip the rest of its directory.
func skipEntry(d fs.DirEntry) error {
	if d.IsDir() {
		return filepath.SkipDir
	}
	return nil
}
