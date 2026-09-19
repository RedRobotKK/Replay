package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// A Codex machine can contribute, and the share it contributes is a fraction.
//
// Until now only a Claude Code corpus could produce a submission, so the pool
// could never answer what Astra costs against what Fable costs no matter how
// many people contributed. `replay cost --contribute` reads Claude Code
// transcripts and nothing else.
//
// The arithmetic here is deliberately NOT the arithmetic on the Claude path,
// and this is the reason. That path divides a re-billed total priced at the
// full base input rate by spend that is mostly cache reads at a tenth of it,
// which is two price scales in one ratio and exceeds 1 on a well-cached
// session that broke badly. Here the re-billed figure is the cold tokens
// priced at the same model's input rate, and those tokens are already inside
// the total because a cold read IS billed as fresh input. So the share is a
// genuine fraction of spend, bounded by construction rather than by a guard.
//
// The fixtures are the real rollout shape, read off a live rollout 2026-09-17:
// session_meta carries the id, turn_context carries the model, and the usage
// rides on an event_msg of type token_count.
func codexRollout(id, model string, turns []struct{ input, cached, output int }) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"type":"session_meta","payload":{"id":%q,"cli_version":"0.9.0"}}`+"\n", id)
	fmt.Fprintf(&b, `{"type":"turn_context","payload":{"model":%q}}`+"\n", model)
	// The cumulative ACCUMULATES, because Codex's does: append_last_usage is
	// `total += last; last = last`, so total_token_usage after turn n is the
	// sum of every delta up to n.
	//
	// This generator used to write the per-turn figure into the cumulative
	// field on every turn, which no client could produce: a two-turn session
	// came out claiming a running total smaller than its own first two deltas.
	// Nothing noticed until the billing basis started reading the cumulative,
	// because until then the field was only ever compared against itself.
	var run struct{ input, cached, output int }
	for _, t := range turns {
		run.input += t.input
		run.cached += t.cached
		run.output += t.output
		fmt.Fprintf(&b, `{"type":"event_msg","payload":{"type":"token_count","info":{`+
			`"total_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"cache_write_input_tokens":0,`+
			`"output_tokens":%d,"reasoning_output_tokens":0,"total_tokens":%d},`+
			`"last_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"cache_write_input_tokens":0,`+
			`"output_tokens":%d,"reasoning_output_tokens":0,"total_tokens":%d}}}}`+"\n",
			run.input, run.cached, run.output, run.input+run.output,
			t.input, t.cached, t.output, t.input+t.output)
	}
	return b.String()
}

