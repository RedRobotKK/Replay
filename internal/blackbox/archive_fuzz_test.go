//go:build mutation

package blackbox

// PDF: a seeded, bounded differential fuzz of archive member selection.
//
// TestPD_ holds 36 hand-built archives. This generates archives from a
// grammar of the constructs those cases found (PAX path and linkpath records,
// GNU long names, global headers, ustar prefixes, GNU and V7 magic, every
// link and file type, deceptive names, lone zero blocks, truncation) and
// judges each by the bytes a consumer would install:
//   upgrade = selfupdate.Unpack, the code `replay upgrade` runs
//   install = install.sh's `tar -xzf archive -C dir replay`
//   gate    = the member-selection lines of scripts/reproduce-release.sh, read
//             from the script, with scripts/archive-member prebuilt from this
//             tree
// The contract: if the gate accepts, the bytes it compares are the bytes both
// consumers would install. A violation is minimized and written out.
//
// Tier A runs the selection lines only, so it can try thousands of archives.
// Tier B runs the whole script on a fixture release with a real Go binary and
// a same-commit Go plant, which is slow, so it tries few. Neither is a proof:
// each covers the archives its seed and count produce, on this machine's tar.
//
//   REPRO_FUZZ_SEED (default 20261013)  REPRO_FUZZ_N (default 1500)
//   REPRO_FUZZ_SECONDS (default 240)     REPRO_FUZZ_B (default 16)
//   REPRO_FUZZ_OUT  directory for counterexamples and stats (default: a temp dir)

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/selfupdate"
)

func pdfEnv(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return d
}

var pdfNames = []string{"replay", "replay", "./replay", "bin/replay", "replay/", "replay//", "/replay",
	"a/../replay", "zzz", "decoy", "LICENSE", "replay.1", "completions/replay.bash", "Replay", "replay "}

// pdfGen draws one archive. bodies[0] is the genuine binary, bodies[1] the plant.
func pdfGen(r *rand.Rand, bodies [][]byte) []pdEnt {
	pick := func(xs []string) string { return xs[r.Intn(len(xs))] }
	n := 1 + r.Intn(4)
	var ents []pdEnt
	for i := 0; i < n; i++ {
		e := pdEnt{name: pick(pdfNames)}
		switch r.Intn(10) {
		case 0, 1, 2, 3:
			e.body = bodies[0]
		case 4, 5, 6:
			e.body = bodies[1]
		default:
			e.body = []byte("text\n")
		}
		switch t := r.Intn(12); {
		case t < 7:
		case t == 7:
			e.typeflag = 1 // TypeRegA
		case t == 8:
			e.typeflag, e.body, e.link = '1', nil, pick([]string{"LICENSE", "replay", "zzz"})
		case t == 9:
			e.typeflag, e.body, e.link = '2', nil, pick([]string{"/bin/sh", "zzz", "replay"})
		case t == 10:
			e.typeflag, e.body = '5', nil
		default:
			e.typeflag = '7'
		}
		if r.Intn(3) == 0 {
			k := 1 + r.Intn(2)
			for j := 0; j < k; j++ {
				key := "path"
				if r.Intn(4) == 0 {
					key = "linkpath"
				}
				e.pax = append(e.pax, [2]string{key, pick(pdfNames)})
			}
		}
		if r.Intn(3) == 0 {
			e.gnuLong, e.longFirst = pick(pdfNames), r.Intn(2) == 0
		}
		if r.Intn(10) == 0 {
			e.global = [][2]string{{"path", pick(pdfNames)}}
		}
		switch r.Intn(10) {
		case 0:
			e.magic = "gnu"
		case 1:
			e.magic = "v7"
		}
		if r.Intn(8) == 0 {
			e.prefix = pick([]string{"bin", "evil", "."})
		}
		ents = append(ents, e)
		if r.Intn(25) == 0 {
			ents = append(ents, pdEnt{raw: make([]byte, 512)})
		}
	}
	return ents
}

func pdfArchive(r *rand.Rand, ents []pdEnt) []byte {
	t := pdTar(ents)
	if r != nil && r.Intn(30) == 0 {
		t = t[:len(t)/2]
	}
	return pdGzip(t)
}

