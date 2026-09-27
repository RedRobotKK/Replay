// Package evidencepin fixes the identity of evidence documents whose history
// cannot be established.
//
// # What a pin is, and what it is not
//
// A pin says: these bytes, from this date. It says nothing about the document
// before that date. That distinction is the entire point of this package,
// because the two documents pinned here both describe themselves as
// preregistrations and neither can be shown to have been one.
//
// A preregistration's value is that the protocol was fixed before results
// existed. For these two, three independent sources were checked on
// 2026-09-27 and none establishes that:
//
//   - Filesystem times are meaningless here. Both files carry mtime
//     2026-09-25 22:05, shared with fourteen other files including ones
//     authored by a different session. That minute is when the worktree was
//     materialised.
//   - Neither file has ever been tracked by git, on any branch, so there is no
//     commit order.
//   - Session transcripts show repeated whole-file rewrites and edits spanning
//     2026-09-25 to 2026-09-27. For the gtm document the earliest write found
//     anywhere is AFTER the date in its own heading.
//
// So the documents are preserved, their original claims are left exactly as
// written, and their evidentiary status is annotated rather than corrected.
// This package pins what is left: the bytes, going forward.
//
// It deliberately does not re-derive any experiment. There is nothing to
// re-derive; the question these documents raise is chronology, not
// computation.
package evidencepin

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// Status is what the surviving evidence establishes about a document's history.
type Status string

const (
	// NotVerifiable means the document's own historical claim could not be
	// corroborated. It is not a accusation that the claim is false; it is a
	// statement that nothing surviving can settle it.
	NotVerifiable Status = "historical status not verifiable from surviving evidence"
)

// Pin is one document's forward-only identity boundary.
type Pin struct {
	Path             string
	SHA256           string
	PinnedFrom       string
	HistoricalStatus Status
	Why              string
}

// Pins is the pinned set.
//
// Both entries are forward-only. Adding a document here asserts that its bytes
// are fixed from PinnedFrom; it asserts nothing about what came before, and
// the Status field exists so that cannot be quietly forgotten.
func Pins() []Pin {
	return []Pin{
		{
			Path:             "../../docs/evidence/gtm-preregistration-2026-09-24.md",
			SHA256:           "2d7651a043d9cb13609f31a8174148480a5a8f6da356e4ed0b5a60ebc1f16204",
			PinnedFrom:       "2026-09-27",
			HistoricalStatus: NotVerifiable,
			Why: "Fourteen modifying operations between 2026-09-25 04:30 and " +
				"2026-09-27 04:33, including a whole-file rewrite and six later " +
				"edits. The earliest write found in any transcript postdates the " +
				"2026-09-24 in the document's own heading.",
		},
		{
			Path:             "../../docs/evidence/seeded-intervention-prereg-2026-09-25.md",
			SHA256:           "0ecd3e46bc46eda318d5a1e56d35001e80803fb11352f9114207a7bb4a8e6648",
			PinnedFrom:       "2026-09-27",
			HistoricalStatus: NotVerifiable,
			Why: "Five modifying operations between 2026-09-25 12:50 and " +
				"2026-09-27 04:33, including a whole-file rewrite. The pilot " +
				"results are structurally separated under their own heading, " +
				"which establishes the document's arrangement and not when the " +
				"protocol above them was fixed.",
		},
	}
}

// Read returns a pinned document's text.
func Read(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // paths are the constants above
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HashOf is the sha256 of a document as it stands on disk.
func HashOf(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // paths are the constants above
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }
