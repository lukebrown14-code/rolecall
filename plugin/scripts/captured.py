"""What the Workspace already holds, so the Sweep never offers a role twice.

Every captured Application counts, whatever its Status: its normalised URL, and its
employer|title key for the same ad listed under another URL.
"""

import re
import urllib.parse

import adfacts
import meta as meta_store


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


def seen_roles(workspace):
    """employer|title keys of every captured Application, whatever its Status.

    The same ad often comes back under another URL (a board's tracking link, a second
    listing). The employer is the company and, for an agency ad, the recruiter too."""
    keys = set()
    for app in meta_store.scan(workspace):
        if app.malformed:
            continue
        title = adfacts.title_key(app.title)
        for employer in (app.company, app.recruiter):
            if title and adfacts.company_key(employer):
                keys.add(f"{adfacts.company_key(employer)}|{title}")
    return keys


def role_key(job):
    return f"{adfacts.company_key(job['company'])}|{adfacts.title_key(job['title'])}"


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
