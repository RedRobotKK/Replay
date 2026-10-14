package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A canonical archive, written the way goreleaser writes one: Go's
// archive/tar, plain ustar, regular files, two zero blocks at the end.
var cmMembers = []struct {
	name string
	body []byte
}{
	{"LICENSE", []byte("license text\n")},
	{"README.md", []byte("readme\n")},
	{"replay", bytes.Repeat([]byte("GENUINE-BINARY\n"), 40)},
}

func cmTar(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for _, m := range cmMembers {
		h := &tar.Header{Name: m.name, Mode: 0o755, Size: int64(len(m.body)), ModTime: time.Unix(1757378973, 0),
			Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(m.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func cmGzip(b []byte) []byte {
	var z bytes.Buffer
	w := gzip.NewWriter(&z)
	_, _ = w.Write(b)
	_ = w.Close()
	return z.Bytes()
}

// cmOffset is where member i's header starts in cmTar's output.
func cmOffset(i int) int {
	off := 0
	for _, m := range cmMembers[:i] {
		off += 512 + (len(m.body)+511)/512*512
	}
	return off
}

// cmSetField overwrites one field of member i's header and recomputes the
// checksum in Go's own format, so the checksum is never the reason for a
// refusal: only the field's encoding is under test.
func cmSetField(tb []byte, i, at int, v []byte) []byte {
	out := append([]byte(nil), tb...)
	h := out[cmOffset(i) : cmOffset(i)+512]
	copy(h[at:], v)
	cmFixSum(h)
	return out
}

func cmFixSum(h []byte) {
	for j := 148; j < 156; j++ {
		h[j] = ' '
	}
	var sum int64
	for _, c := range h {
		sum += int64(c)
	}
	copy(h[148:156], fmt.Sprintf("%06o\x00 ", sum))
}

func base256(n int, v int64) []byte {
	b := make([]byte, n)
	for i := n - 1; i > 0; i-- {
		b[i] = byte(v)
		v >>= 8
	}
	b[0] = 0x80
	return b
}

// goReads lists every value Go's archive/tar reads for each member that the
// canonical encoding is built from, and the link name, or nil if Go refuses
// the archive outright. Format is left out: it is Go's guess at the encoding,
// which is what the variants change.
func goReads(tb []byte) []string {
	tr := tar.NewReader(bytes.NewReader(tb))
	var out []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			return nil
		}
		out = append(out, fmt.Sprintf("%q size=%d mode=%o uid=%d gid=%d mtime=%d type=%q uname=%q gname=%q link=%q dev=%d,%d",
			h.Name, h.Size, h.Mode, h.Uid, h.Gid, h.ModTime.Unix(), h.Typeflag, h.Uname, h.Gname, h.Linkname, h.Devmajor, h.Devminor))
	}
}

func TestCanonicalArchiveIsAccepted(t *testing.T) {
	names, err := plain(cmGzip(cmTar(t)))
	if err != nil {
		t.Fatalf("a canonical archive was refused: %v", err)
	}
	if strings.Join(names, ",") != "LICENSE,README.md,replay" {
		t.Fatalf("names %v", names)
	}
}

// Record padding to a 10240-byte block, as GNU tar writes, is zeros after the
// end-of-archive and changes no reader's view.
func TestCanonicalArchiveWithRecordPaddingIsAccepted(t *testing.T) {
	tb := cmTar(t)
	tb = append(tb, make([]byte, 10240-len(tb)%10240)...)
	if _, err := plain(cmGzip(tb)); err != nil {
		t.Fatalf("record padding was refused: %v", err)
	}
}

// F1 (review of v3.5, 2026-10-14): each variant encodes the same header
// values in a way Go's reader normalizes, so Go reads exactly the canonical
// archive's values, while another reader may not. v3.5 accepted the
// leading-NUL size field, and bsdtar read that size as 0, splitting the
// archive into different members. Every such encoding is refused, not only
// that one: the header bytes must be the ones Go's writer produces for the
// values Go read.
func TestANonCanonicalHeaderEncodingIsRefused(t *testing.T) {
	tb := cmTar(t)
	want := goReads(tb)
	if len(want) != len(cmMembers) {
		t.Fatalf("Go does not read the canonical fixture: %v", want)
	}
	r := 2 // the replay member
	size := int64(len(cmMembers[r].body))
	cases := map[string][]byte{
		"size with a leading NUL":         cmSetField(tb, r, 124, []byte(fmt.Sprintf("\x00%010o\x00", size))),
		"size with a leading space":       cmSetField(tb, r, 124, []byte(fmt.Sprintf(" %010o\x00", size))),
		"size ended by a space":           cmSetField(tb, r, 124, []byte(fmt.Sprintf("%011o ", size))),
		"size with no terminator":         cmSetField(tb, r, 124, []byte(fmt.Sprintf("%012o", size))),
		"size short and NUL padded":       cmSetField(tb, r, 124, append([]byte(fmt.Sprintf("%o", size)), make([]byte, 12)...)[:12]),
		"size in base-256":                cmSetField(tb, r, 124, base256(12, size)),
		"mode with a leading NUL":         cmSetField(tb, r, 100, []byte("\x00000755\x00")),
		"mode ended by a space":           cmSetField(tb, r, 100, []byte("0000755 ")),
		"uid in base-256":                 cmSetField(tb, r, 108, base256(8, 0)),
		"gid all NUL":                     cmSetField(tb, r, 116, make([]byte, 8)),
		"mtime with a leading space":      cmSetField(tb, r, 136, []byte(fmt.Sprintf(" %010o\x00", 1757378973))),
		"devmajor all NUL":                cmSetField(tb, r, 329, make([]byte, 8)),
		"devminor ended by a space":       cmSetField(tb, r, 337, []byte("0000000 ")),
		"typeflag NUL (old regular file)": cmSetField(tb, r, 156, []byte{0}),
		"name with bytes after its NUL":   cmSetField(tb, r, 0, []byte("replay\x00zzz")),
		"uname with bytes after its NUL":  cmSetField(tb, r, 265, []byte("\x00zzz")),
		"ustar version xx":                cmSetField(tb, r, 263, []byte("xx")),
		"ustar version NUL NUL":           cmSetField(tb, r, 263, []byte{0, 0}),
		"star trailer after the prefix":   cmSetField(tb, r, 508, []byte("tar\x00")),
		"non-zero header padding":         cmSetField(tb, r, 500, []byte{'x'}),
	}
	for name, a := range cases {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			// The precondition that makes this a test of encoding: Go reads
			// the same values as from the canonical archive.
			if got := goReads(a); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("precondition: Go reads %v, not the canonical %v; the variant changes values, not only their encoding", got, want)
			}
			_, err := plain(cmGzip(a))
			if err == nil {
				t.Fatal("accepted a header that is not the canonical encoding of the values Go read")
			}
			if !strings.Contains(err.Error(), "not the canonical ustar encoding") {
				t.Fatalf("refused, but not by the canonical-header check: %v", err)
			}
		})
	}
}

