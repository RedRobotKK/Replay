//go:build windows

package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// The Windows half of fileopen.go: the calls only. What each result means is
// decided in untagged code.

var procSetFileInformationByHandle = modkernel32.NewProc("SetFileInformationByHandle")

const (
	// fileRenameInfoEx is FILE_INFO_BY_HANDLE_CLASS FileRenameInfoEx.
	fileRenameInfoEx = 22
	// What a filesystem or Windows build without FileRenameInfoEx answers:
	// ERROR_NOT_SUPPORTED, ERROR_INVALID_FUNCTION, ERROR_INVALID_PARAMETER.
	errNotSupported     = syscall.Errno(50)
	errInvalidFunction  = syscall.Errno(1)
	errInvalidParameter = syscall.Errno(87) // ERROR_INVALID_PARAMETER
)

// openFile opens a ledger file with FILE_SHARE_READ | FILE_SHARE_WRITE |
// FILE_SHARE_DELETE, so that purge can rename a replacement over it while
// this handle is open.
func openFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return openShared(name, flag, perm, createShared)
}

func createShared(name string, access, disposition, attrs uint32) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(name)
	return ifOK(err, func() (uintptr, error) {
		h, err := syscall.CreateFile(p, access, winShareAll, nil, disposition, attrs, 0)
		return uintptr(h), err
	})
}

// renameFile renames with POSIX semantics, replacing to even while other
// handles (the rewrite's own locked one, waiting appenders) have it open;
// they keep the old file, as on unix. Where that is unsupported it falls back
// to os.Rename, which then fails loudly if to is open, and loses nothing.
func renameFile(from, to string) error {
	return renameWith(
		func() error { return renameByHandle(from, to, openForRename, setRenameInfo, closeHandle) },
		func() error { return os.Rename(from, to) },
		func(err error) bool {
			return errors.Is(err, errInvalidParameter) || errors.Is(err, errNotSupported) || errors.Is(err, errInvalidFunction)
		},
	)
}

func openForRename(name string) (uintptr, error) {
	p, err := syscall.UTF16PtrFromString(name)
	return ifOK(err, func() (uintptr, error) {
		h, err := syscall.CreateFile(p, winDelete|winSynchronize, winShareAll, nil, winOpenExisting, winAttrNormal, 0)
		return uintptr(h), err
	})
}

func setRenameInfo(h uintptr, to string) error {
	abs, err := filepath.Abs(to)
	buf, size := renameInfo(verbatimPath(abs), winRenameReplace|winRenamePosix)
	_, err = ifOK(err, func() (struct{}, error) {
		return struct{}{}, procCall{name: "SetFileInformationByHandle", find: procSetFileInformationByHandle.Find, call: func() (uintptr, error) {
			r1, _, e1 := procSetFileInformationByHandle.Call(h, fileRenameInfoEx, uintptr(unsafe.Pointer(&buf[0])), uintptr(size))
			return r1, e1
		}}.do()
	})
	return err
}

func closeHandle(h uintptr) error { return syscall.CloseHandle(syscall.Handle(h)) }
