package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/outcome"
)

// The contract these tests hold is docs/design/outcome-record.md.

// newRecording builds a synthetic substrate and a complete, valid set of
// flags for it. Every test starts from a recording that would PASS and breaks
// one thing.
func newRecording(t *testing.T) (outcome.Manifest, map[string]string) {
	t.Helper()
	m, _, tree := withSyntheticManifest(t)
	dir := t.TempDir()
	reply := filepath.Join(dir, "reply.txt")
	if err := os.WriteFile(reply, []byte("3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return m, map[string]string{
		"task": m.TaskID, "reply-file": reply, "exit-code": "0", "usage-in": "1200",
		"usage-out": "4", "arm": "control", "tree": tree, "out": filepath.Join(dir, "run"),
	}
}

func recordArgs(flags map[string]string, extra ...string) []string {
	keys := make([]string, 0, len(flags))
	for k := range flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	args := []string{"outcome-record"}
	for _, k := range keys {
		args = append(args, "--"+k, flags[k])
	}
	return append(args, extra...)
}

func record(t *testing.T, flags map[string]string, extra ...string) (string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := dispatch(recordArgs(flags, extra...), &out, &errb)
	return out.String() + errb.String(), err
}

func refusalCode(err error) string {
	var rr *recordRefusal
	if errors.As(err, &rr) {
		return rr.Code
	}
	return ""
}

// scoreRecorded scores dir with the shipped scorer, through dispatch.
func scoreRecorded(t *testing.T, dir string) (verdict string, reasons []string) {
	t.Helper()
	var out, errb bytes.Buffer
	if err := dispatch([]string{"outcome", "--json", dir}, &out, &errb); err != nil {
		t.Fatalf("outcome: %v\n%s", err, errb.String())
	}
	r := decodeOutcome(t, out.String())
	return r.Runs[0].Verdict, r.Runs[0].Reasons
}

// mustRefuse asserts the refusal code and that nothing scorable is left.
func mustRefuse(t *testing.T, flags map[string]string, code string, extra ...string) {
	t.Helper()
	out, err := record(t, flags, extra...)
	if got := refusalCode(err); got != code {
		t.Fatalf("refusal = %q (%v), want %q\n%s", got, err, code, out)
	}
	if !strings.Contains(err.Error(), "refused: "+code) {
		t.Errorf("error %q does not name %s", err, code)
	}
	if _, serr := os.Lstat(filepath.Join(flags["out"], "run.json")); serr == nil {
		t.Errorf("%s left a run.json behind", code)
	}
}

func mustBeAbsent(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s exists after a refusal (%v); a refusal removes everything it created", p, err)
	}
}

