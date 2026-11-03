//go:build mutation

package blackbox

// The ledger file lock across real processes (ADR-0030).
//
// internal/ledger's FL tests prove the lock excludes a second handle in one
// process. These prove it between processes, which is the case the lock
// exists for: `replay serve` appending while `replay purge --session`
// rewrites, or two serves on one ledger.
//
// Every second party is a real child process: this test binary re-run in a
// helper mode (TestLockHelperProcess), or the production binary for purge,
// both started through the os/exec this package already uses. No goroutine
// stands in for a process, and no other exec path is added.
//
// Each test is built to fail when the OS lock is removed: a waiter that must
// still be waiting, or a read-modify-write that loses updates without
// exclusion. A test that O_APPEND alone would keep green is not one of them.
//
// The production binary refuses to run on Windows (cmd/replay/platformguard.go),
// so XL2 and XL3 race purge two ways: the binary, where it runs, and a
// helper process calling ledger.RewriteLocked, the binary's own locked
// rewrite, everywhere, Windows included.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

const (
	xlHelperEnv  = "REPLAY_XL_HELPER"
	xlDirEnv     = "REPLAY_XL_DIR"
	xlSessionEnv = "REPLAY_XL_SESSION"
	xlPrefixEnv  = "REPLAY_XL_PREFIX"
	xlCountEnv   = "REPLAY_XL_COUNT"
	xlPathEnv    = "REPLAY_XL_PATH"
	xlPartialEnv = "REPLAY_XL_PARTIAL"
	xlPauseEnv   = "REPLAY_XL_PAUSE"
	xlInflight   = "inflight"

	// xlLockedBanner is what a holder prints once it has the lock.
	xlLockedBanner = "locked"
	// xlBlockWindow is how long a waiting process must stay waiting for the
	// test to call it blocked. An unlocked append finishes in milliseconds.
	xlBlockWindow = 750 * time.Millisecond
	// xlTimeout bounds every wait on a child, so a lock that is never
	// released fails the test instead of hanging the suite.
	xlTimeout = 60 * time.Second

	xlAppenders = 3
	xlPerWriter = 150
	// xlSeedGone is how many records of the erased session a file starts with.
	xlSeedGone = 500
	xlSession  = "a"
	xlGone     = "gone"
)

