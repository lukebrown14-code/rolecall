#!/usr/bin/env python3
"""Collect candidate roles from the sources that can actually be fetched.

    python3 plugin/scripts/sweep.py                 # all workspace tracks
    python3 plugin/scripts/sweep.py --track SWE     # one track
    python3 plugin/scripts/sweep.py --source greenhouse
    python3 plugin/scripts/sweep.py --all-locations # ignore the location filter
    python3 plugin/scripts/sweep.py --probe greenhouse:cultureamp  # one source, health JSON
    python3 plugin/scripts/sweep.py --platforms    # the registry as JSON (for the dashboard)

This does the mechanical half of a job sweep: fetch, filter by track and
location, drop anything already captured as an Application, and flag visa and
clearance signals. It writes jobs/sweep-candidates.md.

Sources come from the Workspace's workspace.json ``sources`` key (platform name
-> target list) when present -- a whole-key replacement. Otherwise the built-in
defaults are extended by jobs/roles.json additional_sources. That catalogue also
supplies title filters and fit guidance; --roles prints it without fetching jobs.
Platforms themselves are code: the PLATFORMS registry is the single place a new
one registers. Tracks come from workspace.json ``tracks``; when it is absent the
built-in title keywords decide which track a posting belongs to.

It deliberately does NOT decide what is worth applying for. That judgement needs
the full ad read against the Profile, and it is what the ``find-jobs`` skill does
with this file. Only roles that pass that judgement become Lead Applications.

Every source reports its health: a board that failed to load is listed as FAILED,
never silently counted as zero jobs.

Seek and LinkedIn return 403 and cannot be fetched by anything here.
"""

import argparse
import importlib.util
import json
import re
import ssl
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import date
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import meta as meta_store  # noqa: E402
from workspace import workspace_from_cli  # noqa: E402

# new-job.py supplies the visa/clearance patterns. Import it for those patterns
# only: the Workspace itself is resolved once, below, through the shared helper.
_nj_spec = importlib.util.spec_from_file_location(
    "nj", Path(__file__).resolve().parent / "new-job.py"
)
if _nj_spec is None or _nj_spec.loader is None:
    sys.exit("Could not load plugin/scripts/new-job.py -- the blocker patterns live there.")
nj = importlib.util.module_from_spec(_nj_spec)
_nj_spec.loader.exec_module(nj)

UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"

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

# Title keywords per track, used only when jobs/roles.json is absent. A role
# matches when any keyword appears in the title. Workspace tracks come from
# workspace.json; these keys are the fallback set when it does not list any.
TRACK_KEYWORDS = {
    "SWE": [
        "software engineer", "software developer", "full stack", "fullstack",
        "backend", "back end", "front end", "frontend", "developer",
        "platform engineer", "integration engineer",
    ],
    "Platform": [
        "platform engineer", "platform developer", "infrastructure engineer",
        "devops", "site reliability", "sre",
    ],
    "Analyst": [
        "business analyst", "systems analyst", "technical analyst",
        "solutions analyst", "digital analyst",
    ],
    "SA": ["solution architect", "solutions architect", "technical architect"],
    "AI": [
        "applied ai", "ai engineer", "machine learning", "ml engineer",
        "gen ai", "genai", "ai developer",
    ],
}
ROLE_CATALOGUE = None  # loaded from the selected workspace by main


def config(workspace):
    """workspace.json as a dict; absent or unreadable behaves as an empty one."""
    try:
        return json.loads((Path(workspace) / "workspace.json").read_text())
    except (OSError, ValueError):
        return {}


def workspace_tracks(workspace):
    """The Workspace's tracks, falling back to the built-in keyword keys."""
    tracks = config(workspace).get("tracks")
    if isinstance(tracks, list) and tracks:
        return [track for track in tracks if isinstance(track, str) and track.strip()]
    return list(TRACK_KEYWORDS)