func TestE2E_OutcomeRecord(t *testing.T) {
	_, flags := newRecording(t)
	var stdout, stderr bytes.Buffer
	args := append([]string{"outcome-record"}, recordArgs(flags)[1:]...)
	if err := dispatch(args, &stdout, &stderr); err != nil {
		t.Fatalf("record: %v\n%s", err, stderr.String())
	}
	out := stdout.String()
	m := outcomeManifestForTest(t)
	clean := "(tree " + m.SubstrateTree[:12] + ", the substrate)"
	for _, want := range []string{"recorded " + flags["out"], clean, "does not prove", "replay outcome " + flags["out"]} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if v, why := scoreRecorded(t, flags["out"]); v != "PASS" {
		t.Fatalf("a faithful recording of the substrate scored %s %v", v, why)
	}

	// No exit code is never an exit code of 0.
	_, noExit := newRecording(t)
	delete(noExit, "exit-code")
	mustRefuse(t, noExit, "flag_missing:exit-code", "--allow-modified-tree")
	mustBeAbsent(t, noExit["out"])

	// A tree that is not the substrate is refused unless the operator asks.
	_, modified := newRecording(t)
	if err := os.WriteFile(filepath.Join(modified["tree"], "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRefuse(t, modified, "tree_differs_from_substrate")
	mustBeAbsent(t, modified["out"])
}

func TestOutcomeRecordRequiresEveryFlag(t *testing.T) {
	for _, name := range []string{"task", "reply-file", "exit-code", "usage-in", "usage-out", "arm", "tree", "out"} {
		t.Run(name, func(t *testing.T) {
			_, flags := newRecording(t)
			out := flags["out"]
			delete(flags, name)
			_, err := record(t, flags)
			if got := refusalCode(err); got != "flag_missing:"+name {
				t.Fatalf("without --%s: %q (%v)", name, got, err)
			}
			mustBeAbsent(t, out)

			// An explicit empty value is not a value either.
			_, flags = newRecording(t)
			out = flags["out"]
			flags[name] = ""
			_, err = record(t, flags)
			if got := refusalCode(err); got != "flag_missing:"+name {
				t.Fatalf("--%s='': %q (%v)", name, got, err)
			}
			mustBeAbsent(t, out)
		})
	}
}

func TestOutcomeRecordRefusesBadValues(t *testing.T) {
	cases := []struct{ flag, value, code string }{
		{"task", "count-b-tests-v1", "task_unknown"},
		{"exit-code", "256", "exit_code_invalid"},
		{"exit-code", "-1", "exit_code_invalid"},
		{"exit-code", "abc", "exit_code_invalid"},
		{"usage-in", "-1", "usage_invalid"},
		{"usage-out", "x", "usage_invalid"},
		{"arm", "a/b", "arm_invalid"},
		{"arm", strings.Repeat("a", 65), "arm_invalid"},
	}
	for _, c := range cases {
		t.Run(c.flag+"="+c.value, func(t *testing.T) {
			_, flags := newRecording(t)
			flags[c.flag] = c.value
			mustRefuse(t, flags, c.code)
			mustBeAbsent(t, flags["out"])
		})
	}
	t.Run("positional argument", func(t *testing.T) {
		_, flags := newRecording(t)
		mustRefuse(t, flags, "unexpected_argument", "stray")
		mustBeAbsent(t, flags["out"])
	})
}

func TestOutcomeRecordReplyRefusals(t *testing.T) {
	write := func(t *testing.T, flags map[string]string, b []byte) {
		t.Helper()
		if err := os.WriteFile(flags["reply-file"], b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, code string
		set        func(t *testing.T, flags map[string]string)
	}{
		{"missing", "reply_missing", func(_ *testing.T, f map[string]string) { f["reply-file"] += ".absent" }},
		{"symlink", "reply_symlink", func(t *testing.T, f map[string]string) {
			link := f["reply-file"] + ".link"
			if err := os.Symlink(f["reply-file"], link); err != nil {
				t.Fatal(err)
			}
			f["reply-file"] = link
		}},
		{"directory", "reply_not_regular", func(t *testing.T, f map[string]string) { f["reply-file"] = t.TempDir() }},
		{"blank", "reply_empty", func(t *testing.T, f map[string]string) { write(t, f, []byte(" \n\t")) }},
		{"too large", "reply_too_large", func(t *testing.T, f map[string]string) { write(t, f, bytes.Repeat([]byte("3"), 1<<20+1)) }},
		{"not utf-8", "reply_not_utf8", func(t *testing.T, f map[string]string) { write(t, f, []byte("3\xff")) }},
		{"unreadable", "reply_unreadable", func(t *testing.T, f map[string]string) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a mode-000 file")
			}
			if err := os.Chmod(f["reply-file"], 0); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, flags := newRecording(t)
			c.set(t, flags)
			mustRefuse(t, flags, c.code)
			mustBeAbsent(t, flags["out"])
		})
	}
	t.Run("exactly 1 MiB is accepted", func(t *testing.T) {
		_, flags := newRecording(t)
		b := append([]byte("3"), bytes.Repeat([]byte(" "), 1<<20-1)...)
		write(t, flags, b)
		if out, err := record(t, flags); err != nil {
			t.Fatalf("a reply of exactly the limit was refused: %v\n%s", err, out)
		}
	})
}

// The recorder records; the scorer judges. These are the scorer's verdicts on
// recorded runs, unchanged.
func TestOutcomeRecordScoresAsTheScorerWould(t *testing.T) {
	cases := []struct {
		name, flag, value, verdict, reason string
	}{
		// A nonzero exit makes the run inadmissible, not wrong: Score lists
		// execution_failed among the reasons it cannot judge the run.
		{"nonzero exit", "exit-code", "1", "UNAVAILABLE", "execution_failed"},
		{"wrong answer", "", "59", "FAIL", "wrong_answer"},
		{"markdown answer", "", "**3**", "FAIL", "answer_not_a_number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, flags := newRecording(t)
			if c.flag != "" {
				flags[c.flag] = c.value
			} else if err := os.WriteFile(flags["reply-file"], []byte(c.value), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := record(t, flags); err != nil {
				t.Fatalf("record: %v\n%s", err, out)
			}
			v, why := scoreRecorded(t, flags["out"])
			if v != c.verdict || len(why) == 0 || why[0] != c.reason {
				t.Fatalf("scored %s %v, want %s [%s]", v, why, c.verdict, c.reason)
			}
		})
	}
}

func TestOutcomeRecordModifiedTreeNeedsTheFlag(t *testing.T) {
	_, flags := newRecording(t)
	if err := os.WriteFile(filepath.Join(flags["tree"], "extra.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRefuse(t, flags, "tree_differs_from_substrate")
	mustBeAbsent(t, flags["out"])

	out, err := record(t, flags, "--allow-modified-tree")
	if err != nil {
		t.Fatalf("with --allow-modified-tree: %v\n%s", err, out)
	}
	if !strings.Contains(out, ", differs from the substrate)") || strings.Contains(out, ", the substrate)") {
		t.Errorf("a modified tree was not reported as differing:\n%s", out)
	}
	if v, why := scoreRecorded(t, flags["out"]); v != "FAIL" || why[0] != "result_tree_modified" {
		t.Fatalf("scored %s %v, want FAIL result_tree_modified", v, why)
	}
	p := readProvenance(t, flags["out"])
	if p.Tree.MatchesSubstrate || !p.Tree.ModifiedTreeAllowed {
		t.Errorf("provenance tree = %+v, want matches_substrate false and modified_tree_allowed true", p.Tree)
	}
}

func TestOutcomeRecordNeverOverwrites(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		_, flags := newRecording(t)
		if err := os.Mkdir(flags["out"], 0o755); err != nil {
			t.Fatal(err)
		}
		mustRefuse(t, flags, "out_exists")
		if entries, _ := os.ReadDir(flags["out"]); len(entries) != 0 {
			t.Errorf("an existing directory was written into: %v", entries)
		}
	})
	t.Run("an earlier run", func(t *testing.T) {
		_, flags := newRecording(t)
		if out, err := record(t, flags); err != nil {
			t.Fatalf("record: %v\n%s", err, out)
		}
		before := snapshotDir(t, flags["out"])
		flags["arm"] = "treatment"
		_, err := record(t, flags)
		if got := refusalCode(err); got != "out_exists" {
			t.Fatalf("second recording: %q (%v)", got, err)
		}
		if after := snapshotDir(t, flags["out"]); after != before {
			t.Errorf("the earlier run changed:\n%s\n---\n%s", before, after)
		}
	})
	t.Run("a dangling symlink", func(t *testing.T) {
		_, flags := newRecording(t)
		if err := os.Symlink(filepath.Join(t.TempDir(), "nowhere"), flags["out"]); err != nil {
			t.Fatal(err)
		}
		mustRefuse(t, flags, "out_exists")
	})
	t.Run("parent missing", func(t *testing.T) {
		_, flags := newRecording(t)
		flags["out"] = filepath.Join(t.TempDir(), "no", "run")
		mustRefuse(t, flags, "out_parent_missing")
	})
	t.Run("parent not writable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root writes a mode-555 directory")
		}
		_, flags := newRecording(t)
		parent := t.TempDir()
		if err := os.Chmod(parent, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
		flags["out"] = filepath.Join(parent, "run")
		mustRefuse(t, flags, "out_create_failed")
		mustBeAbsent(t, flags["out"])
	})
}

