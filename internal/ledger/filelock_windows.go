//go:build windows

package ledger

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// The Windows lock is LockFileEx with LOCKFILE_EXCLUSIVE_LOCK, from kernel32
// through the standard library's syscall package: the module has no
// dependencies and golang.org/x/sys would be its first.
//
// kernel32.dll is on the KnownDLLs list, so the loader maps the system copy
// and never searches the working directory for it: NewLazyDLL's preloading
// caveat does not apply to it.
var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
	// GetOverlappedResult is not in package syscall, so it is loaded the
	// same way.
	procGetOverlappedResult = modkernel32.NewProc("GetOverlappedResult")
)

const (
	// lockfileExclusiveLock is LOCKFILE_EXCLUSIVE_LOCK. LOCKFILE_FAIL_IMMEDIATELY
	// is deliberately absent, so the call blocks until the lock is granted,
	// as flock(LOCK_EX) does on unix. Holders are bounded by the work they do
	// (one record, or one purge rewrite), and a killed holder's lock is
	// released by the system, so a wait ends; a bounded retry would add a
	// failure mode, an Append dropped under load, for no case the blocking
	// call leaves open.
	lockfileExclusiveLock = 0x00000002

	// lockRangeOffset and lockRangeLength are the byte range locked. A
	// LockFileEx range is MANDATORY: no other handle may read or write bytes
	// inside it while it is held. Locking the file's data would make every
	// reader that does not take the lock (ReadRecords, IsLedgerFile, purge's
	// walk) fail with ERROR_LOCK_VIOLATION during an append. One byte far
	// beyond any ledger's size is locked instead; Windows permits locking
	// past end of file, and every writer locks this same byte, so it excludes
	// writers and nothing else. TestFL4 checks the reader is not blocked.
	lockRangeOffset uint64 = 1 << 62
	lockRangeLength uint32 = 1

	// lowWordMask selects the low 32 bits of a 64-bit offset.
	lowWordMask = 0xFFFFFFFF
	// waitTrue is the Win32 BOOL TRUE passed as GetOverlappedResult's bWait.
	waitTrue = 1
)

// lockRange is a fresh OVERLAPPED naming the locked range. Only Offset and
// OffsetHigh are read for a synchronous handle; the rest stays zero, as the
// API requires.
func lockRange() *syscall.Overlapped {
	return &syscall.Overlapped{
		Offset:     uint32(lockRangeOffset & lowWordMask),
		OffsetHigh: uint32(lockRangeOffset >> 32),
	}
}

// lockFile takes the exclusive lock on f, waiting for it.
//
// os.OpenFile opens synchronous handles, so LockFileEx returns only once the
// lock is granted. Should a handle ever be asynchronous, the call returns
// ERROR_IO_PENDING and the wait moves to GetOverlappedResult, which blocks on
// the handle until the same request completes. The OVERLAPPED stays alive for
// the whole wait because it is referenced here until GetOverlappedResult
// returns.
//
// The lock belongs to the handle. Closing it releases the lock, and when a
// process dies the system closes its handles, so a killed holder does not
// leave the file locked (TestXL6).
func lockFile(f *os.File) error {
	h := syscall.Handle(f.Fd())
	ol := lockRange()
	lock := procCall{name: "LockFileEx", find: procLockFileEx.Find, call: func() (uintptr, error) {
		r1, _, e1 := procLockFileEx.Call(
			uintptr(h),
			uintptr(lockfileExclusiveLock),
			0, // reserved, must be zero
			uintptr(lockRangeLength),
			0, // high word of the length
			uintptr(unsafe.Pointer(ol)),
		)
		return r1, e1
	}}
	var transferred uint32
	wait := procCall{name: "GetOverlappedResult", find: procGetOverlappedResult.Find, call: func() (uintptr, error) {
		r1, _, e1 := procGetOverlappedResult.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(ol)),
			uintptr(unsafe.Pointer(&transferred)),
			waitTrue,
		)
		return r1, e1
	}}
	return lockOrWait(lock, wait, func(err error) bool { return errors.Is(err, syscall.ERROR_IO_PENDING) })
}

// unlockFile releases the lock lockFile took on f, over the same range.
func unlockFile(f *os.File) error {
	return procCall{name: "UnlockFileEx", find: procUnlockFileEx.Find, call: func() (uintptr, error) {
		r1, _, e1 := procUnlockFileEx.Call(
			f.Fd(),
			0, // reserved, must be zero
			uintptr(lockRangeLength),
			0, // high word of the length
			uintptr(unsafe.Pointer(lockRange())),
		)
		return r1, e1
	}}.do()
}
