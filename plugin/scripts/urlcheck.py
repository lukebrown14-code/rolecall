"""Check that the web addresses in a document still resolve.

One shared check for ``drift.py --urls`` and the build. Each http(s) URL is
fetched with ``httpfetch.fetch`` (a small guarded GET, since the shared fetch has
no HEAD) and given one verdict:

- ``ok``: the page answered.
- ``dead``: HTTP 404 or 410. The link is gone.
- ``unreachable``: no HTTP answer at all (DNS failure, refused, timeout).
- ``unverified``: any other HTTP status (403, 429, 999 ...). Bot-blocking sites
  answer like this for a live page, so it is reported but never a failure.

``failures`` is what a caller should fail on: the dead links, plus the
unreachable ones unless every URL was unreachable (that is the network being
down, not a dead link, so it warns instead).
"""

import re
from concurrent.futures import ThreadPoolExecutor

import httpfetch

TIMEOUT = 8
DEAD_STATUSES = (404, 410)
_URL = re.compile(r"https?://[^\s<>\[\]\"']+", re.I)
_TRAILING = ".,;:!?*_>}"


def _trim(raw):
    """Drop trailing prose or markdown punctuation, and a ``)`` with no ``(`` before it."""
    url = raw
    while url:
        if url[-1] in _TRAILING or (url[-1] == ")" and url.count(")") > url.count("(")):
            url = url[:-1]
        else:
            break
    return url


def urls_in(text):
    """Every distinct http(s) URL in ``text``, in order."""
    return list(dict.fromkeys(_trim(m.group()) for m in _URL.finditer(text or "")))


def _probe(url, fetcher, timeout):
    try:
        fetcher(url, timeout=timeout, max_bytes=1)
    except httpfetch.FetchError as error:
        if error.status in DEAD_STATUSES:
            return "dead", str(error)
        if error.status is None:
            return "unreachable", str(error)
        return "unverified", f"HTTP {error.status}"
    return "ok", ""


def check(urls, fetcher=httpfetch.fetch, timeout=TIMEOUT):
    """``{url: (verdict, detail)}`` for each URL. ``fetcher`` is injectable for tests."""
    urls = list(dict.fromkeys(urls))
    if not urls:
        return {}
    with ThreadPoolExecutor(max_workers=min(8, len(urls))) as pool:
        results = list(pool.map(lambda url: _probe(url, fetcher, timeout), urls))
    return dict(zip(urls, results, strict=True))


def offline(results):
    """True when there were URLs and none could be reached at all."""
    return bool(results) and all(v == "unreachable" for v, _ in results.values())


def failures(results):
    """The URLs to fail on: dead ones, and unreachable ones unless none answered at all."""
    if offline(results):
        return []
    return [url for url, (v, _) in results.items() if v in ("dead", "unreachable")]


def unverified(results):
    """The URLs that answered with a status that does not prove them dead."""
    return [url for url, (v, _) in results.items() if v == "unverified"]
