#!/usr/bin/env python3
"""Capture a job description into a new Application folder.

    python3 plugin/scripts/new-job.py --track SWE --url https://...
    python3 plugin/scripts/new-job.py --track SWE --paste        # reads stdin
    python3 plugin/scripts/new-job.py --track SWE --file ~/Desktop/jd.txt
    python3 plugin/scripts/new-job.py --track SWE --paste --source https://example.com/job/1

Creates the Application through the shared ``meta.create``: a ``meta.yml`` with
``schema: 1`` and one ``Lead`` entry, plus a front-matter-free ``jd.md``. It
writes no tracker row, no prepared documents and no ``out/``.

Seek and LinkedIn require a login, so ``--url`` will usually fail on them. When
it does, the script says so plainly and asks for a paste instead.
"""

import argparse
import json
import re
import sys
import urllib.error
import urllib.request
from datetime import datetime, timezone
from html import unescape
from html.parser import HTMLParser
from pathlib import Path
from typing import NoReturn

sys.path.insert(0, str(Path(__file__).resolve().parent))
import meta as meta_store  # noqa: E402
from signals import blockers_for, scan  # noqa: E402
from workspace import add_data_argument, data_from_argv, load, workspace_from_cli  # noqa: E402

# Sites that serve job content only behind a login or heavy client-side JS.
LOGIN_WALLED = ("seek.com.au", "linkedin.com", "glassdoor.com", "indeed.com")


class TextExtractor(HTMLParser):
    """Crude tag-stripper. Good enough for a JD we are going to read anyway."""

    SKIP = {"script", "style", "nav", "footer", "header", "svg", "noscript"}
    # Lenient set for ATS pages, where the ad body often sits inside <header>.
    SKIP_LENIENT = {"script", "style", "svg", "noscript", "nav", "footer"}

    def __init__(self, skip=None):
        super().__init__()
        self.SKIP = set(skip or self.SKIP)
        self.parts = []
        self.skipping = 0

    def handle_starttag(self, tag, _attrs):
        if tag in self.SKIP:
            self.skipping += 1
        elif tag in ("p", "br", "li", "div", "h1", "h2", "h3", "h4", "tr"):
            self.parts.append("\n")

    def handle_endtag(self, tag):
        if tag in self.SKIP and self.skipping:
            self.skipping -= 1

    def handle_data(self, data):
        if not self.skipping and data.strip():
            self.parts.append(data.strip())

    def text(self):
        out = " ".join(self.parts)
        out = re.sub(r" *\n *", "\n", out)
        return re.sub(r"\n{3,}", "\n\n", out).strip()


ATS_HOSTS = (
    "greenhouse.io",
    "lever.co",
    "ashbyhq.com",
    "workable.com",
    "smartrecruiters.com",
    "myworkdayjobs.com",
    "bamboohr.com",
    "teamtailor.com",
    "recruitee.com",
    "pageuppeople.com",
    "jobvite.com",
    "breezy.hr",
    "applytojob.com",
    "workforcenow.adp.com",
)


def html_to_text(html, lenient=False):
    parser = TextExtractor(TextExtractor.SKIP_LENIENT if lenient else None)
    parser.feed(html)
    return parser.text()


def _plain(html):
    """HTML fragment (e.g. a JSON-LD description) to readable text."""
    if "&lt;" in html:
        html = unescape(html)
    if "<" not in html:
        return html.strip()
    html = re.sub(r"(?i)<br\s*/?>|</p>|</li>|</div>|</h\d>", "\n", html)
    return html_to_text(html, lenient=True)


def _walk(obj):
    if isinstance(obj, dict):
        yield obj
        for value in obj.values():
            yield from _walk(value)
    elif isinstance(obj, list):
        for value in obj:
            yield from _walk(value)


def json_ld_job(html):
    """The first schema.org JobPosting in the page's JSON-LD blocks, or {}."""
    for match in re.finditer(
        r"(?is)<script[^>]+application/ld\+json[^>]*>(.*?)</script>", html
    ):
        try:
            data = json.loads(match.group(1).strip())
        except ValueError:
            continue
        for node in _walk(data):
            kind = node.get("@type")
            if kind == "JobPosting" or (isinstance(kind, list) and "JobPosting" in kind):
                return node
    return {}


