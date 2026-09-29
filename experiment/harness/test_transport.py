"""Fixture tests for the pooled transport. No credential, no network, no spend.

Run: python3 experiment/harness/test_transport.py
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import transport

FAKE = "sk-NOT-A-REAL-KEY-0000000000000000"


def curlrc(body):
    fd, path = tempfile.mkstemp()
    with os.fdopen(fd, "w") as fh:
        fh.write(body)
    os.chmod(path, 0o600)
    return path


class TestHeaderParsing(unittest.TestCase):

    def test_reads_the_existing_0600_curl_config(self):
        """The campaign already keeps the credential in one 0600 file. The
        transport reuses it rather than introducing a second place a secret
        can live."""
        p = curlrc(f'header = "Authorization: Bearer {FAKE}"\n'
                   'header = "X-Trace: on"\n'
                   'silent\n')
        try:
            h = transport.Transport._read_headers(p)
            self.assertEqual(h["Authorization"], f"Bearer {FAKE}")
            self.assertEqual(h["X-Trace"], "on")
            self.assertEqual(len(h), 2, "non-header directives are not headers")
        finally:
            os.unlink(p)

    def test_a_config_with_no_headers_fails_loudly(self):
        """Silently building an unauthenticated session would send every call
        to the provider without a credential and read as a provider fault."""
        p = curlrc("silent\nretry = 0\n")
        try:
            with self.assertRaises(ValueError):
                transport.Transport._read_headers(p)
        finally:
            os.unlink(p)

    def test_a_value_containing_a_colon_survives(self):
        p = curlrc('header = "X-Ref: https://example.invalid/a:b"\n')
        try:
            h = transport.Transport._read_headers(p)
            self.assertEqual(h["X-Ref"], "https://example.invalid/a:b",
                             "splitting on every colon would truncate a URL")
        finally:
            os.unlink(p)


class TestPooling(unittest.TestCase):

    def test_the_pool_is_at_least_the_worker_count(self):
        """A pool smaller than the concurrency silently serialises the fan-out,
        and the run would then measure the pool rather than the provider."""
        p = curlrc(f'header = "Authorization: Bearer {FAKE}"\n')
        try:
            t = transport.Transport("https://example.invalid", p, pool=64)
            ad = t.session.get_adapter("https://example.invalid")
            self.assertGreaterEqual(ad._pool_maxsize, 64)
            self.assertGreaterEqual(ad._pool_connections, 64)
            t.close()
        finally:
            os.unlink(p)

    def test_the_transport_does_not_retry_on_its_own(self):
        """A silent retry spends money twice on one question and would make the
        call count in the ledger disagree with the provider's."""
        p = curlrc(f'header = "Authorization: Bearer {FAKE}"\n')
        try:
            t = transport.Transport("https://example.invalid", p)
            ad = t.session.get_adapter("https://example.invalid")
            self.assertEqual(ad.max_retries.total, 0)
            t.close()
        finally:
            os.unlink(p)


class TestCredentialContainment(unittest.TestCase):

    def test_the_credential_never_appears_in_repr_or_str(self):
        """Any object that renders its own credential will eventually render it
        into a log line, a traceback or an artifact."""
        p = curlrc(f'header = "Authorization: Bearer {FAKE}"\n')
        try:
            t = transport.Transport("https://example.invalid", p)
            for rendered in (repr(t), str(t), repr(t.__dict__.get("base"))):
                self.assertNotIn(FAKE, rendered)
            t.close()
        finally:
            os.unlink(p)

    def test_no_source_file_in_the_harness_contains_a_credential_literal(self):
        """A repo-wide guard, not a per-file one: the defect is a key pasted
        anywhere, and a check that looks at one file cannot see that."""
        here = os.path.dirname(os.path.abspath(__file__))
        checked = 0
        for name in sorted(os.listdir(here)):
            if not name.endswith(".py"):
                continue
            checked += 1
            with open(os.path.join(here, name)) as fh:
                text = fh.read()
            for i, line in enumerate(text.splitlines(), 1):
                if "sk-" in line and "NOT-A-REAL-KEY" not in line:
                    self.fail(f"{name}:{i} contains what looks like a key literal")
        self.assertGreater(checked, 3, "the scan found almost nothing; a guard "
                                       "that inspects no files proves nothing")


if __name__ == "__main__":
    unittest.main(verbosity=2)
