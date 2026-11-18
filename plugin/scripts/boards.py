"""The job boards the Sweep reads: default targets, collectors and access checks.

Each collector turns one board's public listing into job dicts. Collectors fetch
nothing themselves: the caller passes the fetch, so a dead board raises
``SourceError`` rather than reading as an empty one, and tests stub the network.
"""

import json
import re
import time
from datetime import datetime, timezone

# --------------------------------------------------------------------------
# Default sources. A workspace's workspace.json `sources` key (platform name
# -> target list) wholly replaces these when present; absent = these lists.
#
# Greenhouse and Ashby expose public JSON APIs that include the full ad text.
# Michael Page is server-rendered, so its category pages can be scraped for
# links. Everything else needs the skill's WebFetch path.
# --------------------------------------------------------------------------

GREENHOUSE = [  # slug from job-boards.greenhouse.io/<slug>
    "cultureamp",
    "buildkite",
    "eucalyptus",      # health-tech
    "octopusdeploy",   # remote AU
]

ASHBY = [  # slug from <slug>.ashbyhq.com
    "upguard",         # remote AU
    "airtasker",
    "lorikeet",        # AI support
    "relevanceai",     # AI agents
]

LEVER = []  # slug from jobs.lever.co/<slug>

SMARTRECRUITERS = []  # company identifier

MICHAEL_PAGE = [  # category pages worth sweeping
    "https://www.michaelpage.com.au/jobs/technology",
    "https://www.michaelpage.com.au/jobs/business-analyst",
]


# Cap on Michael Page ad fetches per category page (each is a serial request).
MP_MAX_ADS = 25


class SourceError(Exception):
    """A source could not be read: network error, HTTP error or bad JSON.

    ``access`` says why, in the registry's words: 403, login wall, cloudflare, or error."""

    def __init__(self, message, access="error"):
        super().__init__(message)
        self.access = access


CLOUDFLARE_RE = re.compile(r"just a moment|cf-chl|cf-ray|attention required|cloudflare", re.I)
LOGIN_RE = re.compile(r"<title>[^<]*(sign ?in|log ?in|join)[^<]*</title>|authwall|login-form", re.I)


def classify_access(status, body=""):
    """The registry access status an HTTP response shows: works, 403, login wall or cloudflare."""
    if CLOUDFLARE_RE.search(body or "") and status in (403, 429, 503):
        return "cloudflare"
    if status in (401, 999) or (status < 400 and LOGIN_RE.search(body or "")):
        return "login wall"
    if status == 403:
        return "403"
    return "works" if status < 400 else "error"


def parse_json(raw, what):
    """json.loads that raises SourceError with context instead of a bare ValueError."""
    try:
        return json.loads(raw)
    except (ValueError, TypeError) as e:
        raise SourceError(f"{what}: bad JSON ({e})") from e


def display_name(slug):
    """'relevanceai' / 'octopus-deploy' -> a readable company name."""
    return re.sub(r"[-_]+", " ", slug).strip().title()


def posted_date(value):
    """YYYY-MM-DD from an API's posting time (ISO 8601 text or epoch milliseconds), or ''."""
    if value in (None, ""):
        return ""
    if isinstance(value, (int, float)) and not isinstance(value, bool):
        try:
            return datetime.fromtimestamp(value / 1000, timezone.utc).strftime("%Y-%m-%d")
        except (OverflowError, OSError, ValueError):
            return ""
    m = re.match(r"(\d{4}-\d{2}-\d{2})", str(value))
    return m.group(1) if m else ""


def strip_html(html):
    html = re.sub(r"(?is)<(script|style)[^>]*>.*?</\1>", " ", html)
    html = re.sub(r"(?i)<br\s*/?>|</p>|</li>|</div>", "\n", html)
    text = re.sub(r"<[^>]+>", " ", html)
    for a, b in [("&amp;", "&"), ("&lt;", "<"), ("&gt;", ">"), ("&#39;", "'"),
                 ("&quot;", '"'), ("&nbsp;", " "), ("&rsquo;", "'")]:
        text = text.replace(a, b)
    return re.sub(r"[ \t]+", " ", re.sub(r"\n{3,}", "\n\n", text)).strip()


# ----------------------------------------------------------------- collectors
# Each collector returns a list of jobs, or raises SourceError when the source
# is unreachable or malformed. An empty list means "read fine, nothing posted".
# ``get`` fetches a URL's text or raises SourceError; ``get_quiet`` returns None
# instead, for the optional per-ad fetches. The Sweep passes its own.