def next_data_description(html):
    """Longest description-like string in a __NEXT_DATA__ blob (JS-rendered pages)."""
    match = re.search(
        r'(?is)<script[^>]+id="__NEXT_DATA__"[^>]*>(.*?)</script>', html
    )
    if not match:
        return ""
    try:
        data = json.loads(match.group(1))
    except ValueError:
        return ""
    best = ""
    for node in _walk(data):
        for key, value in node.items():
            if (
                isinstance(value, str)
                and re.search(r"(?i)descrip|content|body|summary|responsib", key)
                and len(value) > len(best)
            ):
                best = value
    return _plain(best) if len(best) > 400 else ""


def _meta_content(html, prop):
    for match in re.finditer(r"(?is)<meta\b[^>]*>", html):
        tag = match.group(0)
        if re.search(
            r'(?i)(?:property|name)\s*=\s*["\']%s["\']' % re.escape(prop), tag
        ):
            content = re.search(
                r'(?is)content\s*=\s*(?:"([^"]*)"|\'([^\']*)\')', tag
            )
            if content:
                return unescape(content.group(1) or content.group(2) or "").strip()
    return ""


def split_title(title):
    """(role, company) from 'Role at Company' / 'Role | Company' / 'Role - Company'."""
    title = re.sub(r"\s+", " ", unescape(title or "")).strip()
    if not title:
        return "", ""
    match = re.match(r"^(.+?)\s+at\s+(.+)$", title)
    parts = (
        [match.group(1), match.group(2)]
        if match
        else re.split(r"\s+[|\-–—·]\s+", title)
    )
    parts = [
        re.sub(
            r"(?i)\b(careers?|jobs?|job openings?|apply( now)?)\b\s*$", "", part
        ).strip(" -|")
        for part in parts
    ]
    parts = [
        part
        for part in parts
        if part and not re.fullmatch(r"(?i)careers?|jobs?|seek|linkedin", part)
    ]
    if not parts:
        return "", ""
    return parts[0], (parts[1] if len(parts) > 1 else "")


def page_meta(html):
    """Structured facts from a fetched page: JSON-LD first, then og:title, then <title>."""
    meta = {
        "role": "",
        "company": "",
        "description": "",
        "salary": "",
        "closes": "",
        "type": "",
    }
    ld = json_ld_job(html)
    if ld:
        meta["role"] = unescape(str(ld.get("title") or "")).strip()
        org = ld.get("hiringOrganization")
        if isinstance(org, dict):
            org = org.get("name")
        meta["company"] = unescape(str(org or "")).strip()
        meta["description"] = _plain(str(ld.get("description") or ""))
        meta["closes"] = str(ld.get("validThrough") or "")[:10]
        employment = ld.get("employmentType")
        meta["type"] = (
            ", ".join(employment) if isinstance(employment, list) else str(employment or "")
        )
        meta["type"] = meta["type"].replace("_", " ").title()
        salary = ld.get("baseSalary")
        if isinstance(salary, dict):
            value = salary.get("value") if isinstance(salary.get("value"), dict) else salary
            low, high = value.get("minValue"), value.get("maxValue")
            currency = salary.get("currency") or value.get("currency") or ""
            unit = str(value.get("unitText") or "").lower()
            amount = (
                low
                if low is not None and high is None
                else (f"{low}-{high}" if low is not None else value.get("value", ""))
            )
            if amount != "":
                meta["salary"] = f"{currency} {amount} {unit}".strip()
    if not (meta["role"] and meta["company"]):
        role, company = split_title(_meta_content(html, "og:title"))
        if not role:
            match = re.search(r"(?is)<title[^>]*>(.*?)</title>", html)
            role, company = split_title(match.group(1) if match else "")
        meta["role"] = meta["role"] or role
        meta["company"] = (
            meta["company"] or company or _meta_content(html, "og:site_name")
        )
    return meta


