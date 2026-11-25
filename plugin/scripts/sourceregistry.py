"""The Source registry (workspace.json ``sources``, SCHEMA.md), checked.

Both forms: the registry (a list of entries) and the older platform name -> target
list object. ``platforms`` is the Sweep's platform registry; only its names are read.
"""

from datetime import date

from manualsweep import valid_template

ACCESS = ("works", "403", "login wall", "cloudflare", "unknown")
SOURCE_TYPES = ("api", "browser", "manual")


def validate_registry(entries, platforms, label="workspace.json sources"):
    """The Source registry entries, checked; raises ValueError naming the bad entry."""
    out, names = [], set()
    for index, entry in enumerate(entries):
        where = f"{label}[{index}]"
        if not isinstance(entry, dict) or not isinstance(entry.get("name"), str) or not entry["name"].strip():
            raise ValueError(f"{where}: each Source wants a name")
        name = entry["name"].strip()
        where = f"{label} {name!r}"
        if name.lower() in names:
            raise ValueError(f"{where}: listed twice")
        names.add(name.lower())
        kind = entry.get("type")
        if kind not in SOURCE_TYPES:
            raise ValueError(f"{where}: type must be one of {', '.join(SOURCE_TYPES)}")
        access = entry.get("access", "unknown")
        if access not in ACCESS:
            raise ValueError(f"{where}: access must be one of {', '.join(ACCESS)}")
        enabled = entry.get("enabled", True)
        if not isinstance(enabled, bool):
            raise ValueError(f"{where}: enabled must be true or false")
        item = {**entry, "name": name, "type": kind, "access": access, "enabled": enabled}
        if "last_swept" in entry:
            value = entry["last_swept"]
            try:
                if not isinstance(value, str) or date.fromisoformat(value).isoformat() != value:
                    raise ValueError
            except ValueError:
                raise ValueError(f"{where}: last_swept must be a YYYY-MM-DD date") from None
        if "search_url" in entry:
            try:
                valid = valid_template(entry["search_url"])
            except ValueError:
                valid = False
            if not valid:
                raise ValueError(f"{where}: search_url must be an HTTP(S) URL with {{location}} in its path or query")
        if kind == "api":
            platform = entry.get("platform")
            if platform not in platforms:
                raise ValueError(f"{where}: api Sources need a platform -- registered: {', '.join(platforms)}")
            targets = entry.get("targets")
            if not isinstance(targets, list) or not all(isinstance(t, str) and t.strip() for t in targets):
                raise ValueError(f"{where}: wants a list of targets")
            item["targets"] = targets
        out.append(item)
    return out


def validate_sources(cfg, label, platforms):
    """The legacy ``sources`` object as ``[(platform, targets)]``; raises ValueError."""
    if not isinstance(cfg, dict):
        raise ValueError(f"{label}: want an object mapping platform name to a target list")
    unknown = [k for k in cfg if k not in platforms]
    if unknown:
        raise ValueError(f"{label}: unknown platform(s) {', '.join(unknown)} -- "
                         f"registered: {', '.join(platforms)}")
    out = []
    for name, targets in cfg.items():
        if not isinstance(targets, list) or not all(isinstance(t, str) and t.strip() for t in targets):
            raise ValueError(f"{label}: {name} wants a list of targets")
        out.append((name, targets))
    return out
