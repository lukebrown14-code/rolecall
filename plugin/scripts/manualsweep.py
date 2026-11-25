"""Record manual Source outcomes without rewriting Workspace settings."""

import json
from datetime import datetime
from pathlib import Path
from urllib.parse import quote, urlsplit
from uuid import uuid4

from workspaceio import atomic_write


def search_link(template, location):
    return template.replace("{location}", quote(location, safe=""))


def valid_template(template):
    if not isinstance(template, str) or "{location}" not in template:
        return False
    expanded = search_link(template, "location")
    if any(char.isspace() or char in "{}" for char in expanded):
        return False
    parsed = urlsplit(template)
    return (
        parsed.scheme in ("https", "http") and bool(parsed.hostname)
        and "{location}" not in parsed.netloc
        and ("{location}" in parsed.path or "{location}" in parsed.query)
    )


def record(workspace, entries, results):
    """Validate a run's outcomes and write one JSON file for the whole run."""
    names = {entry["name"].lower(): entry["name"] for entry in entries if entry["enabled"]}
    if not isinstance(results, list) or not results:
        raise ValueError("manual results must be a non-empty JSON array")
    checked = []
    for result in results:
        if not isinstance(result, dict):
            raise ValueError("each manual result must be an object")
        source = result.get("source")
        if not isinstance(source, str) or source.lower() not in names:
            raise ValueError(f"unknown or disabled manual Source: {source!r}")
        query = result.get("query")
        location = result.get("location", "")
        status = result.get("status")
        count = result.get("count")
        note = result.get("note", "")
        if not isinstance(query, str) or not query.strip():
            raise ValueError(f"{source}: query must be non-empty")
        if not isinstance(location, str) or not isinstance(note, str):
            raise ValueError(f"{source}: location and note must be strings")
        if status == "searched":
            if type(count) is not int or count < 0:
                raise ValueError(f"{source}: searched results need a non-negative count")
        elif status in ("blocked", "skipped"):
            if count is not None or not note.strip():
                raise ValueError(f"{source}: {status} results need a note and no count")
        else:
            raise ValueError(f"{source}: status must be searched, blocked or skipped")
        checked.append({"source": names[source.lower()], "query": query.strip(),
                        "location": location.strip(), "status": status, "count": count,
                        "note": note.strip()})

    now = datetime.now().astimezone()
    run = {"recorded_at": now.isoformat(), "date": now.date().isoformat(), "results": checked}
    directory = Path(workspace) / "jobs" / "manual-sweeps"
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    path = directory / f"{run['date']}-{uuid4().hex}.json"
    atomic_write(path, (json.dumps(run, ensure_ascii=False, indent=2) + "\n").encode(), 0o600)
    return path


def last_swept(workspace):
    """Most recent successful search date per Source, from saved runs."""
    dates = {}
    for path in (Path(workspace) / "jobs" / "manual-sweeps").glob("*.json"):
        try:
            run = json.loads(path.read_text())
            day = datetime.strptime(run["date"], "%Y-%m-%d").date().isoformat()
            searched = [result["source"].lower() for result in run["results"]
                        if result["status"] == "searched"]
        except (OSError, ValueError, TypeError, KeyError, AttributeError):
            continue  # a damaged log must not prevent a sweep
        for name in searched:
            dates[name] = max(day, dates.get(name, ""))
    return dates
