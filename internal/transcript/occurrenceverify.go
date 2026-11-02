package transcript

import "github.com/RedRobotKK/Replay/internal/evidenceverify"

// VerifyIsErrorOccurrence answers the occurrence-pinned question for a single
// raw transcript or ledger line: does this exact line, at this exact byte
// occurrence, explicitly record is_error equal to expect?
//
// It is the forensic view and exists because the ordinary decoded view is
// lossy. RawBlock.IsError is a plain bool (wire.go), and the standard JSON
// decoder that populates it resolves duplicate keys last-wins and collapses an
// absent field into false. A consumer that needs to know which of
// CONFIRMED / REFUTED / AMBIGUOUS / UNVERIFIED the raw bytes establish must
// call this function BEFORE the line is decoded into RawBlock — never fall
// back to Block.IsError, which has already lost the distinction.
//
// This function is deliberately narrow: it does not run on the per-line parse
// path and does not change ordinary consumption. DecodeBlock and its bool
// remain the ordinary view; this is the evidence view for a caller that has a
// pinned occurrence to audit.
func VerifyIsErrorOccurrence(raw []byte, loc evidenceverify.Locator, expect bool) evidenceverify.Result {
	return evidenceverify.Verify(raw, loc, evidenceverify.Predicate{
		Key:    "is_error",
		Expect: evidenceverify.BoolLiteral(expect),
	})
}
