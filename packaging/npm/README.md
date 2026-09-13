# replay-doctor (npm shim)

```sh
npx replay-doctor diff ~/.claude/projects/
```

Replay Doctor names the turn your prompt cache broke on and what it cost. It reads the transcripts your coding agent already keeps on disk; nothing leaves your machine.

npm installs the one platform package that matches your machine (`@replay-doctor/darwin-arm64`, `linux-x64`, and so on, pinned to this exact version as optional dependencies), so the binary arrives through npm with the lockfile's integrity hash and provenance covering it. No postinstall script, no network at run time. If the platform package is missing (`--no-optional`), the launcher says so and fetches the same release tarball from GitHub, verified against the release's `checksums.txt`. The package version and the binary version are the same tag. macOS and Linux, amd64 and arm64.

Documentation, the other install routes and the signature check: <https://replay.doctor/install/>
