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

/**
 * The launcher's fetch fallback (PRD E6). Run from a copy of this package
 * carrying a released version, with no platform package installed and
 * `fetch` replaced by a stub that records the URL and throws, so nothing
 * in these tests reaches the network.
 */
import { spawnSync } from 'node:child_process';
import { mkdtempSync, cpSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const HERE = new URL('.', import.meta.url).pathname;

function runLauncher(env) {
  const dir = mkdtempSync(join(tmpdir(), 'replay-doctor-npm-'));
  try {
    cpSync(join(HERE, 'bin'), join(dir, 'bin'), { recursive: true });
    cpSync(join(HERE, 'lib'), join(dir, 'lib'), { recursive: true });
    writeFileSync(join(dir, 'package.json'), JSON.stringify({ name: 'replay-doctor', version: '1.0.0', type: 'module' }));
    writeFileSync(join(dir, 'stub-fetch.mjs'), "globalThis.fetch = async (url) => { throw new Error('stub fetch: ' + url); };\n");
    return spawnSync(process.execPath, ['--import', pathToFileURL(join(dir, 'stub-fetch.mjs')).href, join(dir, 'bin', 'replay-doctor.js'), 'version'], {
      env: { PATH: process.env.PATH, HOME: dir, REPLAY_DOCTOR_CACHE: join(dir, 'cache'), ...env },
      encoding: 'utf8',
    });
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test('without REPLAY_DOCTOR_ALLOW_FETCH=1 the launcher refuses, names the platform package and the routes, and makes no network call', () => {
  for (const env of [{}, { REPLAY_DOCTOR_ALLOW_FETCH: '0' }, { REPLAY_DOCTOR_ALLOW_FETCH: 'true' }]) {
    const r = runLauncher(env);
    const label = JSON.stringify(env);
    assert.notEqual(r.status, 0, `${label}: exited 0`);
    assert.ok(!r.stderr.includes('stub fetch'), `${label}: a network call was attempted: ${r.stderr}`);
    assert.match(r.stderr, /REPLAY_DOCTOR_ALLOW_FETCH=1/, `${label}: ${r.stderr}`);
    assert.ok(r.stderr.includes(`@replay-doctor/${process.platform}-${process.arch}`), `${label}: ${r.stderr}`);
    assert.match(r.stderr, /go install github\.com\/RedRobotKK\/Replay\/cmd\/replay@latest/, `${label}: ${r.stderr}`);
    assert.match(r.stderr, /https:\/\/github\.com\/RedRobotKK\/Replay\/releases/, `${label}: ${r.stderr}`);
  }
});

test('with REPLAY_DOCTOR_ALLOW_FETCH=1 the launcher fetches this version\'s tarball from the release', () => {
  const r = runLauncher({ REPLAY_DOCTOR_ALLOW_FETCH: '1' });
  const t = target();
  assert.ok(r.stderr.includes(`stub fetch: ${assetURL('1.0.0', archiveName('1.0.0', t))}`), r.stderr);
});