def load_role_catalogue(workspace, tracks=None):
    """Optional workspace-owned search titles and fit guidance; fail on invalid data."""
    workspace = Path(workspace)
    tracks = workspace_tracks(workspace) if tracks is None else tracks
    path = workspace / "jobs" / "roles.json"
    if not path.exists():
        return None
    try:
        catalogue = json.loads(path.read_text())
        if not isinstance(catalogue, dict) or catalogue.get("version") != 1:
            raise ValueError("expected an object with version 1")
        roles = catalogue.get("roles")
        if not isinstance(roles, list) or not roles:
            raise ValueError("roles must be a non-empty list")
        ids = set()
        for role in roles:
            if not isinstance(role, dict):
                raise ValueError("each role must be an object")
            for field in ("id", "category", "track", "evidence", "gaps"):
                if not isinstance(role.get(field), str) or not role[field].strip():
                    raise ValueError(f"each role needs a non-empty {field}")
            if role["id"] in ids:
                raise ValueError(f"duplicate role id: {role['id']}")
            ids.add(role["id"])
            if role["track"] not in tracks:
                raise ValueError(f"unknown track: {role['track']}")
            if role.get("priority") not in ("primary", "secondary", "stretch"):
                raise ValueError(f"invalid priority for {role['id']}")
            for field in ("titles", "search_terms"):
                values = role.get(field)
                if not isinstance(values, list) or not values or not all(
                    isinstance(value, str) and value.strip() for value in values
                ):
                    raise ValueError(f"{role['id']}: {field} must be a non-empty list of strings")
        validate_sources(catalogue.get("additional_sources", {}), "additional_sources")
        return catalogue
    except (OSError, ValueError) as error:
        sys.exit(f"{path}: {error}")


# Search area filter. The Places and Remote rule come from workspace.json
# `search_area`; the gazetteer maps countries to their states and cities.
GAZETTEER_PATH = Path(__file__).resolve().parent.parent / "gazetteer.json"
NO_SEARCH_AREA = "no Search area set: all locations kept"

# Cap on Michael Page ad fetches per category page (each is a serial request).
MP_MAX_ADS = 25
MP_DELAY = 0.3  # seconds between ad fetches


def load_blockers(workspace):
    """The Workspace's visa/clearance patterns plus new-job's favourable list."""
    return nj.configured_blockers(workspace), nj.FAVOURABLE


class SourceError(Exception):
    """A source could not be read: network error, HTTP error or bad JSON."""


def get(url, timeout=25):
    """Fetch a URL; raises SourceError so a dead source is never mistaken for an empty one."""
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=ctx) as r:
            return r.read().decode(r.headers.get_content_charset() or "utf-8", "replace")
    except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError, ssl.SSLError, OSError) as e:
        raise SourceError(f"{url} -- {e}") from e


def get_quiet(url, timeout=20):
    """For optional per-ad fetches: None on failure, never raises."""
    try:
        return get(url, timeout)
    except SourceError as e:
        print(f"  ! {e}", file=sys.stderr)
        return None


def parse_json(raw, what):
    """json.loads that raises SourceError with context instead of a bare ValueError."""
    try:
        return json.loads(raw)
    except (ValueError, TypeError) as e:
        raise SourceError(f"{what}: bad JSON ({e})") from e


def display_name(slug):
    """'relevanceai' / 'octopus-deploy' -> a readable company name."""
    return re.sub(r"[-_]+", " ", slug).strip().title()


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

ACTIVE_TRACKS = list(TRACK_KEYWORDS)  # set by main; SmartRecruiters fetches ad bodies only for these


def from_greenhouse(slug):
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
            "content": strip_html(j.get("content", "")),
            "source": f"greenhouse:{slug}",
        })
    return out


def from_ashby(slug):
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
            "content": strip_html(j.get("descriptionHtml", "") or "") or j.get("descriptionPlain", "") or "",
            "source": f"ashby:{slug}",
        })
    return out


def from_lever(slug):
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
            "content": strip_html(j.get("descriptionPlain", "") or j.get("description", "")),
            "source": f"lever:{slug}",
        })
    return out


def _sr_content(slug, posting_id):
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