// TestLockHelperProcess is not a test. It is the child side of the TestXL
// tests, entered only when xlHelperEnv names a mode.
func TestLockHelperProcess(t *testing.T) {
	mode := os.Getenv(xlHelperEnv)
	if mode == "" {
		return
	}
	switch mode {
	case "append":
		s, err := ledger.Open(os.Getenv(xlDirEnv))
		if err != nil {
			fmt.Fprintln(os.Stderr, "open:", err)
			os.Exit(1)
		}
		n, err := strconv.Atoi(os.Getenv(xlCountEnv))
		if err != nil {
			fmt.Fprintln(os.Stderr, "count:", err)
			os.Exit(1)
		}
		for i := 0; i < n; i++ {
			rec := ledger.Record{SessionID: os.Getenv(xlSessionEnv), RequestID: fmt.Sprintf("%s-%d", os.Getenv(xlPrefixEnv), i)}
			if err := s.Append(rec); err != nil {
				fmt.Fprintln(os.Stderr, "append:", err)
				os.Exit(1)
			}
		}
		os.Exit(0)
	case "hold":
		f, err := ledger.OpenLocked(os.Getenv(xlPathEnv), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "lock:", err)
			os.Exit(1)
		}
		// With xlPartialEnv set this is an appender caught mid-record: half
		// of it is written now, the rest on release.
		partial := os.Getenv(xlPartialEnv) != ""
		head, tail := inflightHalves()
		if partial {
			if _, err := f.WriteString(head); err != nil {
				fmt.Fprintln(os.Stderr, "write:", err)
				os.Exit(1)
			}
		}
		fmt.Println(xlLockedBanner)
		// Held until the parent closes stdin, or kills this process.
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, "stdin:", err)
		}
		if partial {
			if _, err := f.WriteString(tail); err != nil {
				fmt.Fprintln(os.Stderr, "write:", err)
				os.Exit(1)
			}
		}
		if err := f.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "release:", err)
			os.Exit(1)
		}
		os.Exit(0)
	case "count":
		// A read-modify-write under the lock: read the file, append the
		// number of lines it has. Without exclusion two processes read the
		// same count and one number appears twice.
		path := os.Getenv(xlPathEnv)
		n, err := strconv.Atoi(os.Getenv(xlCountEnv))
		if err != nil {
			fmt.Fprintln(os.Stderr, "count:", err)
			os.Exit(1)
		}
		for i := 0; i < n; i++ {
			if err := countOnce(path); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		os.Exit(0)
	case "purge":
		// purge --session's walk for one file, then the binary's own locked
		// rewrite.
		path := os.Getenv(xlPathEnv)
		read, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(1)
		}
		// With xlPauseEnv set, the rewrite stops inside its critical section
		// (lock held, file re-read), says so, and waits for stdin to close:
		// a purge caught mid-rewrite. Where it stops:
		//   write   before the replacement is written
		//   partial with half the replacement written to the sibling
		//   rename  with the replacement written, before the rename
		writeFile, rename := xlWriteFile, xlRename
		switch os.Getenv(xlPauseEnv) {
		case "":
		case "partial":
			writeFile = func(name string, data []byte, perm os.FileMode) error {
				if err := os.WriteFile(name, data[:len(data)/2], perm); err != nil {
					return err
				}
				pauseHere()
				return xlWriteFile(name, data, perm)
			}
		case "rename":
			rename = func(from, to string) error {
				pauseHere()
				return xlRename(from, to)
			}
		default:
			writeFile = func(name string, data []byte, perm os.FileMode) error {
				pauseHere()
				return xlWriteFile(name, data, perm)
			}
		}
		if err := ledger.RewriteLocked(path, read, keptLines(read, os.Getenv(xlSessionEnv)), writeFile, rename); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	default:
		fmt.Fprintln(os.Stderr, "unknown helper mode", mode)
		os.Exit(2)
	}
}

// xlWriteFile and xlRename are what the purge helper writes and renames
// with: purge's own defaults.
var (
	xlWriteFile = ledger.WriteFile
	xlRename    = ledger.Rename
)

// pauseHere tells the parent the helper is at its pause point, holding the
// lock, and waits until the parent closes stdin.
func pauseHere() {
	fmt.Println(xlLockedBanner)
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "stdin:", err)
	}
}

