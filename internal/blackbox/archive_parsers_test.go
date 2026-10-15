//go:build mutation

package blackbox

// PD: the gate judges the bytes `replay upgrade` installs.
//
// Every case is a real fixture release whose archive is rebuilt byte by byte,
// then read three ways and judged by BYTES, never by names:
//   gate    = scripts/reproduce-release.sh end to end (REPRODUCES or not)
//   upgrade = selfupdate.Fetch over HTTP, the path `replay upgrade` runs
//   install = install.sh's `tar -xzf archive -C dir replay`
// A false pass is the gate printing REPRODUCES while upgrade would install
// bytes other than the genuine build.
//
// History: 2026-10-13, three rounds. A member named "replay/" passed the gate
// (awk did not drop trailing slashes as filepath.Base does). Then a planted
// member carrying a PAX path=zzz header followed by a GNU long name "replay"
// passed it: bsdtar listed "zzz", Go's archive/tar read "replay". The gate
// now selects the member with selfupdate's own reader, so the two cannot
// disagree about which bytes are the binary. On the ubuntu runner the gate's
// tar is GNU tar, which no machine this was written on had.

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/selfupdate"
)

// TestMain pins cosign absent, as FC-COSIGN requires of every package whose
// tests reach the signature check: Fetch must not consult the machine.
func TestMain(m *testing.M) {
	selfupdate.LookCosign = func() (string, error) { return "", errors.New("cosign is pinned absent by TestMain") }
	os.Exit(m.Run())
}

var pdPlant = []byte("PLANTED-BINARY\n")

// pdGoPlant is set per case to the fixture's alt binary: a real Go program
// from the tagged commit, stamped identically, with different bytes.
var pdGoPlant []byte

type pdEnt struct {
	name      string
	typeflag  byte // 0 = '0'; 1 = NUL (TypeRegA)
	body      []byte
	link      string
	magic     string // "" ustar, "gnu", "v7"
	prefix    string
	pax       [][2]string // 'x' records ahead of this entry, in order
	global    [][2]string // 'g' records ahead of this entry
	gnuLong   string      // GNU 'L' long name ahead of this entry
	longFirst bool        // emit the 'L' header before the 'x' header
	raw       []byte      // emit these bytes instead
}

func pdOctal(b []byte, v int64) { copy(b, fmt.Sprintf("%0*o", len(b)-1, v)) }

func pdHeader(name string, typeflag byte, size int64, link, magic, prefix string) []byte {
	h := make([]byte, 512)
	copy(h[0:100], name)
	pdOctal(h[100:108], 0o755)
	pdOctal(h[108:116], 0)
	pdOctal(h[116:124], 0)
	pdOctal(h[124:136], size)
	pdOctal(h[136:148], 0)
	h[156] = typeflag
	copy(h[157:257], link)
	switch magic {
	case "":
		copy(h[257:263], "ustar\x00")
		copy(h[263:265], "00")
		// devmajor and devminor as Go's writer, and so goreleaser, encodes
		// them: the gate requires every header byte to be that encoding.
		pdOctal(h[329:337], 0)
		pdOctal(h[337:345], 0)
	case "gnu":
		copy(h[257:265], "ustar  \x00")
	}
	copy(h[345:500], prefix)
	for i := 148; i < 156; i++ {
		h[i] = ' '
	}
	var sum int64
	for _, c := range h {
		sum += int64(c)
	}
	copy(h[148:156], fmt.Sprintf("%06o\x00 ", sum))
	return h
}

func pdPad(b []byte) []byte {
	if r := len(b) % 512; r != 0 {
		b = append(b, make([]byte, 512-r)...)
	}
	return b
}

func pdPAX(kind byte, recs [][2]string) []byte {
	var body []byte
	for _, kv := range recs {
		rec := " " + kv[0] + "=" + kv[1] + "\n"
		n := len(rec)
		for {
			s := fmt.Sprintf("%d%s", n, rec)
			if len(s) == n {
				body = append(body, s...)
				break
			}
			n = len(s)
		}
	}
	return append(pdHeader("PaxHeader", kind, int64(len(body)), "", "", ""), pdPad(body)...)
}