func TestOutcomeRecordPathContainment(t *testing.T) {
	t.Run("out inside the tree", func(t *testing.T) {
		_, flags := newRecording(t)
		before := snapshotDir(t, flags["tree"])
		flags["out"] = filepath.Join(flags["tree"], "cmd", "run")
		mustRefuse(t, flags, "out_inside_tree")
		if after := snapshotDir(t, flags["tree"]); after != before {
			t.Error("the source tree changed")
		}
	})
	t.Run("through dot-dot", func(t *testing.T) {
		_, flags := newRecording(t)
		flags["out"] = filepath.Join(flags["tree"], "cmd", "..", "run")
		mustRefuse(t, flags, "out_inside_tree")
	})
	t.Run("through a linked parent", func(t *testing.T) {
		_, flags := newRecording(t)
		link := filepath.Join(t.TempDir(), "into-tree")
		if err := os.Symlink(filepath.Join(flags["tree"], "cmd"), link); err != nil {
			t.Fatal(err)
		}
		flags["out"] = filepath.Join(link, "run")
		mustRefuse(t, flags, "out_inside_tree")
		mustBeAbsent(t, filepath.Join(flags["tree"], "cmd", "run"))
	})
	t.Run("unresolvable", func(t *testing.T) {
		_, flags := newRecording(t)
		prev := recordOS.resolve
		recordOS.resolve = func(string) (string, error) { return "", errors.New("injected") }
		t.Cleanup(func() { recordOS.resolve = prev })
		mustRefuse(t, flags, "path_unresolvable")
		mustBeAbsent(t, flags["out"])
	})
}

