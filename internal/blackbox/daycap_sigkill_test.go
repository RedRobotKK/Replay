//go:build mutation

package blackbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The day cap must survive the process being killed, through the real binary.
//
// internal/proxy's TestDC1 simulates the crash by never calling SaveState in
// the test process. This is the same claim made from outside: the production
// binary, a real `replay serve`, a priced response from a loopback stub, then
// SIGKILL, which no handler, defer or signal.NotifyContext can intercept. A
// second `replay serve` on the same ledger directory (where spend-day.json
// lives) must still refuse a request the first one's spend already put past
// the cap.
//
// It is in this package for the reason every test here is: it starts
// processes, so it imports os/exec, which the `mutation` build tag confines.
//
// Durability here means "survives the process dying", not "survives the
// machine losing power": the state file is renamed into place without an
// fsync of the file or the directory, and nothing below would notice.

const (
	// dcReady bounds the wait for a started serve to answer /replay/healthz.
	dcReady = 10 * time.Second
	// dcRecorded bounds the wait for /replay/status to count the priced
	// response, which serve does only after SpendGuard.Record has returned.
	dcRecorded = 10 * time.Second
	// dcReaped bounds the wait for a killed serve to be reaped.
	dcReaped = 10 * time.Second
	// dcPoll is the interval between readiness probes. It paces a poll on an
	// observable condition; it is never the thing waited for.
	dcPoll = 25 * time.Millisecond
	// dcDayCap is below one stub response (1000 input + 500 output tokens),
	// so the first request is admitted and every later one today is refused.
	dcDayCap = 1000
)

type dcServe struct {
	cmd  *exec.Cmd
	addr string
	log  *bytes.Buffer
	done chan struct{}
}

// dcStart starts `replay serve` with an isolated HOME and ledger and fails the
// test unless it answers /replay/healthz within dcReady.
func dcStart(t *testing.T, bin, home, ledger, upstream string) *dcServe {
	t.Helper()
	addr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	cmd := exec.Command(bin, "serve",
		"--listen", addr,
		"--upstream", upstream,
		"--ledger", ledger,
		"--max-day-tokens", fmt.Sprint(dcDayCap))
	cmd.Env = append(baseEnv(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_CONFIG_DIR=",
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "NO_COLOR=1")
	s := &dcServe{cmd: cmd, addr: addr, log: &bytes.Buffer{}, done: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = s.log, s.log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = cmd.Wait(); close(s.done) }()
	t.Cleanup(func() {
		select {
		case <-s.done:
		default:
			_ = cmd.Process.Kill()
			<-s.done
		}
	})
	deadline := time.Now().Add(dcReady)
	for {
		select {
		case <-s.done:
			t.Fatalf("serve exited before it was ready: %v\n%s", cmd.ProcessState, tail(s.log.String(), 1200))
		default:
		}
		if resp, err := http.Get("http://" + addr + "/replay/healthz"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return s
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve did not answer /replay/healthz within %s\n%s", dcReady, tail(s.log.String(), 1200))
		}
		time.Sleep(dcPoll)
	}
}

// dcAwait2xx waits until /replay/status counts n successful responses. The
// proxy records spend before it counts the response, in the same goroutine,
// so a count of n means n Records (and whatever they persist) have returned.
func dcAwait2xx(t *testing.T, s *dcServe, n int) {
	t.Helper()
	deadline := time.Now().Add(dcRecorded)
	for {
		if resp, err := http.Get("http://" + s.addr + "/replay/status"); err == nil {
			var st struct {
				Requests map[string]int `json:"requests"`
			}
			derr := json.NewDecoder(resp.Body).Decode(&st)
			resp.Body.Close()
			if derr == nil && st.Requests["2xx"] >= n {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("/replay/status did not count %d 2xx response(s) within %s\n%s", n, dcRecorded, tail(s.log.String(), 1200))
		}
		time.Sleep(dcPoll)
	}
}

// dcSigkill sends SIGKILL and waits for the process to be reaped, failing
// unless it died of that signal (on Windows, Kill is TerminateProcess, which
// is equally uncatchable but reports no signal).
func dcSigkill(t *testing.T, s *dcServe) {
	t.Helper()
	if err := s.cmd.Process.Signal(os.Kill); err != nil {
		t.Fatalf("SIGKILL: %v", err)
	}
	select {
	case <-s.done:
	case <-time.After(dcReaped):
		t.Fatalf("serve was not reaped within %s of SIGKILL", dcReaped)
	}
	if runtime.GOOS == "windows" {
		return
	}
	ws, ok := s.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
		t.Fatalf("serve did not die of SIGKILL: %v", s.cmd.ProcessState)
	}
}

// DCK1: a day total recorded before a SIGKILL is still in force after restart.
//
// PASS: the restarted serve refuses a fresh session with replay_spend_cap and
// forwards nothing.
// FAIL: the restarted serve forwards it, because the total died with the
// process (SpendGuard.Record persisting nothing, leaving only runServe's
// deferred SaveState, which a SIGKILL skips).
func TestDCK1_DayCapSurvivesSigkillOfTheServeProcess(t *testing.T) {
	bin := productionBinary(t)
	home := t.TempDir()
	ledger := filepath.Join(home, ".replay", "ledger")
	up := newStub()
	defer up.srv.Close()

	first := dcStart(t, bin.Path, home, ledger, up.srv.URL)
	if code, body := post(first.addr, "before-kill", plainBody("claude-opus-5")); code != http.StatusOK {
		t.Fatalf("precondition: the first request under the day cap was not admitted: %d %s", code, body)
	}
	dcAwait2xx(t, first, 1)
	// Precondition: the cap is in force in the process that recorded it, so a
	// pass after restart cannot come from a cap that never latched.
	if code, body := post(first.addr, "before-kill-2", plainBody("claude-opus-5")); code != http.StatusBadRequest || !strings.Contains(body, "replay_spend_cap") {
		t.Fatalf("precondition: the day cap did not latch before the kill: %d %s", code, body)
	}
	dcSigkill(t, first)

	second := dcStart(t, bin.Path, home, ledger, up.srv.URL)
	hits := up.hits.Load()
	code, body := post(second.addr, "after-restart", plainBody("claude-opus-5"))
	if code != http.StatusBadRequest || !strings.Contains(body, "replay_spend_cap") {
		t.Fatalf("after SIGKILL and restart the day cap reset: a fresh session got %d %s; "+
			"%d tokens of a %d-token day cap were forgotten\nrestarted serve log:\n%s",
			code, strings.TrimSpace(tail(body, 300)), 1500, dcDayCap, tail(second.log.String(), 1200))
	}
	if !strings.Contains(body, "daily spend cap") {
		t.Errorf("refused, but not by the day cap: %s", body)
	}
	if got := up.hits.Load() - hits; got != 0 {
		t.Errorf("the refused request still reached the provider %d time(s)", got)
	}
}
