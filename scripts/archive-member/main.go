// archive-member writes the bytes `replay upgrade` would install from a
// release archive, selected by selfupdate.Unpack, the code upgrade runs, and
// lists the archive's members as Go reads them.
//
//	go run ./scripts/archive-member <archive.tar.gz> <out> <members-out>
//
// scripts/reproduce-release.sh compares the selected bytes, refuses unless tar
// extracts the same bytes, and refuses unless tar lists the same members.
//
// THE ARCHIVE MUST BE IN THE PLAIN FORM goreleaser WRITES, or nothing is
// selected. Every release archive is one gzip member holding flat ustar
// regular files with no PAX, GNU or global headers (v0.9.0, read 2026-10-14).
// An archive outside that form can mean different things to different tar
// readers: a GNU long name, a PAX path, data after end-of-archive or a second
// gzip member each made the gate pass while a reader resolving them
// differently would install a planted binary (2026-10-14). The gate can only
// check agreement with the tar on its own machine, so it refuses the form
// instead. Exit 1 means upgrade would refuse the archive; exit 3 means the
// archive is not in the plain form.
//
// PLAIN MEANS THE BYTES, NOT THE VALUES. v3.5 checked the values Go's reader
// returned, and Go's reader normalizes: a size field written with a leading
// NUL reads as the same size to Go and as 0 to libarchive, so one archive
// listed the same names to both and held a planted replay that only bsdtar
// extracted (F1, 2026-10-14). Every header must now be byte for byte the block
// Go's writer, which goreleaser uses, produces for the values Go read; the
// content must be where Go read it; padding and the end-of-archive must be
// zeros. An encoding a reader could resolve differently is then refused
// whichever reader it is.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/RedRobotKK/Replay/internal/selfupdate"
)

var component = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// maxTar bounds the decompressed archive. A release archive holds about
// 25 MB; a larger stream is refused rather than read.
const maxTar = 256 << 20

// canonical is the header block Go's writer produces, as plain ustar, for the
// values Go's reader returned. Only the fields a plain release member uses
// are carried, so a link name, device numbers or a name prefix the writer
// would not produce for these values make the block differ.
//
// With Format fixed to FormatUSTAR the writer either refuses the header or
// writes exactly one 512-byte block (archive/tar writer.go, WriteHeader and
// writeRawHeader, go1.27.1 and go1.27.2). There is no length check here: a
// block of any other length could not equal the 512 archive bytes plain
// compares it with, so it would be refused there.
func canonical(h *tar.Header) ([]byte, error) {
	c := &tar.Header{Typeflag: h.Typeflag, Name: h.Name, Mode: h.Mode, Uid: h.Uid, Gid: h.Gid,
		Size: h.Size, ModTime: h.ModTime, Uname: h.Uname, Gname: h.Gname, Format: tar.FormatUSTAR}
	var b bytes.Buffer
	if err := tar.NewWriter(&b).WriteHeader(c); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func zeros(b []byte) bool { return bytes.Count(b, []byte{0}) == len(b) }

// plain returns the member names in order, or why the archive is not plain.
func plain(archive []byte) ([]string, error) {
	br := bufio.NewReader(bytes.NewReader(archive))
	zr, err := gzip.NewReader(br)
	if err != nil {
		return nil, fmt.Errorf("not gzip: %w", err)
	}
	zr.Multistream(false)
	raw, err := io.ReadAll(io.LimitReader(zr, maxTar+1))
	if err != nil {
		return nil, fmt.Errorf("unreadable gzip: %w", err)
	}
	if len(raw) > maxTar {
		return nil, fmt.Errorf("decompresses to more than %d bytes", maxTar)
	}
	// Nothing may follow the one gzip member.
	if _, err := br.Peek(1); err != io.EOF {
		return nil, errors.New("data after the gzip member")
	}
	if len(raw)%512 != 0 {
		return nil, fmt.Errorf("the tar stream is %d bytes, not a multiple of 512", len(raw))
	}
	// Go's reader and this walk advance together: off is where the next
	// header must start if Go read nothing but plain headers and content.
	tr := tar.NewReader(bytes.NewReader(raw))
	off := 0
	var names []string
	seen := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unreadable tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("member %q is type %q, not a regular file", hdr.Name, hdr.Typeflag)
		}
		want, err := canonical(hdr)
		if err != nil {
			return nil, fmt.Errorf("member %q: header is not the canonical ustar encoding of its fields: %v", hdr.Name, err)
		}
		if off+512 > len(raw) || !bytes.Equal(raw[off:off+512], want) {
			return nil, fmt.Errorf("member %q: header is not the canonical ustar encoding of its fields", hdr.Name)
		}
		// There is no check of hdr.Format. It is Go's guess at the encoding,
		// and plain is a property of the bytes: the block at off is the
		// writer's ustar block for these values, typeflag '0', so no extension
		// header came before it and Go reads it as FormatUSTAR.
		if seen[hdr.Name] {
			return nil, fmt.Errorf("member %q appears twice", hdr.Name)
		}
		for _, c := range strings.Split(hdr.Name, "/") {
			if !component.MatchString(c) {
				return nil, fmt.Errorf("member name %q is not a plain relative path", hdr.Name)
			}
		}
		// The block is canonical ustar, so no extension header was consumed
		// and the content is the next Size bytes.
		end := off + 512 + int(hdr.Size)
		next := off + 512 + (int(hdr.Size)+511)/512*512
		if next > len(raw) {
			return nil, fmt.Errorf("member %q: its content runs past the end of the archive", hdr.Name)
		}
		if !zeros(raw[end:next]) {
			return nil, fmt.Errorf("member %q: non-zero padding after its content", hdr.Name)
		}
		seen[hdr.Name] = true
		names = append(names, hdr.Name)
		off = next
	}
	// The end-of-archive is two zero blocks, as Go's writer ends an archive,
	// and the tar stream may hold only zero padding after it.
	if len(raw)-off < 1024 {
		return nil, errors.New("no end-of-archive marker of two zero blocks")
	}
	if !zeros(raw[off:]) {
		return nil, errors.New("data after end-of-archive")
	}
	return names, nil
}

func main() { os.Exit(run(os.Args, os.Stderr)) }

// run is main with its exit status returned, so each refusal can be tested
// in process: 2 usage, 1 unreadable or not installable, 3 not plain, 0 done.
func run(args []string, stderr io.Writer) int {
	if len(args) != 4 {
		_, _ = fmt.Fprintln(stderr, "usage: archive-member <archive.tar.gz> <out> <members-out>")
		return 2
	}
	archive, err := os.ReadFile(args[1])
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	names, err := plain(archive)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "the archive is not in the plain form goreleaser writes:", err)
		return 3
	}
	bin, err := selfupdate.Unpack(archive)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(args[2], bin, 0o600); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(args[3], []byte(strings.Join(names, "\n")+"\n"), 0o600); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