// countOnce appends, under the lock, the number of lines the file holds.
func countOnce(path string) error {
	f, err := ledger.OpenLocked(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		_ = f.Close() // the read error is the one worth reporting
		return fmt.Errorf("read: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%d\n", bytes.Count(b, []byte("\n"))); err != nil {
		_ = f.Close() // the write error is the one worth reporting
		return fmt.Errorf("write: %w", err)
	}
	return f.Close()
}

// keptLines is what purge --session keeps of a file: every non-blank line
// that is not a record of the erased session (cmd/replay/purge.go).
func keptLines(body []byte, erased string) []string {
	var kept []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal([]byte(line), &rec) == nil && rec.SessionID == erased {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// inflightHalves is one complete record split in two, as a holder in
// partial mode writes it.
func inflightHalves() (string, string) {
	line, err := json.Marshal(ledger.Record{Schema: ledger.SchemaVersion, SessionID: xlSession, RequestID: xlInflight})
	if err != nil {
		panic(err) // a fixed struct always encodes
	}
	full := string(line) + "\n"
	return full[:len(full)/2], full[len(full)/2:]
}

// child is a running helper process.
type child struct {
	cmd    *exec.Cmd
	stderr *bytes.Buffer
	done   chan error
}

// startHelper starts this test binary in a helper mode. prepare, if set,
// attaches stdio before the start; anything it leaves unset is discarded.
func startHelper(t *testing.T, prepare func(*exec.Cmd), env ...string) *child {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$", "-test.count=1")
	cmd.Env = append(baseEnv(), env...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if prepare != nil {
		prepare(cmd)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	c := &child{cmd: cmd, stderr: &errb, done: make(chan error, 1)}
	go func() { c.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill() // already exited in every passing path; this only stops a leak
	})
	return c
}

func startAppender(t *testing.T, dir, prefix string, n int) *child {
	return startHelper(t, nil,
		xlHelperEnv+"=append", xlDirEnv+"="+dir, xlSessionEnv+"="+xlSession,
		xlPrefixEnv+"="+prefix, xlCountEnv+"="+strconv.Itoa(n))
}

// startHolder starts a process holding the lock on path and returns once it
// says so. Closing the returned writer makes it release and exit. env is
// appended after the hold mode, and exec keeps the last value of a repeated
// variable, so a caller can start another lock-holding mode (purge, paused).
//
// Operating-system pipes from StdinPipe and StdoutPipe, not io.Pipe: with an
// io.Pipe, Wait also waits for the goroutine copying into the child's stdin,
// which never ends once the child is killed.
func startHolder(t *testing.T, path string, env ...string) (*child, io.WriteCloser) {
	t.Helper()
	var stdin io.WriteCloser
	var stdout io.ReadCloser
	c := startHelper(t, func(cmd *exec.Cmd) {
		var err error
		if stdin, err = cmd.StdinPipe(); err != nil {
			t.Fatal(err)
		}
		if stdout, err = cmd.StdoutPipe(); err != nil {
			t.Fatal(err)
		}
	}, append([]string{xlHelperEnv + "=hold", xlPathEnv + "=" + path}, env...)...)
	locked := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		locked <- strings.TrimSpace(line)
	}()
	select {
	case line := <-locked:
		if line != xlLockedBanner {
			t.Fatalf("holder did not take the lock: %q, stderr %s", line, c.stderr)
		}
	case err := <-c.done:
		t.Fatalf("holder exited before taking the lock: %v, stderr %s", err, c.stderr)
	case <-time.After(xlTimeout):
		t.Fatal("holder never took the lock")
	}
	return c, stdin
}

func (c *child) wait(t *testing.T, what string) {
	t.Helper()
	select {
	case err := <-c.done:
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", what, err, c.stderr)
		}
	case <-time.After(xlTimeout):
		t.Fatalf("%s did not finish within %s", what, xlTimeout)
	}
}

// xlLedger makes a ledger directory with its label key in place, so helper
// processes opening it at once do not race to create the key, and seeds
// a.jsonl with the given bytes.
func xlLedger(t *testing.T, seed []byte) (dir, path string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), ".replay", "ledger")
	if _, err := ledger.Open(dir); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, xlSession+".jsonl")
	if seed != nil {
		if err := os.WriteFile(path, seed, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, path
}

// seedLines is n records for session, one per line, as purge reads them.
func seedLines(t *testing.T, session, prefix string, n int) []byte {
	t.Helper()
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		line, err := json.Marshal(ledger.Record{Schema: ledger.SchemaVersion, SessionID: session, RequestID: fmt.Sprintf("%s-%d", prefix, i)})
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// xlLines splits a ledger file into its lines, requiring the final newline.
func xlLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasSuffix(s, "\n") {
		t.Fatalf("the ledger does not end on a newline: ...%q", s[max(0, len(s)-80):])
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// countIDs decodes every line except those in allow and counts request ids
// per session. An undecodable line not in allow fails the test.
func countIDs(t *testing.T, lines []string, allow map[string]bool) map[string]map[string]int {
	t.Helper()
	got := map[string]map[string]int{}
	for _, l := range lines {
		if allow[l] {
			continue
		}
		var rec ledger.Record
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatalf("an unreadable line in the ledger: %q", l)
		}
		if got[rec.SessionID] == nil {
			got[rec.SessionID] = map[string]int{}
		}
		got[rec.SessionID][rec.RequestID]++
	}
	return got
}

func wantEachOnce(t *testing.T, ids map[string]int, prefix string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%s-%d", prefix, i)
		if ids[id] != 1 {
			t.Errorf("record %s appears %d times, want 1", id, ids[id])
		}
	}
}

// XL1: processes appending to one file at once lose nothing.
//
// Each process repeatedly takes the lock, reads the file and appends its line
// count. Under the lock the file is 0, 1, 2, ... with no gap or repeat;
// without it two processes read the same count, so a number repeats and the
// sequence breaks. O_APPEND alone keeps every write whole and cannot keep
// this sequence, so the test fails without the lock.
//
// PASS: the file is exactly 0..N-1 in order.
// FAIL: a repeated or missing number: an update lost to a concurrent writer.
func TestXL1_ProcessesAppendingConcurrentlyLoseNothing(t *testing.T) {
	_, path := xlLedger(t, nil)
	var cs []*child
	for p := 0; p < xlAppenders; p++ {
		cs = append(cs, startHelper(t, nil, xlHelperEnv+"=count", xlPathEnv+"="+path, xlCountEnv+"="+strconv.Itoa(xlPerWriter)))
	}
	for p, c := range cs {
		c.wait(t, fmt.Sprintf("counter %d", p))
	}
	lines := xlLines(t, path)
	if len(lines) != xlAppenders*xlPerWriter {
		t.Fatalf("%d lines, want %d: updates were lost", len(lines), xlAppenders*xlPerWriter)
	}
	for i, l := range lines {
		if l != strconv.Itoa(i) {
			t.Fatalf("line %d is %q: two processes read the same state and one update was lost", i, l)
		}
	}
}

// purger is one way of running `purge --session gone` on a ledger as a
// separate process: the production binary, or a helper process running the
// binary's locked rewrite.
type purger struct {
	name  string
	start func(t *testing.T, dir, path string) <-chan result
}

// rewritePurger is a helper process running purge's locked rewrite.
var rewritePurger = purger{name: "rewrite-process", start: func(t *testing.T, _, path string) <-chan result {
	c := startHelper(t, nil, xlHelperEnv+"=purge", xlPathEnv+"="+path, xlSessionEnv+"="+xlGone)
	ch := make(chan result, 1)
	go func() {
		err := <-c.done
		code := 0
		if err != nil {
			code = 1
		}
		ch <- result{Exit: code, Stderr: c.stderr.String()}
	}()
	return ch
}}

func purgers(t *testing.T) []purger {
	t.Helper()
	ps := []purger{rewritePurger}
	// The binary refuses on Windows (cmd/replay/platformguard.go), so there
	// the rewrite process above is the purge, and it is the same code.
	if runtime.GOOS != "windows" {
		bin := productionBinary(t).Path
		ps = append(ps, purger{name: "binary", start: func(t *testing.T, dir, _ string) <-chan result {
			home := filepath.Dir(filepath.Dir(dir))
			ch := make(chan result, 1)
			go func() { ch <- run(bin, home, "", "purge", dir, "--session", xlGone, "--yes") }()
			return ch
		}})
	}
	return ps
}

// purgeOK accepts a purge that completed, or that refused the file because it
// read a record still being written; a refusal removes nothing and says to
// run it again.
func purgeOK(t *testing.T, r result) {
	t.Helper()
	if r.Exit != 0 && !strings.Contains(r.Stderr+r.Stdout, "run the erasure again") {
		t.Fatalf("the purge failed: exit %d\n%s%s", r.Exit, r.Stdout, r.Stderr)
	}
}

// finishPurge runs one more purge, now that nothing else writes, and checks
// the outcome: the erased session gone, every kept and appended record there
// exactly once, every line a record.
func finishPurge(t *testing.T, p purger, dir, path string, want map[string]int) {
	t.Helper()
	select {
	case r := <-p.start(t, dir, path):
		if r.Exit != 0 {
			t.Fatalf("the follow-up purge failed: exit %d\n%s%s", r.Exit, r.Stdout, r.Stderr)
		}
	case <-time.After(xlTimeout):
		t.Fatal("the follow-up purge did not finish")
	}
	got := countIDs(t, xlLines(t, path), nil)
	if n := len(got[xlGone]); n != 0 {
		t.Errorf("%d records of the erased session survived", n)
	}
	for id, n := range want {
		if got[xlSession][id] != n {
			t.Errorf("record %s appears %d times, want %d", id, got[xlSession][id], n)
		}
	}
	if len(got[xlSession]) != len(want) {
		t.Errorf("%d distinct kept records, want %d", len(got[xlSession]), len(want))
	}
}

func idsOf(prefix string, n int, into map[string]int) map[string]int {
	for i := 0; i < n; i++ {
		into[fmt.Sprintf("%s-%d", prefix, i)] = 1
	}
	return into
}

// XL2: an append racing a purge. A process is mid-record, holding the lock,
// when the purge starts.
//
// PASS: the purge is still running xlBlockWindow later; once the appender
// finishes, the purge completes (or refuses and a rerun completes) and the
// appended record is there, whole, once, with every kept record.
// FAIL: the purge finishes while the appender holds the lock (it did not
// wait, so it could rename the record away), or a record is lost or split.
func TestXL2_AppendRacingPurgeLosesNothing(t *testing.T) {
	for _, p := range purgers(t) {
		t.Run(p.name, func(t *testing.T) {
			seed := append(seedLines(t, xlSession, "kept", xlPerWriter), seedLines(t, xlGone, "gone", xlSeedGone)...)
			dir, path := xlLedger(t, seed)
			holder, release := startHolder(t, path, xlPartialEnv+"=1")
			done := p.start(t, dir, path)
			select {
			case r := <-done:
				t.Fatalf("the purge finished while an appender held the lock: exit %d\n%s%s", r.Exit, r.Stdout, r.Stderr)
			case <-time.After(xlBlockWindow):
			}
			if err := release.Close(); err != nil {
				t.Fatal(err)
			}
			holder.wait(t, "appender")
			select {
			case r := <-done:
				purgeOK(t, r)
			case <-time.After(xlTimeout):
				t.Fatal("the purge never got the lock")
			}
			finishPurge(t, p, dir, path, idsOf("kept", xlPerWriter, map[string]int{xlInflight: 1}))
		})
	}
}

// XL3: a purge racing an append. The purge is a real process caught
// mid-rewrite, inside ledger.RewriteLocked (the binary's own rewrite) with
// the lock held and the file already re-read, when an appender starts.
//
// The binary cannot be paused there, so this uses the rewrite process on
// every platform; XL2 races the binary itself.
//
// PASS: the appender is still waiting xlBlockWindow later; afterwards the
// erased session is gone and the appended record and every kept one are
// there exactly once.
// FAIL: the appender wrote into the file the rewrite is about to rename
// over, and its record is lost.
func TestXL3_PurgeRacingAppendLosesNothing(t *testing.T) {
	seed := append(seedLines(t, xlSession, "kept", xlPerWriter), seedLines(t, xlGone, "gone", xlSeedGone)...)
	dir, path := xlLedger(t, seed)
	purge, release := startHolder(t, path, xlHelperEnv+"=purge", xlSessionEnv+"="+xlGone, xlPauseEnv+"=1")
	a := startAppender(t, dir, "during", 1)
	select {
	case err := <-a.done:
		t.Fatalf("the appender wrote while a purge held the lock mid-rewrite (err=%v)", err)
	case <-time.After(xlBlockWindow):
	}
	if err := release.Close(); err != nil {
		t.Fatal(err)
	}
	purge.wait(t, "purge")
	a.wait(t, "appender")
	got := countIDs(t, xlLines(t, path), nil)
	if n := len(got[xlGone]); n != 0 {
		t.Errorf("%d records of the erased session survived", n)
	}
	want := idsOf("kept", xlPerWriter, map[string]int{"during-0": 1})
	for id, n := range want {
		if got[xlSession][id] != n {
			t.Errorf("record %s appears %d times, want %d", id, got[xlSession][id], n)
		}
	}
	if len(got[xlSession]) != len(want) {
		t.Errorf("%d distinct kept records, want %d", len(got[xlSession]), len(want))
	}
}

// XL4: the torn-tail repair racing an append.
//
// One process holds the lock with half a record written. To a second
// appender that does not wait, that half is a torn tail: it would end it with
// a newline and split a live record in two. Under the lock the second waits,
// and finds a clean tail once the first finishes.
//
// PASS: the second appender is still waiting xlBlockWindow later; afterwards
// the file is exactly the two records, whole, with no blank line.
// FAIL: it wrote while the first held the lock, or the record was split.
func TestXL4_TornTailRepairRacingAppendLosesNothing(t *testing.T) {
	dir, path := xlLedger(t, nil)
	head, _ := inflightHalves()
	holder, release := startHolder(t, path, xlPartialEnv+"=1")
	a := startAppender(t, dir, "waiter", 1)
	select {
	case err := <-a.done:
		t.Fatalf("the appender wrote while a record was in flight under the lock (err=%v)", err)
	case <-time.After(xlBlockWindow):
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != head {
		t.Fatalf("the in-flight record was touched while held: %q (err=%v)", b, err)
	}
	if err := release.Close(); err != nil {
		t.Fatal(err)
	}
	holder.wait(t, "holder")
	a.wait(t, "appender")
	lines := xlLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("%d lines, want the two records: %q", len(lines), lines)
	}
	ids := countIDs(t, lines, nil)[xlSession]
	if ids[xlInflight] != 1 || ids["waiter-0"] != 1 {
		t.Fatalf("records %v, want %s and waiter-0 once each", ids, xlInflight)
	}
}

// waitsWhileHeld starts an appender while a holder has the lock and requires
// it to be still waiting, with nothing written, after xlBlockWindow.
func waitsWhileHeld(t *testing.T, dir, path string) *child {
	t.Helper()
	a := startAppender(t, dir, "waiter", 1)
	select {
	case err := <-a.done:
		t.Fatalf("the appender finished while another process held the lock (err=%v)\n%s", err, a.stderr)
	case <-time.After(xlBlockWindow):
	}
	if b, err := os.ReadFile(path); err != nil || len(b) != 0 {
		t.Fatalf("something was written while the lock was held: %q (err=%v)", b, err)
	}
	return a
}

// XL5: a second process waits while the first holds the lock, and proceeds
// when it releases.
//
// PASS: the appender is still waiting after xlBlockWindow with the file
// empty, then completes once the holder releases, and its record is there.
// FAIL: it writes while the lock is held, or never gets the lock.
func TestXL5_ASecondProcessWaitsWhileTheFirstHoldsTheLock(t *testing.T) {
	dir, path := xlLedger(t, nil)
	holder, release := startHolder(t, path)
	a := waitsWhileHeld(t, dir, path)
	if err := release.Close(); err != nil {
		t.Fatal(err)
	}
	holder.wait(t, "holder")
	a.wait(t, "appender after release")
	wantEachOnce(t, countIDs(t, xlLines(t, path), nil)[xlSession], "waiter", 1)
}

// XL6: a process killed while holding the lock does not leave the file
// locked. The operating system releases the lock when the process dies.
//
// PASS: the appender, waiting while the holder lives, completes after the
// holder is killed.
// FAIL: it did not wait (no lock), or is still waiting xlTimeout later.
func TestXL6_AProcessKilledHoldingTheLockDoesNotLeaveItLocked(t *testing.T) {
	dir, path := xlLedger(t, nil)
	holder, _ := startHolder(t, path)
	a := waitsWhileHeld(t, dir, path)
	if err := holder.cmd.Process.Kill(); err != nil {
		t.Fatalf("kill holder: %v", err)
	}
	select {
	case <-holder.done: // killed: a non-nil error is expected
	case <-time.After(xlTimeout):
		t.Fatal("the killed holder did not exit")
	}
	a.wait(t, "appender after the holder was killed")
	wantEachOnce(t, countIDs(t, xlLines(t, path), nil)[xlSession], "waiter", 1)
}

// xlRepeatRounds is how many times XL7 repeats a full append/purge race. On
// windows-latest this is the bounded repeat the Windows purge is held to.
const (
	xlRepeatRounds    = 20
	xlRepeatPerWriter = 50
)

// XL7: repeated concurrent append and purge lose nothing.
//
// xlRepeatRounds rounds, each a purge process and xlAppenders appending
// processes started together on a fresh ledger.
//
// PASS: every round, the erased session is gone after a follow-up purge and
// every kept and appended record is there exactly once.
// FAIL: any round losing, duplicating or splitting a record, or a purge that
// fails for any reason but a read of an in-flight record.
func TestXL7_RepeatedAppendAndPurgeRacesLoseNothing(t *testing.T) {
	for r := 0; r < xlRepeatRounds; r++ {
		seed := append(seedLines(t, xlSession, "kept", xlPerWriter), seedLines(t, xlGone, "gone", xlSeedGone)...)
		dir, path := xlLedger(t, seed)
		done := rewritePurger.start(t, dir, path)
		var cs []*child
		for a := 0; a < xlAppenders; a++ {
			cs = append(cs, startAppender(t, dir, fmt.Sprintf("r%dp%d", r, a), xlRepeatPerWriter))
		}
		for a, c := range cs {
			c.wait(t, fmt.Sprintf("round %d appender %d", r, a))
		}
		select {
		case res := <-done:
			purgeOK(t, res)
		case <-time.After(xlTimeout):
			t.Fatalf("round %d: the purge did not finish", r)
		}
		want := idsOf("kept", xlPerWriter, map[string]int{})
		for a := 0; a < xlAppenders; a++ {
			idsOf(fmt.Sprintf("r%dp%d", r, a), xlRepeatPerWriter, want)
		}
		finishPurge(t, rewritePurger, dir, path, want)
	}
}

// XL8: a purge killed mid-rewrite loses nothing and does not leave the
// ledger locked.
//
// The purge process is killed holding the lock, with half its replacement
// written to the sibling and nothing renamed. An appender waiting on the
// lock must then get it.
//
// PASS: the appender waits while the purge lives and completes after the
// kill; the ledger is the original plus the appended record (nothing erased,
// nothing lost, nothing twice); a fresh purge then completes and leaves no
// sibling behind.
// FAIL: the lock outlives the process, or the half-written replacement
// reached the ledger, or a record is lost or duplicated.
func TestXL8_APurgeKilledMidRewriteLosesNothingAndReleasesTheLock(t *testing.T) {
	seed := append(seedLines(t, xlSession, "kept", xlPerWriter), seedLines(t, xlGone, "gone", xlSeedGone)...)
	dir, path := xlLedger(t, seed)
	purge, _ := startHolder(t, path, xlHelperEnv+"=purge", xlSessionEnv+"="+xlGone, xlPauseEnv+"=partial")
	a := startAppender(t, dir, "during", 1)
	select {
	case err := <-a.done:
		t.Fatalf("the appender wrote while the purge held the lock (err=%v)", err)
	case <-time.After(xlBlockWindow):
	}
	if err := purge.cmd.Process.Kill(); err != nil {
		t.Fatalf("kill purge: %v", err)
	}
	select {
	case <-purge.done: // killed: a non-nil error is expected
	case <-time.After(xlTimeout):
		t.Fatal("the killed purge did not exit")
	}
	a.wait(t, "appender after the purge was killed")

	got := countIDs(t, xlLines(t, path), nil)
	wantEachOnce(t, got[xlGone], "gone", xlSeedGone)
	if len(got[xlGone]) != xlSeedGone {
		t.Errorf("%d erased-session records, want all %d: an interrupted purge removed some", len(got[xlGone]), xlSeedGone)
	}
	finishPurge(t, rewritePurger, dir, path, idsOf("kept", xlPerWriter, map[string]int{"during-0": 1}))
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("the sibling is still there after a completed purge (err=%v)", err)
	}
}