def from_smartrecruiters(slug):
    data = parse_json(get(f"https://api.smartrecruiters.com/v1/companies/{slug}/postings?limit=100"),
                      f"smartrecruiters:{slug}")
    out = []
    for j in (data.get("content", []) if isinstance(data, dict) else []):
        loc = j.get("location") or {}
        title = j.get("name", "")
        # Only fetch the ad body for roles on an active track; the blocker scan needs it.
        content = _sr_content(slug, j.get("id")) if matches_track(title, ACTIVE_TRACKS) else ""
        out.append({
            "title": title,
            "company": (j.get("company") or {}).get("name") or display_name(slug),
            "location": ", ".join(filter(None, [loc.get("city"), loc.get("region"),
                                                (loc.get("country") or "").upper()])),
            "url": f"https://jobs.smartrecruiters.com/{slug}/{j.get('id')}",
            "closes": "",
            "content": content,
            "source": f"smartrecruiters:{slug}",
        })
    return out


def from_michael_page(page_url):
    """Scrape the category page for links, then fetch each ad (capped, throttled)."""
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
                time.sleep(MP_DELAY)
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


# ------------------------------------------------------------ platform registry
# The single registration point for a new platform: name -> (collector, default
# targets). Platforms are code; which of them a workspace sweeps is config.
PLATFORMS = {
    "greenhouse": (from_greenhouse, GREENHOUSE),
    "ashby": (from_ashby, ASHBY),
    "lever": (from_lever, LEVER),
    "smartrecruiters": (from_smartrecruiters, SMARTRECRUITERS),
    "michaelpage": (from_michael_page, MICHAEL_PAGE),
}

SLUG_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]*")  # michaelpage takes full URLs instead


def resolve_sources(workspace):
    """Effective sources for this workspace: the `sources` key in the
    workspace's workspace.json wholly replaces the registry defaults; absent
    = defaults plus catalogue additions. Returns [(name, targets)], raising on unusable config so
    an unsupported board is never silently misconfigured as if it worked."""
    cfg = config(workspace).get("sources")
    if cfg is None:
        extra = (ROLE_CATALOGUE or {}).get("additional_sources", {})
        return [(name, list(dict.fromkeys([*defaults, *extra.get(name, [])])))
                for name, (_, defaults) in PLATFORMS.items()]
    return validate_sources(cfg, "workspace.json sources")


def validate_sources(cfg, label):
    if not isinstance(cfg, dict):
        raise ValueError(f"{label}: want an object mapping platform name to a target list")
    unknown = [k for k in cfg if k not in PLATFORMS]
    if unknown:
        raise ValueError(f"{label}: unknown platform(s) {', '.join(unknown)} -- "
                         f"registered: {', '.join(PLATFORMS)}")
    out = []
    for name, targets in cfg.items():
        if not isinstance(targets, list) or not all(isinstance(t, str) and t.strip() for t in targets):
            raise ValueError(f"{label}: {name} wants a list of targets")
        out.append((name, targets))
    return out


def probe(spec):
    """Fetch exactly one source, print its health JSON, exit 0 only if healthy.
    Zero postings is healthy; unreachable or unparseable is not."""
    if ":" not in spec:
        sys.exit("--probe wants platform:target, e.g. --probe greenhouse:cultureamp")
    name, target = spec.split(":", 1)
    if name not in PLATFORMS:
        sys.exit(f"Unknown platform '{name}' -- registered: {', '.join(PLATFORMS)}")
    if name == "michaelpage":
        if not target.startswith(("http://", "https://")):
            sys.exit(f"michaelpage targets are full category-page URLs, not '{target}'")
    elif not SLUG_RE.fullmatch(target):
        sys.exit(f"'{target}' is not a valid {name} slug")
    collector, _defaults = PLATFORMS[name]
    _, health = collect(name, target, collector)
    if health["ok"] and health["count"] == 0 and not health["note"]:
        health["note"] = "ok (0 posted)"  # machine-checkable: zero postings is healthy
    print(json.dumps(health))
    sys.exit(0 if health["ok"] else 1)


# -------------------------------------------------------------------- filters

def normalise_title(value):
    return re.sub(r"[^a-z0-9]+", " ", value.lower()).strip()