// Checksum fields that Go's reader accepts but its writer would not produce.
func TestANonCanonicalChecksumIsRefused(t *testing.T) {
	tb := cmTar(t)
	want := goReads(tb)
	if len(want) != len(cmMembers) {
		t.Fatalf("Go does not read the canonical fixture: %v", want)
	}
	r := 2
	sumOf := func(h []byte) int64 {
		var s int64
		for j, c := range h {
			if j >= 148 && j < 156 {
				c = ' '
			}
			s += int64(c)
		}
		return s
	}
	for name, format := range map[string]string{
		"seven digits and NUL":      "%07o\x00",
		"six digits and two spaces": "%06o  ",
		"leading space":             " %05o\x00 ",
	} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			a := append([]byte(nil), tb...)
			h := a[cmOffset(r) : cmOffset(r)+512]
			copy(h[148:156], fmt.Sprintf(format, sumOf(h)))
			if got := goReads(a); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("precondition: Go reads %v, want %v", got, want)
			}
			_, err := plain(cmGzip(a))
			if err == nil || !strings.Contains(err.Error(), "not the canonical ustar encoding") {
				t.Fatalf("want a canonical-header refusal, got %v", err)
			}
		})
	}
}

// The structure around the headers: data padding, the end-of-archive marker
// and what follows it, and the gzip framing.
func TestANonCanonicalArchiveStructureIsRefused(t *testing.T) {
	tb := cmTar(t)
	end := cmOffset(len(cmMembers)) // the end-of-archive marker starts here
	padAt := cmOffset(2) + 512 + len(cmMembers[2].body)
	set := func(at int, v byte) []byte { a := append([]byte(nil), tb...); a[at] = v; return a }
	cases := map[string]struct {
		archive []byte
		reason  string
	}{
		"non-zero data padding":           {cmGzip(set(padAt, 'x')), "padding"},
		"one zero block at the end":       {cmGzip(tb[:end+512]), "end-of-archive"},
		"no end-of-archive marker":        {cmGzip(tb[:end]), "end-of-archive"},
		"a block after end-of-archive":    {cmGzip(append(append([]byte(nil), tb...), append([]byte{'x'}, make([]byte, 511)...)...)), "after end-of-archive"},
		"length not a multiple of 512":    {cmGzip(append(append([]byte(nil), tb...), make([]byte, 100)...)), "multiple of 512"},
		"a second gzip member":            {append(cmGzip(tb), cmGzip(tb)...), "after the gzip member"},
		"bytes after the gzip member":     {append(cmGzip(tb), 'x'), "after the gzip member"},
		"a header that is a link":         {cmGzip(cmSetField(tb, 2, 156, []byte{'1'})), "not a regular file"},
		"content that runs past the end":  {cmGzip(tb[:cmOffset(2)+1024]), "runs past the end"},
		"a member under a prefix":         {cmGzip(cmSetField(tb, 2, 345, []byte("bin"))), "canonical ustar encoding"},
		"a regular file with a link name": {cmGzip(cmSetField(tb, 2, 157, []byte("LICENSE"))), "canonical ustar encoding"},
	}
	for name, c := range cases {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			_, err := plain(c.archive)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("refused, but not for the reason under test (%q): %v", c.reason, err)
			}
		})
	}
}