def fetch_page(url):
    """(text, meta) for a URL; exits via warn() when the ad cannot be read."""
    host = re.sub(r"^https?://(www\.)?", "", url).split("/")[0]
    if any(walled in host for walled in LOGIN_WALLED):
        warn(
            f"{host} serves job ads behind a login, so fetching it will not work.",
            "Open the ad in your browser, copy the description, and run:",
            "    python3 plugin/scripts/new-job.py --track <TRACK> --paste",
        )

    request = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            html = response.read().decode(
                response.headers.get_content_charset() or "utf-8", "replace"
            )
    except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError) as error:
        warn(
            f"Could not fetch {url} -- {error}",
            "Copy the job description from your browser and run:",
            "    python3 plugin/scripts/new-job.py --track <TRACK> --paste",
        )

    text, meta = html_to_job(html, ats=any(host_part in host for host_part in ATS_HOSTS))
    if len(text) < 400:
        warn(
            f"Fetched {url} but only got {len(text)} characters of text.",
            "The page is probably rendered client-side. Paste the description instead:",
            "    python3 plugin/scripts/new-job.py --track <TRACK> --paste",
        )
    return text, meta


def html_to_job(html, ats=False):
    """Best JD text from an HTML page plus its structured meta.

    Tries the plain extraction (nav/header/footer stripped), a lenient one that
    keeps header (ATS pages put the ad there), then JSON-LD and __NEXT_DATA__
    for client-rendered pages, and keeps whichever is longest.
    """
    meta = page_meta(html)
    candidates = [html_to_text(html, lenient=ats)]
    if not ats and len(candidates[0]) < 400:
        candidates.append(html_to_text(html, lenient=True))
    candidates.append(meta["description"])
    candidates.append(next_data_description(html))
    text = max(candidates, key=len)
    return text, meta


def looks_like_html(text):
    return bool(re.search(r"(?i)<(!doctype|html|head|body|script)\b", text[:3000]))


def fetch(url):
    return fetch_page(url)[0]


def warn(*lines) -> NoReturn:
    print("\n  WARNING", file=sys.stderr)
    for line in lines:
        print(f"  {line}", file=sys.stderr)
    print(file=sys.stderr)
    sys.exit(2)


def guess(jd, pattern, fallback):
    match = re.search(pattern, jd, re.I | re.M)
    return match.group(1).strip() if match else fallback


_MONTHS = "jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec"
_DATE = (
    r"(\d{4}-\d{2}-\d{2}|\d{1,2}(?:st|nd|rd|th)?\s+(?:%s)[a-z]*\.?,?\s+\d{4}"
    r"|(?:%s)[a-z]*\.?\s+\d{1,2}(?:st|nd|rd|th)?,?\s+\d{4}|\d{1,2}/\d{1,2}/\d{2,4})"
) % (_MONTHS, _MONTHS)


def _iso(value):
    value = (
        re.sub(r"(?<=\d)(st|nd|rd|th)", "", value.replace(",", ""))
        .replace(".", "")
        .strip()
    )
    for fmt in (
        "%Y-%m-%d",
        "%d %B %Y",
        "%d %b %Y",
        "%B %d %Y",
        "%b %d %Y",
        "%d/%m/%Y",
        "%d/%m/%y",
    ):
        try:
            return datetime.strptime(value, fmt).date().isoformat()
        except ValueError:
            pass
    retry = value.replace("Sept", "Sep")
    if retry != value:
        return _iso(retry)
    return value


def extract_closing(text):
    match = re.search(
        r"(?i)(?:closes?|closing(?:\s+date)?|applications?\s+close|apply\s+by|expires?|deadline)"
        r"[^\n\d]{0,25}" + _DATE,
        text,
    )
    return _iso(match.group(1)) if match else ""


_AMT = r"\$\s?\d[\d,]*(?:\.\d+)?\s?[kK]?"
_PERIOD = (
    r"(?:\s*(?:per|/|a|p\.?)\s*(?:year|annum|yr|hour|hr|day|week|a\.?|pa)\b\.?"
    r"|\s*p\.?a\.?\b|\s*(?:pa|ph|p/h)\b)?"
)
SALARY = re.compile(
    r"(?:AUD\s*)?(%s)(?:\s*(?:-|–|—|to)\s*(?:AUD\s*)?(%s))?(%s)" % (_AMT, _AMT, _PERIOD),
    re.I,
)