func pdLong(name string) []byte {
	body := append([]byte(name), 0)
	return append(pdHeader("././@LongLink", 'L', int64(len(body)), "", "gnu", ""), pdPad(body)...)
}

func pdTar(ents []pdEnt) []byte {
	var t []byte
	for _, e := range ents {
		if e.raw != nil {
			t = append(t, e.raw...)
			continue
		}
		if e.global != nil {
			t = append(t, pdPAX('g', e.global)...)
		}
		if e.gnuLong != "" && e.longFirst {
			t = append(t, pdLong(e.gnuLong)...)
		}
		if e.pax != nil {
			t = append(t, pdPAX('x', e.pax)...)
		}
		if e.gnuLong != "" && !e.longFirst {
			t = append(t, pdLong(e.gnuLong)...)
		}
		tf := e.typeflag
		switch tf {
		case 0:
			tf = '0'
		case 1:
			tf = 0
		}
		t = append(t, pdHeader(e.name, tf, int64(len(e.body)), e.link, e.magic, e.prefix)...)
		t = append(t, pdPad(append([]byte(nil), e.body...))...)
	}
	return append(t, make([]byte, 1024)...)
}

func pdGzip(b []byte) []byte {
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write(b)
	_ = w.Close()
	return gz.Bytes()
}

