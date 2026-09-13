import { test } from 'node:test';
import assert from 'node:assert/strict';
import { target, archiveName, assetURL, expectedHash, releaseVersion } from './lib/release.mjs';

test('platforms map to the names goreleaser uses, and nothing else is admitted', () => {
  assert.deepEqual(target('darwin', 'arm64'), { os: 'darwin', cpu: 'arm64' });
  assert.deepEqual(target('linux', 'x64'), { os: 'linux', cpu: 'amd64' });
  assert.equal(target('win32', 'x64'), null);
  assert.equal(target('linux', 'ia32'), null);
});

test('the archive name and URL are the release\'s own', () => {
  const t = { os: 'linux', cpu: 'arm64' };
  assert.equal(archiveName('1.0.0', t), 'replay_1.0.0_linux_arm64.tar.gz');
  assert.equal(assetURL('1.0.0', 'checksums.txt'), 'https://github.com/RedRobotKK/Replay/releases/download/v1.0.0/checksums.txt');
});

test('the expected hash is read from checksums.txt and a missing file is null', () => {
  const sums = 'a'.repeat(64) + '  replay_1.0.0_linux_arm64.tar.gz\n' + 'b'.repeat(64) + '  replay_1.0.0_darwin_arm64.tar.gz\n';
  assert.equal(expectedHash(sums, 'replay_1.0.0_darwin_arm64.tar.gz'), 'b'.repeat(64));
  assert.equal(expectedHash(sums, 'replay_1.0.0_windows_amd64.zip'), null);
});

test('the platform packages are named the way npm resolves them, and there are four', async () => {
  const { TARGETS, platformPackageName } = await import('./scripts/build-platform-packages.mjs');
  assert.equal(TARGETS.length, 4);
  assert.equal(platformPackageName({ os: 'darwin', cpu: 'arm64' }), '@replay-doctor/darwin-arm64');
  // The launcher resolves `@replay-doctor/${process.platform}-${process.arch}`,
  // so the package name must be built from Node's names, not goreleaser's.
  for (const t of TARGETS) assert.match(platformPackageName(t), /^@replay-doctor\/(darwin|linux)-(x64|arm64)$/);
});

test('an unreleased package version is refused rather than fetching a release that does not exist', () => {
  assert.throws(() => releaseVersion('0.0.0-set-by-release-workflow'));
  assert.equal(releaseVersion('1.0.0'), '1.0.0');
});