// A header Go's writer cannot encode as plain ustar fails closed: a non-ASCII
// user name needs PAX.
func TestAHeaderTheWriterCannotEncodeIsRefused(t *testing.T) {
	a := cmSetField(cmTar(t), 2, 265, []byte("r\xc3\xa9"))
	_, err := plain(cmGzip(a))
	if err == nil || !strings.Contains(err.Error(), "canonical ustar") {
		t.Fatalf("want a canonical-header refusal, got %v", err)
	}
}

// Names Go's writer encodes canonically, so only the name rule can refuse
// them: each is a path a reader may resolve its own way, or an option to a
// tool that is handed member names.
func TestAMemberNameThatIsNotAPlainRelativePathIsRefused(t *testing.T) {
	for _, name := range []string{"./replay", "../replay", "/replay", "-replay", ".replay", "bin//replay", "re play", "replay\\x"} {
		t.Run(strings.NewReplacer("/", "_", " ", "_").Replace(name), func(t *testing.T) {
			var b bytes.Buffer
			tw := tar.NewWriter(&b)
			if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: 1, ModTime: time.Unix(1757378973, 0),
				Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
				t.Fatal(err)
			}
			_, _ = tw.Write([]byte("x"))
			_ = tw.Close()
			_, err := plain(cmGzip(b.Bytes()))
			if err == nil || !strings.Contains(err.Error(), "not a plain relative path") {
				t.Fatalf("want the name rule to refuse %q, got %v", name, err)
			}
		})
	}
}

// A stream that decompresses past the bound is refused, not read: 256 MiB of
// zeros compresses to about 256 KiB.
func TestAnArchiveThatDecompressesPastTheBoundIsRefused(t *testing.T) {
	var z bytes.Buffer
	w, _ := gzip.NewWriterLevel(&z, gzip.BestCompression)
	block := make([]byte, 1<<20)
	for i := 0; i <= maxTar>>20; i++ {
		_, _ = w.Write(block)
	}
	_ = w.Close()
	_, err := plain(z.Bytes())
	if err == nil || !strings.Contains(err.Error(), "decompresses to more than") {
		t.Fatalf("want the size bound to refuse, got %v", err)
	}
}