// pdPublish replaces the fixture's archive and rewrites checksums.txt to vouch
// for it, as an attacker able to replace both would.
func pdPublish(t *testing.T, f rrFixture, archive []byte) {
	t.Helper()
	name := fmt.Sprintf("replay_%s_%s_%s.tar.gz", rrVersion, rrOS, rrArch)
	if err := os.WriteFile(filepath.Join(f.assets, name), archive, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive)
	if err := os.WriteFile(filepath.Join(f.assets, "checksums.txt"), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pdFetch serves one archive and its checksum and returns what Fetch unpacks.
func pdFetch(t *testing.T, archive []byte) ([]byte, error) {
	t.Helper()
	name := selfupdate.ArchiveName(rrTag, rrOS, rrArch)
	sum := sha256.Sum256(archive)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/"+name):
			_, _ = w.Write(archive)
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &selfupdate.Client{HTTP: srv.Client(), ReleasesBase: srv.URL}
	b, _, err := c.Fetch(context.Background(), rrTag, rrOS, rrArch)
	return b, err
}

func pdInstall(t *testing.T, archive []byte) []byte {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.tar.gz")
	_ = os.WriteFile(p, archive, 0o644)
	x := filepath.Join(dir, "x")
	_ = os.Mkdir(x, 0o755)
	if err := exec.Command("tar", "-xzf", p, "-C", x, "replay").Run(); err != nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(x, "replay"))
	if err != nil {
		return nil
	}
	return b
}

func TestPD_TheGateJudgesTheBytesUpgradeInstalls(t *testing.T) {
	lic := pdEnt{name: "LICENSE", body: []byte("l\n")}
	type pdCase struct {
		id   string
		must string // pass, fail, either
		ents func(g []byte) []pdEnt
		arch func(g []byte) []byte // overrides ents
	}
	plantFirst := func(p pdEnt) func(g []byte) []pdEnt {
		return func(g []byte) []pdEnt { p.body = pdPlant; return []pdEnt{p, {name: "replay", body: g}, lic} }
	}
	cases := []pdCase{
		{"C01 control: replay and LICENSE", "pass", func(g []byte) []pdEnt { return []pdEnt{lic, {name: "replay", body: g}} }, nil},
		{"C02 duplicate replay, plant first", "fail", plantFirst(pdEnt{name: "replay"}), nil},
		{"C03 bin/replay plant first", "fail", plantFirst(pdEnt{name: "bin/replay"}), nil},
		{"C04 ./replay plant first", "fail", plantFirst(pdEnt{name: "./replay"}), nil},
		{"C05 replay/ regular file, plant first", "fail", plantFirst(pdEnt{name: "replay/"}), nil},
		{"C06 replay// regular file, plant first", "fail", plantFirst(pdEnt{name: "replay//"}), nil},
		{"C07 bin/replay/ regular file, plant first", "fail", plantFirst(pdEnt{name: "bin/replay/"}), nil},
		{"C08 /replay absolute, plant first", "fail", plantFirst(pdEnt{name: "/replay"}), nil},
		{"C09 a/../replay, plant first", "fail", plantFirst(pdEnt{name: "a/../replay"}), nil},
		{"C10 PAX path=bin/replay over ustar replay, plant first", "fail", plantFirst(pdEnt{name: "replay", pax: [][2]string{{"path", "bin/replay"}}}), nil},
		{"C11 PAX path=replay over ustar decoy, plant only", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "decoy", body: pdPlant, pax: [][2]string{{"path", "replay"}}}, {name: "zzz", body: g}, lic}
		}, nil},
		{"C12 global path=zzz, plant replay, genuine via PAX path=replay", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: pdPlant, global: [][2]string{{"path", "zzz"}}}, {name: "other", body: g, pax: [][2]string{{"path", "replay"}}}, lic}
		}, nil},
		{"C13 GNU magic with prefix bytes, plant first", "fail", plantFirst(pdEnt{name: "replay", magic: "gnu", prefix: "evil"}), nil},
		{"C14 ustar prefix=bin name=replay, plant first", "fail", plantFirst(pdEnt{name: "replay", prefix: "bin"}), nil},
		{"C15 V7 header with prefix bytes, plant only", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: pdPlant, magic: "v7", prefix: "bin"}, {name: "zzz", body: g}, lic}
		}, nil},
		{"C16 TypeRegA plant first", "fail", plantFirst(pdEnt{name: "replay", typeflag: 1}), nil},
		{"C17 lone zero block, then genuine", "fail", func(g []byte) []pdEnt {
			return []pdEnt{lic, {raw: make([]byte, 512)}, {name: "replay", body: g}}
		}, nil},
		{"C18 genuine, end-of-archive, then plant", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g}, {raw: make([]byte, 1024)}, {name: "replay", body: pdPlant}}
		}, nil},
		{"C19 typeflag 7 plant only", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: pdPlant, typeflag: '7'}, {name: "zzz", body: g}}
		}, nil},
		{"C20 NUL inside name replay\\x00junk, plant only", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay\x00junk", body: pdPlant}, {name: "zzz", body: g}}
		}, nil},
		{"C21 PAX path=zzz then GNU long name replay, plant first", "fail", plantFirst(pdEnt{name: "decoy", pax: [][2]string{{"path", "zzz"}}, gnuLong: "replay"}), nil},
		{"C22 GNU long name replay then PAX path=zzz, plant first", "fail", plantFirst(pdEnt{name: "decoy", pax: [][2]string{{"path", "zzz"}}, gnuLong: "replay", longFirst: true}), nil},
		{"C23 PAX path=zzz then path=replay in one header, plant first", "fail", plantFirst(pdEnt{name: "decoy", pax: [][2]string{{"path", "zzz"}, {"path", "replay"}}}), nil},
		{"C24 PAX path=replay then path=zzz in one header, plant first", "fail", plantFirst(pdEnt{name: "decoy", pax: [][2]string{{"path", "replay"}, {"path", "zzz"}}}), nil},
		{"C25 GNU long name zzz over ustar replay, plant first", "fail", plantFirst(pdEnt{name: "replay", gnuLong: "zzz"}), nil},
		{"C26 hardlink replay -> LICENSE first, then genuine", "fail", func(g []byte) []pdEnt {
			return []pdEnt{lic, {name: "replay", typeflag: '1', link: "LICENSE"}, {name: "replay", body: g}}
		}, nil},
		{"C27 symlink replay -> /bin/sh first, then genuine", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay", typeflag: '2', link: "/bin/sh"}, {name: "replay", body: g}, lic}
		}, nil},
		{"C28 ../replay plant first", "fail", plantFirst(pdEnt{name: "../replay"}), nil},
		{"C29 directory entry replay/ first, then genuine", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay/", typeflag: '5'}, {name: "replay", body: g}, lic}
		}, nil},
		{"C30 genuine first, then duplicate plant", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g}, {name: "replay", body: pdPlant}, lic}
		}, nil},
		{"C31 no replay member", "fail", func(g []byte) []pdEnt { return []pdEnt{lic, {name: "zzz", body: g}} }, nil},
		{"C32 genuine plus replay.1, completions/replay.bash, README.md", "pass", func(g []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g}, {name: "replay.1", body: []byte(".TH\n")}, {name: "completions/replay.bash", body: []byte("c\n")}, {name: "README.md", body: []byte("r\n")}, lic}
		}, nil},
		{"C33 truncated archive (cut inside the binary)", "fail", nil, func(g []byte) []byte {
			full := pdTar([]pdEnt{lic, {name: "replay", body: g}})
			return pdGzip(full[:len(full)/2])
		}},
		{"C34 not gzip at all", "fail", nil, func([]byte) []byte { return []byte("not a gzip stream\n") }},
		// The mirror of C21: upgrade (Go) reads the genuine member as replay
		// and the plant as zzz; bsdtar reads them the other way round, so
		// install.sh would install the plant. Only the byte agreement between
		// the two readers refuses this.
		{"C35 crossed PAX and GNU names: tar picks plant, upgrade genuine", "fail", func(g []byte) []pdEnt {
			return []pdEnt{
				{name: "a", body: g, pax: [][2]string{{"path", "zzz"}}, gnuLong: "replay"},
				{name: "b", body: pdGoPlant, pax: [][2]string{{"path", "replay"}}, gnuLong: "zzz"},
				lic,
			}
		}, nil},
		// C21 again with a plant that is a real Go binary from the same
		// commit, so no stamp check can catch it: only the bytes differ.
		{"C36 C21 with a same-commit Go binary as the plant", "fail", func(g []byte) []pdEnt {
			return []pdEnt{{name: "decoy", body: pdGoPlant, pax: [][2]string{{"path", "zzz"}}, gnuLong: "replay"}, {name: "replay", body: g}, lic}
		}, nil},
	}

	var rows []string
	failures := 0
	for _, c := range cases {
		f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
		genuine, err := os.ReadFile(filepath.Join(f.stage, "replay"))
		if err != nil {
			t.Fatal(err)
		}
		if pdGoPlant, err = os.ReadFile(f.alt); err != nil {
			t.Fatal(err)
		}
		var archive []byte
		if c.arch != nil {
			archive = c.arch(genuine)
		} else {
			archive = pdGzip(pdTar(c.ents(genuine)))
		}
		pdPublish(t, f, archive)
		out, runErr := f.run(t)
		gatePass := runErr == nil && strings.Contains(out, "REPRODUCES: ")
		up, upErr := pdFetch(t, archive)
		inst := pdInstall(t, archive)
		label := func(b []byte, e error) string {
			switch {
			case e != nil:
				return "error"
			case bytes.Equal(b, genuine):
				return "GENUINE"
			case bytes.Equal(b, pdPlant), bytes.Equal(b, pdGoPlant):
				return "PLANT"
			case b == nil:
				return "none"
			}
			return "other"
		}
		upgrade, install := label(up, upErr), label(inst, nil)
		gate := "refused"
		if gatePass {
			gate = "REPRODUCES"
		}
		verdict := "ok"
		switch {
		case gatePass && upgrade != "GENUINE":
			verdict = "FALSE PASS: upgrade would install " + upgrade
		case c.must == "pass" && !gatePass:
			verdict = "FALSE FAIL"
		case c.must == "fail" && gatePass:
			verdict = "UNEXPECTED PASS"
		}
		if verdict != "ok" {
			failures++
		}
		rows = append(rows, fmt.Sprintf("%-62s must=%-6s gate=%-10s upgrade=%-7s install=%-7s %s", c.id, c.must, gate, upgrade, install, verdict))
	}
	t.Log("\n" + strings.Join(rows, "\n"))
	if failures > 0 {
		t.Errorf("%d case(s) violate the gate's contract", failures)
	}
}