func TestOutcomeRecordTreeRefusals(t *testing.T) {
	cases := []struct {
		name, code string
		set        func(t *testing.T, f map[string]string)
	}{
		{"missing", "tree_missing", func(_ *testing.T, f map[string]string) { f["tree"] += ".absent" }},
		{"symlinked root", "tree_symlink_root", func(t *testing.T, f map[string]string) {
			link := filepath.Join(t.TempDir(), "tree")
			if err := os.Symlink(f["tree"], link); err != nil {
				t.Fatal(err)
			}
			f["tree"] = link
		}},
		{"a file", "tree_not_dir", func(_ *testing.T, f map[string]string) { f["tree"] = f["reply-file"] }},
		{"nested repository", "tree_nested_repo", func(t *testing.T, f map[string]string) {
			if err := os.MkdirAll(filepath.Join(f["tree"], "cmd", ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"nested gitfile", "tree_nested_repo", func(t *testing.T, f map[string]string) {
			if err := os.WriteFile(filepath.Join(f["tree"], "cmd", ".git"), []byte("gitdir: x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"socket", "tree_unsupported_object", func(t *testing.T, f map[string]string) {
			// Bound by a relative name: a socket path is limited to about
			// 104 bytes, and a test's temporary directory is longer. A
			// failure here fails the test rather than skipping it, so the
			// unsupported-object refusal cannot go unexercised. Close
			// unlinks the socket.
			t.Chdir(f["tree"])
			l, err := net.Listen("unix", "s")
			if err != nil {
				t.Fatalf("unix socket: %v", err)
			}
			t.Cleanup(func() { _ = l.Close() })
		}},
		{"unreadable directory", "tree_unreadable", func(t *testing.T, f map[string]string) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a mode-000 directory")
			}
			d := filepath.Join(f["tree"], "cmd")
			if err := os.Chmod(d, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(d, 0o755) })
		}},
		{"unreadable file", "tree_unverifiable", func(t *testing.T, f map[string]string) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a mode-000 file")
			}
			p := filepath.Join(f["tree"], "README.md")
			if err := os.Chmod(p, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, flags := newRecording(t)
			c.set(t, flags)
			mustRefuse(t, flags, c.code, "--allow-modified-tree")
			mustBeAbsent(t, flags["out"])
		})
	}
}

func TestOutcomeRecordCopiesLinksAndSkipsTopGit(t *testing.T) {
	t.Run("top-level .git directory and file", func(t *testing.T) {
		for _, form := range []string{"dir", "file"} {
			_, flags := newRecording(t)
			g := filepath.Join(flags["tree"], ".git")
			var err error
			if form == "dir" {
				err = os.MkdirAll(filepath.Join(g, "objects"), 0o755)
			} else {
				err = os.WriteFile(g, []byte("gitdir: /home/someone/repo/.git/worktrees/x\n"), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			if out, err := record(t, flags); err != nil {
				t.Fatalf("%s: %v\n%s", form, err, out)
			}
			mustBeAbsent(t, filepath.Join(flags["out"], "tree", ".git"))
			if v, why := scoreRecorded(t, flags["out"]); v != "PASS" {
				t.Fatalf("%s: scored %s %v", form, v, why)
			}
		}
	})
	t.Run("a link inside the tree", func(t *testing.T) {
		_, flags := newRecording(t)
		clean := filepath.Join(t.TempDir(), "clean")
		copyOutcomeTree(t, filepath.Join(flags["tree"], "cmd"), clean)
		cmd := filepath.Join(flags["tree"], "cmd")
		if err := os.RemoveAll(cmd); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(clean, cmd); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("README.md", filepath.Join(flags["tree"], "readme-link")); err != nil {
			t.Fatal(err)
		}
		if out, err := record(t, flags, "--allow-modified-tree"); err != nil {
			t.Fatalf("record: %v\n%s", err, out)
		}
		for name, target := range map[string]string{"cmd": clean, "readme-link": "README.md"} {
			p := filepath.Join(flags["out"], "tree", name)
			info, err := os.Lstat(p)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s was not copied as a link: %v %v", name, info, err)
			}
			if got, _ := os.Readlink(p); got != target {
				t.Errorf("%s -> %q, want %q", name, got, target)
			}
		}
		if v, why := scoreRecorded(t, flags["out"]); v != "FAIL" || why[0] != "result_tree_modified" {
			t.Fatalf("a directory swapped for a link scored %s %v", v, why)
		}
	})
	t.Run("executable bit", func(t *testing.T) {
		_, flags := newRecording(t)
		p := filepath.Join(flags["tree"], "run.sh")
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := record(t, flags, "--allow-modified-tree"); err != nil {
			t.Fatalf("record: %v\n%s", err, out)
		}
		info, err := os.Stat(filepath.Join(flags["out"], "tree", "run.sh"))
		if err != nil || info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("executable bit lost: %v %v", info, err)
		}
	})
}

func TestOutcomeRecordDetectsAChangeDuringTheCopy(t *testing.T) {
	_, flags := newRecording(t)
	prev := outcomeRecordAfterCopy
	outcomeRecordAfterCopy = func(src string) {
		_ = os.WriteFile(filepath.Join(src, "README.md"), []byte("changed during the copy\n"), 0o644)
	}
	t.Cleanup(func() { outcomeRecordAfterCopy = prev })
	mustRefuse(t, flags, "tree_unstable_during_copy")
	mustBeAbsent(t, flags["out"])
}

// A write that fails at any step leaves nothing behind, and above all no
// run.json.
func TestOutcomeRecordCleansUpAfterAWriteFailure(t *testing.T) {
	injected := errors.New("injected")
	cases := []struct {
		name, code string
		set        func()
	}{
		{"mkdir", "tree_copy_failed", func() {
			recordOS.mkdirAll = func(string, os.FileMode) error { return injected }
		}},
		{"read", "tree_copy_failed", func() {
			recordOS.readFile = func(string) ([]byte, error) { return nil, injected }
		}},
		{"readlink", "tree_copy_failed", func() {
			recordOS.readlink = func(string) (string, error) { return "", injected }
		}},
		{"symlink", "tree_copy_failed", func() {
			recordOS.symlink = func(string, string) error { return injected }
		}},
		{"tree file", "tree_copy_failed", func() {
			w := recordOS.writeFile
			recordOS.writeFile = func(p string, b []byte, m os.FileMode) error {
				if strings.Contains(p, string(filepath.Separator)+"tree"+string(filepath.Separator)) {
					return injected
				}
				return w(p, b, m)
			}
		}},
		{"provenance", "write_failed", func() {
			w := recordOS.writeFile
			recordOS.writeFile = func(p string, b []byte, m os.FileMode) error {
				if filepath.Base(p) == "provenance.json" {
					return injected
				}
				return w(p, b, m)
			}
		}},
		{"run record", "write_failed", func() {
			w := recordOS.writeFile
			recordOS.writeFile = func(p string, b []byte, m os.FileMode) error {
				if strings.HasPrefix(filepath.Base(p), ".run.json") {
					return injected
				}
				return w(p, b, m)
			}
		}},
		{"rename", "write_failed", func() {
			recordOS.rename = func(string, string) error { return injected }
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, flags := newRecording(t)
			if c.name == "readlink" || c.name == "symlink" {
				if err := os.Symlink("README.md", filepath.Join(flags["tree"], "link")); err != nil {
					t.Fatal(err)
				}
			}
			prev := recordOS
			t.Cleanup(func() { recordOS = prev })
			c.set()
			mustRefuse(t, flags, c.code, "--allow-modified-tree")
			mustBeAbsent(t, flags["out"])
		})
	}
}

type recordedProvenance struct {
	Schema      string            `json:"schema"`
	Recorder    string            `json:"recorder"`
	Limitation  string            `json:"limitation"`
	Fields      map[string]string `json:"fields"`
	ReplySHA256 string            `json:"reply_sha256"`
	Tree        struct {
		Capture             string `json:"capture"`
		ExecutionTree       string `json:"execution_tree"`
		SourceTreeIDBefore  string `json:"source_tree_id_before"`
		CopyTreeID          string `json:"copy_tree_id"`
		SourceTreeIDAfter   string `json:"source_tree_id_after"`
		MatchesSubstrate    bool   `json:"matches_substrate"`
		ModifiedTreeAllowed bool   `json:"modified_tree_allowed"`
		StabilityCheck      string `json:"stability_check"`
	} `json:"tree"`
}

func readProvenance(t *testing.T, dir string) recordedProvenance {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p recordedProvenance
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("provenance.json: %v\n%s", err, b)
	}
	return p
}

