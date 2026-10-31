package ledger

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

// PC1-PC6: how the Windows lock reads its kernel32 results, with fakes, so
// every branch is exercised here and not only on windows-latest.

var (
	errPCMissing = errors.New("procedure not found")
	errPCPending = errors.New("pending")
	errPCDenied  = errors.New("denied")
)

func pcFound() error { return nil }

func pcCall(r1 uintptr, err error, calls *int) procCall {
	return procCall{name: "Op", find: pcFound, call: func() (uintptr, error) {
		*calls++
		return r1, err
	}}
}

func pcIsPending(err error) bool { return errors.Is(err, errPCPending) }

func TestPC1_AProcedureThatCannotBeFoundIsNotCalled(t *testing.T) {
	called := false
	err := procCall{name: "Op", find: func() error { return errPCMissing }, call: func() (uintptr, error) {
		called = true
		return 1, nil
	}}.do()
	if !errors.Is(err, errPCMissing) || !strings.Contains(err.Error(), "Op unavailable") || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestPC2_ANonZeroReturnIsSuccess(t *testing.T) {
	var n int
	if err := pcCall(1, syscall.Errno(0), &n).do(); err != nil || n != 1 {
		t.Fatalf("err=%v calls=%d", err, n)
	}
}

func TestPC3_AZeroReturnIsAFailureEvenWithoutAnErrorCode(t *testing.T) {
	var n int
	err := pcCall(0, syscall.Errno(0), &n).do()
	if err == nil || !strings.Contains(err.Error(), "without an error code") {
		t.Fatalf("got %v", err)
	}
	if err := pcCall(0, errPCDenied, &n).do(); !errors.Is(err, errPCDenied) {
		t.Fatalf("the failure's reason was lost: %v", err)
	}
}

func TestPC4_AFailedLockIsReturnedAndNotWaitedOn(t *testing.T) {
	var locks, waits int
	err := lockOrWait(pcCall(0, errPCDenied, &locks), pcCall(1, nil, &waits), pcIsPending)
	if !errors.Is(err, errPCDenied) || waits != 0 {
		t.Fatalf("err=%v waits=%d", err, waits)
	}
	if err := lockOrWait(pcCall(1, nil, &locks), pcCall(1, nil, &waits), pcIsPending); err != nil || waits != 0 {
		t.Fatalf("a granted lock: err=%v waits=%d", err, waits)
	}
}

func TestPC5_APendingLockIsWaitedFor(t *testing.T) {
	var locks, waits int
	if err := lockOrWait(pcCall(0, errPCPending, &locks), pcCall(1, nil, &waits), pcIsPending); err != nil || waits != 1 {
		t.Fatalf("err=%v waits=%d", err, waits)
	}
}

func TestPC6_AFailedWaitIsReported(t *testing.T) {
	var locks, waits int
	err := lockOrWait(pcCall(0, errPCPending, &locks), pcCall(0, errPCDenied, &waits), pcIsPending)
	if !errors.Is(err, errPCDenied) || !strings.Contains(err.Error(), "waiting for the lock") {
		t.Fatalf("got %v", err)
	}
}
