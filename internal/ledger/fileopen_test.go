package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
	"unsafe"
)

// FO1-FO8: the platform-independent half of the Windows open and rename
// (fileopen.go), with fakes where the real call is Windows-only.

func TestFO1_OpenFlagsMapAsSyscallOpenMapsThem(t *testing.T) {
	cases := []struct {
		flag                       int
		perm                       os.FileMode
		access, disposition, attrs uint32
	}{
		{os.O_RDONLY, 0, winGenericRead, winOpenExisting, winAttrNormal},
		{os.O_WRONLY | os.O_CREATE | os.O_TRUNC, 0o600, winGenericWrite, winCreateAlways, winAttrNormal},
		{os.O_RDWR | os.O_CREATE | os.O_APPEND, 0o600, winGenericRead | winFileAppendData, winOpenAlways, winAttrNormal},
		{os.O_WRONLY | os.O_CREATE | os.O_EXCL, 0o400, winGenericWrite, winCreateNew, winAttrReadonly},
		{os.O_RDWR | os.O_TRUNC, 0, winGenericRead | winGenericWrite, winTruncateExisting, winAttrNormal},
	}
	for _, c := range cases {
		a, d, at := createArgs(c.flag, c.perm)
		if a != c.access || d != c.disposition || at != c.attrs {
			t.Errorf("flag %#x perm %o: got %#x %d %#x, want %#x %d %#x", c.flag, c.perm, a, d, at, c.access, c.disposition, c.attrs)
		}
	}
}

func TestFO2_AFailedCreateIsAPathError(t *testing.T) {
	denied := errors.New("denied")
	var gotShare uint32
	_, err := openShared("x.jsonl", os.O_RDWR, 0, func(_ string, access, _, _ uint32) (uintptr, error) {
		gotShare = access
		return 0, denied
	})
	var pe *os.PathError
	if !errors.As(err, &pe) || !errors.Is(err, denied) || pe.Path != "x.jsonl" || gotShare == 0 {
		t.Fatalf("got %v", err)
	}
}

func TestFO3_IfOKRunsOnlyOnNil(t *testing.T) {
	stop := errors.New("stop")
	ran := false
	if _, err := ifOK(stop, func() (int, error) { ran = true; return 1, nil }); !errors.Is(err, stop) || ran {
		t.Fatalf("err=%v ran=%v", err, ran)
	}
	if v, err := ifOK(nil, func() (int, error) { return 7, nil }); v != 7 || err != nil {
		t.Fatalf("v=%d err=%v", v, err)
	}
}

func TestFO4_RenameByHandleReportsEachFailureAndClosesTheHandle(t *testing.T) {
	boom := errors.New("boom")
	closed := 0
	closeOK := func(uintptr) error { closed++; return nil }
	okOpen := func(string) (uintptr, error) { return 1, nil }
	if err := renameByHandle("a", "b", func(string) (uintptr, error) { return 0, boom }, nil, closeOK); !errors.Is(err, boom) || closed != 0 {
		t.Errorf("open failure: err=%v closed=%d", err, closed)
	}
	if err := renameByHandle("a", "b", okOpen, func(uintptr, string) error { return boom }, closeOK); !errors.Is(err, boom) || closed != 1 {
		t.Errorf("rename failure: err=%v closed=%d", err, closed)
	}
	if err := renameByHandle("a", "b", okOpen, func(uintptr, string) error { return nil }, func(uintptr) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("close failure: err=%v", err)
	}
	var le *os.LinkError
	if err := renameByHandle("a", "b", okOpen, func(uintptr, string) error { return boom }, closeOK); !errors.As(err, &le) {
		t.Errorf("not a LinkError: %v", err)
	}
}

func TestFO5_RenameFallsBackOnlyWhenUnsupported(t *testing.T) {
	unsupported, other := errors.New("unsupported"), errors.New("other")
	is := func(err error) bool { return errors.Is(err, unsupported) }
	fell := false
	fallback := func() error { fell = true; return nil }
	if err := renameWith(func() error { return nil }, fallback, is); err != nil || fell {
		t.Errorf("success: err=%v fell=%v", err, fell)
	}
	if err := renameWith(func() error { return other }, fallback, is); !errors.Is(err, other) || fell {
		t.Errorf("other failure was hidden: err=%v fell=%v", err, fell)
	}
	if err := renameWith(func() error { return unsupported }, fallback, is); err != nil || !fell {
		t.Errorf("unsupported did not fall back: err=%v fell=%v", err, fell)
	}
}

func TestFO6_RenameInfoIsLaidOutAsFileRenameInfo(t *testing.T) {
	const to = `\\?\C:\Users\é\a.jsonl`
	buf, size := renameInfo(to, winRenameReplace|winRenamePosix)
	info := (*fileRenameInfo)(unsafe.Pointer(&buf[0]))
	want := utf16.Encode([]rune(to))
	off := unsafe.Offsetof(fileRenameInfo{}.FileName)
	if info.Flags != winRenameReplace|winRenamePosix || info.RootDirectory != 0 || info.FileNameLength != uint32(2*len(want)) {
		t.Fatalf("header %+v", *info)
	}
	if uintptr(size) != off+uintptr(2*(len(want)+1)) || uintptr(len(buf)*8) < uintptr(size) {
		t.Fatalf("size %d for %d units, buffer %d bytes", size, len(want), len(buf)*8)
	}
	got := unsafe.Slice((*uint16)(unsafe.Add(unsafe.Pointer(info), off)), len(want)+1)
	if string(utf16.Decode(got[:len(want)])) != to || got[len(want)] != 0 {
		t.Fatalf("name %q, terminator %d", string(utf16.Decode(got[:len(want)])), got[len(want)])
	}
}

func TestFO7_VerbatimPathPrefixesDrivePathsOnly(t *testing.T) {
	for in, want := range map[string]string{
		`C:\a\b.jsonl`:          `\\?\C:\a\b.jsonl`,
		`\\server\share\b`:      `\\server\share\b`,
		`\\?\C:\already\b.json`: `\\?\C:\already\b.json`,
	} {
		if got := verbatimPath(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestFO8_WriteFileAndRenameReplaceAFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFile(filepath.Join(dir, "missing", "x"), nil, 0o600); err == nil {
		t.Error("a write into a missing directory succeeded")
	}
	to := filepath.Join(dir, "a.jsonl")
	if err := WriteFile(to, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(to+".tmp", []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Rename(to+".tmp", to); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(to); string(b) != "new\n" {
		t.Fatalf("got %q", b)
	}
}
