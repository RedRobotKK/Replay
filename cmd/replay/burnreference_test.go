package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/reference"
)

// A cache hit rate means little on its own.
//
// "99% of the prompt served from cache" reads as excellent to anybody who has
// not seen another machine's, and as unremarkable to anybody who has. TraceLab
// measured 4,265 sessions across Claude Code and Codex and reports 95.7%, which
// is the first thing that makes 99% mean anything.
//
// Same rule as the context screen: the population travels with the figure on
// the same line, and the vocabulary describes a difference rather than judging
// one. See docs/design/reference-distribution.md.

// BR1: the comparison names its population and its citation.
func TestBR1_TheCacheShareCarriesItsPopulation(t *testing.T) {
	got := cachedShareLine(0.99)
	for _, want := range []string{"95.7%", "4,265 sessions", "arXiv:2606.30560"} {
		if !strings.Contains(got, want) {
			t.Errorf("the line omits %q: %q", want, got)
		}
	}
	// And it does NOT restate the local figure. burn prints that on the row
	// above, in its own precision; two renderings of one measurement on
	// consecutive rows is the failure this comparison exists to reveal.
	if strings.Contains(got, "here") {
		t.Errorf("the line restates the local share the caller already printed: %q", got)
	}
}

// BR2: it describes rather than judges.
//
// A cache share above a published median is not "good". One machine differing
// from a population is one draw, and language implying a verdict is a claim
// this tool cannot support.
func TestBR2_TheCacheShareDoesNotJudge(t *testing.T) {
	for _, share := range []float64{0.50, 0.957, 0.99} {
		got := strings.ToLower(cachedShareLine(share))
		for _, banned := range []string{"good", "bad", "excellent", "poor", "average", "typical", "should"} {
			if strings.Contains(got, banned) {
				t.Errorf("share %.2f judged (%q): %q", share, banned, got)
			}
		}
	}
}

// BR3: a surface that reported no cache share is not compared.
//
// burn.go clears hasCached on Ollama deliberately: every observed request is a
// back-off case, and averaging the unlabelled ones as zero "would look like a
// measurement of the cache instead of a measurement of the logging". A machine
// with no reading has not measured a 0% hit rate.
func TestBR3_NoReadingIsNotAZeroShare(t *testing.T) {
	if got := cachedShareLine(0); got != "" {
		t.Errorf("a surface with no cache reading was compared anyway: %q", got)
	}
}

// BR4: and the reference it reads is the one that ships.
func TestBR4_TheReferenceExists(t *testing.T) {
	r, ok := reference.For("cachedShare")
	if !ok {
		t.Fatal("no compiled reference for cachedShare, so nothing can be compared")
	}
	if r.Citation == "" || r.Population == "" {
		t.Errorf("the cachedShare reference is missing provenance: %+v", r)
	}
}

// BR5: the lookup returns the metric asked for, not merely a metric.
//
// Reported inert: forcing the match true leaves every assertion passing,
// because a wrong reference still renders a well-formed line. What must hold
// is that the cache comparison quotes the cache population and not, say, the
// system-prompt one.
func TestBR5_TheLookupIsByMetric(t *testing.T) {
	cache, ok := reference.For("cachedShare")
	if !ok {
		t.Fatal("no cachedShare reference")
	}
	sys, ok := reference.For("systemPromptShare")
	if !ok {
		t.Fatal("no systemPromptShare reference")
	}
	if cache.Metric != "cachedShare" || sys.Metric != "systemPromptShare" {
		t.Fatalf("For returned %q and %q", cache.Metric, sys.Metric)
	}
	if cache.Citation == sys.Citation {
		t.Fatalf("both references cite %q, so this test cannot tell them apart",
			cache.Citation)
	}
	// The cache line must carry the cache paper, not the other one.
	got := cachedShareLine(0.99)
	if !strings.Contains(got, cache.Citation) {
		t.Errorf("the cache comparison does not cite %s: %q", cache.Citation, got)
	}
	if strings.Contains(got, sys.Citation) {
		t.Errorf("the cache comparison cites the system-prompt paper: %q", got)
	}
}

// BR6: a metric nothing publishes places nothing.
func TestBR6_AnUnpublishedMetricPlacesNothing(t *testing.T) {
	if got := publishedShare("no-such-metric", 0.99); got != "" {
		t.Errorf("an unpublished metric was compared: %q", got)
	}
}

// BR7: the report carries the comparison only where a surface reported a
// cache share, and never more than once per surface.
func TestBR7_TheReportComparesOnlyWhereThereIsAReading(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatalf("burn: %v (%s)", err, errOut.String())
	}
	got := out.String()
	cites := strings.Count(got, "arXiv:2606.30560")
	shares := strings.Count(got, "of the prompt served from cache")
	if cites != shares {
		t.Errorf("%d cache share(s) reported and %d comparison(s) printed:\n%s",
			shares, cites, got)
	}
}