// CF: the gate refuses an archive that is not in the plain form goreleaser
// writes. Every real release archive is flat ustar with regular members and no
// PAX, GNU or global headers (v0.9.0, all four platforms, read 2026-10-14).
// Anything else has a meaning that depends on which tar features the reader
// supports, and the gate only checks agreement with the tar on its own
// machine. Each case below passed v3.4 with the genuine binary, because Go and
// bsdtar happened to agree; a reader that resolves the extension differently
// (a tar that ignores GNU long names, or one that reads past end-of-archive)
// would install the plant. Refusing the form closes that for every reader.
func TestCF_ANonCanonicalArchiveIsRefusedEvenWhenBothReadersAgree(t *testing.T) {
	cases := map[string]func(g, plant []byte) []pdEnt{
		"GNU long name renames a planted replay": func(g, plant []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: plant, gnuLong: "zzz"}, {name: "replay", body: g}}
		},
		"PAX path renames a planted replay": func(g, plant []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: plant, pax: [][2]string{{"path", "zzz"}}}, {name: "replay", body: g}}
		},
		"planted replay after end-of-archive": func(g, plant []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g}, {raw: make([]byte, 1024)}, {name: "replay", body: plant}}
		},
		"global PAX header": func(g, _ []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g, global: [][2]string{{"comment", "x"}}}}
		},
		"GNU-format header on the binary": func(g, _ []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g, magic: "gnu"}}
		},
		// Not the binary, so neither consumer installs these; but no release
		// archive holds them, and an archive that does was not built by
		// goreleaser.
		"symlink member beside the binary": func(g, _ []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g}, {name: "LICENSE", typeflag: '2', link: "/etc/passwd"}}
		},
		"traversal member beside the binary": func(g, _ []byte) []pdEnt {
			return []pdEnt{{name: "replay", body: g}, {name: "../NOTICE", body: []byte("n\n")}}
		},
		"duplicated member beside the binary": func(g, _ []byte) []pdEnt {
			return []pdEnt{{name: "LICENSE", body: []byte("a\n")}, {name: "replay", body: g}, {name: "LICENSE", body: []byte("b\n")}}
		},
	}
	for name, mk := range cases {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
			g, err := os.ReadFile(filepath.Join(f.stage, "replay"))
			if err != nil {
				t.Fatal(err)
			}
			plant, err := os.ReadFile(f.alt)
			if err != nil {
				t.Fatal(err)
			}
			pdPublish(t, f, pdGzip(pdTar(mk(g, plant))))
			out, err := f.run(t)
			rrMustFail(t, out, err, 2, "NOT COMPARED: replay_9.9.9_linux_amd64.tar.gz is not in the plain form goreleaser writes")
		})
	}
}

