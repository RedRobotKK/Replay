//go:build mutation

package blackbox

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

var updateProductionMatrix = flag.Bool("update-production-matrix", false,
	"rewrite docs/evidence/production-grade-matrix.md from this run")

const matrixPath = "docs/evidence/production-grade-matrix.md"

// row is one surface judged, every cell measured in this run.
type row struct {
	Surface     string
	Entry       string   // the production invocation
	Dispatch    string   // main.go line → entry function
	Deciders    []string // mutant file @ anchor
	Caller      string
	BlackBox    string // PASS (n checks) or the failures
	Oracle      string // the E2E test and the frozen mutants' killers
	Mutants     []string
	Killed      []string
	Survived    []string
	Stillborn   []string
	Negative    string
	Invalid     string
	JSON        string
	Status      string
	Concurrency string
}

var (
	rowsMu sync.Mutex
	rows   = map[string]*row{}
)

func rowFor(name string) *row {
	rowsMu.Lock()
	defer rowsMu.Unlock()
	r, ok := rows[name]
	if !ok {
		r = &row{Surface: name}
		rows[name] = r
	}
	return r
}

// TestBB_ProductionGrade is the gate. One subtest per dispatched surface:
// the real binary must pass every check, refuse the invalid input, emit valid
// JSON where it claims to, and every frozen mutant whose killer is the
// surface's E2E test must, built into a binary, fail the same checks.
func TestBB_ProductionGrade(t *testing.T) {
	bin := productionBinary(t)
	t.Logf("binary %s, %s, commit %s", bin.Path, bin.Version, bin.Commit)
	surfaces := discoverSurfaces(t)
	mutants := loadMutants(t)

	for _, s := range surfaces {
		s := s
		t.Run(s.Canon(), func(t *testing.T) {
			sp, ok := specs[s.Canon()]
			if !ok {
				t.Fatalf("no spec (TestBB_EverySurfaceHasABlackBoxSpec reports this)")
			}
			r := rowFor(s.Canon())
			r.Dispatch = fmt.Sprintf("main.go:%d → %s", s.Line, s.Entry)
			r.Caller = s.Entry
			home, checks := sp.Setup(t, bin.Path)
			r.Entry = "replay " + strings.Join(relativise(home, checks[0].Args), " ")

			// 1. the real binary passes every check
			var failures []string
			for _, c := range checks {
				for _, f := range evaluate(bin.Path, home, c) {
					failures = append(failures, c.Label+": "+f)
				}
			}
			if len(failures) > 0 {
				r.BlackBox = "FAIL: " + strings.Join(failures, "; ")
				t.Errorf("black box: %s", strings.Join(failures, "\n  "))
			} else {
				r.BlackBox = fmt.Sprintf("PASS (%d checks)", len(checks))
			}

			// 2. invalid input is refused
			inv := sp.Invalid(home)
			if f := evaluate(bin.Path, home, inv); len(f) > 0 {
				r.Invalid = "FAIL: " + strings.Join(f, "; ")
				t.Errorf("invalid input (%s): %s", inv.Label, strings.Join(f, "; "))
			} else {
				r.Invalid = "PASS: " + inv.Label
			}

			// 3. machine-readable output parses
			r.JSON = "n/a"
			if sp.JSONOut != nil {
				var bad []string
				n := 0
				for _, args := range sp.JSONOut(home) {
					n++
					out := run(bin.Path, home, "", args...)
					var v any
					if err := json.Unmarshal([]byte(out.Stdout), &v); err != nil {
						bad = append(bad, strings.Join(args[:2], " ")+": "+err.Error())
					}
				}
				if len(bad) > 0 {
					r.JSON = "FAIL: " + strings.Join(bad, "; ")
					t.Errorf("JSON: %s", strings.Join(bad, "; "))
				} else {
					r.JSON = fmt.Sprintf("PASS (%d invocations)", n)
				}
			}

			// 4. the negative matrix: every frozen mutant this surface is
			// held to must, as a binary, fail the same black-box checks.
			r.Oracle = s.E2E()
			ms := mutantsKilledBy(mutants, s.E2E())
			if len(ms) == 0 {
				r.Negative = "NO MUTANT"
				t.Errorf("no frozen mutant names %s as its killer", s.E2E())
			}
			for _, m := range ms {
				r.Mutants = append(r.Mutants, m.ID)
				r.Deciders = append(r.Deciders, m.File+" @ "+firstLine(m.Anchor))
				mb, err := mutatedBinary(t, m)
				if err != nil {
					r.Stillborn = append(r.Stillborn, m.ID)
					t.Errorf("%s: %v", m.ID, err)
					continue
				}
				// A fresh world for the mutated binary: the real binary's
				// checks may have left state behind (a cost index, a since
				// marker) that would let a broken cold path read a warm one.
				mhome, mchecks := sp.Setup(t, bin.Path)
				mchecks = append(mchecks, sp.Invalid(mhome))
				var any bool
				for _, c := range mchecks {
					if len(evaluate(mb, mhome, c)) > 0 {
						any = true
						break
					}
				}
				if any {
					r.Killed = append(r.Killed, m.ID)
				} else {
					r.Survived = append(r.Survived, m.ID)
					t.Errorf("%s (%s) survived the black-box checks: the binary with that mutation passes every check the real one passes", m.ID, m.Name)
				}
			}
			switch {
			case len(ms) == 0:
				r.Negative = "NO MUTANT"
			case len(r.Survived)+len(r.Stillborn) == 0:
				r.Negative = fmt.Sprintf("PASS (%d/%d killed)", len(r.Killed), len(ms))
			default:
				r.Negative = fmt.Sprintf("FAIL (%d killed, %d survived, %d stillborn)", len(r.Killed), len(r.Survived), len(r.Stillborn))
			}
			r.Status = status(r)
		})
	}
}