def role_matches(title, tracks):
    """Explain each matching catalogue family using its most specific title phrase."""
    t = f" {normalise_title(title)} "
    matches = []
    for role in (ROLE_CATALOGUE or {}).get("roles", []):
        if role["track"] not in tracks:
            continue
        terms = [term for term in role["search_terms"] if f" {normalise_title(term)} " in t]
        if terms:
            matches.append({"id": role["id"], "title": role["titles"][0],
                            "track": role["track"], "priority": role["priority"],
                            "term": max(terms, key=lambda term: len(normalise_title(term)))})
    return matches


def matches_track(title, tracks):
    if ROLE_CATALOGUE is not None:
        matched = {match["track"] for match in role_matches(title, tracks)}
        return [track for track in tracks if track in matched]
    t = f" {normalise_title(title)} "
    return [track for track in tracks
            if any(f" {normalise_title(term)} " in t for term in TRACK_KEYWORDS.get(track, []))]


def match_lines(matches):
    return [f"- match: `{m['id']}` · {m['title']} · {m['track']} · {m['priority']} · "
            f"title contains `{m['term']}`" for m in matches]


def coverage_rows(tracks, health, source_filter):
    """Counts describe fetched postings only; source failures limit every active family."""
    return [{"id": role["id"], "title": role["titles"][0], "track": role["track"],
             "priority": role["priority"], "matched": 0, "known": 0, "outside": 0,
             "blocked": 0, "candidates": 0,
             "coverage": ("Not searched" if role["track"] not in tracks or not health
                          else "Partial" if source_filter or any(not h["ok"] for h in health)
                          else "Fetched boards")}
            for role in (ROLE_CATALOGUE or {}).get("roles", [])]


def coverage_lines(rows):
    if not rows:
        return ["## Role coverage", "",
                "No `jobs/roles.json` catalogue in this Workspace, so postings are matched by",
                "the built-in title keywords for each track. Add the catalogue to record",
                "coverage per role family.", ""]
    lines = ["## Role coverage", "",
             "Counts are title matches in unique fetched postings, not fit verdicts or a market-wide search.",
             "Known, outside location, blocked and to assess divide the title matches into outcomes.",
             "Partial means a source filter was used or a source failed. Not searched means the track",
             "was excluded or no sources ran. Manual searches are not counted here.", "",
             "| Role family | Role | Track | Priority | Title matches | Known | Outside location | Blocked | To assess | Coverage |",
             "| :-- | :-- | :-- | :-- | --: | --: | --: | --: | --: | :-- |"]
    for row in rows:
        cells = [row[k] for k in ("id", "title", "track", "priority", "matched", "known",
                                  "outside", "blocked", "candidates", "coverage")]
        lines.append("| " + " | ".join(str(value).replace("|", "/").replace("\n", " ")
                                      for value in cells) + " |")
    return lines + [""]


def _word_re(term):
    return re.compile(r"(?<![a-z0-9])%s(?![a-z0-9])" % re.escape(term.lower()))


def _load_gazetteer():
    """(term, level, names) rows. `names` are the lower-case Place names that
    cover a mention of `term`; level 0 is a country or group, 1 a state or city."""
    data = json.loads(GAZETTEER_PATH.read_text())
    groups = {}
    for group, members in data["groups"].items():
        for member in members:
            groups.setdefault(member, []).append(group)
    rows = []
    for country, entry in data["countries"].items():
        names = {country.lower(), *(g.lower() for g in groups.get(country, []))}
        for term in [country, *entry["aliases"]]:
            rows.append((term.lower(), 0, names))
        for term in entry["places"]:
            rows.append((term.lower(), 1, {term.lower(), *names}))
    for group in data["groups"]:
        members = {m.lower() for m in data["groups"][group]}
        rows.append((group.lower(), 0, {group.lower(), *members}))  # a group covers its members
    return [(_word_re(term), level, names) for term, level, names in rows]


_GAZETTEER = None


def _gazetteer():
    global _GAZETTEER
    if _GAZETTEER is None:
        _GAZETTEER = _load_gazetteer()
    return _GAZETTEER


def active_search_area(search_area):
    """The Search area when it names at least one Place, else None (no filter)."""
    if not isinstance(search_area, dict):
        return None
    places = search_area.get("places")
    if not isinstance(places, list) or not any(isinstance(p, str) and p.strip() for p in places):
        return None
    return search_area