// The writer's refusal is canonical's answer: a header plain ustar cannot
// encode has no canonical block, and none is returned.
func TestCanonicalReturnsNoBlockForAHeaderTheWriterCannotEncode(t *testing.T) {
	h := &tar.Header{Name: "replay", Mode: 0o755, Size: 1, ModTime: time.Unix(1757378973, 0),
		Typeflag: tar.TypeReg, Uname: "r\xc3\xa9"}
	b, err := canonical(h)
	if err == nil || b != nil {
		t.Fatalf("want the writer's refusal and no block, got %d bytes, err %v", len(b), err)
	}
	h.Uname = "replay"
	if b, err := canonical(h); err != nil || len(b) != 512 {
		t.Fatalf("control: an encodable header gives one block, got %d bytes, err %v", len(b), err)
	}
}

// The refusal says the writer could not encode the header, not only that the
// bytes differ, so a reader of the refusal can tell the two apart.
func TestARefusedHeaderNamesTheWritersReason(t *testing.T) {
	a := cmSetField(cmTar(t), 2, 265, []byte("r\xc3\xa9"))
	tr := tar.NewReader(bytes.NewReader(a))
	var hdr *tar.Header
	for i := 0; i < 3; i++ {
		h, err := tr.Next()
		if err != nil {
			t.Fatalf("precondition: Go reads member %d: %v", i, err)
		}
		hdr = h
	}
	_, werr := canonical(hdr)
	if werr == nil {
		t.Fatal("precondition: the writer encodes this header, so it is not under test")
	}
	_, err := plain(cmGzip(a))
	if err == nil || !strings.Contains(err.Error(), werr.Error()) {
		t.Fatalf("want a refusal carrying %q, got %v", werr, err)
	}
}

// Bytes that are not gzip are refused before anything is read.
func TestAnArchiveThatIsNotGzipIsRefused(t *testing.T) {
	_, err := plain(cmTar(t))
	if !errors.Is(err, gzip.ErrHeader) {
		t.Fatalf("want gzip.ErrHeader, got %v", err)
	}
}