// pdfGate runs the script's own member-selection lines on one archive.
func pdfGate(t *testing.T, frag, helper string, archive []byte) (accepted bool, compared []byte) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.tar.gz"), archive, 0o644); err != nil {
		t.Fatal(err)
	}
	script := "set -eu\nrefuse() { echo \"REFUSE: $*\"; exit 2; }\n" +
		"pinned_go() { [ \"$4\" = ./scripts/archive-member ] || exit 99; '" + helper + "' \"$5\" \"$6\" \"$7\"; }\n" +
		"work='" + dir + "'\nNAME=a.tar.gz\n" + frag + "\necho ACCEPT\n"
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C", "HOME=" + dir}
	out, err := cmd.CombinedOutput()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 99 {
			t.Fatalf("the selection lines no longer call scripts/archive-member the way this harness expects:\n%s", out)
		}
		return false, nil
	}
	if !strings.Contains(string(out), "ACCEPT") {
		t.Fatalf("selection lines exited 0 without reaching the end:\n%s", out)
	}
	b, err := os.ReadFile(filepath.Join(dir, "replay.upgrade"))
	if err != nil {
		t.Fatalf("the gate accepted but wrote no upgrade bytes: %v", err)
	}
	return true, b
}

func pdfFragment(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "reproduce-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i, j := strings.Index(s, "members=$(tar tzf"), strings.Index(s, "published=$(shasum")
	if i < 0 || j < i || !strings.Contains(s[i:j], "scripts/archive-member") {
		t.Fatal("the member-selection block, including the archive-member call, was not found in the script")
	}
	return s[i:j]
}

func pdfHelper(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "archive-member")
	cmd := exec.Command("go", "build", "-o", out, "./scripts/archive-member")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building scripts/archive-member: %v\n%s", err, b)
	}
	return out
}

// pdfViolates reports whether one archive breaks the contract.
func pdfViolates(t *testing.T, frag, helper string, archive []byte) (bool, string) {
	accepted, compared := pdfGate(t, frag, helper, archive)
	if !accepted {
		return false, ""
	}
	up, upErr := selfupdate.Unpack(archive)
	inst := pdInstall(t, archive)
	switch {
	case upErr != nil:
		return true, "gate accepted an archive upgrade refuses: " + upErr.Error()
	case !bytes.Equal(compared, up):
		return true, "gate compared bytes other than the ones upgrade installs"
	case !bytes.Equal(compared, inst):
		return true, "gate compared bytes other than the ones install.sh's tar installs"
	}
	return false, ""
}

// pdfMinimize drops entries and header features while the violation holds.
func pdfMinimize(t *testing.T, frag, helper string, ents []pdEnt) []pdEnt {
	still := func(e []pdEnt) bool { v, _ := pdfViolates(t, frag, helper, pdfArchive(nil, e)); return v }
	for changed := true; changed; {
		changed = false
		for i := range ents {
			cand := append(append([]pdEnt(nil), ents[:i]...), ents[i+1:]...)
			if len(cand) > 0 && still(cand) {
				ents, changed = cand, true
				break
			}
			for _, strip := range []func(*pdEnt){
				func(e *pdEnt) { e.pax = nil }, func(e *pdEnt) { e.gnuLong = "" }, func(e *pdEnt) { e.global = nil },
				func(e *pdEnt) { e.prefix = "" }, func(e *pdEnt) { e.magic = "" }, func(e *pdEnt) { e.typeflag = 0; e.link = "" },
			} {
				c := append([]pdEnt(nil), ents...)
				before := fmt.Sprintf("%+v", c[i])
				strip(&c[i])
				if fmt.Sprintf("%+v", c[i]) != before && still(c) {
					ents, changed = c, true
				}
			}
		}
	}
	return ents
}

func pdfOut(t *testing.T) string {
	if d := os.Getenv("REPRO_FUZZ_OUT"); d != "" {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		return d
	}
	return t.TempDir()
}