def location_ok(loc, search_area=None):
    """Keep a role unless it names a known place outside the Search area.

    No Search area keeps everything. A role is kept if it names one of the Places,
    directly or through the gazetteer (a country covers its states and cities).
    A remote role follows the Remote rule: `anywhere` keeps it, `no` drops it and
    `in-places` (the default) keeps it unless it names only places outside.
    An empty or unknown location is kept."""
    search_area = active_search_area(search_area)
    if search_area is None:
        return True
    place = (loc or "").lower().strip()
    if not place:
        return True
    remote = search_area.get("remote")
    is_remote = bool(re.search(r"(?<![a-z])(remote|anywhere)(?![a-z])", place))
    if is_remote and remote == "anywhere":
        return True
    if is_remote and remote == "no":
        return False
    places = {p.strip().lower() for p in search_area.get("places") or [] if isinstance(p, str)}
    if any(_word_re(p).search(place) for p in places if p):
        return True
    mentions = [(level, names) for pattern, level, names in _gazetteer() if pattern.search(place)]
    if any(level == 0 for level, _ in mentions):
        mentions = [m for m in mentions if m[0] == 0]  # an explicit country decides
    if not mentions:
        return True
    return any(names & places for _, names in mentions)


def norm_key(company, title):
    """company|title with case, punctuation, 'via X' and Pty Ltd noise removed."""
    def n(x):
        x = re.sub(r"(?i)\s+via\s+.*$|\(client undisclosed\)|\b(pty|ltd|limited|inc)\b", " ", x or "")
        return re.sub(r"[^a-z0-9]+", " ", x.lower()).strip()
    return f"{n(company)}|{n(title)}"


def norm_url(u):
    """URL without query string, fragment (tracking params), trailing punctuation or slash."""
    u = (u or "").strip().rstrip(".,);")
    try:
        sp = urllib.parse.urlsplit(u)
    except ValueError:
        return u
    if not sp.scheme:
        return u
    return urllib.parse.urlunsplit((sp.scheme, sp.netloc.lower(), sp.path.rstrip("/"), "", ""))


def seen_urls(workspace):
    """Normalised URLs of every captured Application, whatever its Status.

    A Dismissed Application is still a role the Owner has dealt with, so its URL
    must never come back as a fresh candidate. The Application scan is the only
    source of truth here: no Markdown list feeds this set.
    """
    seen = set()
    for application in meta_store.scan(workspace):
        url = norm_url(application.url)
        if url:
            seen.add(url)
    return seen


# new-job.py's favourable list includes a bare \bcontract\b, which is fine when
# scanning one pasted ad but matches site navigation ("Contract Recruitment") on a
# scraped page. Only keep patterns specific enough to survive a full-page scrape.
def scan(text, blockers, favourable):
    # new-job.py exports compiled patterns (re.I baked in) -- never pass flags again.
    # Blocker matching (negation, EEO boilerplate) lives in new-job.py's blocker_hits.
    hits = [(lbl, "BLOCKER") for lbl in nj.blocker_hits(text, blockers)]
    for pat, lbl in favourable:
        text_pat = pat.pattern if isinstance(pat, re.Pattern) else pat
        if "month" not in text_pat:
            continue  # skip the generic contract/day-rate catch-all
        if (pat.search(text) if isinstance(pat, re.Pattern) else re.search(pat, text, re.I)):
            hits.append((lbl, "favourable"))
    return hits


# ---------------------------------------------------------------------- health

def collect(name, target, fn):
    """Run one collector. Returns (jobs, health) -- a failure is recorded, not swallowed."""
    try:
        jobs = fn(target)
        note = ""
        if name == "michaelpage" and len(jobs) > MP_MAX_ADS:
            note = f"{len(jobs) - MP_MAX_ADS} ads over the {MP_MAX_ADS}-fetch cap listed without ad text"
        return jobs, {"source": name, "target": target, "ok": True, "count": len(jobs), "note": note}
    except SourceError as e:
        err = str(e)
    except Exception as e:  # malformed payload: still a dead source, not zero jobs
        err = f"{type(e).__name__}: {e}"
    return [], {"source": name, "target": target, "ok": False, "count": 0, "note": err}


