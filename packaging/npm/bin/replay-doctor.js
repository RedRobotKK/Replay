#!/usr/bin/env node
/**
 * npx replay-doctor <args>
 *
 * Resolves the binary from the platform package npm installed alongside this
 * one (@replay-doctor/<os>-<cpu>, an optionalDependency pinned to this exact
 * version) and execs it. That is the whole path for a normal install: no
 * network, no postinstall, the lockfile's integrity hash covers the binary.
 *
 * When the platform package is absent (--no-optional, an unusual installer, a
 * mirror that dropped it) the launcher stops and says so on stderr. It fetches
 * the goreleaser tarball for this version from the GitHub release, verifies it
 * against checksums.txt, caches it and execs that only when the environment
 * variable REPLAY_DOCTOR_ALLOW_FETCH is exactly "1". A run that went to the
 * network on its own would be a different promise than the one on the package
 * page, so the network needs the user's word first (PRD E6).
 */
import { createReadStream, existsSync, mkdirSync, renameSync, rmSync, writeFileSync, chmodSync, readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { execFileSync, spawn } from 'node:child_process';
import { join, dirname } from 'node:path';
import { homedir, tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { target, archiveName, assetURL, expectedHash, releaseVersion } from '../lib/release.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const pkg = JSON.parse(readFileSync(join(here, '..', 'package.json'), 'utf8'));
const version = releaseVersion(pkg.version);
const require = createRequire(import.meta.url);

function fromPlatformPackage() {
  const t = target();
  if (!t) return null;
  const name = `@replay-doctor/${process.platform}-${process.arch}`;
  try {
    const p = join(dirname(require.resolve(`${name}/package.json`)), 'bin', 'replay');
    return existsSync(p) ? p : null;
  } catch {
    return null;
  }
}

function cacheDir() {
  if (process.env.REPLAY_DOCTOR_CACHE) return process.env.REPLAY_DOCTOR_CACHE;
  if (process.env.XDG_CACHE_HOME) return join(process.env.XDG_CACHE_HOME, 'replay-doctor');
  return process.platform === 'darwin' ? join(homedir(), 'Library', 'Caches', 'replay-doctor') : join(homedir(), '.cache', 'replay-doctor');
}

async function sha256(path) {
  return new Promise((resolve, reject) => {
    const h = createHash('sha256');
    createReadStream(path).on('data', (d) => h.update(d)).on('end', () => resolve(h.digest('hex'))).on('error', reject);
  });
}

async function download(url, to) {
  const r = await fetch(url, { headers: { 'user-agent': `replay-doctor npm shim ${version}` } });
  if (!r.ok) throw new Error(`${r.status} fetching ${url}`);
  writeFileSync(to, Buffer.from(await r.arrayBuffer()));
}

async function fromRelease() {
  const t = target();
  if (!t) {
    console.error(`replay-doctor: ${process.platform}/${process.arch} is not supported. macOS and Linux on amd64 and arm64 are; Windows is not.`);
    process.exit(2);
  }
  const dir = join(cacheDir(), version, `${t.os}_${t.cpu}`);
  const bin = join(dir, 'replay');
  if (existsSync(bin)) return bin;
  const name = archiveName(version, t);
  const work = join(tmpdir(), `replay-doctor-${process.pid}`);
  mkdirSync(work, { recursive: true });
  try {
    console.error(`replay-doctor: the platform package @replay-doctor/${process.platform}-${process.arch} is not installed; fetching ${name} from the v${version} release instead (once per version)`);
    await download(assetURL(version, name), join(work, name));
    await download(assetURL(version, 'checksums.txt'), join(work, 'checksums.txt'));
    const want = expectedHash(readFileSync(join(work, 'checksums.txt'), 'utf8'), name);
    if (!want) throw new Error(`${name} is not listed in checksums.txt for v${version}`);
    const got = await sha256(join(work, name));
    if (got !== want) throw new Error(`sha256 mismatch for ${name}: release says ${want}, downloaded ${got}. Nothing was installed.`);
    mkdirSync(dir, { recursive: true });
    execFileSync('tar', ['-xzf', join(work, name), '-C', work, 'replay']);
    chmodSync(join(work, 'replay'), 0o755);
    renameSync(join(work, 'replay'), bin);
    console.error(`replay-doctor: verified against checksums.txt (sha256 ${want.slice(0, 12)}). Signature check: https://replay.doctor/install/`);
    return bin;
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}

function refuseFetch() {
  const t = target();
  if (!t) {
    console.error(`replay-doctor: ${process.platform}/${process.arch} is not supported. macOS and Linux on amd64 and arm64 are; Windows is not.`);
    process.exit(2);
  }
  const name = `@replay-doctor/${process.platform}-${process.arch}`;
  console.error(
    `replay-doctor: the platform package ${name} is not installed, and this launcher does not go to the network on its own.\n` +
    `  Reinstall without --no-optional so npm brings in ${name}, or take one of these:\n` +
    `    REPLAY_DOCTOR_ALLOW_FETCH=1 npx replay-doctor ...   fetches ${archiveName(version, t)} from the v${version} GitHub release, verified against its checksums.txt\n` +
    `    go install github.com/RedRobotKK/Replay/cmd/replay@latest\n` +
    `  Release page: https://github.com/RedRobotKK/Replay/releases/tag/v${version}`
  );
  process.exit(2);
}

const bin = fromPlatformPackage() || (process.env.REPLAY_DOCTOR_ALLOW_FETCH === '1' ? await fromRelease() : refuseFetch());
const child = spawn(bin, process.argv.slice(2), { stdio: 'inherit' });
child.on('exit', (code, signal) => { if (signal) process.kill(process.pid, signal); else process.exit(code ?? 1); });