func TestOutcomeRecordProvenanceIsHonestAndPrivate(t *testing.T) {
	m, flags := newRecording(t)
	if out, err := record(t, flags); err != nil {
		t.Fatalf("record: %v\n%s", err, out)
	}
	p := readProvenance(t, flags["out"])
	if p.Schema != "replay.outcome.provenance.v1" || !strings.HasPrefix(p.Recorder, "replay outcome-record ") {
		t.Errorf("schema %q, recorder %q", p.Schema, p.Recorder)
	}
	want := map[string]string{
		"reply": "operator_attested", "exit_code": "operator_attested", "usage": "operator_attested",
		"arm": "operator_attested", "task_id": "reconstructed_from_manifest",
		"manifest_sha256": "reconstructed_from_manifest", "substrate_commit": "reconstructed_from_manifest",
	}
	if len(p.Fields) != len(want) {
		t.Errorf("fields = %v, want %v", p.Fields, want)
	}
	for k, v := range want {
		if p.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, p.Fields[k], v)
		}
	}
	if p.Tree.Capture != "post_run_copy" || p.Tree.ExecutionTree != "not_established" {
		t.Errorf("tree claims %q / %q", p.Tree.Capture, p.Tree.ExecutionTree)
	}
	if p.Tree.SourceTreeIDBefore != m.SubstrateTree || p.Tree.CopyTreeID != m.SubstrateTree ||
		p.Tree.SourceTreeIDAfter != m.SubstrateTree || !p.Tree.MatchesSubstrate || p.Tree.ModifiedTreeAllowed {
		t.Errorf("tree ids %+v, want all %s", p.Tree, m.SubstrateTree)
	}
	if !strings.Contains(p.Tree.StabilityCheck, "not isolation") {
		t.Errorf("stability check %q claims more than it does", p.Tree.StabilityCheck)
	}
	if !strings.Contains(p.Limitation, "does not prove that an agent executed the task") {
		t.Errorf("limitation %q", p.Limitation)
	}
	if sum := sha256.Sum256([]byte("3\n")); p.ReplySHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("reply_sha256 %s", p.ReplySHA256)
	}

	home, _ := os.UserHomeDir()
	forbidden := []string{flags["tree"], flags["reply-file"], filepath.Dir(flags["out"]), home}
	// The user name is checked only when the environment states one; it is
	// one more string that must not appear, not a statement of identity.
	if u := os.Getenv("USER"); len(u) > 2 {
		forbidden = append(forbidden, u)
	}
	for _, name := range []string{"run.json", "provenance.json"} {
		b, err := os.ReadFile(filepath.Join(flags["out"], name))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range forbidden {
			if f != "" && bytes.Contains(b, []byte(f)) {
				t.Errorf("%s contains %q", name, f)
			}
		}
	}
	entries, err := os.ReadDir(flags["out"])
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "provenance.json,run.json,tree" {
		t.Errorf("out holds %v, want provenance.json, run.json and tree only", names)
	}
}