// XL9: a leftover sibling from an interrupted replacement does not corrupt
// the ledger, and the next append and purge proceed.
//
// PASS: with a half-written a.jsonl.tmp beside it, the ledger reads with no
// skipped or incomplete record; an append lands; a purge completes and
// replaces the leftover.
// FAIL: the leftover is read as part of the ledger, or blocks the append or
// the purge.
func TestXL9_ALeftoverReplacementDoesNotCorruptTheLedger(t *testing.T) {
	for _, p := range purgers(t) {
		t.Run(p.name, func(t *testing.T) {
			seed := append(seedLines(t, xlSession, "kept", xlPerWriter), seedLines(t, xlGone, "gone", xlSeedGone)...)
			dir, path := xlLedger(t, seed)
			if err := os.WriteFile(path+".tmp", seed[:len(seed)/3], 0o600); err != nil {
				t.Fatal(err)
			}
			recs, skipped, mismatch, incomplete, err := ledger.ReadRecords(path)
			if err != nil || skipped != 0 || mismatch != 0 || incomplete || len(recs) != xlPerWriter+xlSeedGone {
				t.Fatalf("the ledger did not read cleanly beside a leftover: %d records, skipped %d, mismatch %d, incomplete %v, err %v",
					len(recs), skipped, mismatch, incomplete, err)
			}
			startAppender(t, dir, "after", 1).wait(t, "appender beside a leftover")
			finishPurge(t, p, dir, path, idsOf("kept", xlPerWriter, map[string]int{"after-0": 1}))
			if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
				t.Errorf("the leftover sibling survived a completed purge (err=%v)", err)
			}
		})
	}
}

