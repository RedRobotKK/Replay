package analysis

import (
	"strings"
	"testing"
)

// The footnote under an estimated figure has to say two different things,
// because two different things happened.
//
// A session that fitted its own turns measured its own ratio. A Japanese
// session fits a Japanese ratio and is right about it. Telling that user the
// number came from an English constant would be false.
//
// A session with no fittable turn fell back to defaultTokensPerByte, which is
// an English prose average. That user is owed the provenance, because the
// constant does not describe their script.
//
// The existing footnote says "estimated through the byte-to-token fit" in both
// cases, which is true and insufficient: it does not distinguish a measured
// ratio from a borrowed one.
func TestFitNote_SeparatesAFittedRatioFromTheFallbackConstant(t *testing.T) {
	fitted := TokenFit{TokensPerByte: 0.62, Turns: 4}
	note := FitNote(fitted)
	if strings.Contains(note, "English") {
		t.Errorf("a session that fitted %d turns measured its own ratio; calling it "+
			"English-derived is false:\n%s", fitted.Turns, note)
	}
	if !strings.Contains(note, "byte-to-token fit") {
		t.Errorf("the fitted note must still explain what the mark means:\n%s", note)
	}

	fallback := TokenFit{TokensPerByte: defaultTokensPerByte, Turns: 0}
	note = FitNote(fallback)
	if !strings.Contains(note, "English") {
		t.Errorf("with no fittable turn the ratio is defaultTokensPerByte, an English "+
			"prose average. The footnote must say so, or a CJK session reads a "+
			"borrowed constant as a measurement:\n%s", note)
	}
	if !strings.Contains(note, "no turn") {
		t.Errorf("the note must say why the constant was used, so the user knows a "+
			"longer session would fix it:\n%s", note)
	}
}