func TestPDF_A_SelectionDifferential(t *testing.T) {
	seed := int64(pdfEnv("REPRO_FUZZ_SEED", 20261013))
	n, budget := pdfEnv("REPRO_FUZZ_N", 1500), time.Duration(pdfEnv("REPRO_FUZZ_SECONDS", 240))*time.Second
	frag, helper, out := pdfFragment(t), pdfHelper(t), pdfOut(t)
	r := rand.New(rand.NewSource(seed))
	bodies := [][]byte{[]byte("GENUINE-BINARY\n"), pdPlant}
	start := time.Now()
	stats := map[string]int{}
	var found []string
	i := 0
	for ; i < n && time.Since(start) < budget; i++ {
		ents := pdfGen(r, bodies)
		archive := pdfArchive(r, ents)
		accepted, _ := pdfGate(t, frag, helper, archive)
		up, upErr := selfupdate.Unpack(archive)
		inst := pdInstall(t, archive)
		switch {
		case accepted:
			stats["gate accepted"]++
		case upErr != nil:
			stats["gate refused, upgrade refuses too"]++
		case !bytes.Equal(up, inst):
			stats["gate refused, upgrade and install.sh disagree"]++
		default:
			stats["gate refused, upgrade and install.sh agree"]++
		}
		if v, why := pdfViolates(t, frag, helper, archive); v {
			minimal := pdfMinimize(t, frag, helper, ents)
			name := fmt.Sprintf("A-%d-%04d", seed, i)
			_ = os.WriteFile(filepath.Join(out, name+".tar.gz"), archive, 0o644)
			_ = os.WriteFile(filepath.Join(out, name+".min.tar.gz"), pdfArchive(nil, minimal), 0o644)
			_ = os.WriteFile(filepath.Join(out, name+".min.txt"), []byte(fmt.Sprintf("%s\n%+v\n", why, minimal)), 0o644)
			found = append(found, name+": "+why)
		}
	}
	elapsed := time.Since(start).Round(time.Millisecond)
	summary, _ := json.MarshalIndent(map[string]any{"tier": "A", "seed": seed, "requested": n, "ran": i,
		"budget_s": budget.Seconds(), "elapsed": elapsed.String(), "outcomes": stats, "violations": found,
		"tar": pdfTarVersion(), "go": runtime.Version()}, "", "  ")
	_ = os.WriteFile(filepath.Join(out, fmt.Sprintf("A-%d-summary.json", seed)), summary, 0o644)
	t.Logf("%s", summary)
	if i < n {
		t.Logf("time budget reached after %d of %d archives", i, n)
	}
	if len(found) > 0 {
		t.Errorf("%d archive(s) break the contract; minimized counterexamples in %s", len(found), out)
	}
}

func TestPDF_B_EndToEnd(t *testing.T) {
	seed := int64(pdfEnv("REPRO_FUZZ_SEED", 20261013)) + 1
	n, out := pdfEnv("REPRO_FUZZ_B", 16), pdfOut(t)
	r := rand.New(rand.NewSource(seed))
	stats := map[string]int{}
	var found []string
	start := time.Now()
	for i := 0; i < n; i++ {
		f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
		genuine, err := os.ReadFile(filepath.Join(f.stage, "replay"))
		if err != nil {
			t.Fatal(err)
		}
		plant, err := os.ReadFile(f.alt)
		if err != nil {
			t.Fatal(err)
		}
		// Biased toward archives that can pass: the genuine member is always
		// present, at a random position, beside zero to three generated
		// entries. A tier that never reaches REPRODUCES cannot see a false
		// pass, and the unbiased first version reached it 0 times in 40.
		extra := pdfGen(r, [][]byte{genuine, plant})
		if k := r.Intn(4); k < len(extra) {
			extra = extra[:k]
		}
		pos := r.Intn(len(extra) + 1)
		ents := append(append(append([]pdEnt(nil), extra[:pos]...), pdEnt{name: "replay", body: genuine}), extra[pos:]...)
		archive := pdfArchive(r, ents)
		pdPublish(t, f, archive)
		o, runErr := f.run(t)
		pass := runErr == nil && strings.Contains(o, "REPRODUCES: ")
		up, upErr := pdFetch(t, archive)
		inst := pdInstall(t, archive)
		if pass {
			stats["REPRODUCES"]++
		} else {
			stats["refused or differs"]++
		}
		if pass && (upErr != nil || !bytes.Equal(up, genuine) || !bytes.Equal(inst, genuine)) {
			name := fmt.Sprintf("B-%d-%02d", seed, i)
			_ = os.WriteFile(filepath.Join(out, name+".tar.gz"), archive, 0o644)
			_ = os.WriteFile(filepath.Join(out, name+".txt"), []byte(fmt.Sprintf("%+v\n%s", ents, o)), 0o644)
			found = append(found, name)
		}
	}
	if stats["REPRODUCES"] == 0 {
		t.Errorf("no archive reached REPRODUCES, so this tier observed no passing path and proves nothing")
	}
	summary, _ := json.MarshalIndent(map[string]any{"tier": "B", "seed": seed, "ran": n,
		"elapsed": time.Since(start).Round(time.Millisecond).String(), "outcomes": stats, "violations": found,
		"tar": pdfTarVersion(), "go": runtime.Version()}, "", "  ")
	_ = os.WriteFile(filepath.Join(out, fmt.Sprintf("B-%d-summary.json", seed)), summary, 0o644)
	t.Logf("%s", summary)
	if len(found) > 0 {
		t.Errorf("%d archive(s) passed the full gate while a consumer would install other bytes", len(found))
	}
}

func pdfTarVersion() string {
	b, _ := exec.Command("tar", "--version").Output()
	return strings.SplitN(strings.TrimSpace(string(b)), "\n", 2)[0]
}
