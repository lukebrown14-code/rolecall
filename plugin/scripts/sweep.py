#!/usr/bin/env python3
"""Collect candidate roles from the sources that can actually be fetched.

    python3 plugin/scripts/sweep.py                 # all workspace tracks
    python3 plugin/scripts/sweep.py --track SWE     # one track
    python3 plugin/scripts/sweep.py --source greenhouse
    python3 plugin/scripts/sweep.py --all-locations # ignore the location filter
    python3 plugin/scripts/sweep.py --probe greenhouse:cultureamp  # one source, health JSON
    python3 plugin/scripts/sweep.py --platforms    # the registry as JSON
    python3 plugin/scripts/sweep.py --test-source linkedin  # one registry Source, health JSON

This does the mechanical half of a job sweep: fetch, filter by track and
location, drop anything already captured as an Application, and flag visa and
clearance signals. It writes jobs/sweep-candidates.md.

Sources come from the Workspace's workspace.json ``sources`` key when present -- a
whole-key replacement. It is either the Source registry (a list of entries, SCHEMA.md) or
the older platform name -> target list object. Registry entries of type ``browser`` or
``manual`` are not fetched; the report lists them, with their access status, parsing hints
and rules, for the find-jobs skill to search by hand. Otherwise the built-in
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

Search rules (workspace.json ``search_rules``, applied by rules.py) drop roles past the
posting-age limit; the report counts what each rule removed.

Seek and LinkedIn return 403 and cannot be fetched by anything here.
"""

import argparse
import json
import re
import sys
from pathlib import Path

# The Sweep's parts live in modules beside it: boards (the collectors), sourceregistry,
# rolecatalogue, searcharea, captured (what the Workspace holds) and sweepreport.
sys.path.insert(0, str(Path(__file__).resolve().parent))
import boards  # noqa: E402
import httpfetch  # noqa: E402
import rolecatalogue  # noqa: E402
import rules as search_rules  # noqa: E402
import sweepreport  # noqa: E402
from boards import (  # noqa: E402
    ASHBY,
    GREENHOUSE,
    LEVER,
    MICHAEL_PAGE,
    MP_MAX_ADS,
    SMARTRECRUITERS,
    SourceError,
    classify_access,
)
from captured import norm_key, norm_url, role_key, seen_roles, seen_urls  # noqa: E402
from rolecatalogue import TRACK_KEYWORDS  # noqa: E402
from searcharea import active_search_area, location_ok  # noqa: E402
from signals import FAVOURABLE, blocker_hits, blockers_for  # noqa: E402
from sourceregistry import validate_registry, validate_sources  # noqa: E402
from workspace import add_data_argument, data_from_argv, load, workspace_from_cli  # noqa: E402

# Kept reachable as sweep.<name>, as before the split.
from boards import posted_date  # noqa: E402, F401
from sweepreport import health_lines, manual_lines  # noqa: E402, F401

UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"


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
    """jobs/roles.json, validated (rolecatalogue.load); None when absent."""
    tracks = workspace_tracks(workspace) if tracks is None else tracks
    return rolecatalogue.load(workspace, tracks, PLATFORMS)


def role_matches(title, tracks):
    """The catalogue role families a title matches (rolecatalogue.role_matches)."""
    return rolecatalogue.role_matches(ROLE_CATALOGUE, title, tracks)


def matches_track(title, tracks):
    """The tracks a title belongs to (rolecatalogue.matches_track)."""
    return rolecatalogue.matches_track(ROLE_CATALOGUE, title, tracks)


NO_SEARCH_AREA = "no Search area set: all locations kept"


MP_DELAY = 0.3  # seconds between ad fetches


def load_blockers(workspace):
    """The Workspace's visa/clearance patterns plus the shared favourable list."""
    return blockers_for(load(workspace).blockers), FAVOURABLE


def get(url, timeout=25):
    """Fetch a URL; raises SourceError so a dead source is never mistaken for an empty one."""
    try:
        return httpfetch.fetch(url, timeout=timeout, user_agent=UA)
    except httpfetch.FetchError as e:
        if e.status is None:
            raise SourceError(str(e)) from e
        access = classify_access(e.status, e.body)
        label = "" if access == "error" else f" [{access}]"
        raise SourceError(f"{e}{label}", access) from e


def get_quiet(url, timeout=20):
    """For optional per-ad fetches: None on failure, never raises."""
    try:
        return get(url, timeout)
    except SourceError as e:
        print(f"  ! {e}", file=sys.stderr)
        return None


ACTIVE_TRACKS = list(TRACK_KEYWORDS)  # set by main; SmartRecruiters fetches ad bodies only for these


# Each collector reads one board through this module's get, so a test that stubs
# get stubs every board.
def from_greenhouse(slug):
    return boards.from_greenhouse(slug, get)


def from_ashby(slug):
    return boards.from_ashby(slug, get)


def from_lever(slug):
    return boards.from_lever(slug, get)


def from_smartrecruiters(slug):
    return boards.from_smartrecruiters(
        slug, get, get_quiet, lambda title: matches_track(title, ACTIVE_TRACKS)
    )


def from_michael_page(page_url):
    return boards.from_michael_page(page_url, get, get_quiet, MP_DELAY)


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
    if isinstance(cfg, list):
        plan = {}
        for entry in validate_registry(cfg, PLATFORMS):
            if entry["type"] == "api" and entry["enabled"]:
                targets = plan.setdefault(entry["platform"], [])
                targets.extend(t for t in entry["targets"] if t not in targets)
        return list(plan.items())
    return validate_sources(cfg, "workspace.json sources", PLATFORMS)


def source_registry(workspace):
    """Every registry entry, validated; the legacy object form reads as api entries."""
    cfg = config(workspace).get("sources")
    if isinstance(cfg, list):
        return validate_registry(cfg, PLATFORMS)
    return load(workspace).source_registry


