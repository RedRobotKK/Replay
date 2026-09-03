package learn

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// synthetic builds a score corpus: "good" saves a steady share on every
// session, "decoy" swings widely around zero, "tiny" saves a lot on too
// few sessions, and "simple" (a ttl family) saves the same as good with
// less machinery.
func synthetic(n int, seed uint64) ([]Candidate, []SessionScore) {
	r := rand.New(rand.NewPCG(seed, seed))
	candidates := []Candidate{
		{Name: "simple", Family: FamilyTTL},
		{Name: "good", Family: FamilyContextEdit},
		{Name: "decoy", Family: FamilyContextEdit},
		{Name: "tiny", Family: FamilyContextEdit},
	}
	var scores []SessionScore
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("session-%d", i)
		s := SessionScore{SessionID: id, Holdout: isHoldout(id), AsRun: analysis.Tally{PromptTokens: 100, Reads: 80, EffectiveTokens: 100}, Saving: map[string]float64{}, Cached: map[string]float64{}, Estimated: map[string]bool{}}
		s.Saving["good"] = 0.20 + 0.02*r.NormFloat64()
		s.Saving["decoy"] = 0.30 * r.NormFloat64()
		s.Saving["simple"] = 0.20 + 0.02*r.NormFloat64()
		if i < 3 {
			s.Saving["tiny"] = 0.5
		}
		for k := range s.Saving {
			s.Cached[k] = 0.8
		}
		scores = append(scores, s)
	}
	return candidates, scores
}

func find(res Result, name string) Verdict {
	for _, v := range res.Verdicts {
		if v.Name == name {
			return v
		}
	}
	panic("no verdict for " + name)
}

func TestSelectPicksTheKnownBestAndRejectsTheDecoy(t *testing.T) {
	candidates, scores := synthetic(40, 7)
	res := Select(candidates, scores, 40, Options{}, time.Now())
	if res.Selected == nil {
		t.Fatalf("nothing selected: %s", res.Reason)
	}
	// good and simple save the same within noise, so the simpler one
	// wins; the decoy's interval straddles zero; tiny lacks sessions.
	if res.Selected.Name != "simple" {
		t.Fatalf("selected %q, want the simpler tie: %+v", res.Selected.Name, res.Verdicts)
	}
	if v := find(res, "good"); !strings.HasPrefix(v.Decision, "rejected:") || v.Interval[0] <= 0 || v.HoldoutMean <= 0 {
		t.Fatalf("good must qualify and still lose the tie to simple: %+v", v)
	}
	if v := find(res, "decoy"); v.Decision != rejectNoMargin {
		t.Fatalf("decoy must fail the margin rule: %+v", v)
	}
	if v := find(res, "tiny"); !strings.HasPrefix(v.Decision, "rejected: fewer than") {
		t.Fatalf("tiny must fail the evidence rule: %+v", v)
	}
	if res.Sessions.Holdout == 0 || res.Sessions.Holdout >= res.Sessions.Calibrated {
		t.Fatalf("holdout split wrong: %+v", res.Sessions)
	}
}

func TestSelectRequiresTheWinToRepeatOnHoldout(t *testing.T) {
	candidates, scores := synthetic(40, 3)
	// Make "good" fail on every held-out session.
	for i := range scores {
		if scores[i].Holdout {
			scores[i].Saving["good"] = -0.1
			scores[i].Saving["simple"] = -0.1
		}
	}
	res := Select(candidates, scores, 40, Options{}, time.Now())
	if res.Selected != nil {
		t.Fatalf("a win that does not repeat must not be selected: %+v", res.Selected)
	}
	if v := find(res, "good"); v.Decision != rejectNoRepeat {
		t.Fatalf("good: %+v", v)
	}
}

func TestSelectNeedsEvidenceNotJustSessions(t *testing.T) {
	candidates, scores := synthetic(40, 5)
	// A trigger no session reaches scores exactly zero everywhere.
	for i := range scores {
		scores[i].Saving["good"] = 0
	}
	res := Select(candidates, scores, 40, Options{}, time.Now())
	if v := find(res, "good"); v.Sessions != 0 || !strings.HasPrefix(v.Decision, "rejected: fewer than") {
		t.Fatalf("zero savings are not evidence: %+v", v)
	}
}

