package main

// The Windows refusal.
//
// Replay writes two things a reader is entitled to assume are private: a
// derived-data ledger, and a masking vault encrypted under a key file beside
// it. `internal/ownerdir` verifies and tightens the mode of both on every open
// and refuses when it cannot, which is finding 7 in the security review, closed
// on 2026-09-10.
//
// On Windows it does none of that. `modeIsChecked()` returns false, so
// `tighten` returns nil before it chmods or re-stats, and `EnsureDir` and
// `EnsureFile` are no-ops. That is a deliberate decision rather than an
// oversight, and its reasoning is sound: the real analogue is an ACL, and
// approximating an access-control decision from a synthetic mode is worse than
// declining to make one.
//
// What was missing is the other half of that decision. The tree builds and vets
// clean on Windows on every push, and until 2026-09-13 install.sh refused
// Windows while offering `go install` as an alternative, so the one sentence a
// Windows user actually encountered routed them to a working binary that would
// write both artifacts into a directory whose privacy it had declined to check.
//
// So the binary refuses, and the refusal is the supported behaviour rather than
// a crash. "Unsupported" is now a thing the program does, not a line in a
// README.
//
// WHY NOT PORT IT INSTEAD. An owner-only DACL is genuinely reachable from the
// standard library: `syscall` and `unsafe` are both already on the import
// allowlist, and a protected DACL carrying one full-control ACE for the current
// user's SID is a fair equivalent of 0700, arguably stronger because no umask
// can loosen it. The blocker is not the ACL, it is the evidence. `guard
// reachability` and `frozen mutants` both run on ubuntu only, so every refusal
// in an ACL layer would ship unmutated, and this project's whole position is
// that an unmutated guard is indistinguishable from an absent one. Porting
// before those jobs can see Windows would buy a second green light over an
// unmeasured surface, which is the thing being fixed here rather than repeated.
//
// The path back is written down in RELEASE-CRITERIA.md: a Windows leg on the
// mutation jobs, and then this refusal has earned its own removal.

// platformRefusalAtEntry is the guard run() consults.
//
// It is a variable rather than a direct call, and the reason is the sentence
// four paragraphs up: the mutation and reachability jobs run on ubuntu only,
// where platformRefusal returns "". `guard reachability` reported the branch
// UNREACHED on 2026-09-13 and it was right — the one refusal standing between a
// Windows user and an unverified secrets directory was shipping unmutated,
// which this project treats as indistinguishable from having no guard.
//
// A seam, not a second code path. PRODUCTION NEVER ASSIGNS THIS. A test
// substitutes it to make the condition true and watch what the entry point
// does (PG3); another test asserts the default is the real guard, so a Windows
// build still refuses (PG4). Neither of those is reachable if run() stops
// calling it, which is the property the flag this replaced was checking less
// directly.
var platformRefusalAtEntry = platformRefusal
