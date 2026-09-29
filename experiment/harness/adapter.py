"""Provider adapters: one call, one normalised measurement.

Separated from experiment definition, scoring and analysis on purpose. An
adapter's only job is: send a request to a surface, and return what that surface
actually reported, normalised to one shape with its provenance intact.

Credentials are read from the environment or a curl config path. No key is ever
written here, logged, or returned in a Measurement.
"""
import json, os, subprocess, time, uuid

# Canonical measurement. Anything the surface did not report stays None, which
# is NOT_MEASURED. It is never coerced to 0: "reported nothing" and "reported
# zero" are different facts and the whole project depends on keeping them apart.
FIELDS = ("fresh_in", "cache_read", "cache_write", "out", "reasoning",
          "total_in_reported", "wall_ms", "status", "model_returned",
          # O2 telemetry. Recorded so a later question about which model served a
          # request, or how a call ended, is answerable from the record rather
          # than re-derived. A fingerprint identifies a serving configuration;
          # it is NOT evidence of model weights and must not be read as such.
          "requested_model", "system_fingerprint", "finish_reason",
          # O1. A response that ran out of room is not a short answer, and the
          # difference is not cosmetic: WP-01 paid for 6,000 output tokens that
          # returned empty visible content at finish_reason "length", 14.6% of
          # that work package, and the controller banked it as a result.
          "truncated")


class Measurement(dict):
    def __init__(self, **kw):
        super().__init__({f: None for f in FIELDS})
        self.update(kw)

    def is_truncated(self):
        """True when the provider stopped because it ran out of output room.

        Both dialects are checked by their own vocabulary: chat/completions says
        "length", the Anthropic-compatible shape says "max_tokens". A caller that
        knows only one of them would silently accept a truncated call on the
        other endpoint.
        """
        return self.get("finish_reason") in ("length", "max_tokens")


class Truncated(Exception):
    """Raised instead of returning a truncated response as a deliverable.

    Deliberately NOT a retry. A retry spends money on the same prompt that just
    overran, and doing it automatically is how a bounded work package quietly
    becomes an unbounded one. The default is TRUNCATED -> STOP; a work package
    that wants a recovery call must ask for it explicitly.
    """

    def __init__(self, measurement, raw_path):
        self.measurement, self.raw_path = measurement, raw_path
        super().__init__(
            f"response truncated at finish_reason={measurement.get('finish_reason')!r}; "
            f"out={measurement.get('out')} tokens. Not a deliverable. Raw: {raw_path}")


class Adapter:
    """One endpoint of one provider."""
    name = "abstract"
    inclusive = None      # True: reported input includes cached. False: excludes.

    def __init__(self, curlrc, base):
        self.curlrc, self.base = curlrc, base

    def _post(self, path, body, tmp):
        p = os.path.join(tmp, f"req-{uuid.uuid4().hex[:8]}.json")
        o = p.replace("req-", "res-")
        open(p, "w").write(json.dumps(body))
        t0 = time.time()
        r = subprocess.run(
            ["curl", "-sS", "-K", self.curlrc, "-X", "POST", self.base + path,
             "-H", "Content-Type: application/json", "--data", f"@{p}",
             "-o", o, "-w", "%{http_code}"],
            capture_output=True, text=True)
        wall = (time.time() - t0) * 1000
        try:
            doc = json.load(open(o))
        except Exception:
            doc = {}
        return doc, int(r.stdout.strip() or 0), wall, o


class DeepSeekAnthropic(Adapter):
    """/anthropic/v1/messages. EXCLUSIVE counting, confirmed 2026-09-28:
    a cold call read input=9657/read=0 and the warm repeat read 185/9472,
    both summing to 9657."""
    name = "deepseek:anthropic"
    inclusive = False

    def call(self, model, prompt, max_tokens, tmp, allow_truncated=False, **kw):
        body = {"model": model, "max_tokens": max_tokens,
                "messages": [{"role": "user", "content": prompt}]}
        body.update(kw)
        doc, code, wall, path = self._post("/anthropic/v1/messages", body, tmp)
        u = doc.get("usage") or {}
        txt = "".join(b.get("text", "") for b in (doc.get("content") or [])
                      if isinstance(b, dict))
        m = Measurement(
            requested_model=model,
            system_fingerprint=doc.get("system_fingerprint"),
            finish_reason=doc.get("stop_reason"),
            fresh_in=u.get("input_tokens"),
            cache_read=u.get("cache_read_input_tokens"),
            cache_write=u.get("cache_creation_input_tokens"),
            out=u.get("output_tokens"),
            total_in_reported=None,     # this dialect reports no combined figure
            wall_ms=round(wall), status=code,
            model_returned=doc.get("model"))
        m["truncated"] = m.is_truncated()
        if m["truncated"] and not allow_truncated:
            raise Truncated(m, path)
        return m, txt, path


class DeepSeekChat(Adapter):
    """/v1/chat/completions. INCLUSIVE counting: prompt_tokens already contains
    the cached share, so fresh input is the miss figure, not prompt_tokens."""
    name = "deepseek:chat"
    inclusive = True

    def call(self, model, prompt, max_tokens, tmp, allow_truncated=False, **kw):
        body = {"model": model, "max_tokens": max_tokens, "stream": False,
                "messages": [{"role": "user", "content": prompt}]}
        body.update(kw)
        doc, code, wall, path = self._post("/v1/chat/completions", body, tmp)
        u = doc.get("usage") or {}
        choice = (doc.get("choices") or [{}])[0]
        ch = choice.get("message", {})
        m = Measurement(
            requested_model=model,
            system_fingerprint=doc.get("system_fingerprint"),
            finish_reason=choice.get("finish_reason"),
            fresh_in=u.get("prompt_cache_miss_tokens"),
            cache_read=u.get("prompt_cache_hit_tokens"),
            cache_write=None,           # this provider publishes no write charge
            out=u.get("completion_tokens"),
            reasoning=(u.get("completion_tokens_details") or {}).get("reasoning_tokens"),
            total_in_reported=u.get("prompt_tokens"),
            wall_ms=round(wall), status=code,
            model_returned=doc.get("model"))
        m["truncated"] = m.is_truncated()
        if m["truncated"] and not allow_truncated:
            raise Truncated(m, path)
        return m, ch.get("content", ""), path
