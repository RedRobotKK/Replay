"""python3 -m unittest packaging/pypi/test_build_wheels.py

Builds a wheel from a fake release directory and checks the things that break
silently: the binary is executable inside the zip, the tag is a platform tag
pip accepts, RECORD lists every file, and a tarball whose hash is not in
checksums.txt is refused.
"""
import hashlib
import io
import os
import sys
import tarfile
import tempfile
import unittest
import zipfile

sys.path.insert(0, os.path.dirname(__file__))
import build_wheels  # noqa: E402


def fake_release(dirpath, version, keys, corrupt=False):
    sums = []
    for key in keys:
        name = f"replay_{version}_{key}.tar.gz"
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w:gz") as tar:
            data = b"#!/bin/sh\necho replay\n"
            info = tarfile.TarInfo("replay")
            info.size = len(data)
            info.mode = 0o755
            tar.addfile(info, io.BytesIO(data))
        blob = buf.getvalue()
        with open(os.path.join(dirpath, name), "wb") as f:
            f.write(blob if not corrupt else blob + b"x")
        sums.append(f"{hashlib.sha256(blob).hexdigest()}  {name}")
    with open(os.path.join(dirpath, "checksums.txt"), "w") as f:
        f.write("\n".join(sums) + "\n")


class BuildWheels(unittest.TestCase):
    def test_wheel_is_well_formed(self):
        with tempfile.TemporaryDirectory() as d:
            # ALL FOUR targets. This fixture carried two until 2026-09-13 and
            # passed, because the builder skipped the missing ones. It now
            # refuses, so a complete release is what a wheel-shape test has to
            # be given.
            fake_release(d, "1.0.0", list(build_wheels.TARGETS))
            out = os.path.join(d, "wheelhouse")
            built = build_wheels.build("1.0.0", d, out)
            # Four tarballs, six wheels: each Linux binary is static, so it
            # ships under both the glibc and the musl tag.
            self.assertEqual(len(built), 6)
            names = sorted(os.path.basename(p) for p in built)
            self.assertIn("replay_doctor-1.0.0-py3-none-macosx_11_0_arm64.whl", names)
            self.assertIn("replay_doctor-1.0.0-py3-none-manylinux_2_17_x86_64.manylinux2014_x86_64.whl", names)
            self.assertIn("replay_doctor-1.0.0-py3-none-musllinux_1_2_x86_64.whl", names)
            with zipfile.ZipFile(built[0]) as z:
                entries = z.namelist()
                self.assertIn("replay_doctor/bin/replay", entries)
                self.assertIn("replay_doctor/__init__.py", entries)
                self.assertIn("replay_doctor-1.0.0.dist-info/RECORD", entries)
                mode = (z.getinfo("replay_doctor/bin/replay").external_attr >> 16) & 0o777
                self.assertEqual(mode, 0o755, "the binary must be executable or the console script cannot exec it")
                record = z.read("replay_doctor-1.0.0.dist-info/RECORD").decode()
                for e in entries:
                    self.assertIn(e + ",", record)
                self.assertIn("replay-doctor = replay_doctor:main", z.read("replay_doctor-1.0.0.dist-info/entry_points.txt").decode())

    def test_refuses_bytes_the_release_did_not_sign(self):
        with tempfile.TemporaryDirectory() as d:
            fake_release(d, "1.0.0", ["linux_amd64"], corrupt=True)
            with self.assertRaises(SystemExit):
                build_wheels.build("1.0.0", d, os.path.join(d, "out"))

    def test_a_missing_platform_refuses_rather_than_publishing_a_partial_set(self):
        """A PyPI filename cannot be reused once taken.

        This builder skipped a missing tarball and raised only when NOTHING was
        built, so a partial download published a partial platform set and the
        names were then permanently spent. The npm builder already refused in
        that case; these two now agree.

        Found on 2026-09-13 while reviewing a release pipeline that had never
        run, alongside the reason it had never run.
        """
        with tempfile.TemporaryDirectory() as d:
            present = [k for k in build_wheels.TARGETS if k != "darwin_amd64"]
            fake_release(d, "1.0.0", present)
            out = os.path.join(d, "wheelhouse")
            with self.assertRaises(SystemExit) as caught:
                build_wheels.build("1.0.0", d, out)
            msg = str(caught.exception)
            self.assertIn("darwin_amd64", msg,
                          "the refusal must name the platform that is missing")
            self.assertIn("cannot be reused", msg,
                          "the refusal must say why a partial publish is unrecoverable")
            self.assertFalse(
                os.path.exists(out) and os.listdir(out),
                "a refused build left wheels on disk for a later step to publish")

if __name__ == "__main__":
    unittest.main()
