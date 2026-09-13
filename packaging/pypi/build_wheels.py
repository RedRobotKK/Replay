#!/usr/bin/env python3
"""Build one wheel per platform tag from the goreleaser tarballs.

    python3 packaging/pypi/build_wheels.py --version 1.0.0 --dist dist-release --out wheelhouse

Each wheel carries the signed release binary for one platform plus a tiny
console-script wrapper, so `uvx replay-doctor` and `pipx run replay-doctor`
work with nothing else installed. This is how ruff and uv ship a compiled
binary through PyPI. The binary is static (CGO disabled), which is what makes
the manylinux and musllinux tags honest, and why one Linux tarball feeds both.

The tarballs are read from a directory holding the release assets (downloaded
with `gh release download`), and each one is verified against checksums.txt
before it is unpacked, so a wheel cannot be built from bytes the release did
not sign. Standard library only; no build backend to keep in step.
"""
import argparse
import base64
import hashlib
import io
import os
import sys
import tarfile
import zipfile

# One tarball can feed more than one wheel: the Linux binary is static, so the
# same bytes are honest under the glibc (manylinux) and musl (Alpine) tags,
# which is how ruff and uv cover both.
TARGETS = {
    "darwin_arm64": ["macosx_11_0_arm64"],
    "darwin_amd64": ["macosx_10_13_x86_64"],
    "linux_amd64": ["manylinux_2_17_x86_64.manylinux2014_x86_64", "musllinux_1_2_x86_64"],
    "linux_arm64": ["manylinux_2_17_aarch64.manylinux2014_aarch64", "musllinux_1_2_aarch64"],
}

PKG = "replay_doctor"

WRAPPER = '''"""Console-script wrapper: exec the packaged binary with the caller's arguments."""
import os
import sys


def main() -> None:
    here = os.path.dirname(os.path.abspath(__file__))
    binary = os.path.join(here, "bin", "replay")
    os.execv(binary, [binary] + sys.argv[1:])
'''


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def expected_hash(checksums: str, name: str):
    for line in checksums.splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1].lstrip("*") == name:
            return parts[0]
    return None


def urlsafe_b64(digest: bytes) -> str:
    return base64.urlsafe_b64encode(digest).rstrip(b"=").decode()


def metadata(version: str) -> bytes:
    return (
        "Metadata-Version: 2.1\n"
        f"Name: replay-doctor\nVersion: {version}\n"
        "Summary: Replay Doctor names the turn your prompt cache broke on and what it cost. Signed release binary, no telemetry.\n"
        "Home-page: https://replay.doctor\n"
        "License: BUSL-1.1\n"
        "Requires-Python: >=3.8\n"
        "Classifier: Environment :: Console\n"
        "Classifier: Operating System :: MacOS\n"
        "Classifier: Operating System :: POSIX :: Linux\n"
        "Classifier: Topic :: Software Development\n"
        "Project-URL: Documentation, https://replay.doctor/install/\n"
        "Project-URL: Source, https://github.com/RedRobotKK/Replay\n\n"
        "Replay Doctor reads the transcripts your coding agent already keeps on disk and names the turn the provider re-read everything at write prices. "
        "This wheel carries the signed release binary for one platform and a console script that runs it. macOS and Linux, amd64 and arm64.\n"
    ).encode()


def write_wheel(version: str, tag: str, binary: bytes, out: str) -> str:
    dist_info = f"{PKG}-{version}.dist-info"
    files = {
        f"{PKG}/__init__.py": WRAPPER.encode(),
        f"{PKG}/bin/replay": binary,
        f"{dist_info}/METADATA": metadata(version),
        f"{dist_info}/WHEEL": (
            f"Wheel-Version: 1.0\nGenerator: replay build_wheels.py\nRoot-Is-Purelib: false\nTag: py3-none-{tag}\n"
        ).encode(),
        f"{dist_info}/entry_points.txt": b"[console_scripts]\nreplay-doctor = replay_doctor:main\n",
    }
    wheel_name = f"{PKG}-{version}-py3-none-{tag}.whl"
    wheel_path = os.path.join(out, wheel_name)
    record_lines = []
    with zipfile.ZipFile(wheel_path, "w", zipfile.ZIP_DEFLATED) as z:
        for arc, data in files.items():
            info = zipfile.ZipInfo(arc, date_time=(2026, 1, 1, 0, 0, 0))
            # 0o755 for the binary so the console script can exec it; 0o644 otherwise.
            mode = 0o755 if arc.endswith("/bin/replay") else 0o644
            info.external_attr = (0o100000 | mode) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(info, data)
            record_lines.append(f"{arc},sha256={urlsafe_b64(hashlib.sha256(data).digest())},{len(data)}")
        record_lines.append(f"{dist_info}/RECORD,,")
        z.writestr(f"{dist_info}/RECORD", "\n".join(record_lines) + "\n")
    print(f"build_wheels: wrote {wheel_name} ({len(binary)} byte binary, verified against checksums.txt)")
    return wheel_path


def build(version: str, dist: str, out: str) -> list:
    with open(os.path.join(dist, "checksums.txt"), encoding="utf-8") as f:
        checksums = f.read()
    os.makedirs(out, exist_ok=True)
    built = []
    for key, tags in TARGETS.items():
        name = f"replay_{version}_{key}.tar.gz"
        path = os.path.join(dist, name)
        if not os.path.exists(path):
            print(f"build_wheels: {name} not in {dist}, skipping", file=sys.stderr)
            continue
        with open(path, "rb") as f:
            blob = f.read()
        want = expected_hash(checksums, name)
        if want is None:
            raise SystemExit(f"{name} is not listed in checksums.txt")
        if sha256(blob) != want:
            raise SystemExit(f"sha256 mismatch for {name}: refusing to build a wheel from unsigned bytes")
        with tarfile.open(fileobj=io.BytesIO(blob), mode="r:gz") as tar:
            binary = tar.extractfile(tar.getmember("replay")).read()
        for tag in tags:
            built.append(write_wheel(version, tag, binary, out))
    if not built:
        raise SystemExit("build_wheels: no tarballs found; nothing built")
    return built


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--version", required=True, help="release version without the v, e.g. 1.0.0")
    ap.add_argument("--dist", default="dist-release", help="directory holding the release assets and checksums.txt")
    ap.add_argument("--out", default="wheelhouse")
    a = ap.parse_args()
    build(a.version, a.dist, a.out)