def manual_sources(workspace):
    """Sources this script cannot fetch: registry browser/manual entries, then the
    catalogue's older manual_search_sources (free text)."""
    try:
        entries = [e for e in source_registry(workspace) if e["type"] != "api"]
    except ValueError:
        entries = []
    for text in (ROLE_CATALOGUE or {}).get("manual_search_sources", []) or []:
        if isinstance(text, str) and text.strip():
            entries.append({"name": text.strip(), "type": "manual", "access": "unknown",
                            "enabled": True, "legacy": True})
    return entries


def test_source(workspace, name):
    """Check one registry Source; print one JSON line and exit 0 only when it works.

    An api Source fetches every target. A browser Source with a url is fetched once to
    read its access status. A manual Source is searched by hand and cannot be tested."""
    try:
        entries = source_registry(workspace)
    except ValueError as error:
        sys.exit(str(error))
    entry = next((e for e in entries if e["name"].lower() == name.strip().lower()), None)
    if entry is None:
        sys.exit(f"No Source named '{name}' -- listed: {', '.join(e['name'] for e in entries) or 'none'}")
    result = {"name": entry["name"], "ok": False, "count": 0, "access": "error", "note": ""}
    if entry["type"] == "api":
        collector, _ = PLATFORMS[entry["platform"]]
        failed, count, access = [], 0, "works"
        for target in entry["targets"]:
            _, h = collect(entry["platform"], target, collector)
            count += h["count"]
            if not h["ok"]:
                failed.append(f"{target}: {h['note']}")
                access = h.get("access", "error")
        result.update(ok=not failed, count=count, access=access if failed else "works",
                      note="; ".join(failed) or f"{len(entry['targets'])} target(s) read")
    elif entry.get("url"):
        try:
            body = get(entry["url"])
            access = classify_access(200, body)
        except SourceError as error:
            access, body = error.access, ""
            result["note"] = str(error)
        result.update(ok=access == "works", access=access)
        if access == "works" and not result["note"]:
            result["note"] = "page loads; search it by hand"
    else:
        result.update(access="manual", note="manual Source: no url to test; search it by hand")
    print(json.dumps(result))
    sys.exit(0 if result["ok"] else 1)


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


# new-job.py's favourable list includes a bare \bcontract\b, which is fine when
# scanning one pasted ad but matches site navigation ("Contract Recruitment") on a
# scraped page. Only keep patterns specific enough to survive a full-page scrape.
def scan(text, blockers, favourable):
    # The shared signals module exports compiled patterns (re.I baked in) -- never pass
    # flags again. Blocker matching (negation, EEO boilerplate) lives in signals.blocker_hits.
    hits = [(lbl, "BLOCKER") for lbl in blocker_hits(text, blockers)]
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
        err, access = str(e), e.access
    except Exception as e:  # malformed payload: still a dead source, not zero jobs
        err, access = f"{type(e).__name__}: {e}", "error"
    return [], {"source": name, "target": target, "ok": False, "count": 0, "note": err,
                "access": access}


# ----------------------------------------------------------------------- main

def main(argv=None):
    global ACTIVE_TRACKS, ROLE_CATALOGUE
    workspace = workspace_from_cli(data_from_argv(argv))
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
    ap.add_argument("--test-source", metavar="NAME",
                    help="check one registry Source, print its health JSON, exit non-zero if it fails")
    ap.add_argument("--all-locations", action="store_true")
    add_data_argument(ap)
    args = ap.parse_args(argv)
    if args.test_source:
        test_source(workspace, args.test_source)
        return
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
    try:
        rules = search_rules.load_rules(workspace)
    except ValueError as error:
        sys.exit(f"workspace.json {error}")
    seen = seen_urls(workspace)
    seen_keys = seen_roles(workspace)
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

    coverage = rolecatalogue.coverage_rows(ROLE_CATALOGUE, tracks, health, args.source)
    by_role = {row["id"]: row for row in coverage}
    kept, skipped_seen, skipped_loc, skipped_track = [], 0, 0, 0
    filtered = []  # (job, Finding) a Search rule removed
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
        if ukey in seen or role_key(j) in seen_keys:
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
        findings = search_rules.evaluate(rules, source=j["source"], posted=j.get("posted", ""))
        rule = search_rules.deciding(findings)
        if rule:
            filtered.append((j, rule))
            continue
        j["borderline"] = [f.note for f in findings if not f.certain]
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
    lines = sweepreport.candidates_report(
        tracks=tracks, fetched=len(jobs), matched=len(kept), known=skipped_seen, health=health,
        rules=rules, filtered=filtered, coverage=coverage, clean=clean, blocked=blocked,
        manual=manual_sources(workspace), catalogued=ROLE_CATALOGUE is not None,
        search_area_note=NO_SEARCH_AREA if search_area is None else None,
    )

    out = Path(workspace) / "jobs" / "sweep-candidates.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text("\n".join(lines))
    print(f"\n  {len(jobs)} fetched · {len(clean)} to assess · {len(blocked)} blocked")
    print(f"  skipped: {skipped_seen} already known, {skipped_track} off-track, "
          f"{skipped_loc} wrong location")
    print(f"  sources: {len(health) - len(failed)} ok, {len(failed)} FAILED")
    if filtered:
        print(f"  filtered by Search rules: {len(filtered)}")
    for h in failed:
        print(f"    FAILED {h['source']} {h['target']}: {h['note']}")
    print(f"\nWrote {out}")
    print("Next: ask the agent to run the find-jobs skill, or assess these against the Profile.")
    if health and len(failed) == len(health):
        sys.exit(1)


if __name__ == "__main__":
    main()