def from_greenhouse(slug, get):
    data = parse_json(get(f"https://boards-api.greenhouse.io/v1/boards/{slug}/jobs?content=true"),
                      f"greenhouse:{slug}")
    out = []
    for j in (data.get("jobs", []) if isinstance(data, dict) else []):
        out.append({
            "title": j.get("title", ""),
            "company": j.get("company_name") or display_name(slug),
            "location": (j.get("location") or {}).get("name", ""),
            "url": j.get("absolute_url", ""),
            "closes": j.get("application_deadline") or "",
            "posted": posted_date(j.get("first_published") or j.get("updated_at")),
            "content": strip_html(j.get("content", "")),
            "source": f"greenhouse:{slug}",
        })
    return out


def from_ashby(slug, get):
    data = parse_json(get(f"https://api.ashbyhq.com/posting-api/job-board/{slug}?includeCompensation=true"),
                      f"ashby:{slug}")
    out = []
    for j in (data.get("jobs", []) if isinstance(data, dict) else []):
        out.append({
            "title": j.get("title", ""),
            "company": display_name(slug),
            "location": j.get("location", ""),
            "url": j.get("jobUrl", ""),
            "closes": "",
            "posted": posted_date(j.get("publishedAt")),
            "content": strip_html(j.get("descriptionHtml", "") or "") or j.get("descriptionPlain", "") or "",
            "source": f"ashby:{slug}",
        })
    return out


def from_lever(slug, get):
    data = parse_json(get(f"https://api.lever.co/v0/postings/{slug}?mode=json"), f"lever:{slug}")
    if isinstance(data, dict):  # {"ok": false, "error": ...}
        raise SourceError(f"lever:{slug}: {data.get('error') or 'unexpected response'}")
    out = []
    for j in data:
        out.append({
            "title": j.get("text", ""),
            "company": display_name(slug),
            "location": (j.get("categories") or {}).get("location", ""),
            "url": j.get("hostedUrl", ""),
            "closes": "",
            "posted": posted_date(j.get("createdAt")),
            "content": strip_html(j.get("descriptionPlain", "") or j.get("description", "")),
            "source": f"lever:{slug}",
        })
    return out


def _sr_content(slug, posting_id, get_quiet):
    """Ad text for one SmartRecruiters posting (a second call; '' on failure)."""
    raw = get_quiet(f"https://api.smartrecruiters.com/v1/companies/{slug}/postings/{posting_id}")
    if not raw:
        return ""
    try:
        sections = ((json.loads(raw).get("jobAd") or {}).get("sections")) or {}
    except (ValueError, AttributeError):
        return ""
    return "\n\n".join(strip_html(sec.get("text", "")) for sec in sections.values()
                       if isinstance(sec, dict) and sec.get("text"))


def from_smartrecruiters(slug, get, get_quiet, wants_content):
    """``wants_content(title)`` says whether a posting's ad body is worth a second fetch."""
    data = parse_json(get(f"https://api.smartrecruiters.com/v1/companies/{slug}/postings?limit=100"),
                      f"smartrecruiters:{slug}")
    out = []
    for j in (data.get("content", []) if isinstance(data, dict) else []):
        loc = j.get("location") or {}
        title = j.get("name", "")
        # Only fetch the ad body for roles on an active track; the blocker scan needs it.
        content = _sr_content(slug, j.get("id"), get_quiet) if wants_content(title) else ""
        out.append({
            "title": title,
            "company": (j.get("company") or {}).get("name") or display_name(slug),
            "location": ", ".join(filter(None, [loc.get("city"), loc.get("region"),
                                                (loc.get("country") or "").upper()])),
            "url": f"https://jobs.smartrecruiters.com/{slug}/{j.get('id')}",
            "closes": "",
            "posted": posted_date(j.get("releasedDate")),
            "content": content,
            "source": f"smartrecruiters:{slug}",
        })
    return out


def from_michael_page(page_url, get, get_quiet, delay):
    """Scrape the category page for links, then fetch each ad (capped, ``delay`` seconds apart)."""
    html = get(page_url)
    paths = sorted(set(re.findall(r"job-detail/[a-z0-9-]+/ref/[a-z0-9-]+", html)))
    city = display_name(page_url.rstrip("/").rsplit("/", 1)[-1])
    out = []
    for i, p in enumerate(paths):
        url = f"https://www.michaelpage.com.au/{p}"
        title = p.split("/")[1].replace("-", " ").title()
        body = None
        if i < MP_MAX_ADS:  # past the cap an ad is listed without text and flagged
            if i:
                time.sleep(delay)
            body = get_quiet(url)
        text = strip_html(body) if body else ""
        m = re.search(r"(?i)\b(contract|temporary|permanent|fixed term)\b", text)
        out.append({
            "title": title,
            # MP ads rarely name the client; say so rather than a bare "via Michael Page".
            "company": "Michael Page (client undisclosed)",
            "location": city,
            "url": url,
            "closes": "",
            "content": text,
            "source": "michaelpage",
            "type_hint": m.group(1).title() if m else "",
        })
    return out
