// Package regression holds the frozen-defect harness: one entry per defect
// this project has found and fixed, and a test that fails if it comes back.
//
// It exists because of what the defects have in common. Almost none of them
// announced themselves. A session-wide field compared two lanes and scored a
// cache hit that never happened; a display cap showed 38 of 119 lint issues;
// a rate-limit header was written up as a falling counter before any value had
// been read. In every case an instrument reported success, and success and
// failure were indistinguishable through it. Nobody audits the direction that
// looks fine.
//
// So an entry here is not a note. It is a check that can fail, and every one of
// them has been made to fail on purpose — the relevant code or claim broken,
// the red observed, the break reverted — because an entry that cannot go red is
// the same defect one level up.
//
// Two kinds of entry live here, and the difference is deliberate:
//
//   - Behavioural guards, where a fix is in this tree and its absence is
//     observable. These read the tree: the wire family a client is claimed to
//     speak, the platforms a document says are built against the platforms the
//     release config builds, the figures the installer prints against the
//     evidence they came from.
//
//   - Registry rows, in frozen_defects_test.go, for defects whose guard lives
//     with the code that owns it, and for defects found and fixed on a branch
//     that has not reached this tree. A row cannot claim to be guarded without
//     naming a test function that exists, which is the same rule the surface
//     registry applies to a LIVE status: a claim carries its evidence, and the
//     test fails when the status outruns it.
//
// The package deliberately contains no production code. Nothing imports it.
package regression
