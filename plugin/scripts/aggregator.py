"""Find the original posting on a job aggregator's listing page (ticket #220).

Aggregators (Jora, Adzuna, Jooble ...) often hold a cut-down copy of the employer's own ad.
``original_link`` reads the raw listing HTML and returns the link to the employer's posting,
or "" when the page has none. It only ever returns a link that is on the page: a URL is never
built or guessed. The caller fetches it through ``httpfetch`` like any other ad.
"""

import re
from html.parser import HTMLParser
from urllib.parse import parse_qs, urljoin, urlsplit

# Listing sites whose pages we can fetch and that copy ads from employers. Seek, LinkedIn,
# Indeed and Glassdoor are login-walled and handled before this module is reached.
AGGREGATORS = (
    "jora.com",
    "adzuna.com",
    "adzuna.com.au",
    "jooble.org",
    "careerjet.com",
    "careerjet.com.au",
    "talent.com",
    "ziprecruiter.com",
    "simplyhired.com",
    "jobrapido.com",
    "whatjobs.com",
    "careerone.com.au",
)

# Hosts that are never the employer's posting: social sites, search, app stores, mail.
_NOT_AN_AD = (
    "facebook.com", "twitter.com", "x.com", "linkedin.com", "instagram.com", "youtube.com",
    "google.com", "apple.com", "play.google.com", "whatsapp.com", "pinterest.com",
    "t.co", "bit.ly",
)

# Where applicant tracking systems host employer ads (new-job.py's list is the reader's).
ATS_HOSTS = (
    "greenhouse.io", "lever.co", "ashbyhq.com", "workable.com", "smartrecruiters.com",
    "myworkdayjobs.com", "bamboohr.com", "teamtailor.com", "recruitee.com",
    "pageuppeople.com", "jobvite.com", "breezy.hr", "applytojob.com", "workforcenow.adp.com",
)

_REDIRECT_PARAMS = ("url", "u", "redirect", "redirect_url", "target", "dest", "destination",
                    "link", "ru", "to")
_LINK_TEXT = re.compile(
    r"(?i)\b(apply|company (site|website|page)|original|employer|view (job|ad|posting|on)"
    r"|careers?|see (the )?(full|original))\b"
)
_JOB_PATH = re.compile(r"(?i)/(jobs?|careers?|apply|positions?|vacanc|opening)")


def host_of(url):
    host = urlsplit(url or "").netloc.lower().split("@")[-1].split(":")[0]
    return host[4:] if host.startswith("www.") else host


def _in(host, domains):
    return any(host == d or host.endswith("." + d) for d in domains)


def is_aggregator(url):
    return _in(host_of(url), AGGREGATORS)


def is_ats(url):
    return _in(host_of(url), ATS_HOSTS)


class _Anchors(HTMLParser):
    def __init__(self):
        super().__init__()
        self.found = []  # [href, text]
        self._open = None

    def handle_starttag(self, tag, attrs):
        if tag == "a":
            self._open = [dict(attrs).get("href") or "", ""]
            self.found.append(self._open)

    def handle_endtag(self, tag):
        if tag == "a":
            self._open = None

    def handle_data(self, data):
        if self._open is not None:
            self._open[1] += " " + data


def _unwrap(href, base):
    """The absolute URL, with an aggregator's own redirect link replaced by its target."""
    absolute = urljoin(base, href.strip())
    parts = urlsplit(absolute)
    if parts.scheme.lower() not in ("http", "https") or not parts.netloc:
        return ""
    if is_aggregator(absolute) or host_of(absolute) == host_of(base):
        query = parse_qs(parts.query)
        for name in _REDIRECT_PARAMS:
            for value in query.get(name, []):
                if urlsplit(value).scheme.lower() in ("http", "https") and urlsplit(value).netloc:
                    return value
    return absolute


def original_link(html, base_url):
    """The best link on the listing to the employer's own ad, or "" when there is none."""
    parser = _Anchors()
    parser.feed(html or "")
    base_host = host_of(base_url)
    best, best_score = "", 0
    for href, text in parser.found:
        url = _unwrap(href, base_url)
        host = host_of(url)
        if not url or host == base_host or is_aggregator(url) or _in(host, _NOT_AN_AD):
            continue
        text = re.sub(r"\s+", " ", text).strip()
        score = (3 if is_ats(url) else 0) + (2 if _LINK_TEXT.search(text) else 0)
        score += 1 if _JOB_PATH.search(urlsplit(url).path) else 0
        if score >= 2 and score > best_score:
            best, best_score = url, score
    return best


def saved_with_listing(workspace, url):
    """Folders whose ``aggregator_url`` is ``url``: ads captured from that listing earlier."""
    from pathlib import Path  # noqa: PLC0415

    found = []
    for path in sorted(Path(workspace).glob("jobs/*/*/meta.yml")):
        try:
            lines = path.read_text(errors="replace").splitlines()
        except OSError:
            continue
        for line in lines:
            if line.startswith("aggregator_url:") and line.split(":", 1)[1].strip().strip('"') == url:
                found.append(path.parent)
                break
    return found
