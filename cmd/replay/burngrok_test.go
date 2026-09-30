package main

import (
	"bytes"
	"strings"
	"testing"
)

// The Grok local session store reaches the cross-surface report.
//
// `replay grok` already reads ~/.grok/sessions and reconciles the reconstructed
// per-turn stream against Grok's own usage.json. Until this test, that reading
// stopped at that one command's stdout: `replay burn`, which is the report
// answering "what are my agents consuming", enumerated codex, ollama and
// claude-code and did not know Grok existed. A surface readable by one command
// and invisible to the aggregate report is not wired into production
// reporting, whatever the adapter can do.
func TestGW1_GrokAppearsInTheCrossSurfaceReport(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "grok") {
		t.Errorf("grok is missing from the cross-surface report:\n%s", out.String())
	}
}

// The tokens reported are the per-turn sum, not the ledger's session object.
//
// This is GK1 restated at the reporting boundary. The two turns in the fixture
// carry 5,000 and 7,000 prompt tokens; a reader that maximised, or that read
// the session object instead of summing the stream, would report something
// other than 12,000.
func TestGW2_PromptTokensAreThePerTurnSum(t *testing.T) {
	s := burnGrok("", "burndata")
	if s.tokens != 12000 {
		t.Errorf("prompt tokens = %d, want 12000 (5000+7000 summed per turn)", s.tokens)
	}
	if s.sessions != 1 {
		t.Errorf("sessions = %d, want 1", s.sessions)
	}
	// modelCalls is what the column means: calls made to a provider. The two
	// turns declare 1 and 2.
	if s.requests != 3 {
		t.Errorf("requests = %d, want 3 (modelCalls 1+2), not the turn count", s.requests)
	}
}

// Cached reads sit inside the prompt, so the share divides by the prompt.
func TestGW3_CachedShareIsOfTheInclusivePrompt(t *testing.T) {
	s := burnGrok("", "burndata")
	if !s.hasCached {
		t.Fatal("no cached share reported; Grok states cachedReadTokens on every turn")
	}
	if want := 9600.0 / 12000.0; s.cached != want {
		t.Errorf("cached share = %v, want %v (9600 of 12000)", s.cached, want)
	}
}

// Grok is unpriced, and that is a different cell from free.
//
// No xAI rules document exists, so no dollar figure is available. Marking the
// surface localOnly would say nobody bills for it, which is false: xAI does.
// The surface must therefore count as unpriced so the report's own
// missing-spend notice fires.
func TestGW4_GrokIsUnpricedNotFree(t *testing.T) {
	s := burnGrok("", "burndata")
	if s.localOnly {
		t.Error("grok marked localOnly, which says nobody bills for it; xAI does")
	}
	if s.costUSD != 0 {
		t.Errorf("costUSD = %v, want 0: the tick scale is unreconciled and no xAI rate table is installed", s.costUSD)
	}
	if s.unpricedReqs != s.requests {
		t.Errorf("unpricedReqs = %d, want %d: every Grok request is unpriced", s.unpricedReqs, s.requests)
	}
}

// A cache-creation figure of zero is not evidence that no write was billed.
//
// xAI's cache-write pricing was not found, so surface.XAIContract is
// deliberately ContractUnknown. The report must not let a column of zeros read
// as "no write premium", which is the Codex defect the contracts file exists to
// prevent.
func TestGW5_AZeroCacheWriteIsNotReportedAsNoWritePremium(t *testing.T) {
	s := burnGrok("", "burndata")
	joined := strings.ToLower(strings.Join(s.problems, " "))
	if !strings.Contains(joined, "unknown") {
		t.Errorf("the surface reports no unknown-write caveat; a zero cacheCreationTokens with unknown vendor pricing must not read as zero cost:\n%v", s.problems)
	}
}

// An absent store is an ordinary machine, not an error and not a zero reading.
func TestGW6_AnAbsentGrokStoreReadsAsEmpty(t *testing.T) {
	s := burnGrok("", t.TempDir())
	if s.requests != 0 || s.sessions != 0 {
		t.Errorf("absent store reported requests=%d sessions=%d, want 0/0", s.requests, s.sessions)
	}
	if s.hasCached {
		t.Error("absent store reported a cached share; there is nothing to take a share of")
	}
}
