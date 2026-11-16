"""One guarded HTTP fetch shared by every plugin script that reads a web page.

``fetch`` allows only http and https URLs, caps the body at ``max_bytes`` and
gives the whole read an overall deadline, so a schemeless URL, a ``file://``
path or a host that drips data never crashes or hangs a script. Every failure
is a ``FetchError`` with a one-line message; ``status`` and ``body`` carry an
HTTP error's code and the start of its page for callers that classify it.
"""

import time
import urllib.error
import urllib.parse
import urllib.request

USER_AGENT = "Mozilla/5.0 rolecall"
MAX_BYTES = 2_000_000
_CHUNK = 64 * 1024


class FetchError(Exception):
    """A fetch that produced no page. ``status`` is the HTTP code, or None."""

    def __init__(self, message, status=None, body=""):
        super().__init__(message)
        self.status = status
        self.body = body


def check_url(url):
    """Raise FetchError unless ``url`` is an http or https URL with a host."""
    parts = urllib.parse.urlsplit(url or "")
    if parts.scheme.lower() not in ("http", "https") or not parts.netloc:
        raise FetchError(
            f"{url or '(empty)'} -- not a web address; it must start with http:// or https://"
        )


def fetch(url, timeout=20, max_bytes=MAX_BYTES, user_agent=USER_AGENT):
    """The decoded body of ``url``: at most ``max_bytes`` bytes, read within ``timeout`` seconds."""
    check_url(url)
    request = urllib.request.Request(url, headers={"User-Agent": user_agent})
    deadline = time.monotonic() + timeout
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            chunks, size = [], 0
            while size < max_bytes:
                if time.monotonic() > deadline:
                    raise FetchError(f"{url} -- timed out after {timeout}s")
                chunk = response.read(min(_CHUNK, max_bytes - size))
                if not chunk:
                    break
                chunks.append(chunk)
                size += len(chunk)
            charset = response.headers.get_content_charset() or "utf-8"
    except urllib.error.HTTPError as error:
        try:
            body = error.read(4096).decode("utf-8", "replace")
        except OSError:
            body = ""
        raise FetchError(f"{url} -- {error}", status=error.code, body=body) from error
    except (urllib.error.URLError, OSError, ValueError) as error:
        raise FetchError(f"{url} -- {error}") from error
    return b"".join(chunks).decode(charset, "replace")
