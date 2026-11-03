package ledger

import (
	"errors"
	"os"
	"strings"
	"unicode/utf16"
	"unsafe"
)

// Opening and replacing ledger files so that a rename over an open file works
// on Windows (ADR-0030, option (a)).
//
// purge --session replaces a ledger by renaming a sibling over it while it
// holds the writers' lock, and appenders waiting for that lock hold the file
// open too. Windows refuses to replace a file that any handle has open without
// FILE_SHARE_DELETE, and os.OpenFile does not pass it. So on Windows every
// ledger handle is opened through CreateFile with read, write and delete
// sharing, and the replacement is a rename with POSIX semantics
// (FileRenameInfoEx), which unlinks the old file while its handles stay valid,
// as on unix. On every other platform openFile is os.OpenFile and renameFile
// is os.Rename, unchanged.
//
// What follows is the platform-independent part, kept untagged so that it is
// tested and scored on every host; fileopen_windows.go holds only the calls.

// Win32 values used by the Windows side, named here so the mapping below is
// testable everywhere.
const (
	winGenericRead      = 0x80000000
	winGenericWrite     = 0x40000000
	winFileAppendData   = 0x00000004
	winDelete           = 0x00010000
	winSynchronize      = 0x00100000
	winShareAll         = 0x00000001 | 0x00000002 | 0x00000004 // READ | WRITE | DELETE
	winCreateNew        = 1
	winCreateAlways     = 2
	winOpenExisting     = 3
	winOpenAlways       = 4
	winTruncateExisting = 5
	winAttrReadonly     = 0x00000001
	winAttrNormal       = 0x00000080
	// winRenameReplace and winRenamePosix are FILE_RENAME_FLAG_REPLACE_IF_EXISTS
	// and FILE_RENAME_FLAG_POSIX_SEMANTICS.
	winRenameReplace = 0x00000001
	winRenamePosix   = 0x00000002
	// ownerWriteBit is the permission bit whose absence makes a new file
	// read-only on Windows, as syscall.Open treats it.
	ownerWriteBit = 0o200
)

// createArgs maps os.OpenFile's flag and perm to CreateFile's access,
// creation disposition and attributes, the same mapping syscall.Open makes.
func createArgs(flag int, perm os.FileMode) (access, disposition, attrs uint32) {
	switch flag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_WRONLY:
		access = winGenericWrite
	case os.O_RDWR:
		access = winGenericRead | winGenericWrite
	default:
		access = winGenericRead
	}
	if flag&os.O_APPEND != 0 {
		access = access&^winGenericWrite | winFileAppendData
	}
	switch {
	case flag&(os.O_CREATE|os.O_EXCL) == os.O_CREATE|os.O_EXCL:
		disposition = winCreateNew
	case flag&(os.O_CREATE|os.O_TRUNC) == os.O_CREATE|os.O_TRUNC:
		disposition = winCreateAlways
	case flag&os.O_CREATE != 0:
		disposition = winOpenAlways
	case flag&os.O_TRUNC != 0:
		disposition = winTruncateExisting
	default:
		disposition = winOpenExisting
	}
	attrs = winAttrNormal
	if flag&os.O_CREATE != 0 && perm&ownerWriteBit == 0 {
		attrs = winAttrReadonly
	}
	return access, disposition, attrs
}

// openShared opens path through create, which makes the CreateFile call,
// and wraps the handle as an *os.File.
func openShared(path string, flag int, perm os.FileMode, create func(name string, access, disposition, attrs uint32) (uintptr, error)) (*os.File, error) {
	access, disposition, attrs := createArgs(flag, perm)
	h, err := create(path, access, disposition, attrs)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(h, path), nil
}

// ifOK runs f only when err is nil, and otherwise returns err.
func ifOK[T any](err error, f func() (T, error)) (T, error) {
	if err != nil {
		var zero T
		return zero, err
	}
	return f()
}

// renameByHandle renames from to to by opening from, setting its new name
// through setName, and closing it. Both the rename's and the close's
// failures are reported.
func renameByHandle(from, to string, open func(string) (uintptr, error), setName func(h uintptr, to string) error, closeHandle func(uintptr) error) error {
	h, err := open(from)
	if err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	serr := setName(h, to)
	cerr := closeHandle(h)
	if serr != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: serr}
	}
	return cerr
}

// renameWith tries primary and falls back only when it reports the
// operation unsupported (a filesystem or Windows build without POSIX rename
// semantics). Any other failure is returned as it is: the fallback cannot
// do better, and must not hide it.
func renameWith(primary, fallback func() error, unsupported func(error) bool) error {
	err := primary()
	if err == nil || !unsupported(err) {
		return err
	}
	return fallback()
}

// fileRenameInfo is FILE_RENAME_INFO's layout: Go aligns these fields as C
// does on both 32- and 64-bit, with FileName running past the struct.
type fileRenameInfo struct {
	Flags          uint32
	RootDirectory  uintptr
	FileNameLength uint32
	FileName       [1]uint16
}

// renameInfo builds a FILE_RENAME_INFO naming to, NUL-terminated, in an
// 8-byte-aligned buffer, and returns it with the size to pass.
func renameInfo(to string, flags uint32) ([]uint64, uint32) {
	name := utf16.Encode([]rune(to))
	off := unsafe.Offsetof(fileRenameInfo{}.FileName)
	size := off + uintptr(len(name)+1)*2
	const word = 8
	buf := make([]uint64, (size+word-1)/word)
	info := (*fileRenameInfo)(unsafe.Pointer(&buf[0]))
	info.Flags = flags
	info.FileNameLength = uint32(len(name) * 2)
	copy(unsafe.Slice((*uint16)(unsafe.Add(unsafe.Pointer(info), off)), len(name)+1), name)
	return buf, uint32(size)
}

// verbatimPath prefixes an absolute drive path with \\?\, so the name is
// taken literally and long paths work. A path already in \\ form (UNC or
// verbatim) is returned unchanged.
func verbatimPath(abs string) string {
	if strings.HasPrefix(abs, `\\`) {
		return abs
	}
	return `\\?\` + abs
}

// WriteFile is os.WriteFile through openFile, so on Windows the file is
// opened with delete sharing like every other ledger handle. purge writes
// its replacement with it.
func WriteFile(name string, data []byte, perm os.FileMode) error {
	f, err := openFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	return errors.Join(werr, f.Close())
}

// Rename replaces to with from: os.Rename everywhere but Windows, where it
// is a POSIX-semantics rename that succeeds while to is open (renameFile).
func Rename(from, to string) error { return renameFile(from, to) }
