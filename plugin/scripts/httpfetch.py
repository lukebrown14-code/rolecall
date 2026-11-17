"""One guarded HTTP fetch shared by every plugin script that reads a web page.

``fetch`` allows only http and https URLs, caps the body at ``max_bytes`` and
gives the whole read an overall deadline, so a schemeless URL, a ``file://``
path or a host that drips data never crashes or hangs a script. Every failure
is a ``FetchError`` with a one-line message; ``status`` and ``body`` carry an
HTTP error's code and the start of its page for callers that classify it, and
``kind`` names the failure in one word (``bad_url``, ``timeout``, ``not_found``,
``blocked``, ``non_html``, ``unreachable``) so a caller never parses the message.
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

    def __init__(self, message, status=None, body="", kind=""):
        super().__init__(message)
        self.status = status
        self.body = body
        self.kind = kind or kind_for(status)


def kind_for(status):
    """The failure kind an HTTP status stands for; "unreachable" when there was none."""
    if status in (404, 410):
        return "not_found"
    if status in (401, 403, 429, 999):
        return "blocked"
    return "unreachable"


def check_url(url):
    """Raise FetchError unless ``url`` is an http or https URL with a host."""
    parts = urllib.parse.urlsplit(url or "")
    if parts.scheme.lower() not in ("http", "https") or not parts.netloc:
        raise FetchError(
            f"{url or '(empty)'} -- not a web address; it must start with http:// or https://",
            kind="bad_url",
        )


def fetch(url, timeout=20, max_bytes=MAX_BYTES, user_agent=USER_AGENT, html_only=False):
    """The decoded body of ``url``: at most ``max_bytes`` bytes, read within ``timeout`` seconds.

    ``html_only`` refuses a page whose Content-Type says it is not a web page (a PDF, an
    image, JSON) with kind ``non_html``; a missing Content-Type is let through.
    """
    check_url(url)
    request = urllib.request.Request(url, headers={"User-Agent": user_agent})
    deadline = time.monotonic() + timeout
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            chunks, size = [], 0
            while size < max_bytes:
                if time.monotonic() > deadline:
                    raise FetchError(f"{url} -- timed out after {timeout}s", kind="timeout")
                chunk = response.read(min(_CHUNK, max_bytes - size))
                if not chunk:
                    break
                chunks.append(chunk)
                size += len(chunk)
            charset = response.headers.get_content_charset() or "utf-8"
            ctype = response.headers.get_content_type() if html_only else ""
    except urllib.error.HTTPError as error:
        try:
            body = error.read(4096).decode("utf-8", "replace")
        except OSError:
            body = ""
        raise FetchError(f"{url} -- {error}", status=error.code, body=body) from error
    except (urllib.error.URLError, OSError, ValueError) as error:
        reason = getattr(error, "reason", error)
        kind = "timeout" if isinstance(reason, TimeoutError) else "unreachable"
        raise FetchError(f"{url} -- {error}", kind=kind) from error
    if html_only and ctype and not _is_web_page(ctype):
        raise FetchError(f"{url} -- is {ctype}, not a web page", kind="non_html")
    return b"".join(chunks).decode(charset, "replace")


def _is_web_page(content_type):
    return content_type in ("text/html", "application/xhtml+xml") or content_type == "text/plain"
