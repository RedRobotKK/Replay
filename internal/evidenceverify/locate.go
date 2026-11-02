package evidenceverify

import "crypto/sha256"

// LocateValue builds a Locator for the value token that starts at valueOffset
// in transcript. It is the "create" half of the locator lifecycle; Verify is
// the "verify" half. It returns ok=false when no value token starts at
// valueOffset or when the document is not one valid JSON value, so a caller
// cannot construct a locator for a non-occurrence.
//
// The returned locator always has FragmentLength equal to the value token's
// byte span, and FragmentSHA256 equal to that span's digest, so the
// create/verify pair is correct by construction rather than by convention.
func LocateValue(transcript []byte, valueOffset int64) (Locator, bool) {
	if valueOffset < 0 || valueOffset >= int64(len(transcript)) {
		return Locator{}, false
	}
	occ, err := scanWhole(transcript, valueOffset, DefaultLimits())
	if err != nil || occ == nil {
		return Locator{}, false
	}
	start := occ.valueStart
	end := occ.valueEnd
	fragment := transcript[start:end]
	return Locator{
		TranscriptSHA256: sha256.Sum256(transcript),
		ValueOffset:      valueOffset,
		FragmentLength:   int64(end - start),
		FragmentSHA256:   sha256.Sum256(fragment),
	}, true
}