// XL10: the rename happens while the lock is held.
//
// The purge process stops with its replacement written and the rename not
// yet made. Releasing the lock any earlier (before the rename) would let a
// waiting appender write into the file about to be renamed over, and its
// record would be lost: S-3. This is the regression guard for that order.
//
// PASS: the appender is still waiting at that point and the ledger is
// unchanged; afterwards the erased session is gone and the appended record
// is there once.
// FAIL: the appender got in before the rename, or its record was lost.
func TestXL10_TheRenameHappensUnderTheLock(t *testing.T) {
	seed := append(seedLines(t, xlSession, "kept", xlPerWriter), seedLines(t, xlGone, "gone", xlSeedGone)...)
	dir, path := xlLedger(t, seed)
	purge, release := startHolder(t, path, xlHelperEnv+"=purge", xlSessionEnv+"="+xlGone, xlPauseEnv+"=rename")
	a := startAppender(t, dir, "during", 1)
	select {
	case err := <-a.done:
		t.Fatalf("an appender got the lock before the purge renamed: the lock was released too early (err=%v)", err)
	case <-time.After(xlBlockWindow):
	}
	if b, err := os.ReadFile(path); err != nil || !bytes.Equal(b, seed) {
		t.Fatalf("the ledger changed before the rename (err=%v)", err)
	}
	if err := release.Close(); err != nil {
		t.Fatal(err)
	}
	purge.wait(t, "purge")
	a.wait(t, "appender")
	got := countIDs(t, xlLines(t, path), nil)
	if n := len(got[xlGone]); n != 0 {
		t.Errorf("%d records of the erased session survived", n)
	}
	want := idsOf("kept", xlPerWriter, map[string]int{"during-0": 1})
	for id, n := range want {
		if got[xlSession][id] != n {
			t.Errorf("record %s appears %d times, want %d", id, got[xlSession][id], n)
		}
	}
}
