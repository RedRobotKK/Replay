"""python3 experiment/harness/test_cleanstate.py

Fixture-only: a scratch directory, no network, no credential, no spend.

These tests construct the stale-bytecode defect deliberately, in a temporary
tree, and assert that cleanstate names it. The construction is the D1
reproduction: a same-size edit whose mtime lands inside the whole second the
.pyc recorded. os.utime is used instead of racing the clock so the test is
deterministic rather than usually true.
"""
import os
import shutil
import struct
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import cleanstate  # noqa: E402

ONE = "def f():\n    return 1\n"
TWO = "def f():\n    return 2\n"          # same byte length as ONE
LONGER = "def f():\n    return 22\n"      # one byte longer


class Tree(unittest.TestCase):
    """A scratch package with a real, interpreter-written .pyc."""

    def setUp(self):
        self.dir = tempfile.mkdtemp(prefix="cleanstate-")
        self.addCleanup(shutil.rmtree, self.dir, True)
        self.src = os.path.join(self.dir, "m.py")

    def compile_pyc(self, source):
        """Write source and let a real interpreter cache its bytecode."""
        shutil.rmtree(os.path.join(self.dir, "__pycache__"), ignore_errors=True)
        with open(self.src, "w") as fh:
            fh.write(source)
        env = dict(os.environ)
        env.pop("PYTHONDONTWRITEBYTECODE", None)
        r = subprocess.run([sys.executable, "-c", "import m"], cwd=self.dir,
                           env=env, capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        cache = os.path.join(self.dir, "__pycache__")
        pycs = [os.path.join(cache, n) for n in os.listdir(cache)
                if n.endswith(".pyc")]
        self.assertEqual(len(pycs), 1, "expected exactly one cached module")
        return pycs[0]

    def rewrite_inside_the_recorded_second(self, pyc, source):
        """The defect, constructed: edit the source, keep the recorded mtime."""
        h = cleanstate.read_header(pyc)
        self.assertFalse(h["hash_based"], "this platform writes timestamp pycs")
        with open(self.src, "w") as fh:
            fh.write(source)
        t = h["source_mtime"] + 0.75      # same whole second, later sub-second
        os.utime(self.src, (t, t))
        return h


class TestTheStaleBytecodeDefectIsNamed(Tree):

    def test_a_same_size_edit_inside_the_second_is_fatal(self):
        """The mutation that reads as SURVIVED against code no longer on disk."""
        pyc = self.compile_pyc(ONE)
        h = self.rewrite_inside_the_recorded_second(pyc, TWO)
        self.assertEqual(h["source_size"], os.stat(self.src).st_size,
                         "the size field cannot distinguish these two files")
        self.assertTrue(cleanstate.would_be_reused(self.src, pyc),
                        "CPython would accept this .pyc; that is the defect")
        self.assertFalse(cleanstate.bytecode_matches_source(self.src, pyc))
        rec = cleanstate.inspect(pyc)
        self.assertEqual(rec["verdict"], cleanstate.ACCEPTED_AND_STALE)
        with self.assertRaises(cleanstate.CleanStateError) as cm:
            cleanstate.assert_clean(self.dir, strict=False)
        self.assertIn("not on disk", str(cm.exception))

    def test_the_interpreter_really_does_run_the_stale_bytecode(self):
        """The detector is checked against the behaviour, not against itself.

        Without this the module could be describing a rule CPython does not
        actually follow, and every verdict above would be theatre.
        """
        pyc = self.compile_pyc(ONE)
        self.rewrite_inside_the_recorded_second(pyc, TWO)
        env = dict(os.environ)
        env.pop("PYTHONDONTWRITEBYTECODE", None)
        r = subprocess.run([sys.executable, "-c", "import m; print(m.f())"],
                           cwd=self.dir, env=env, capture_output=True, text=True)
        self.assertEqual(r.stdout.strip(), "1",
                         "source says 2; the interpreter must have run the "
                         "cached 1 for this defect to exist at all")

    def test_the_restore_direction_is_the_same_defect(self):
        """Original restored, mutant bytecode executed: the control reads FAILING.

        This direction bites after EVERY mutation, whatever its size, because the
        restored file is always the same length as the original.
        """
        pyc = self.compile_pyc(TWO)
        self.rewrite_inside_the_recorded_second(pyc, ONE)
        self.assertEqual(cleanstate.inspect(pyc)["verdict"],
                         cleanstate.ACCEPTED_AND_STALE)


class TestTheDetectorCanAlsoSayFine(Tree):
    """A check that cannot pass is not a check. These are the negative controls."""

    def test_a_matching_cache_entry_is_fresh_not_stale(self):
        pyc = self.compile_pyc(ONE)
        rec = cleanstate.inspect(pyc)
        self.assertEqual(rec["verdict"], cleanstate.ACCEPTED_AND_FRESH)
        self.assertTrue(rec["reused"])
        self.assertTrue(rec["matches"])

    def test_a_size_changing_edit_is_rejected_by_cpython_and_reported_so(self):
        """The partial guard. It is why some mutations were tested correctly."""
        pyc = self.compile_pyc(ONE)
        self.rewrite_inside_the_recorded_second(pyc, LONGER)
        self.assertFalse(cleanstate.would_be_reused(self.src, pyc))
        self.assertEqual(cleanstate.inspect(pyc)["verdict"], cleanstate.REJECTED)

    def test_a_tree_with_no_cache_passes_even_in_strict_mode(self):
        with open(self.src, "w") as fh:
            fh.write(ONE)
        saved, sys.dont_write_bytecode = sys.dont_write_bytecode, True
        try:
            self.assertEqual(cleanstate.assert_clean(self.dir, strict=True), [])
        finally:
            sys.dont_write_bytecode = saved

    def test_an_orphaned_pyc_is_not_reported_as_stale(self):
        pyc = self.compile_pyc(ONE)
        os.remove(self.src)
        self.assertEqual(cleanstate.inspect(pyc)["verdict"], cleanstate.NO_SOURCE)


class TestStrictModeIsWhatASweepNeeds(Tree):

    def test_strict_refuses_a_cache_that_is_currently_correct(self):
        """Fresh today is stale one same-second edit later. A sweep gets none."""
        self.compile_pyc(ONE)
        saved, sys.dont_write_bytecode = sys.dont_write_bytecode, True
        try:
            with self.assertRaises(cleanstate.CleanStateError) as cm:
                cleanstate.assert_clean(self.dir, strict=True)
        finally:
            sys.dont_write_bytecode = saved
        self.assertIn("exists at all", str(cm.exception))

    def test_bytecode_writing_being_enabled_is_itself_a_failure(self):
        saved, sys.dont_write_bytecode = sys.dont_write_bytecode, False
        try:
            with self.assertRaises(cleanstate.CleanStateError) as cm:
                cleanstate.assert_clean(self.dir, strict=False)
        finally:
            sys.dont_write_bytecode = saved
        self.assertIn("PYTHONDONTWRITEBYTECODE", str(cm.exception))


class TestTheHarnessRunsUnderTheGuard(unittest.TestCase):

    def test_this_very_suite_was_started_with_bytecode_writing_off(self):
        """scripts/harness-test exports PYTHONDONTWRITEBYTECODE=1. If that line
        is ever dropped, this fails here rather than silently three sweeps later."""
        self.assertTrue(sys.dont_write_bytecode,
                        "run the harness via scripts/harness-test, or with "
                        "PYTHONDONTWRITEBYTECODE=1; a sweep in this tree is "
                        "otherwise not evidence")

    def test_the_harness_directory_carries_no_stale_bytecode(self):
        here = os.path.dirname(os.path.abspath(__file__))
        bad = [r for r in cleanstate.audit(here)
               if r["verdict"] in cleanstate.FATAL]
        self.assertEqual(bad, [], "stale cached bytecode in the harness")


if __name__ == "__main__":
    unittest.main(verbosity=2)
