"""python3 experiment/harness/test_adapter.py

Fixture-driven. Makes no API call and needs no credential.

Covers the two controller guards added after the WP-01 post-mortem: a truncated
response must not become a deliverable, and every call must record who served it
and how it ended.
"""
import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import adapter  # noqa: E402


class FakePost:
    """Stands in for _post so a recorded response shape can be replayed."""

    def __init__(self, doc, code=200):
        self.doc, self.code = doc, code

    def __call__(self, path, body, tmp):
        p = os.path.join(tmp, "res.json")
        with open(p, "w") as fh:
            json.dump(self.doc, fh)
        return self.doc, self.code, 12.0, p


# Shapes below are the field names these endpoints actually returned during the
# 2026-09-28 campaign. They are not invented.
CHAT_OK = {
    "model": "deepseek-v4-pro",
    "system_fingerprint": "a307abda487cd1b463329ccb945ce396",
    "choices": [{"finish_reason": "stop", "message": {"content": "answer"}}],
    "usage": {"prompt_tokens": 100, "prompt_cache_hit_tokens": 60,
              "prompt_cache_miss_tokens": 40, "completion_tokens": 7},
}
# The WP-01 failure, reproduced: the budget went to reasoning and the visible
# content came back empty, at finish_reason "length".
CHAT_TRUNCATED = {
    "model": "deepseek-v4-pro",
    "system_fingerprint": "a307abda487cd1b463329ccb945ce396",
    "choices": [{"finish_reason": "length", "message": {"content": ""}}],
    "usage": {"prompt_tokens": 57967, "prompt_cache_hit_tokens": 0,
              "prompt_cache_miss_tokens": 57967, "completion_tokens": 6000},
}
ANTHROPIC_TRUNCATED = {
    "model": "deepseek-flash",
    "stop_reason": "max_tokens",
    "content": [{"type": "thinking"}],
    "usage": {"input_tokens": 500, "cache_read_input_tokens": 0,
              "cache_creation_input_tokens": 0, "output_tokens": 24},
}


class TestTruncationIsNotADeliverable(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_chat_length_raises_rather_than_returning(self):
        """finish_reason "length" must not reach the caller as a result.

        WP-01 paid for 6,000 output tokens that returned empty content and the
        controller banked it. The guard exists so that cannot recur silently.
        """
        ad = adapter.DeepSeekChat("/dev/null", "https://example.invalid")
        ad._post = FakePost(CHAT_TRUNCATED)
        with self.assertRaises(adapter.Truncated) as cm:
            ad.call("deepseek-v4-pro", "p", 6000, self.tmp)
        self.assertTrue(cm.exception.measurement["truncated"])
        self.assertEqual(cm.exception.measurement["finish_reason"], "length")
        self.assertTrue(os.path.exists(cm.exception.raw_path),
                        "the raw response must survive for audit")

    def test_anthropic_max_tokens_is_also_truncation(self):
        """The Anthropic-compatible shape says "max_tokens", not "length".

        A guard that knew only one vocabulary would accept a truncated call on
        the other endpoint, which is the endpoint the campaign used most.
        """
        ad = adapter.DeepSeekAnthropic("/dev/null", "https://example.invalid")
        ad._post = FakePost(ANTHROPIC_TRUNCATED)
        with self.assertRaises(adapter.Truncated):
            ad.call("deepseek-flash", "p", 24, self.tmp)

    def test_recovery_requires_an_explicit_opt_in(self):
        """A work package may accept a truncated call, but must say so.

        Never a retry: retrying the prompt that just overran spends money on the
        same overrun. The opt-in returns the evidence, it does not re-request.
        """
        ad = adapter.DeepSeekChat("/dev/null", "https://example.invalid")
        ad._post = FakePost(CHAT_TRUNCATED)
        m, text, _ = ad.call("deepseek-v4-pro", "p", 6000, self.tmp,
                             allow_truncated=True)
        self.assertTrue(m["truncated"])
        self.assertEqual(text, "", "the empty content is preserved, not repaired")

    def test_a_normal_response_is_not_flagged(self):
        """The positive control. Without it the guard could reject everything."""
        ad = adapter.DeepSeekChat("/dev/null", "https://example.invalid")
        ad._post = FakePost(CHAT_OK)
        m, text, _ = ad.call("deepseek-v4-pro", "p", 100, self.tmp)
        self.assertFalse(m["truncated"])
        self.assertEqual(text, "answer")


class TestIdentityTelemetry(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def test_requested_and_returned_model_are_both_recorded(self):
        """They are different facts.

        A prior audit could not settle which model served the WP-01 and WP-02
        calls, because only the requested id had been kept. The provider is known
        to rewrite ids: requesting deepseek-v4-flash returns deepseek-flash.
        """
        ad = adapter.DeepSeekChat("/dev/null", "https://example.invalid")
        ad._post = FakePost(CHAT_OK)
        m, _, _ = ad.call("deepseek-v4-pro", "p", 100, self.tmp)
        self.assertEqual(m["requested_model"], "deepseek-v4-pro")
        self.assertEqual(m["model_returned"], "deepseek-v4-pro")
        self.assertEqual(m["system_fingerprint"], "a307abda487cd1b463329ccb945ce396")
        self.assertEqual(m["finish_reason"], "stop")

    def test_absent_telemetry_stays_none(self):
        """A field the provider did not send is NOT_MEASURED, never zero or ""."""
        ad = adapter.DeepSeekChat("/dev/null", "https://example.invalid")
        ad._post = FakePost({"model": "deepseek-flash",
                             "choices": [{"finish_reason": "stop",
                                          "message": {"content": "x"}}],
                             "usage": {}})
        m, _, _ = ad.call("deepseek-flash", "p", 100, self.tmp)
        self.assertIsNone(m["system_fingerprint"])
        self.assertIsNone(m["fresh_in"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
