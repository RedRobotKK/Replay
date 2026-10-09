module github.com/RedRobotKK/Replay

go 1.24

// A floor on the toolchain, not a pin. Go switches toolchains only when the
// installed one is OLDER than this line, so a newer compiler is used as-is:
// checked on 2026-09-12 with go1.27.1 installed, which is the version that
// actually compiled the tests. That is deliberate and the go-latest job in
// ci.yml depends on it, because that job exists to catch standard-library
// changes and a real pin would have quietly switched it off.
//
// What it buys is the other direction: a build on a runner older than this
// cannot silently produce a release with a different compiler than the one
// this module was verified against.
//
// The number was 1.25.13 because govulncheck said so on its first run. The floor
// was 1.24.7 for one commit, and CI immediately reported three reachable
// standard-library vulnerabilities at that version: a quadratic parse in
// net/url reached through the self-update client, and post-handshake message
// handling plus an HTTP/2 header timeout in crypto/tls and net/http reached
// through the proxy, which is code that sits in a credential path. All three
// are fixed in 1.25.13. Zero third-party dependencies never meant zero
// dependencies, and this is what the distinction cost.
//
// Raised to 1.27.2 on 2026-10-09 for the thirteen advisories of 2026-10-08
// (GO-2026-6599 to GO-2026-6617), nine of them reachable from this module.
// Every one of their OSV records reads fixed 1.26.9 and fixed 1.27.2, and no
// 1.25.x carries the fix because 1.25 left upstream support when 1.27.0
// shipped. 1.26.9 is the minimum that clears govulncheck; 1.27.2 is chosen
// because 1.26 leaves support when 1.28 ships, and the go-latest job had
// already been green on 1.27.2 with -race before this line moved. The
// language floor above stays at 1.24 on purpose: nothing in the tree needs
// a newer language, and raising it would be a separate decision.
toolchain go1.27.2