// CF2: a trailing gzip member after the archive is refused. gzip readers
// differ on whether they read on; Go's does by default.
func TestCF2_ATrailingGzipMemberIsRefused(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	g, err := os.ReadFile(filepath.Join(f.stage, "replay"))
	if err != nil {
		t.Fatal(err)
	}
	plant, err := os.ReadFile(f.alt)
	if err != nil {
		t.Fatal(err)
	}
	a := append(pdGzip(pdTar([]pdEnt{{name: "replay", body: g}})), pdGzip(pdTar([]pdEnt{{name: "replay", body: plant}}))...)
	pdPublish(t, f, a)
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "NOT COMPARED: replay_9.9.9_linux_amd64.tar.gz is not in the plain form goreleaser writes")
}

// CF3: tar and Go must list the same members. Once every header is the
// canonical block a conforming tar lists what Go lists, which is why this is
// shown with a tar that reads the archive differently: here, a tar on PATH
// that lists one member more than the archive holds, standing in for a reader
// that resolves the archive its own way. Until 2026-10-14 this said the plain
// form alone guaranteed agreement; CF5's archive was plain by v3.5's rules and
// bsdtar split it differently from Go.
func TestCF3_ATarThatListsTheArchiveDifferentlyIsRefused(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	realTar, err := exec.LookPath("tar")
	if err != nil {
		t.Fatal(err)
	}
	lying := "#!/bin/sh\ncase \"$1\" in tzf) '" + realTar + "' \"$@\" && echo ghost ;; *) exec '" + realTar + "' \"$@\" ;; esac\n"
	if err := os.WriteFile(filepath.Join(f.bin, "tar"), []byte(lying), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "list the members")
}