func writeCodexCorpus(t *testing.T, sessions map[string]string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, body := range sessions {
		p := filepath.Join(dir, "rollout-"+id+".jsonl")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

type turn = struct{ input, cached, output int }

// withOpenAIRules installs the OpenAI document for the process, which is what a
// Codex user has to do before any of this prices at all. The compiled table is
// Anthropic-only, so without this the corpus is correctly unpriceable and the
// refusal path below is the only one reachable.
func withOpenAIRules(t *testing.T) {
	t.Helper()
	restore := cachemodel.Override(&cachemodel.Rules{
		Schema:  cachemodel.RulesSchema,
		Version: "openai-2026-09-15",
		Models: []cachemodel.ModelRule{
			{Match: "gpt-6-astra", MinPrefix: 1024, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true},
			{Match: "gpt-5.6-terra", MinPrefix: 1024, InputPerMTok: 2, OutputPerMTok: 12, ReadMult: 0.1, Priced: true},
		},
	})
	t.Cleanup(restore)
}

func TestBurnContributionCarriesTheCodexSurfaceAndItsModels(t *testing.T) {
	withOpenAIRules(t)
	home := writeCodexCorpus(t, map[string]string{
		"a": codexRollout("a", "gpt-6-astra", []turn{{40000, 16000, 200}, {60000, 50000, 300}}),
		"b": codexRollout("b", "gpt-5.6-terra", []turn{{20000, 8000, 100}}),
	})

	f, err := codexContribution(home, "")
	if err != nil {
		t.Fatalf("codexContribution: %v", err)
	}

	if f.Tasks != 2 {
		t.Errorf("Tasks = %d, want 2: a task is a session on this surface", f.Tasks)
	}
	if len(f.Surfaces) != 1 || f.Surfaces[0] != "codex" {
		t.Errorf("Surfaces = %v, want [codex]", f.Surfaces)
	}
	for model, want := range map[string]int{"gpt-6-astra": 2, "gpt-5.6-terra": 1} {
		if f.Models[model] != want {
			t.Errorf("Models[%s] = %d, want %d", model, f.Models[model], want)
		}
	}
}

// The property the Claude path does not have.
func TestBurnContributionShareIsAFractionByConstruction(t *testing.T) {
	withOpenAIRules(t)
	// A session that cached heavily and broke: the second turn's cached share
	// collapses, which is what a break looks like on this surface.
	home := writeCodexCorpus(t, map[string]string{
		"a": codexRollout("a", "gpt-6-astra", []turn{
			{100000, 95000, 100}, // well cached
			{120000, 0, 100},     // the break: nothing served from cache
			{130000, 120000, 100},
		}),
	})

	f, err := codexContribution(home, "")
	if err != nil {
		t.Fatalf("codexContribution: %v", err)
	}
	if f.TotalUSD <= 0 {
		t.Fatalf("TotalUSD = %f, want a priced total", f.TotalUSD)
	}
	if f.RebilledShare < 0 || f.RebilledShare > 1 {
		t.Errorf("RebilledShare = %.4f, want a fraction of spend.\n"+
			"  RebilledUSD %.6f is the cold tokens at the model's input rate\n"+
			"  TotalUSD    %.6f is what the session cost\n"+
			"A cold read is billed as fresh input, so the first is inside the second.",
			f.RebilledShare, f.RebilledUSD, f.TotalUSD)
	}
	if f.RebilledUSD > f.TotalUSD {
		t.Errorf("RebilledUSD %.6f exceeds TotalUSD %.6f, so the two are not on one price scale",
			f.RebilledUSD, f.TotalUSD)
	}
}

// An unpriced corpus refuses, and says the one thing that would fix it.
func TestBurnContributionRefusesWhenNothingCanBePriced(t *testing.T) {
	home := writeCodexCorpus(t, map[string]string{
		// A model no table carries. Real: gpt-5.1-codex-mini is the most
		// common model on a Codex machine and OpenAI publishes no rate for it.
		"a": codexRollout("a", "gpt-5.1-codex-mini", []turn{{40000, 16000, 200}}),
	})

	_, err := codexContribution(home, "")
	if err == nil {
		t.Fatal("an unpriced corpus produced a submission, which would pool a total of zero as though it were a measurement")
	}
	for _, want := range []string{"rules", "docs/rules/openai-2026-09-15.json"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q, so the reader is told no and not what to do: %v", want, err)
		}
	}
}

// Tasks counts the sessions the money covers, not every session on disk.
//
// TotalUSD is the sum over PRICED sessions. If Tasks counted the unpriced ones
// too, a pool dividing one by the other would report a per-task cost lower than
// anything that happened, and the error grows with how much of the corpus is
// unpriceable. On a real Codex machine that is most of it: gpt-5.1-codex-mini
// is the most common model there and OpenAI publishes no rate for it.
//
// Unpriced carries the requests that were left out, so the submission says how
// much of the corpus the total does not cover rather than hiding it.
func TestBurnContributionCountsOnlyTheSessionsTheMoneyCovers(t *testing.T) {
	withOpenAIRules(t)
	home := writeCodexCorpus(t, map[string]string{
		"priced":   codexRollout("priced", "gpt-6-astra", []turn{{40000, 16000, 200}}),
		"unpriced": codexRollout("unpriced", "gpt-5.1-codex-mini", []turn{{50000, 20000, 300}, {10000, 0, 50}}),
	})

	f, err := codexContribution(home, "")
	if err != nil {
		t.Fatalf("codexContribution: %v", err)
	}
	if f.Tasks != 1 {
		t.Errorf("Tasks = %d, want 1: TotalUSD covers one session, so dividing by %d would "+
			"report a per-task cost lower than anything that happened", f.Tasks, f.Tasks)
	}
	if f.Unpriced != 2 {
		t.Errorf("Unpriced = %d, want 2: the submission must say how much of the corpus the total leaves out", f.Unpriced)
	}
	// The unpriced model is still named. A pool that knows gpt-5.1-codex-mini
	// ran and could not be priced knows something worth knowing; a pool that
	// never hears the name cannot tell an absent model from an absent machine.
	if f.Models["gpt-5.1-codex-mini"] != 2 {
		t.Errorf("Models[gpt-5.1-codex-mini] = %d, want 2: an unpriced model is still a model that ran", f.Models["gpt-5.1-codex-mini"])
	}
}

// A rollout that could not be read is counted and reported, never skipped into
// silence.
//
// internal/regression TestED1 enforces this on the contribution path by name,
// and the reason it gives is this exact failure: a skipped transcript becomes a
// published figure with no note that anything was skipped. A pool cannot tell a
// corpus of 170 sessions from a corpus of 176 where 6 were unreadable, and the
// second is a weaker measurement wearing the first one's confidence.
func TestBurnContributionCountsWhatItCouldNotRead(t *testing.T) {
	withOpenAIRules(t)
	home := writeCodexCorpus(t, map[string]string{
		"good": codexRollout("good", "gpt-6-astra", []turn{{40000, 16000, 200}}),
	})
	// A rollout that is not JSON at all. On a real machine this is a truncated
	// write, a half-synced file, or a format that moved.
	bad := filepath.Join(home, ".codex", "sessions", "rollout-bad.jsonl")
	if err := os.WriteFile(bad, []byte("\x00\x01 not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := codexContribution(home, "")
	if err != nil {
		t.Fatalf("codexContribution: %v", err)
	}
	if f.Unreadable == 0 {
		t.Errorf("an unreadable rollout was skipped without being counted; " +
			"the submission would report a clean corpus that was not clean")
	}
}

// codexRolloutRaw writes a rollout from explicit (last, cumulative) pairs, so a
// test can state a sequence the accumulating generator above cannot: a repeated
// broadcast, or a cumulative that does not match the deltas.
func codexRolloutRaw(id, model string, events [][4]int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"type":"session_meta","payload":{"id":%q,"cli_version":"0.155.0-alpha.2.6"}}`+"\n", id)
	fmt.Fprintf(&b, `{"type":"turn_context","payload":{"model":%q}}`+"\n", model)
	for _, e := range events {
		lastIn, lastOut, totIn, totOut := e[0], e[1], e[2], e[3]
		fmt.Fprintf(&b, `{"type":"event_msg","payload":{"type":"token_count","info":{`+
			`"total_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"cache_write_input_tokens":0,`+
			`"output_tokens":%d,"reasoning_output_tokens":0,"total_tokens":%d},`+
			`"last_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"cache_write_input_tokens":0,`+
			`"output_tokens":%d,"reasoning_output_tokens":0,"total_tokens":%d}}}}`+"\n",
			totIn, totOut, totIn+totOut, lastIn, lastOut, lastIn+lastOut)
	}
	return b.String()
}

// A session whose billing basis is not established contributes nothing, and is
// not counted as unpriced.
//
// Those are different cells. Unpriced means the rules document does not carry
// the model and installing one fixes it; this means the token counts could not
// be reconstructed, and no price table touches that.
func TestContributionExcludesAnUnestablishedBasisWithoutCallingItUnpriced(t *testing.T) {
	withOpenAIRules(t)
	repeat := [4]int{60000, 300, 100200, 500}
	home := writeCodexCorpus(t, map[string]string{
		"good": codexRollout("good", "gpt-6-astra", []turn{{40000, 16000, 200}, {60000, 50000, 300}}),
		"dup": codexRolloutRaw("dup", "gpt-6-astra", [][4]int{
			{40000, 200, 40000, 200},
			repeat,
			repeat,
		}),
	})

	f, err := codexContribution(home, "")
	if err != nil {
		t.Fatalf("codexContribution: %v", err)
	}
	if f.Tasks != 1 {
		t.Errorf("Tasks = %d, want 1: only the reconciling session may be pooled", f.Tasks)
	}
	if f.UnmeasuredSessions != 1 {
		t.Errorf("UnmeasuredSessions = %d, want 1: the excluded session must be represented, "+
			"not silently dropped", f.UnmeasuredSessions)
	}
	if f.Unpriced != 0 {
		t.Errorf("Unpriced = %d, want 0: a session with no billing basis is not an unpriced "+
			"one, and saying so sends a contributor after a rules document that cannot help",
			f.Unpriced)
	}
	// The contributed money is the good session's alone.
	if f.TotalUSD <= 0 {
		t.Fatal("the reconciling session was not priced")
	}
	one := writeCodexCorpus(t, map[string]string{
		"good": codexRollout("good", "gpt-6-astra", []turn{{40000, 16000, 200}, {60000, 50000, 300}}),
	})
	alone, err := codexContribution(one, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.TotalUSD != alone.TotalUSD {
		t.Errorf("TotalUSD = %v with the refused session present, %v without it: a refused "+
			"session must contribute no money at all", f.TotalUSD, alone.TotalUSD)
	}
}

// The contribution note states the exclusion.
func TestTheContributionNoteReportsTheUnmeasuredSessions(t *testing.T) {
	var b strings.Builder
	writeCodexContributionNote(&b, "/tmp/x.json", nil, corpusFigures{
		Tasks: 1, TotalUSD: 1.5, UnmeasuredSessions: 2,
	})
	got := b.String()
	if !strings.Contains(got, "UNMEASURED") {
		t.Errorf("the note does not report the excluded sessions:\n%s", got)
	}
	if !strings.Contains(got, "2 session(s)") {
		t.Errorf("the note does not say how many were excluded:\n%s", got)
	}
}

// An all-refused corpus is not reported as an unpriced one.
func TestAnAllRefusedCorpusIsNotSentToTheRulesDocument(t *testing.T) {
	withOpenAIRules(t)
	repeat := [4]int{60000, 300, 100200, 500}
	home := writeCodexCorpus(t, map[string]string{
		"dup": codexRolloutRaw("dup", "gpt-6-astra", [][4]int{
			{40000, 200, 40000, 200}, repeat, repeat,
		}),
	})
	_, err := codexContribution(home, "")
	if err == nil {
		t.Fatal("a corpus with no establishable basis produced a contribution")
	}
	if !strings.Contains(err.Error(), "NOT MEASURED") {
		t.Errorf("the refusal does not name what happened:\n%v", err)
	}
	if strings.Contains(err.Error(), "replay rules --update") {
		t.Errorf("the refusal advises installing a rules document, which cannot establish a "+
			"billing basis:\n%v", err)
	}
}

// burn excludes an unestablished session from the priced totals, and does not
// count it as unpriced.
func TestBurnExcludesAnUnestablishedBasisFromThePricedTotals(t *testing.T) {
	withOpenAIRules(t)
	good := codexRollout("good", "gpt-6-astra", []turn{{40000, 16000, 200}, {60000, 50000, 300}})
	repeat := [4]int{60000, 300, 100200, 500}
	dup := codexRolloutRaw("dup", "gpt-6-astra", [][4]int{
		{40000, 200, 40000, 200}, repeat, repeat,
	})

	clean := burnCodex(writeCodexCorpus(t, map[string]string{"good": good}), "")
	both := burnCodex(writeCodexCorpus(t, map[string]string{"good": good, "dup": dup}), "")

	if both.tokens != clean.tokens {
		t.Errorf("tokens = %d with the refused session present, %d without: a session with no "+
			"billing basis must contribute no tokens", both.tokens, clean.tokens)
	}
	if both.costUSD != clean.costUSD {
		t.Errorf("costUSD = %v with the refused session, %v without: it must contribute no money",
			both.costUSD, clean.costUSD)
	}
	if both.unpricedReqs != clean.unpricedReqs {
		t.Errorf("unpricedReqs went %d -> %d: a refused session is not an unpriced one",
			clean.unpricedReqs, both.unpricedReqs)
	}
	if both.sessions != 2 {
		t.Errorf("sessions = %d, want 2: the session was read, it just cannot be billed", both.sessions)
	}
	var said bool
	for _, p := range both.problems {
		if strings.Contains(p, "NOT MEASURED") {
			said = true
		}
	}
	if !said {
		t.Errorf("burn dropped a session from its totals without saying so:\n%v", both.problems)
	}
}
