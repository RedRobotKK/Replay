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
toolchain go1.24.7