func TestMeanInterval(t *testing.T) {
	if m, iv := meanInterval(nil); m != 0 || iv != [2]float64{} {
		t.Fatal("empty")
	}
	if _, iv := meanInterval([]float64{0.5}); !math.IsInf(iv[0], -1) {
		t.Fatal("one sample must have an unbounded band")
	}
	m, iv := meanInterval([]float64{0.1, 0.2, 0.3})
	if math.Abs(m-0.2) > 1e-9 || iv[0] >= m || iv[1] <= m {
		t.Fatalf("mean %v interval %v", m, iv)
	}
}

// Score must run the catalog over a real session and report the two
// tiers honestly: TTL candidates measured, context edits estimated.
func TestScoreOnTheFixture(t *testing.T) {
	s, err := transcript.ParseClaudeCodeFile("../transcript/testdata/session-redacted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := Score(s, Catalog())
	if !ok {
		t.Fatal("fixture must calibrate")
	}
	if len(sc.Saving) != len(Catalog()) || sc.Estimated["ttl-1h"] || !sc.Estimated["context-edit(keep=6,trigger=200000)"] {
		t.Fatalf("scores: %+v", sc)
	}
	// One session is never enough to select anything.
	res := Select(Catalog(), []SessionScore{sc}, 1, Options{}, time.Now())
	if res.Selected != nil || res.Reason == "" {
		t.Fatalf("one session must not elect a policy: %+v", res)
	}
}

func TestLoadSelectedRejectsStaleFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, res Result) string {
		path := filepath.Join(dir, name)
		data, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	chosen := Catalog()[3]
	good := Result{Schema: PolicyFileSchema, Rules: cachemodel.RulesVersion, Selected: &chosen}
	if c, note, err := LoadSelected(write("good.json", good)); err != nil || note != "" || c == nil || c.ContextEdit == nil || c.ContextEdit.TriggerTokens != chosen.ContextEdit.TriggerTokens {
		t.Fatalf("good file: %+v %q %v", c, note, err)
	}
	stale := good
	stale.Schema = 99
	if c, note, err := LoadSelected(write("schema.json", stale)); err != nil || c != nil || !strings.Contains(note, "schema 99") {
		t.Fatalf("stale schema: %+v %q %v", c, note, err)
	}
	stale = good
	stale.Rules = "other"
	if c, note, err := LoadSelected(write("rules.json", stale)); err != nil || c != nil || !strings.Contains(note, "learned under rules") {
		t.Fatalf("stale rules: %+v %q %v", c, note, err)
	}
	none := Result{Schema: PolicyFileSchema, Rules: cachemodel.RulesVersion, Reason: "too few"}
	if c, note, err := LoadSelected(write("none.json", none)); err != nil || c != nil || !strings.Contains(note, "too few") {
		t.Fatalf("no selection: %+v %q %v", c, note, err)
	}
	if _, _, err := LoadSelected(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing file must be an error")
	}
}

func TestTypeFromEarlySignals(t *testing.T) {
	cases := []struct {
		model  string
		tokens int
		want   string
	}{
		{"claude-opus-5", 100, "opus/small-prefix"},
		{"claude-fable-5-1", 50000, "fable/large-prefix"},
		{"claude-sonnet-4-6", largePrefixTokens, "sonnet/large-prefix"},
		{"gpt-x", 10, "other/small-prefix"},
	}
	for _, c := range cases {
		if got := TypeOf(c.model, c.tokens); got != c.want {
			t.Errorf("TypeOf(%q, %d) = %q, want %q", c.model, c.tokens, got, c.want)
		}
	}
	s, err := transcript.ParseClaudeCodeFile("../transcript/testdata/session-redacted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got := Type(s); got != "fable/large-prefix" {
		t.Fatalf("fixture type = %q", got)
	}
	if Type(&transcript.Session{}) != "" {
		t.Fatal("an empty session has no type")
	}
}