// A gzip member whose content is a canonical archive but whose checksum or
// length is wrong is refused: the bytes were not the ones the release wrote.
func TestAGzipMemberThatDoesNotVerifyIsRefused(t *testing.T) {
	z := cmGzip(cmTar(t))
	badSum := append([]byte(nil), z...)
	badSum[len(badSum)-8] ^= 0xff // the CRC-32 in the gzip trailer
	cases := map[string]struct {
		archive []byte
		want    error
	}{
		"a wrong checksum":   {badSum, gzip.ErrChecksum},
		"a truncated member": {z[:len(z)-4], io.ErrUnexpectedEOF},
	}
	for name, c := range cases {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			_, err := plain(c.archive)
			if !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}

// A header Go's reader refuses is refused: here its checksum is wrong.
func TestATarHeaderGoCannotReadIsRefused(t *testing.T) {
	tb := cmTar(t)
	copy(tb[148:156], "0000000\x00")
	if goReads(tb) != nil {
		t.Fatal("precondition: Go reads the header, so it is not under test")
	}
	_, err := plain(cmGzip(tb))
	if !errors.Is(err, tar.ErrHeader) {
		t.Fatalf("want tar.ErrHeader, got %v", err)
	}
}

// Two canonical members with one name are refused: which one a reader keeps
// is that reader's choice.
func TestAMemberThatAppearsTwiceIsRefused(t *testing.T) {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for _, body := range []string{"GENUINE", "PLANTED"} {
		if err := tw.WriteHeader(&tar.Header{Name: "replay", Mode: 0o755, Size: int64(len(body)),
			ModTime: time.Unix(1757378973, 0), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write([]byte(body))
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := plain(cmGzip(b.Bytes()))
	if err == nil || !strings.Contains(err.Error(), "appears twice") {
		t.Fatalf("want the duplicate refused, got %v", err)
	}
}

// cmRun runs the command with paths in a fresh directory and reports its
// status and whether each output file exists.
type cmResult struct {
	code             int
	out, members     []byte
	outOK, membersOK bool
}

func cmRun(t *testing.T, archive []byte, mutate func(dir string, args []string) []string) cmResult {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "replay.tar.gz")
	if archive != nil {
		if err := os.WriteFile(in, archive, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"archive-member", in, filepath.Join(dir, "out"), filepath.Join(dir, "members")}
	if mutate != nil {
		args = mutate(dir, args)
	}
	var stderr bytes.Buffer
	r := cmResult{code: run(args, &stderr)}
	var err error
	r.out, err = os.ReadFile(filepath.Join(dir, "out"))
	r.outOK = err == nil
	r.members, err = os.ReadFile(filepath.Join(dir, "members"))
	r.membersOK = err == nil
	return r
}

func TestRunWritesTheSelectedMemberAndTheNames(t *testing.T) {
	r := cmRun(t, cmGzip(cmTar(t)), nil)
	if r.code != 0 || !bytes.Equal(r.out, cmMembers[2].body) || string(r.members) != "LICENSE\nREADME.md\nreplay\n" {
		t.Fatalf("got code %d, out %q, members %q", r.code, r.out, r.members)
	}
}

// Every refusal exits with its status and writes no selected member, so a
// caller that ignored the status would still have nothing to compare.
func TestRunRefusesAndWritesNoMember(t *testing.T) {
	good := cmGzip(cmTar(t))
	r := 2
	size := int64(len(cmMembers[r].body))
	// F1's archive: Go's reader, which upgrade uses, reads the canonical
	// values from it, so only the plain check stops it. This file does not
	// import selfupdate, which would oblige it to pin cosign (FC-COSIGN).
	f1tar := cmSetField(cmTar(t), r, 124, []byte(fmt.Sprintf("\x00%010o\x00", size)))
	if strings.Join(goReads(f1tar), ",") != strings.Join(goReads(cmTar(t)), ",") {
		t.Fatal("precondition: Go reads the F1 archive as the canonical one")
	}
	f1 := cmGzip(f1tar)
	// No replay member: plain, and upgrade refuses it.
	var nb bytes.Buffer
	tw := tar.NewWriter(&nb)
	_ = tw.WriteHeader(&tar.Header{Name: "LICENSE", Mode: 0o644, Size: 1, ModTime: time.Unix(1757378973, 0),
		Typeflag: tar.TypeReg, Format: tar.FormatUSTAR})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	noBinary := cmGzip(nb.Bytes())
	if _, err := plain(noBinary); err != nil {
		t.Fatalf("precondition: the archive with no replay member is plain: %v", err)
	}
	missing := func(name string) func(string, []string) []string {
		return func(dir string, args []string) []string {
			for i, a := range args {
				if a == filepath.Join(dir, name) {
					args[i] = filepath.Join(dir, "no-such-dir", name)
				}
			}
			return args
		}
	}
	cases := []struct {
		name    string
		archive []byte
		mutate  func(string, []string) []string
		code    int
		// outOK is whether the member file may exist: only the names file
		// fails after the member was written.
		outOK bool
	}{
		{"no arguments", good, func(_ string, a []string) []string { return a[:1] }, 2, false},
		{"one argument too many", good, func(_ string, a []string) []string { return append(a, "extra") }, 2, false},
		{"an archive that cannot be read", nil, nil, 1, false},
		{"an archive that is not plain", f1, nil, 3, false},
		{"an archive upgrade would not install from", noBinary, nil, 1, false},
		{"an output that cannot be written", good, missing("out"), 1, false},
		{"a names file that cannot be written", good, missing("members"), 1, true},
	}
	for _, c := range cases {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			got := cmRun(t, c.archive, c.mutate)
			if got.code != c.code {
				t.Fatalf("exit %d, want %d", got.code, c.code)
			}
			if got.outOK != c.outOK || got.membersOK {
				t.Fatalf("member written %v (want %v), names written %v (want false)", got.outOK, c.outOK, got.membersOK)
			}
		})
	}
}