// CF4: tar must extract the bytes Go selects. With every header canonical a
// conforming tar does, so, like CF3, this is shown with a tar on PATH that
// extracts other bytes than the archive holds for replay: a reader that
// disagrees with Go about the content. install.sh installs what tar extracts.
// CF5 is the same disagreement from a real tar on a real archive, which v3.5's
// plain form let through and only this comparison caught, on bsdtar.
func TestCF4_ATarThatExtractsDifferentBytesIsRefused(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	realTar, err := exec.LookPath("tar")
	if err != nil {
		t.Fatal(err)
	}
	lying := "#!/bin/sh\ncase \"$1\" in xzf) '" + realTar + "' \"$@\" && cp '" + f.alt + "' replay ;; *) exec '" + realTar + "' \"$@\" ;; esac\n"
	if err := os.WriteFile(filepath.Join(f.bin, "tar"), []byte(lying), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "tar and replay upgrade select different bytes")
}

// CF5: an archive whose members two readers split differently, although they
// list the same names, is refused before anything is compared (F1 of the
// v3.5 review, 2026-10-14). Member "a" has a size field with a leading NUL:
// Go reads the full size and skips a planted replay hidden in a's content,
// then selects the genuine replay that follows; libarchive (bsdtar) read the
// size as 0, found the planted header next, and its size swallowed the
// genuine header. Both listed "a replay", so v3.5's listing check saw no
// difference, and on a machine whose tar agrees with Go the gate would have
// compared the genuine bytes while bsdtar users installed the plant. The
// header bytes must be what Go's writer produces for the values Go read.
func TestCF5_AnArchiveTwoReadersSplitDifferentlyIsRefused(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	g, err := os.ReadFile(filepath.Join(f.stage, "replay"))
	if err != nil {
		t.Fatal(err)
	}
	plant, err := os.ReadFile(f.alt)
	if err != nil {
		t.Fatal(err)
	}
	inner := append(pdHeader("replay", '0', int64(len(pdPad(append([]byte(nil), plant...)))+512+len(g)), "", "", ""), pdPad(append([]byte(nil), plant...))...)
	a := pdHeader("a", '0', 0, "", "", "")
	copy(a[124:136], fmt.Sprintf("\x00%010o\x00", len(inner)))
	pdFixSum(a)
	tb := append(append(a, inner...), pdHeader("replay", '0', int64(len(g)), "", "", "")...)
	tb = append(tb, pdPad(append([]byte(nil), g...))...)
	tb = append(tb, make([]byte, 1024)...)
	archive := pdGzip(tb)
	// The precondition: upgrade's own code accepts this archive and installs
	// the genuine binary, so a refusal is the gate's, not upgrade's.
	got, err := pdFetch(t, archive)
	if err != nil || !bytes.Equal(got, g) {
		t.Fatalf("precondition: replay upgrade does not install the genuine binary from this archive (err %v)", err)
	}
	t.Logf("this machine's tar installs %d bytes (genuine %d, plant %d)", len(pdInstall(t, archive)), len(g), len(plant))
	pdPublish(t, f, archive)
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "NOT COMPARED: replay_9.9.9_linux_amd64.tar.gz is not in the plain form goreleaser writes")
	if !strings.Contains(out, `member "a": header is not the canonical ustar encoding`) {
		t.Fatal("refused, but not by the canonical-header check")
	}
}

// pdFixSum recomputes a header's checksum after a field was edited, so the
// checksum is never the reason a crafted header is refused.
func pdFixSum(h []byte) {
	for i := 148; i < 156; i++ {
		h[i] = ' '
	}
	var sum int64
	for _, c := range h {
		sum += int64(c)
	}
	copy(h[148:156], fmt.Sprintf("%06o\x00 ", sum))
}
