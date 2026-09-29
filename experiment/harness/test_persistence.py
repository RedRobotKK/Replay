"""python3 experiment/harness/test_persistence.py

Fixture-only: no network, no credential, no spend.

A billed call whose response was never written to disk is NOT_OBSERVED. It
cannot be priced, cannot be reconciled and cannot be re-read by anyone checking
the work. Before these guards the harness could produce one silently, in two
different ways, and both are covered here:

  RUNTIME   curl reported HTTP 200 and wrote no output file. `json.load` threw,
            the exception was swallowed to `doc = {}`, every usage field stayed
            None, pricing returned None, the meter coerced that to $0.00 and the
            budget settled a real billed call at nothing. The row said ANSWERED.

  STATIC    transport.py is a pooled requests.Session that posts to the provider
            and writes no artifact at all. It is the documented replacement for
            the curl subprocess, it is tested, and nothing stops a future script
            importing it. TestNoScriptBypassesTheRunner in test_session.py
            guards the adapter the same way; this guards the transport.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import adapter  # noqa: E402
import session  # noqa: E402
from fanout import Outcome  # noqa: E402
from policy import TaskClass  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))


class FakeCurl:
    """Stands in for subprocess.run, and decides what lands on disk."""

    def __init__(self, status="200", body=None, write=True):
        self.status, self.body, self.write = status, body, write

    def __call__(self, argv, **kw):
        out = argv[argv.index("-o") + 1]
        if self.write:
            with open(out, "w") as fh:
                fh.write(self.body if self.body is not None else "{}")

        class R:
            stdout = self.status
            stderr = ""
        return R()


class TestTheResponseArtifactIsNotOptional(unittest.TestCase):

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="persist-")
        self.ad = adapter.DeepSeekChat("/dev/null", "https://example.invalid")
        self._real = adapter.subprocess.run
        self.addCleanup(setattr, adapter.subprocess, "run", self._real)

    def test_a_call_that_wrote_no_response_file_raises(self):
        adapter.subprocess.run = FakeCurl(write=False)
        with self.assertRaises(adapter.ArtifactMissing) as cm:
            self.ad.call("deepseek-flash", "p", 100, self.tmp)
        self.assertIn("NOT_OBSERVED", str(cm.exception))
        self.assertEqual(cm.exception.status, 200)

    def test_an_empty_response_file_is_the_same_failure(self):
        adapter.subprocess.run = FakeCurl(body="")
        with self.assertRaises(adapter.ArtifactMissing):
            self.ad.call("deepseek-flash", "p", 100, self.tmp)

    def test_the_request_artifact_survives_the_failure(self):
        """What was sent is evidence too, and it is what a re-run needs."""
        adapter.subprocess.run = FakeCurl(write=False)
        with self.assertRaises(adapter.ArtifactMissing):
            self.ad.call("deepseek-flash", "the-prompt", 100, self.tmp)
        reqs = [f for f in os.listdir(self.tmp) if f.startswith("req-")]
        self.assertEqual(len(reqs), 1)
        with open(os.path.join(self.tmp, reqs[0])) as fh:
            self.assertEqual(json.load(fh)["messages"][0]["content"],
                             "the-prompt")

    def test_a_body_that_is_on_disk_but_unparseable_is_kept_not_raised(self):
        """The negative control, and the distinction that matters.

        Unparseable is not missing. The bytes the provider sent are on disk,
        classifiable and re-readable, so the call stays auditable and must not
        be turned into an exception that discards it.
        """
        adapter.subprocess.run = FakeCurl(body="<html>502 Bad Gateway</html>")
        m, text, path = self.ad.call("deepseek-flash", "p", 100, self.tmp)
        self.assertTrue(os.path.exists(path))
        self.assertIsNone(m["fresh_in"], "NOT_MEASURED, never zero")
        self.assertEqual(text, "")

    def test_a_normal_response_still_works(self):
        """Without this the guard could be refusing everything."""
        adapter.subprocess.run = FakeCurl(body=json.dumps({
            "model": "deepseek-flash",
            "choices": [{"finish_reason": "stop", "message": {"content": "ok"}}],
            "usage": {"prompt_tokens": 10, "prompt_cache_hit_tokens": 0,
                      "prompt_cache_miss_tokens": 10, "completion_tokens": 3}}))
        m, text, _ = self.ad.call("deepseek-flash", "p", 100, self.tmp)
        self.assertEqual((text, m["out"], m["fresh_in"]), ("ok", 3, 10))


class NoUsage:
    """An adapter that returns HTTP 200 and no usage block."""
    name = "nousage"

    def call(self, model, prompt, max_tokens, tmp, **kw):
        return adapter.Measurement(status=200, model_returned=model), "", None


class TestAnUnpricedBilledCallIsNotBankedAsZero(unittest.TestCase):

    def _run(self):
        return session.Run(curlrc=None, tmp="/tmp", ceiling_usd=1.0,
                           label="t", ad=NoUsage(), quiet=True)

    def test_a_200_with_no_usage_is_not_an_answer(self):
        r = self._run()
        row = r.ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(row["outcome"], Outcome.ERROR,
                         "a call that cannot be priced is not a result")
        self.assertIn("UNACCOUNTED", row["error"])

    def test_it_does_not_settle_the_call_at_zero_dollars(self):
        r = self._run()
        r.ask(TaskClass.AGGREGATE, "q")
        self.assertEqual(r.budget.snapshot()["settled_usd"], 0.0)
        self.assertGreater(
            r.budget.snapshot()["outstanding_usd"], 0.0,
            "the provider answered, so the money is gone; the reservation is "
            "held because under-running is the safe error")

    def test_it_stays_out_of_the_pass_rate_denominator(self):
        r = self._run()
        r.ask(TaskClass.AGGREGATE, "q", check=lambda t: True)
        self.assertEqual(r.pass_rate(), (None, 0, 0))


class TestNoModuleReachesTheProviderWithoutPersisting(unittest.TestCase):
    """The architecture, enforced against the source.

    PERSISTS   may POST to the provider, and writes both artifacts.
    QUARANTINED  may POST and writes nothing, so no non-test module may import
                 it. transport.py is a real pooled client with real tests; t.py
                 is a byte-identical copy of it. Both are kept (deleting an
                 instrument is not the same as disconnecting it) and both are
                 wired to nothing.
    """

    POST_MARKERS = (".post(", '"-X", "POST"')
    PERSISTS = {"adapter.py"}
    QUARANTINED = {"transport.py", "t.py"}

    def _modules(self):
        return [n for n in sorted(os.listdir(HERE))
                if n.endswith(".py") and not n.startswith("test_")]

    def test_every_posting_module_is_declared(self):
        offenders, checked = [], 0
        for name in self._modules():
            with open(os.path.join(HERE, name)) as fh:
                src = fh.read()
            checked += 1
            if not any(mk in src for mk in self.POST_MARKERS):
                continue
            if name in self.PERSISTS or name in self.QUARANTINED:
                continue
            offenders.append(name)
        self.assertEqual(offenders, [],
                         "these POST to a provider and are neither declared as "
                         "persisting their artifacts nor quarantined, so they "
                         "can bill money nobody can audit:\n  "
                         + "\n  ".join(offenders))
        self.assertGreater(checked, 5, "the scan found almost nothing; a guard "
                                       "that inspects no files proves nothing")

    def test_the_persisting_module_really_writes_both_artifacts(self):
        with open(os.path.join(HERE, "adapter.py")) as fh:
            src = fh.read()
        self.assertIn('f"req-{uuid', src, "the request artifact")
        self.assertIn('"-o", o', src, "the response artifact")
        self.assertIn("raise ArtifactMissing", src,
                      "and it must refuse to proceed when one is absent")

    def test_nothing_imports_a_quarantined_transport(self):
        stems = {q[:-3] for q in self.QUARANTINED}
        offenders = []
        for name in self._modules():
            if name in self.QUARANTINED:
                continue
            with open(os.path.join(HERE, name)) as fh:
                for i, line in enumerate(fh, 1):
                    s = line.strip()
                    if s.startswith("#"):
                        continue
                    for stem in stems:
                        if s in (f"import {stem}",) or s.startswith(
                                (f"import {stem} ", f"from {stem} ",
                                 f"import {stem},")):
                            offenders.append(f"{name}:{i}: {s}")
        self.assertEqual(offenders, [],
                         "these import a transport that posts to the provider "
                         "and writes no artifact:\n  " + "\n  ".join(offenders))

    def test_the_quarantined_modules_are_the_ones_that_persist_nothing(self):
        """The list is checked against the source, not trusted as a label.

        If transport.py ever learns to write artifacts, this fails and the
        module should move to PERSISTS rather than stay quarantined.
        """
        for name in sorted(self.QUARANTINED):
            path = os.path.join(HERE, name)
            self.assertTrue(os.path.exists(path), f"{name} is listed but absent")
            with open(path) as fh:
                src = fh.read()
            self.assertTrue(any(mk in src for mk in self.POST_MARKERS),
                            f"{name} does not post; it does not belong here")
            self.assertNotIn("req-", src, f"{name} appears to persist now")


if __name__ == "__main__":
    unittest.main(verbosity=2)
