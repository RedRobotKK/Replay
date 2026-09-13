/**
 * Pure helpers for the npm shim, kept apart from the launcher so they can be
 * tested without touching the network or the filesystem.
 *
 * The shim does one thing: fetch the goreleaser tarball for this platform from
 * the GitHub release that matches the package version, verify its sha256
 * against the release's checksums.txt, unpack it once into a cache, and exec
 * the binary. The package itself contains no binary, so `npm install` is
 * instant and the bytes that run are the bytes the release signed. The
 * checksums file is the same one cosign signs; the shim verifies the hash and
 * tells you how to verify the signature, because sigstore in Node would be a
 * second implementation of the thing the installer already does.
 */
export const REPO = 'RedRobotKK/Replay';

/** Map Node's platform/arch to goreleaser's names. Returns null when unsupported. */
export function target(platform = process.platform, arch = process.arch) {
  const os = { darwin: 'darwin', linux: 'linux' }[platform];
  const cpu = { x64: 'amd64', arm64: 'arm64' }[arch];
  if (!os || !cpu) return null;
  return { os, cpu };
}

/** The tarball name goreleaser produced for a version and target. */
export function archiveName(version, t) {
  return `replay_${version}_${t.os}_${t.cpu}.tar.gz`;
}

export function assetURL(version, name) {
  return `https://github.com/${REPO}/releases/download/v${version}/${name}`;
}

/** Find the expected sha256 for a file in checksums.txt (goreleaser's "hash  name" lines). */
export function expectedHash(checksums, name) {
  for (const line of checksums.split('\n')) {
    const m = line.trim().match(/^([0-9a-f]{64})\s+\*?(.+)$/);
    if (m && m[2] === name) return m[1];
  }
  return null;
}

/**
 * The version the launcher will fetch, which must be a real release tag.
 *
 * The checked-in package.json carries a placeholder that the publish workflow
 * replaces from the tag. A prefix match let "0.0.0-set-by-release-workflow"
 * through as 0.0.0, which would have sent a launcher run from the repository
 * to fetch a release that does not exist. Whole-string semver, with an
 * optional pre-release suffix for release candidates, and the placeholder is
 * refused by name.
 */
export function releaseVersion(pkgVersion) {
  const v = String(pkgVersion || '');
  if (v.includes('set-by-release-workflow') || !/^\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$/.test(v)) {
    throw new Error(`package version "${v}" is not a released version; the publish workflow sets it from the tag`);
  }
  return v;
}
