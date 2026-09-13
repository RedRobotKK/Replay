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
// The number is 1.25.13 because govulncheck said so on its first run. The floor
// was 1.24.7 for one commit, and CI immediately reported three reachable
// standard-library vulnerabilities at that version: a quadratic parse in
// net/url reached through the self-update client, and post-handshake message
// handling plus an HTTP/2 header timeout in crypto/tls and net/http reached
// through the proxy, which is code that sits in a credential path. All three
// are fixed in 1.25.13. Zero third-party dependencies never meant zero
// dependencies, and this is what the distinction cost.
toolchain go1.25.13
