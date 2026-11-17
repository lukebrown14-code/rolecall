#!/usr/bin/env python3
"""List the unsent drafts that no longer match the current Profile.

    python3 plugin/scripts/drift.py --data <workspace> [--urls] [--json]

For every Application still a Lead or a Draft that has a resume.md or
cover-letter.md, this runs the build's own fact rules (``build.fact_errors``)
against the Profile as it is now. A draft that cites a fact since removed from
the Profile, or claims something the Profile now marks not available, is stale.
Run it after every Profile edit.

``--urls`` also checks that every http(s) URL in those documents resolves
(``urlcheck.py``): 404 and 410 are dead, a refusal such as 403 is only
unverified, and no network at all is a warning, not a failure.

Exit 0 when nothing is stale, 1 when anything is. Read-only: it writes nothing.
"""

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import build  # noqa: E402
import meta as meta_module  # noqa: E402
import urlcheck  # noqa: E402
from workspace import add_data_argument, workspace_from_cli  # noqa: E402

UNSENT = ("Lead", "Draft")
DOCUMENTS = (("resume.md", "resume"), ("cover-letter.md", "letter"))


def find_drift(workspace, check_urls=False, fetcher=None):
    """``(stale, warnings)``: stale is a list of ``{folder, document, reasons}``."""
    profile = build.Profile.read(workspace)
    entries = {}
    texts = {}
    for app in meta_module.scan(workspace):
        if app.malformed or app.status not in UNSENT:
            continue
        for name, kind in DOCUMENTS:
            path = Path(app.folder) / name
            text = path.read_text() if path.is_file() else ""
            if not text.strip():
                continue
            key = (str(app.folder), name)
            texts[key] = text
            entries[key] = list(build.fact_errors(text, kind, profile.master, profile.projects))

    warnings = []
    if check_urls:
        urls = {url for text in texts.values() for url in urlcheck.urls_in(text)}
        results = urlcheck.check(urls, **({"fetcher": fetcher} if fetcher else {}))
        if urlcheck.offline(results):
            warnings.append("no URL could be reached; the network looks down, links unchecked")
        warnings += [f"{u} -- unverified ({results[u][1]})" for u in urlcheck.unverified(results)]
        bad = set(urlcheck.failures(results))
        for key, text in texts.items():
            for url in urlcheck.urls_in(text):
                if url in bad:
                    entries[key].append(f"dead link: {results[url][1]}")

    stale = [
        {"folder": folder, "document": name, "reasons": reasons}
        for (folder, name), reasons in entries.items()
        if reasons
    ]
    return stale, warnings


def main(argv=None):
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    add_data_argument(parser)
    parser.add_argument("--urls", action="store_true", help="also check every URL resolves")
    parser.add_argument("--json", action="store_true", help="machine-readable output")
    args = parser.parse_args(argv)
    workspace = workspace_from_cli(args.data)
    stale, warnings = find_drift(workspace, check_urls=args.urls)

    if args.json:
        print(json.dumps({"stale": stale, "warnings": warnings}, indent=2))
    else:
        for entry in stale:
            print(f"{entry['folder']}\n  {entry['document']}")
            for reason in entry["reasons"]:
                print(f"    - {reason}")
        for warning in warnings:
            print(f"warning: {warning}")
        folders = len({entry["folder"] for entry in stale})
        if stale:
            print(f"{len(stale)} stale document(s) in {folders} Application(s).")
        else:
            print("No drift: every unsent draft matches the Profile.")
    return 1 if stale else 0


if __name__ == "__main__":
    sys.exit(main())