def _amount(value):
    number = float(re.sub(r"[^\d.]", "", value) or 0)
    return number * 1000 if value.strip().lower().endswith("k") else number


def extract_salary(text):
    """First plausible pay figure ('$120k - $140k', '$85 per hour'); '' if none."""
    for match in SALARY.finditer(text):
        low, _high, period = match.group(1), match.group(2), match.group(3) or ""
        context = text[max(0, match.start() - 60) : match.start()].lower()
        number = _amount(low)
        hourly_or_daily = bool(
            re.search(r"hour|hr|day|ph\b|p/h", period.lower())
        ) or "rate" in context
        if (
            number >= 20000
            or (hourly_or_daily and 20 <= number < 5000)
            or (
                re.search(
                    r"salary|package|remuneration|base|ote|rate|\bpay\b", context
                )
                and number >= 1000
            )
        ):
            return re.sub(r"\s+", " ", match.group(0)).strip().rstrip(".")
    return ""


_TYPES = [
    ("Fixed term", r"fixed[\s-]term"),
    ("Contract", r"\bcontract(?:or)?\b"),
    ("Temporary", r"\btemp(?:orary)?\b"),
    ("Casual", r"\bcasual\b"),
    ("Part time", r"part[\s-]time"),
    ("Permanent", r"\bpermanent\b"),
    ("Full time", r"full[\s-]time"),
]


def extract_type(text):
    match = re.search(
        r"(?i)(?:employment|work|job)\s+type\s*[:\-]\s*([A-Za-z][A-Za-z \-/]{2,30})",
        text,
    )
    labelled = match.group(1) if match else text
    for name, pattern in _TYPES:
        if re.search(pattern, labelled, re.I):
            return name
    return ""


def extract_seniority(role, text=""):
    """'senior' / 'junior' / 'mid' from the title; falls back to years asked for."""
    title = (role or "").lower()
    if re.search(r"\b(senior|snr|sr\.?|lead|principal|staff|head of)\b", title):
        return "senior"
    if re.search(r"\b(junior|jr\.?|graduate|entry[\s-]level|intern)\b", title):
        return "junior"
    if re.search(r"\b(mid[\s-]?level|intermediate)\b", title):
        return "mid"
    years = [
        int(year)
        for year in re.findall(
            r"(\d{1,2})\s*\+?\s*(?:-\s*\d+\s*)?years", text, re.I
        )
    ]
    if years and max(years) >= 8:
        return "senior"
    return ""


def extract_facts(jd, role="", meta=None):
    """Salary, closing date, employment type, seniority -- structured data wins over text."""
    meta = meta or {}
    return {
        "salary": meta.get("salary") or extract_salary(jd),
        "closes": meta.get("closes") or extract_closing(jd),
        "type": meta.get("type") or extract_type(jd),
        "seniority": extract_seniority(role, jd),
    }


def _one_line(value):
    return re.sub(r"\s+", " ", str(value)).strip()


def _valid_closes(value):
    """Only a plain YYYY-MM-DD is written; extraction can return a non-date."""
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}", value or ""):
        return ""
    try:
        datetime.strptime(value, "%Y-%m-%d")
    except ValueError:
        return ""
    return value


def read_input(args):
    """(jd text, source, meta) from --url, --file or stdin. HTML in a file or
    paste is parsed like a fetched page."""
    meta = {}
    if args.url:
        jd, meta = fetch_page(args.url)
        source = args.url
    elif args.file:
        path = Path(args.file).expanduser()
        if not path.exists():
            sys.exit(f"No such file: {path}")
        jd, source = path.read_text(errors="replace"), str(path)
    else:
        if not getattr(args, "fetch_only", False):
            print("Paste the job description, then press Ctrl-D:", file=sys.stderr)
        jd, source = sys.stdin.read(), "pasted"
    if not args.url and looks_like_html(jd):
        jd, meta = html_to_job(jd)
    if args.source and not args.url:
        source = args.source
    return jd.strip(), source, meta


