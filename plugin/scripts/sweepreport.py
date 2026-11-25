"""The Sweep's report, jobs/sweep-candidates.md, as Markdown lines.

Source health, what the Search rules removed, role coverage, the roles to assess,
the blocked ones and the Sources left to search by hand.
"""

from datetime import date, datetime, timezone

import rules as search_rules


def match_lines(matches):
    return [f"- match: `{m['id']}` · {m['title']} · {m['track']} · {m['priority']} · "
            f"title contains `{m['term']}`" for m in matches]


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


def health_lines(health):
    failed = [h for h in health if not h["ok"]]
    lines = ["## Source health", ""]
    if failed:
        lines += [f"**{len(failed)} of {len(health)} sources FAILED** — their jobs are missing from this "
                  "sweep, not absent from the market.", ""]
    lines += ["| Source | Target | Status | Jobs | Note |", "| :-- | :-- | :-- | --: | :-- |"]
    for h in health:
        status = "ok" if h["ok"] else "**FAILED**"
        if not h["ok"] and h.get("access") not in (None, "error"):
            status = f"**FAILED** ({h['access']})"
        if h["ok"] and h["count"] == 0:
            status = "ok (0 posted)"
        lines.append(f"| {h['source']} | {h['target'].replace('|', '/')} | {status} | {h['count']} | "
                     f"{h['note'].replace('|', '/')} |")
    return lines + [""]


def rule_lines(rules, filtered):
    """How many roles each Search rule removed, and which, so nothing vanishes unseen."""
    lines = ["## Filtered by Search rules", ""]
    active = []
    if rules.max_posting_age_days:
        active.append(f"posting age under {rules.max_posting_age_days} days")
    for name, days in sorted(rules.source_age.items()):
        active.append(f"{name}: posting age under {days} days")
    for name, cap in sorted(rules.max_applicants.items()):
        active.append(f"{name}: at most {cap} applicants (checked by hand; APIs give no count)")
    if not active:
        return lines + ["No Search rules set (workspace.json `search_rules`).", ""]
    lines += ["Rules: " + "; ".join(active) + ".", ""]
    counts = {}
    for _, finding in filtered:
        counts[finding.reason] = counts.get(finding.reason, 0) + 1
    lines += ["| Rule | Removed |", "| :-- | --: |"]
    for reason in search_rules.REASONS:
        if reason in counts:
            lines.append(f"| {reason} | {counts[reason]} |")
    if not counts:
        lines.append("| (none) | 0 |")
    lines.append("")
    for job, finding in filtered:
        lines.append(f"- {job['title']} — {job['company']} — {finding.reason}: {finding.note}")
    return lines + ([""] if filtered else [])


def manual_lines(entries, rules):
    """The Sources the skill must search by hand, with what it needs to read them."""
    lines = ["## Not reachable by this script", ""]
    for entry in entries:
        if not entry["enabled"]:
            lines.append(f"- ~~{entry['name']}~~ (disabled)")
            continue
        bits = [entry["type"], f"access: {entry['access']}"]
        if entry.get("url"):
            bits.append(entry["url"])
        lines.append(f"- **{entry['name']}** — " + " · ".join(bits))
        if entry.get("last_swept"):
            lines.append(f"  - last swept: {entry['last_swept']}")
        for place, link in entry.get("search_links", []):
            lines.append(f"  - {place}: {link}")
        if entry.get("search_url") and not entry.get("search_links"):
            lines.append(f"  - location search template: {entry['search_url']}")
        if entry.get("age_format"):
            lines.append(f"  - posting age reads like: {entry['age_format']}")
        if entry.get("applicants_format"):
            lines.append(f"  - applicant count reads like: {entry['applicants_format']}")
        cap = rules.applicant_cap(entry["name"])
        limit = rules.age_limit(entry["name"])
        own = [f"at most {cap} applicants"] if cap is not None else []
        own += [f"posted under {limit} days ago"] if limit else []
        if own:
            lines.append("  - rules: " + ", ".join(own))
        if entry.get("notes"):
            lines.append(f"  - {entry['notes']}")
    lines += ["- Sources without a public API need the skill's WebFetch path. Record every one",
              "  searched, and why any could not be (403, login wall, Cloudflare), in the hand-back.",
              ""]
    return lines


def candidates_report(*, tracks, fetched, matched, known, health, rules, filtered, coverage,
                      clean, blocked, manual, catalogued, search_area_note):
    """jobs/sweep-candidates.md as lines.

    ``clean`` and ``blocked`` are the kept roles, ``manual`` the Sources searched by hand,
    ``catalogued`` whether jobs/roles.json exists, and ``search_area_note`` the reason no
    Search area applied (None when one did)."""
    failed = [h for h in health if not h["ok"]]
    swept_at = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    lines = [
        "# Sweep candidates",
        "",
        "Raw output of `plugin/scripts/sweep.py`. **Not a shortlist** — nothing here has been judged",
        "against the Profile yet. The `find-jobs` skill reads each ad and records only what is worth",
        "applying for as a Lead Application (`jobs/<track>/<slug>/meta.yml` + `jd.md`).",
        "",
        *(["Search catalogue: [Master role list](roles.json).", ""]
          if catalogued else []),
        f"Swept {date.today().isoformat()} · tracks {', '.join(tracks)} · "
        f"{fetched} fetched, {matched} matched, {known} already known"
        + (f", {len(filtered)} filtered by rules" if filtered else "")
        + (f" · **{len(failed)} source(s) FAILED**" if failed else ""),
        "",
        f"Scanned at {swept_at}.",
        "",
        *([f"Search area: {search_area_note}.", ""] if search_area_note is not None else []),
        *health_lines(health),
        *rule_lines(rules, filtered),
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
            *([f"- posted: {j['posted']}"] if j.get("posted") else []),
            *[f"- **borderline age, ask the Owner:** {note}" for note in j.get("borderline", [])],
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

    lines += manual_lines(manual, rules)
    return lines
