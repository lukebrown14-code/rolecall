"""The Search area filter: whether a role's location is inside the Owner's Places.

The Places and the Remote rule come from workspace.json ``search_area``; the
gazetteer (plugin/gazetteer.json) maps countries to their states and cities.
"""

import json
import re
from pathlib import Path

GAZETTEER_PATH = Path(__file__).resolve().parent.parent / "gazetteer.json"


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
