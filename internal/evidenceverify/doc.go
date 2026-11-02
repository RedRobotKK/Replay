// Package evidenceverify answers one narrow question about immutable evidence
// bytes: what do they establish about a physical occurrence and a literal
// predicate?
//
// The claim under test is
//
//	"this exact transcript, at this exact byte occurrence, explicitly
//	 records <key>: <literal>"
//
// and nothing more. A verdict of CONFIRMED says the pinned bytes record the
// literal; it does NOT say the recorded event actually happened. Recorded
// evidence is not execution truth, and this package never equates the two.
//
// # Why this package exists
//
// Replay decodes provider transcripts into internal/transcript.Block, whose
// IsError field is a plain bool. A JSON decoder resolves the duplicate-key
// trap, and a bool field collapses an absent key into an explicit false:
//
//	{"is_error":false}   and   {"is_error":true,"is_error":false}
//	both leave no trace that the first is "recorded false" while the second
//	is "recorded true AND recorded false".
//
// A normal decoded object is therefore not a verification oracle. This package
// is a byte-level verifier that preserves every physical occurrence and object
// scope before any decoded representation can destroy them, so it can report
// four first-class outcomes instead of a flattened bool.
//
// # Epistemic contract
//
// Verification proceeds through gates, in order:
//
//	target immutable transcript   (digest)
//	  -> target byte occurrence   (offset, fragment digest, JSON structure)
//	  -> key identity             (decoded key at the pinned value)
//	  -> same-object uniqueness   (duplicate-key conflict detection)
//	  -> literal predicate        (match or contradiction)
//
// REFUTED is unreachable until the evidence basis — transcript identity,
// occurrence binding, structural validity, and key identity — has all been
// established. A failure to inspect evidence is UNVERIFIED, never REFUTED.
// A conflicting duplicate key is AMBIGUOUS, never a determined result.
//
// # Source binding
//
// A Locator binds evidence twice: the transcript digest binds the whole
// document globally, and the fragment digest binds the pinned value's bytes
// locally. Offsets are raw physical byte offsets from byte zero — never rune
// positions, character counts, or line/column approximations.
//
// # Grammar
//
// The scanner implements the RFC 8259 grammar strictly. It rejects malformed
// evidence rather than repairing it: invalid UTF-8, unescaped control bytes,
// invalid or truncated escapes, lone surrogates, malformed numbers, and
// trailing bytes after the single root value all fail closed.
package evidenceverify