func status(r *row) string {
	if strings.HasPrefix(r.BlackBox, "PASS") && strings.HasPrefix(r.Invalid, "PASS") && (r.JSON == "n/a" || strings.HasPrefix(r.JSON, "PASS")) && strings.HasPrefix(r.Negative, "PASS") {
		return "PRODUCTION-GRADE"
	}
	if r.Negative == "NO MUTANT" {
		return "UNMEASURED"
	}
	return "BLOCKED"
}

// relativise strips the two paths that name the generating machine, the
// repository root first because on a laptop it sits under the home directory.
func relativise(home string, args []string) []string {
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		if repo != "" {
			out[i] = strings.ReplaceAll(out[i], repo, "$REPO")
		}
		out[i] = strings.ReplaceAll(out[i], home, "$HOME")
		out[i] = strings.ReplaceAll(out[i], filepath.Dir(filepath.Dir(home)), "…")
	}
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	if len(s) > 56 {
		s = s[:53] + "…"
	}
	return s
}

// TestBB_Matrix renders the matrix from this run and compares it with the
// committed document. It runs after TestBB_ProductionGrade and
// TestBB_ServeUnderLoad in source order.
func TestBB_Matrix(t *testing.T) {
	if len(rows) == 0 {
		t.Skip("no rows: run the whole package")
	}
	bin := productionBinary(t)
	want := renderMatrix(bin)
	path := filepath.Join(repoRoot(t), matrixPath)
	if *updateProductionMatrix {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	have, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (run with -update-production-matrix)", matrixPath, err)
	}
	if string(have) != want {
		t.Errorf("%s is stale against this run; regenerate with\n  go test -tags mutation ./internal/blackbox/ -run 'TestBB' -update-production-matrix\n--- want ---\n%s", matrixPath, want)
	}
}

func renderMatrix(bin binary) string {
	names := make([]string, 0, len(rows))
	for n := range rows {
		names = append(names, n)
	}
	// dispatch order, not alphabetical: the order main.go declares them
	sort.Slice(names, func(i, j int) bool { return rows[names[i]].Dispatch < rows[names[j]].Dispatch })
	sort.SliceStable(names, func(i, j int) bool {
		var a, b int
		fmt.Sscanf(rows[names[i]].Dispatch, "main.go:%d", &a)
		fmt.Sscanf(rows[names[j]].Dispatch, "main.go:%d", &b)
		return a < b
	})
	var b strings.Builder
	b.WriteString("# Production-grade wiring matrix\n\n")
	b.WriteString("Generated by `TestBB_ProductionGrade` in `internal/blackbox` from the production binary\n")
	b.WriteString("(`go build ./cmd/replay`, run as a child process with its own HOME), the `dispatch()`\n")
	b.WriteString("switch, and `internal/mutation/testdata/mutants.json`. Every cell is measured: the real\n")
	b.WriteString("binary passes the checks; the invalid input is refused; the JSON parses; each frozen\n")
	b.WriteString("mutant whose killer is the surface's E2E test, built into a binary, fails the same checks.\n")
	b.WriteString("`TestBB_Matrix` fails when this file and the run disagree. Regenerate with\n")
	b.WriteString("`go test -tags mutation ./internal/blackbox/ -run 'TestBB' -update-production-matrix`.\n")
	b.WriteString("Statuses are PRODUCTION-GRADE, NOT WIRED, BLOCKED or UNMEASURED and nothing else.\n\n")
	b.WriteString("| Surface | Production Entry | Dispatch | Decider | Production Caller | Black Box | Oracle | Mutation Killed | Negative Test | Status |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	counts := map[string]int{}
	for _, n := range names {
		r := rows[n]
		counts[r.Status]++
		cell := func(s string) string { return "`" + strings.ReplaceAll(s, "|", "\\|") + "`" }
		killed := strings.Join(r.Killed, ", ")
		if killed == "" {
			killed = "none"
		}
		extra := ""
		if r.Concurrency != "" {
			extra = "; " + r.Concurrency
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s%s | %s | %s | %s | **%s** |\n",
			r.Surface, cell(r.Entry), cell(r.Dispatch), cell(strings.Join(r.Deciders, "; ")), cell(r.Caller),
			r.BlackBox+"; invalid: "+r.Invalid+"; JSON: "+r.JSON, extra, cell(r.Oracle), killed, r.Negative, r.Status)
	}
	fmt.Fprintf(&b, "\n**%d surfaces: %d PRODUCTION-GRADE, %d NOT WIRED, %d BLOCKED, %d UNMEASURED.**\n\n",
		len(names), counts["PRODUCTION-GRADE"], counts["NOT WIRED"], counts["BLOCKED"], counts["UNMEASURED"])
	if counts["PRODUCTION-GRADE"] == len(names) && len(names) > 0 {
		b.WriteString("ALL WIRED SURFACES PRODUCTION-GRADE\n")
	} else {
		b.WriteString("PRODUCTION-GRADE WIRING INCOMPLETE\n")
	}
	return b.String()
}
