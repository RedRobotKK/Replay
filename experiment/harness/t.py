"""HTTP transport with connection reuse.

Replaces a curl subprocess per call. That was adequate for sequential probes and
is the wrong shape for fan-out: 64 concurrent calls meant 64 process spawns and
64 TLS handshakes, paying setup cost on every request against a provider whose
measured advantage is high concurrency.

A pooled Session keeps connections alive across calls, which is what a
production client does, so latency measured here is latency the real thing would
see rather than an artefact of process startup.

Credentials are read from a file and set on the session once. They are never
logged, never returned in a result, and never written to an artefact.
"""
import json
import threading

import requests
from requests.adapters import HTTPAdapter


class Transport:
    """One pooled session, safe to share across worker threads.

    requests.Session is documented as not guaranteed thread-safe for mutation,
    so the session is fully configured in __init__ and only read afterwards. The
    connection pool itself is urllib3's and is thread-safe for concurrent use.
    """

    def __init__(self, base, auth_header_file, pool=64, timeout=120):
        self.base, self.timeout = base.rstrip("/"), timeout
        self.session = requests.Session()
        # Pool sized to the concurrency we measured as safe. A pool smaller than
        # the worker count silently serialises the fan-out and would make a
        # concurrency measurement describe the pool rather than the provider.
        ad = HTTPAdapter(pool_connections=pool, pool_maxsize=pool, max_retries=0)
        self.session.mount("https://", ad)
        self.session.mount("http://", ad)
        self.session.headers.update(self._read_headers(auth_header_file))
        self.session.headers["Content-Type"] = "application/json"
        self._lock = threading.Lock()
        self.requests_made = 0

    @staticmethod
    def _read_headers(path):
        """Parse a curl config file's `header = "Name: value"` lines.

        Reuses the credential file the campaign already created at 0600 rather
        than introducing a second place a secret can live.
        """
        headers = {}
        with open(path) as fh:
            for line in fh:
                line = line.strip()
                if not line.startswith("header"):
                    continue
                _, _, rest = line.partition("=")
                rest = rest.strip().strip('"')
                name, _, value = rest.partition(":")
                if name and value:
                    headers[name.strip()] = value.strip()
        if not headers:
            raise ValueError(f"no header lines parsed from {path}")
        return headers

    def post_json(self, path, body):
        """POST and return (parsed_doc, status, elapsed_ms, raw_bytes).

        Raises nothing for an HTTP error status: a 4xx or 5xx is evidence and is
        returned to the caller to classify, not an exception that discards the
        usage the provider may still have reported.
        """
        r = self.session.post(self.base + path, data=json.dumps(body),
                              timeout=self.timeout)
        with self._lock:
            self.requests_made += 1
        try:
            doc = r.json()
        except ValueError:
            doc = {}
        return doc, r.status_code, r.elapsed.total_seconds() * 1000, r.content

    def close(self):
        self.session.close()
