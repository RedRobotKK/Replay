#!/usr/bin/env node
/**
 * Build the per-platform npm packages from the release tarballs.
 *
 *   node packaging/npm/scripts/build-platform-packages.mjs --version 1.0.0 --dist dist-release
 *
 * This is the esbuild pattern, which is what every widely installed native
 * CLI on npm uses in 2026: one small package per target that carries the
 * binary (@replay-doctor/darwin-arm64 and so on), and the main package lists
 * them as optionalDependencies pinned to the same version. npm installs
 * exactly the one that matches the machine, the lockfile's integrity hash
 * covers the binary bytes, provenance attests them, and it works offline and
 * behind proxies that cannot reach GitHub. No postinstall script anywhere.
 *
 * Each tarball is verified against checksums.txt before its binary is copied,
 * so a platform package cannot be built from bytes the release did not sign.
 * Output goes to packaging/npm/build/<target>/ and the main package.json is
 * rewritten with the version and the four optionalDependencies.
 */
import { readFileSync, writeFileSync, mkdirSync, rmSync, existsSync, chmodSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expectedHash } from '../lib/release.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const NPM_DIR = join(here, '..');
export const TARGETS = [
  { key: 'darwin_amd64', os: 'darwin', cpu: 'x64' },
  { key: 'darwin_arm64', os: 'darwin', cpu: 'arm64' },
  { key: 'linux_amd64', os: 'linux', cpu: 'x64' },
  { key: 'linux_arm64', os: 'linux', cpu: 'arm64' },
];
export const SCOPE = '@replay-doctor';

export function platformPackageName(t) {
  return `${SCOPE}/${t.os}-${t.cpu}`;
}

export function build(version, dist, outRoot = join(NPM_DIR, 'build')) {
  const checksums = readFileSync(join(dist, 'checksums.txt'), 'utf8');
  rmSync(outRoot, { recursive: true, force: true });
  const optional = {};
  for (const t of TARGETS) {
    const name = `replay_${version}_${t.key}.tar.gz`;
    const tarball = join(dist, name);
    if (!existsSync(tarball)) throw new Error(`${name} is not in ${dist}`);
    const want = expectedHash(checksums, name);
    if (!want) throw new Error(`${name} is not listed in checksums.txt`);
    const got = createHash('sha256').update(readFileSync(tarball)).digest('hex');
    if (got !== want) throw new Error(`sha256 mismatch for ${name}; refusing to package unsigned bytes`);

    const pkgName = platformPackageName(t);
    const dir = join(outRoot, `${t.os}-${t.cpu}`);
    mkdirSync(join(dir, 'bin'), { recursive: true });
    execFileSync('tar', ['-xzf', tarball, '-C', join(dir, 'bin'), 'replay']);
    chmodSync(join(dir, 'bin', 'replay'), 0o755);
    writeFileSync(join(dir, 'package.json'), JSON.stringify({
      name: pkgName,
      version,
      description: `Replay Doctor binary for ${t.os} ${t.cpu}. Installed by the replay-doctor package; not for direct use.`,
      license: 'BUSL-1.1',
      repository: { type: 'git', url: 'git+https://github.com/RedRobotKK/Replay.git', directory: 'packaging/npm' },
      os: [t.os],
      cpu: [t.cpu],
      files: ['bin'],
      publishConfig: { access: 'public', provenance: true },
    }, null, 2) + '\n');
    writeFileSync(join(dir, 'README.md'), `Binary package for replay-doctor (${t.os} ${t.cpu}). Install \`replay-doctor\` instead.\n`);
    optional[pkgName] = version;
  }
  const mainPath = join(NPM_DIR, 'package.json');
  const main = JSON.parse(readFileSync(mainPath, 'utf8'));
  main.version = version;
  main.optionalDependencies = optional;
  writeFileSync(mainPath, JSON.stringify(main, null, 2) + '\n');
  return Object.keys(optional);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const args = process.argv.slice(2);
  const version = args[args.indexOf('--version') + 1];
  const dist = args.includes('--dist') ? args[args.indexOf('--dist') + 1] : 'dist-release';
  if (!version || !/^\d+\.\d+\.\d+/.test(version)) throw new Error('--version x.y.z is required');
  const built = build(version, dist);
  console.log(`built ${built.length} platform package(s) for ${version}: ${built.join(', ')}`);
}
