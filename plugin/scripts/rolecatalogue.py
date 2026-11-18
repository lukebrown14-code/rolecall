"""The Workspace's role catalogue (jobs/roles.json) and how a title matches a track.

The catalogue names role families with their titles, search terms and fit guidance.
Without one, the built-in ``TRACK_KEYWORDS`` decide which track a title belongs to.
Every function takes the loaded catalogue (or None) rather than reading a global.
"""

import json
import re
import sys
from pathlib import Path

from sourceregistry import validate_sources

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


def load(workspace, tracks, platforms):
    """Optional workspace-owned search titles and fit guidance; fail on invalid data.

    ``tracks`` are the Workspace's; ``platforms`` the registered platform names that
    ``additional_sources`` may extend."""
    workspace = Path(workspace)
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
        validate_sources(catalogue.get("additional_sources", {}), "additional_sources", platforms)
        return catalogue
    except (OSError, ValueError) as error:
        sys.exit(f"{path}: {error}")


def normalise_title(value):
    return re.sub(r"[^a-z0-9]+", " ", value.lower()).strip()


def role_matches(catalogue, title, tracks):
    """Explain each matching catalogue family using its most specific title phrase."""
    t = f" {normalise_title(title)} "
    matches = []
    for role in (catalogue or {}).get("roles", []):
        if role["track"] not in tracks:
            continue
        terms = [term for term in role["search_terms"] if f" {normalise_title(term)} " in t]
        if terms:
            matches.append({"id": role["id"], "title": role["titles"][0],
                            "track": role["track"], "priority": role["priority"],
                            "term": max(terms, key=lambda term: len(normalise_title(term)))})
    return matches


def matches_track(catalogue, title, tracks):
    """The tracks a title belongs to: by the catalogue, or else by TRACK_KEYWORDS."""
    if catalogue is not None:
        matched = {match["track"] for match in role_matches(catalogue, title, tracks)}
        return [track for track in tracks if track in matched]
    t = f" {normalise_title(title)} "
    return [track for track in tracks
            if any(f" {normalise_title(term)} " in t for term in TRACK_KEYWORDS.get(track, []))]


def coverage_rows(catalogue, tracks, health, source_filter):
    """Counts describe fetched postings only; source failures limit every active family."""
    return [{"id": role["id"], "title": role["titles"][0], "track": role["track"],
             "priority": role["priority"], "matched": 0, "known": 0, "outside": 0,
             "blocked": 0, "candidates": 0,
             "coverage": ("Not searched" if role["track"] not in tracks or not health
                          else "Partial" if source_filter or any(not h["ok"] for h in health)
                          else "Fetched boards")}
            for role in (catalogue or {}).get("roles", [])]
