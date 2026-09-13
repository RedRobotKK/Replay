# replay-doctor (npm shim)

```sh
npx replay-doctor diff ~/.claude/projects/
```

Replay Doctor names the turn your prompt cache broke on and what it cost. It reads the transcripts your coding agent already keeps on disk; nothing leaves your machine.

npm installs the one platform package that matches your machine (`@replay-doctor/darwin-arm64`, `linux-x64`, and so on, pinned to this exact version as optional dependencies), so the binary arrives through npm with the lockfile's integrity hash and provenance covering it. No postinstall script, no network at run time. If the platform package is missing (`--no-optional`), the launcher says so and stops; it does not fetch anything unless you tell it to (below). The package version and the binary version are the same tag. macOS and Linux, amd64 and arm64.

## `REPLAY_DOCTOR_ALLOW_FETCH`

The launcher goes to the network only when this variable is exactly `1`:

```sh
REPLAY_DOCTOR_ALLOW_FETCH=1 npx replay-doctor version
```

With it set, a missing platform package makes the launcher fetch the release tarball for your platform from the GitHub release that matches the package version, verify its sha256 against the release's `checksums.txt`, cache it once per version, and run it. Any other value, or no value, is a refusal: the launcher exits non-zero and prints the package it could not find, this variable, and the two other routes to the same binary, `go install github.com/RedRobotKK/Replay/cmd/replay@latest` and the release page at <https://github.com/RedRobotKK/Replay/releases>.

Documentation, the other install routes and the signature check: <https://replay.doctor/install/>