def health_lines(health):
    failed = [h for h in health if not h["ok"]]
    lines = ["## Source health", ""]
    if failed:
        lines += [f"**{len(failed)} of {len(health)} sources FAILED** — their jobs are missing from this "
                  "sweep, not absent from the market.", ""]
    lines += ["| Source | Target | Status | Jobs | Note |", "| :-- | :-- | :-- | --: | :-- |"]
    for h in health:
        status = "ok" if h["ok"] else "**FAILED**"
        if h["ok"] and h["count"] == 0:
            status = "ok (0 posted)"
        lines.append(f"| {h['source']} | {h['target'].replace('|', '/')} | {status} | {h['count']} | "
                     f"{h['note'].replace('|', '/')} |")
    return lines + [""]


# ----------------------------------------------------------------------- main

def main():
    global ACTIVE_TRACKS, ROLE_CATALOGUE
    workspace = workspace_from_cli()
    available = workspace_tracks(workspace)

    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--track", action="append", choices=available or None,
                    help="repeatable; defaults to all workspace tracks")
    ap.add_argument("--roles", action="store_true",
                    help="print the workspace role catalogue as JSON without fetching or writing")
    ap.add_argument("--sources", action="store_true",
                    help="print the effective workspace source plan as JSON without fetching or writing")
    ap.add_argument("--source", choices=list(PLATFORMS))
    ap.add_argument("--probe", metavar="PLATFORM:TARGET",
                    help="fetch exactly one source, print its health JSON, exit non-zero if unhealthy")
    ap.add_argument("--platforms", action="store_true",
                    help="print the platform registry (name -> default targets) as JSON and exit")
    ap.add_argument("--all-locations", action="store_true")
    ap.add_argument("--data", help="Workspace path; or set ROLECALL_WORKSPACE")
    args = ap.parse_args()
    if args.platforms:
        print(json.dumps({name: list(defaults) for name, (_, defaults) in PLATFORMS.items()}))
        return
    if args.probe:
        probe(args.probe)
        return
    ROLE_CATALOGUE = load_role_catalogue(workspace, available)
    if args.roles:
        print(json.dumps(ROLE_CATALOGUE, ensure_ascii=False, indent=2))
        return
    if args.sources:
        try:
            print(json.dumps(dict(resolve_sources(workspace))))
        except ValueError as error:
            sys.exit(str(error))
        return
    tracks = args.track or available
    ACTIVE_TRACKS = tracks

    search_area = active_search_area(config(workspace).get("search_area"))
    blockers, favourable = load_blockers(workspace)
    seen = seen_urls(workspace)
    if search_area is None and not args.all_locations:
        print(NO_SEARCH_AREA)
    print(f"Sweeping tracks {', '.join(tracks)} ... ({len(seen)} URLs already known)")

    jobs, health = [], []
    try:
        plan = resolve_sources(workspace)
    except ValueError as error:
        sys.exit(str(error))
    for name, targets in plan:
        if args.source and args.source != name:
            continue
        collector, _defaults = PLATFORMS[name]
        for target in targets:
            got, h = collect(name, target, collector)
            tag = f"{h['count']} jobs" if h["ok"] else f"FAILED: {h['note']}"
            print(f"  {name}: {target} -> {tag}")
            jobs += got
            health.append(h)

    coverage = coverage_rows(tracks, health, args.source)
    by_role = {row["id"]: row for row in coverage}
    kept, skipped_seen, skipped_loc, skipped_track = [], 0, 0, 0
    this_run = set()
    for j in jobs:
        ukey = norm_url(j["url"])
        nkey = norm_key(j["company"], j["title"])
        if ukey in this_run or nkey in this_run:
            continue  # same role under two category pages, or on two boards
        this_run.update(k for k in (ukey, nkey) if k)
        matches = role_matches(j["title"], tracks)
        for match in matches:
            by_role[match["id"]]["matched"] += 1
        if ukey in seen:
            for match in matches:
                by_role[match["id"]]["known"] += 1
            skipped_seen += 1
            continue
        hit = matches_track(j["title"], tracks)
        if not hit:
            skipped_track += 1
            continue
        if not args.all_locations and not location_ok(j["location"], search_area):
            for match in matches:
                by_role[match["id"]]["outside"] += 1
            skipped_loc += 1
            continue
        j["tracks"] = hit
        j["matches"] = matches
        j["signals"] = scan(j.get("content", "") or j["title"], blockers, favourable)
        outcome = "blocked" if any(k == "BLOCKER" for _, k in j["signals"]) else "candidates"
        for match in matches:
            by_role[match["id"]][outcome] += 1
        kept.append(j)

    blocked = [j for j in kept if any(k == "BLOCKER" for _, k in j["signals"])]
    clean = [j for j in kept if j not in blocked]
    # Favourable signals (short contracts) float to the top -- the visa is moot there.
    clean.sort(key=lambda j: (not any(k == "favourable" for _, k in j["signals"]), j["title"]))

    failed = [h for h in health if not h["ok"]]
    lines = [
        "# Sweep candidates",
        "",
        "Raw output of `plugin/scripts/sweep.py`. **Not a shortlist** — nothing here has been judged",
        "against the Profile yet. The `find-jobs` skill reads each ad and records only what is worth",
        "applying for as a Lead Application (`jobs/<track>/<slug>/meta.yml` + `jd.md`).",
        "",
        *(["Search catalogue: [Master role list](roles.json).", ""]
          if ROLE_CATALOGUE is not None else []),
        f"Swept {date.today().isoformat()} · tracks {', '.join(tracks)} · "
        f"{len(jobs)} fetched, {len(kept)} matched, {skipped_seen} already known"
        + (f" · **{len(failed)} source(s) FAILED**" if failed else ""),
        "",
        *([f"Search area: {NO_SEARCH_AREA}.", ""] if search_area is None else []),
        *health_lines(health),
        *coverage_lines(coverage),
        "---",
        "",
        f"## To assess ({len(clean)})",
        "",
    ]
    for j in clean:
        sig = "".join(f" `{lbl}`" for lbl, k in j["signals"] if k == "favourable")
        lines += [
            f"### {j['title']} — {j['company']}",
            f"- {j['location']}{' · closes ' + j['closes'] if j['closes'] else ''}"
            f" · {'/'.join(j['tracks'])}{' · ' + j['type_hint'] if j.get('type_hint') else ''}{sig}",
            f"- {j['url']}",
            f"- source: `{j['source']}`" + ("" if j.get("content") else "  **(no ad text — fetch it)**"),
            *match_lines(j["matches"]),
            "",
        ]

    if blocked:
        lines += [f"## Blocked ({len(blocked)})", "",
                  "Visa, citizenship or clearance signals found in the ad text.", ""]
        for j in blocked:
            why = "; ".join(lbl for lbl, k in j["signals"] if k == "BLOCKER")
            lines += [f"- **{j['title']}** — {j['company']} — {why}", f"  {j['url']}",
                      *match_lines(j["matches"]), ""]

    lines += [
        "## Not reachable by this script",
        "",
        "- **Seek** and **LinkedIn** return 403. The skill covers them via WebSearch, which",
        "  yields leads rather than readable ads.",
        "- Sources without a public API need the skill's WebFetch path.",
        "",
    ]

    out = Path(workspace) / "jobs" / "sweep-candidates.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text("\n".join(lines))
    print(f"\n  {len(jobs)} fetched · {len(clean)} to assess · {len(blocked)} blocked")
    print(f"  skipped: {skipped_seen} already known, {skipped_track} off-track, "
          f"{skipped_loc} wrong location")
    print(f"  sources: {len(health) - len(failed)} ok, {len(failed)} FAILED")
    for h in failed:
        print(f"    FAILED {h['source']} {h['target']}: {h['note']}")
    print(f"\nWrote {out}")
    print("Next: ask the agent to run the find-jobs skill, or assess these against the Profile.")
    if health and len(failed) == len(health):
        sys.exit(1)


if __name__ == "__main__":
    main()