def main(argv=None):
    workspace = workspace_from_cli(data_from_argv(argv))
    tracks = load(workspace).tracks

    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    fetch_only = "--fetch-only" in (sys.argv[1:] if argv is None else argv)
    parser.add_argument("--track", required=not fetch_only, choices=tracks or None)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--url")
    source.add_argument("--file")
    source.add_argument("--paste", action="store_true", help="read the JD from stdin")
    parser.add_argument(
        "--fetch-only",
        action="store_true",
        help="print the JD text and visa signals as JSON and exit; writes nothing",
    )
    parser.add_argument("--company")
    parser.add_argument("--role")
    parser.add_argument(
        "--source",
        help="ad URL to record when the text comes from --paste or --file",
    )
    add_data_argument(parser)
    args = parser.parse_args(argv)

    # Fetch (or read stdin) and report as JSON, writing nothing: the ad text
    # plus the free visa/clearance signals, so a caller can see them before
    # spending a model call.
    if args.fetch_only:
        jd, _src, meta = read_input(args)
        if len(jd) < 200:
            sys.exit(f"Job description is only {len(jd)} characters -- that looks wrong.")
        facts = extract_facts(jd, args.role or meta.get("role", ""), meta)
        print(
            json.dumps(
                {
                    "jd": jd,
                    "signals": scan(jd, blockers_for(load(workspace).blockers)),
                    "meta": {**meta, **facts, "description": ""},
                }
            )
        )
        return

    jd, ad_source, meta = read_input(args)
    if len(jd) < 200:
        sys.exit(f"Job description is only {len(jd)} characters -- that looks wrong.")

    # Company/role: explicit flag, then a Company:/Role: line, then the page's
    # structured data (JSON-LD, og:title, <title>), then a placeholder.
    company = (
        args.company
        or guess(jd, r"^\s*(?:company|employer)\s*[:\-]\s*(.+)$", "")
        or meta.get("company")
        or "company"
    )
    role = (
        args.role
        or guess(jd, r"^\s*(?:role|position|job title)\s*[:\-]\s*(.+)$", "")
        or meta.get("role")
        or "role"
    )
    company, role = _one_line(company), _one_line(role)
    facts = extract_facts(jd, role, meta)

    fields = {
        "title": role,
        "company": company,
        "track": args.track,
        "url": "" if ad_source == "pasted" else ad_source,
        "closes": _valid_closes(facts["closes"]),
    }
    try:
        folder = meta_store.create(workspace, fields, datetime.now(timezone.utc))
    except meta_store.MetaError as error:
        sys.exit(f"rolecall: {error}")

    try:
        (folder / "jd.md").write_text(jd.strip() + "\n")
    except OSError as error:
        print(f"rolecall: created {folder} but could not write jd.md: {error}", file=sys.stderr)
        raise SystemExit(1) from error

    relative = folder.relative_to(workspace)
    print(f"\nCreated {relative}")
    print(f"  jd.md            {len(jd):,} characters captured")
    for key in ("salary", "closes", "type", "seniority"):
        if facts[key]:
            print(f"  {key:<16} {facts[key]}")
    if company == "company" or role == "role":
        print("\n  Company/role could not be read from the JD -- re-run with --company")
        print("  and --role, or correct meta.yml before assessing fit.")

    hits = scan(jd, blockers_for(load(workspace).blockers))
    if hits:
        print("\n  Signals found in the ad:")
        for label, kind in hits:
            print(f"    [{kind}] {label}")
        if any(kind == "BLOCKER" for _, kind in hits):
            print("\n  A blocker is not always fatal -- some ads state citizenship or")
            print("  clearance for roles that turn out to be flexible -- but check before")
            print("  spending time on it.")

    print(f"\nNext: ask the agent to assess fit for {relative}")
    print("  the agent writes fit.md with a verdict before any tailoring happens.")


if __name__ == "__main__":
    main()