func TestOutcomeRecordRunJSONIsStrict(t *testing.T) {
	m, flags := newRecording(t)
	if out, err := record(t, flags); err != nil {
		t.Fatalf("record: %v\n%s", err, out)
	}
	b, err := os.ReadFile(filepath.Join(flags["out"], "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var run outcome.Run
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&run); err != nil {
		t.Fatalf("run.json does not decode strictly: %v\n%s", err, b)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatal(err)
	}
	var got []string
	for k := range keys {
		got = append(got, k)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "arm,exit_code,manifest_sha256,reply,schema,substrate_commit,task_id,usage" {
		t.Errorf("run.json keys = %v", got)
	}
	if run.Schema != outcome.RunSchema || run.TaskID != m.TaskID || run.SubstrateCommit != m.SubstrateCommit ||
		run.Arm != "control" || run.Reply == nil || *run.Reply != "3\n" || run.ExitCode == nil || *run.ExitCode != 0 ||
		run.Usage == nil || *run.Usage.InputTokens != 1200 || *run.Usage.OutputTokens != 4 {
		t.Errorf("run.json = %s", b)
	}
}

func TestOutcomeRecordLeavesItsSourcesUnchanged(t *testing.T) {
	_, flags := newRecording(t)
	if err := os.Symlink("README.md", filepath.Join(flags["tree"], "link")); err != nil {
		t.Fatal(err)
	}
	tree, reply := snapshotDir(t, flags["tree"]), snapshotDir(t, filepath.Dir(flags["reply-file"]))
	if out, err := record(t, flags, "--allow-modified-tree"); err != nil {
		t.Fatalf("record: %v\n%s", err, out)
	}
	if snapshotDir(t, flags["tree"]) != tree {
		t.Error("the source tree changed")
	}
	// The reply's directory also holds --out; compare the reply file alone.
	after := snapshotDir(t, filepath.Dir(flags["reply-file"]))
	if lineFor(after, "reply.txt") != lineFor(reply, "reply.txt") {
		t.Errorf("the reply file changed:\n%s\n%s", lineFor(reply, "reply.txt"), lineFor(after, "reply.txt"))
	}
}

func lineFor(snapshot, name string) string {
	for _, l := range strings.Split(snapshot, "\n") {
		if strings.HasPrefix(l, name+" ") {
			return l
		}
	}
	return ""
}

// Whatever a refusal leaves, the scorer can admit none of it.
func TestOutcomeRecordRefusalIsNeverScorable(t *testing.T) {
	breakers := map[string]func(t *testing.T, f map[string]string) []string{
		"no exit code": func(_ *testing.T, f map[string]string) []string { delete(f, "exit-code"); return nil },
		"modified tree": func(_ *testing.T, f map[string]string) []string {
			_ = os.WriteFile(filepath.Join(f["tree"], "README.md"), []byte("y\n"), 0o644)
			return nil
		},
		"unstable copy": func(t *testing.T, _ map[string]string) []string {
			prev := outcomeRecordAfterCopy
			outcomeRecordAfterCopy = func(src string) { _ = os.WriteFile(filepath.Join(src, "new"), []byte("n"), 0o644) }
			t.Cleanup(func() { outcomeRecordAfterCopy = prev })
			return nil
		},
		"rename fails": func(t *testing.T, _ map[string]string) []string {
			prev := recordOS
			recordOS.rename = func(string, string) error { return errors.New("injected") }
			t.Cleanup(func() { recordOS = prev })
			return nil
		},
	}
	for name, brk := range breakers {
		t.Run(name, func(t *testing.T) {
			_, flags := newRecording(t)
			extra := brk(t, flags)
			if _, err := record(t, flags, extra...); err == nil {
				t.Fatal("not refused")
			}
			if v, why := scoreRecorded(t, flags["out"]); v != "UNAVAILABLE" {
				t.Fatalf("what the refusal left scored %s %v", v, why)
			}
		})
	}
}

// Score reads run.json and tree/ only. A forged sidecar changes nothing.
func TestOutcomeRecordProvenanceCannotChangeAScore(t *testing.T) {
	_, flags := newRecording(t)
	if err := os.WriteFile(flags["reply-file"], []byte("59"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := record(t, flags); err != nil {
		t.Fatalf("record: %v\n%s", err, out)
	}
	v1, why1 := scoreRecorded(t, flags["out"])
	forged := `{"verdict":"PASS","truth":59,"answer":59,"fields":{"reply":"observed"}}`
	if err := os.WriteFile(filepath.Join(flags["out"], "provenance.json"), []byte(forged), 0o644); err != nil {
		t.Fatal(err)
	}
	v2, why2 := scoreRecorded(t, flags["out"])
	if v1 != "FAIL" || v2 != v1 || strings.Join(why1, ",") != strings.Join(why2, ",") {
		t.Fatalf("score %s %v became %s %v after the sidecar was forged", v1, why1, v2, why2)
	}
}

func TestOutcomeRecordRefusesAnInvalidManifest(t *testing.T) {
	_, flags := newRecording(t)
	prev := outcomeManifest
	outcomeManifest = func() (outcome.Manifest, []byte, error) {
		_, err := outcome.ParseManifest([]byte(`{"schema":"replay.outcome.manifest.v0"}`))
		return outcome.Manifest{}, nil, err
	}
	t.Cleanup(func() { outcomeManifest = prev })
	out, err := record(t, flags)
	if err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("an invalid embedded manifest was not refused: %v\n%s", err, out)
	}
	mustBeAbsent(t, flags["out"])
}

func TestOutcomeRecordRefusesAnUnsupportedPlatform(t *testing.T) {
	_, flags := newRecording(t)
	prev := outcomeRecordPlatformOK
	outcomeRecordPlatformOK = false
	t.Cleanup(func() { outcomeRecordPlatformOK = prev })
	mustRefuse(t, flags, "platform_unsupported")
	mustBeAbsent(t, flags["out"])
}

func TestOutcomeRecordUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if err := dispatch([]string{"outcome-record", "--help"}, &out, &errb); err != nil && !errors.Is(err, errHelpShown) {
		t.Fatalf("--help: %v", err)
	}
	for _, want := range []string{"--reply-file", "--allow-modified-tree", "does not prove"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help lacks %q:\n%s", want, out.String())
		}
	}
	if err := dispatch([]string{"outcome-record", "--no-such-flag"}, &out, &errb); !errors.Is(err, errUsage) {
		t.Errorf("an unknown flag: %v, want errUsage", err)
	}
}

// A trailing separator names the same paths, and must not turn --out into its
// own parent.
func TestOutcomeRecordAcceptsTrailingSeparators(t *testing.T) {
	_, flags := newRecording(t)
	out := flags["out"]
	flags["out"] += string(filepath.Separator)
	flags["tree"] += string(filepath.Separator)
	if msg, err := record(t, flags); err != nil {
		t.Fatalf("record: %v\n%s", err, msg)
	}
	if v, why := scoreRecorded(t, out); v != "PASS" {
		t.Fatalf("scored %s %v", v, why)
	}
}

// A tree whose root directory is itself named .git is a repository's own
// store, never a run's working copy, and is refused as a nested repository.
func TestOutcomeRecordRefusesATreeRootNamedGit(t *testing.T) {
	_, flags := newRecording(t)
	gitRoot := filepath.Join(t.TempDir(), ".git")
	copyOutcomeTree(t, flags["tree"], gitRoot)
	flags["tree"] = gitRoot
	mustRefuse(t, flags, "tree_nested_repo")
	mustBeAbsent(t, flags["out"])
}

func outcomeManifestForTest(t *testing.T) outcome.Manifest {
	t.Helper()
	m, _, err := outcomeManifest()
	if err != nil {
		t.Fatal(err)
	}
	return m
}