// LN-3: a candidate that helps one type and hurts another is selected
// for the first, withheld from the second, and the overall selection
// says none because the corpus as a whole does not agree.
func TestSelectPerType(t *testing.T) {
	candidates := []Candidate{{Name: "edit", Family: FamilyContextEdit}}
	var scores []SessionScore
	r := rand.New(rand.NewPCG(11, 11))
	for i := 0; i < 60; i++ {
		id := fmt.Sprintf("s-%d", i)
		typ, saving := "opus/large-prefix", 0.25+0.02*r.NormFloat64()
		if i%2 == 1 {
			typ, saving = "opus/small-prefix", -0.25+0.02*r.NormFloat64()
		}
		scores = append(scores, SessionScore{SessionID: id, Type: typ, Holdout: isHoldout(id), AsRun: analysis.Tally{PromptTokens: 1, EffectiveTokens: 1}, Saving: map[string]float64{"edit": saving}, Cached: map[string]float64{"edit": 0.8}, Estimated: map[string]bool{}})
	}
	res := Select(candidates, scores, 60, Options{}, time.Now())
	if res.Selected != nil {
		t.Fatalf("overall must not select a candidate the types disagree on: %+v", res.Verdicts)
	}
	if len(res.Types) != 2 || res.Types[0].Type != "opus/large-prefix" || res.Types[0].Selected == nil || res.Types[1].Selected != nil {
		t.Fatalf("per-type selection wrong: %+v", res.Types)
	}
	if c := res.SelectedFor("opus/large-prefix"); c == nil || c.Name != "edit" {
		t.Fatalf("SelectedFor large: %+v", c)
	}
	if c := res.SelectedFor("opus/small-prefix"); c != nil {
		t.Fatalf("SelectedFor small must fall back to the overall none: %+v", c)
	}
}

func armCosts(treated, control []float64) []ArmCost {
	var out []ArmCost
	for i, c := range treated {
		out = append(out, ArmCost{SessionID: fmt.Sprintf("t%d", i), Arm: "treated", CostPerNewToken: c})
	}
	for i, c := range control {
		out = append(out, ArmCost{SessionID: fmt.Sprintf("c%d", i), Arm: "control", CostPerNewToken: c})
	}
	return out
}

// DR-2: a policy graduates when treated sessions cost less per new token
// than controls, above noise, by at least half the predicted saving.
func TestGraduate(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 5))
	noisy := func(mean float64, n int) []float64 {
		out := make([]float64, n)
		for i := range out {
			out[i] = mean * (1 + 0.05*r.NormFloat64())
		}
		return out
	}
	if rep := Graduate("edit", armCosts(noisy(8, 10), noisy(10, 10)), 0.2, 0); rep == nil || !rep.Graduated || rep.Realized < 0.1 {
		t.Fatalf("a held saving must graduate: %+v", rep)
	}
	if rep := Graduate("edit", armCosts(noisy(9.5, 10), noisy(10, 10)), 0.2, 0); rep == nil || rep.Graduated || !strings.Contains(rep.Reason, "not graduated") {
		t.Fatalf("a saving far under the prediction must not graduate: %+v", rep)
	}
	if rep := Graduate("edit", armCosts(noisy(8, 3), noisy(10, 10)), 0.2, 0); rep == nil || rep.Graduated || !strings.Contains(rep.Reason, "not judged") {
		t.Fatalf("too few treated sessions must not be judged: %+v", rep)
	}
	if rep := Graduate("edit", armCosts(noisy(10, 10), noisy(10, 10)), 0.2, 0); rep == nil || rep.Graduated || !strings.Contains(rep.Reason, "not separated") {
		t.Fatalf("equal arms must not graduate: %+v", rep)
	}
	if Graduate("edit", nil, 0.2, 0) != nil {
		t.Fatal("no arms means no trial report")
	}
}
