#!/usr/bin/env python3
"""Read Applications from a Workspace, mirroring the Go store.

The read side of SCHEMA.md: no YAML library, stock Python 3. A folder is an
Application if and only if it holds ``meta.yml``. A malformed file is returned
as an Application carrying the error, never as an exception that stops the scan.

Usage:  python3 meta.py [--data <workspace>]
"""

# The CLI, and the one import every other script uses for meta.yml. The work lives in
# modules beside it: metayaml (the line grammar), metaread (reading Applications),
# metawrite (the validating writers) and followup (Follow-up due). This module
# re-exports their API, so ``import meta`` stays the single entry point.

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import metawrite  # noqa: E402, F401 -- meta.metawrite is the module that publishes meta.yml
from followup import (  # noqa: E402
    DEFAULT_FOLLOW_UP_DAYS,
    FOLLOW_UP_STATUSES,
    UNDO_WINDOW,
    effective_log,
    follow_up_from,
    follow_up_status,
)
from metaread import (  # noqa: E402
    FACTS,
    FILTER_REASONS,
    FITS,
    STATUSES,
    Application,
    StatusEntry,
    fact_value,
    read_meta,
    rescan,
    scan,
)
from metawrite import (  # noqa: E402
    CREATE_KEYS,
    append_status,
    create,
    create_migrated,
    dedupe_status_log,
    dismiss_by_rule,
    duplicate_status_entries,
    patch,
    set_status_log,
    update,
)
from metayaml import (  # noqa: E402
    DATE,
    RFC3339,
    MetaError,
    decode_quoted,
    parse_moment,
    valid_date,
    valid_timestamp,
)
from workspace import add_data_argument, workspace_from_cli  # noqa: E402
from workspaceio import atomic_write, quote_scalar, unique_folder  # noqa: E402

__all__ = [
    "CREATE_KEYS", "DATE", "DEFAULT_FOLLOW_UP_DAYS", "FACTS", "FILTER_REASONS", "FITS",
    "FOLLOW_UP_STATUSES", "RFC3339", "STATUSES", "UNDO_WINDOW", "Application", "FolderError",
    "MetaError", "StatusEntry", "append_status", "atomic_write", "create", "create_migrated",
    "decode_quoted", "dedupe_status_log", "dismiss_by_rule", "duplicate_status_entries",
    "duplicates_of", "effective_log", "fact_value", "find_by_url", "follow_up_from",
    "follow_up_status", "parse_moment", "patch", "quote_scalar", "read_meta",
    "rescan", "resolve_folder", "rule_findings", "scan", "set_status_log", "unique_folder",
    "update", "valid_date", "valid_timestamp",
]

# The name callers used before parse_moment moved to metayaml.
_parse_moment = parse_moment


class FolderError(Exception):
    """A folder argument does not name exactly one Application."""


def rule_findings(workspace, fields, today=None):
    """The Search rules a new Lead with these fields breaks (rules.py), as Findings."""
    import rules  # noqa: PLC0415 -- rules imports meta lazily; keep the cycle out of import time

    applicants = fields.get("applicants")
    found = rules.evaluate(
        rules.load_rules(workspace),
        source=fields.get("source", ""),
        posted=fields.get("posted", ""),
        posted_label=fields.get("posted_label", ""),
        applicants=int(applicants) if str(applicants or "").isdigit() else None,
        today=today,
    )
    return [f for f in found if not f.advisory]


def duplicates_of(workspace, fields, jd_text=""):
    """Saved Applications that look like the role in ``fields`` (adfacts.likely_duplicates)."""
    import adfacts  # noqa: PLC0415

    return adfacts.likely_duplicates(
        scan(workspace), fields.get("company", ""), fields.get("title", ""), jd_text,
        fields.get("recruiter", ""),
    )


def resolve_folder(workspace, value):
    """The Application folder named by ``value`` under ``workspace``.

    An absolute path is taken as given. A relative path resolves against the
    Workspace, then ``jobs/``; a bare folder name also resolves to the one
    track that holds it. An ambiguous name is refused rather than guessed.
    """
    raw = Path(value).expanduser()
    if raw.is_absolute():
        return raw
    workspace = Path(workspace)
    jobs = workspace / "jobs"
    candidates = []
    for base in (workspace, jobs):
        candidate = base / raw
        if candidate.is_dir():
            candidates.append(candidate.resolve())
    if len(raw.parts) == 1 and jobs.is_dir():
        for track in sorted(jobs.iterdir()):
            candidate = track / raw
            if track.is_dir() and candidate.is_dir():
                candidates.append(candidate.resolve())
    unique = sorted(set(candidates))
    if not unique:
        raise FolderError(f"no Application folder {value!r} under {jobs}")
    if len(unique) > 1:
        listed = "\n  ".join(str(path) for path in unique)
        raise FolderError(f"--folder {value!r} is ambiguous; name one of:\n  {listed}")
    return unique[0]


def find_by_url(workspace, url):
    """Every exact URL match, regardless of Status."""
    return [application for application in scan(workspace) if application.url == url]


def _parser():
    parser = argparse.ArgumentParser(description=__doc__)
    add_data_argument(parser)
    commands = parser.add_subparsers(dest="command")

    patch_parser = commands.add_parser("patch", help="set scalar keys")
    patch_parser.add_argument("--folder", required=True)
    patch_parser.add_argument("--set", action="append", default=[], metavar="KEY=VALUE")

    append_parser = commands.add_parser("append-status", help="append a Status entry")
    append_parser.add_argument("--folder", required=True)
    append_parser.add_argument("--status", required=True)
    append_parser.add_argument("--at", help="RFC 3339 or YYYY-MM-DD (default: now)")
    append_parser.add_argument(
        "--only-if-current",
        help="append only when the Status read at write time is this value",
    )

    create_parser = commands.add_parser("create", help="create a Lead Application")
    create_parser.add_argument("--track", required=True)
    create_parser.add_argument("--title", required=True)
    create_parser.add_argument("--company", required=True)
    create_parser.add_argument("--url", default="")
    create_parser.add_argument("--closes", default="")
    create_parser.add_argument("--fit", default="")
    create_parser.add_argument("--at", help="RFC 3339 or YYYY-MM-DD (default: now)")
    create_parser.add_argument("--posted", default="", help="YYYY-MM-DD the ad was posted")
    create_parser.add_argument(
        "--posted-label", default="", help='the age as the site shows it, e.g. "14d ago"'
    )
    create_parser.add_argument("--applicants", default="", help="applicant count the site shows")
    create_parser.add_argument("--source", default="", help="Source registry name, e.g. linkedin")
    create_parser.add_argument(
        "--override-rules",
        action="store_true",
        help="save even though a Search rule rules it out (only when the Owner asks)",
    )

    for key in FACTS:
        create_parser.add_argument("--" + key.replace("_", "-"), dest="fact_" + key, default="")
    create_parser.add_argument(
        "--allow-duplicate",
        action="store_true",
        help="save even though a saved Application looks like the same role",
    )
    create_parser.add_argument("--jd-file", help="ad text, checked for a duplicate ad")

    show_parser = commands.add_parser("show", help="print one Application as JSON")
    show_parser.add_argument("--folder", required=True)

    follow_parser = commands.add_parser(
        "follow-up", help="print when one Application is Follow-up due, as JSON"
    )
    follow_parser.add_argument("--folder", required=True)
    follow_parser.add_argument(
        "--days",
        type=int,
        default=DEFAULT_FOLLOW_UP_DAYS,
        help=f"the Owner's follow_up_days (default {DEFAULT_FOLLOW_UP_DAYS})",
    )

    dup_parser = commands.add_parser("find-duplicates", help="saved Applications like this role")
    dup_parser.add_argument("--company", required=True)
    dup_parser.add_argument("--title", required=True)
    dup_parser.add_argument("--recruiter", default="")
    dup_parser.add_argument("--jd-file")

    dismiss_parser = commands.add_parser(
        "dismiss-by-rule", help="dismiss an Application a Search rule rules out"
    )
    dismiss_parser.add_argument("--folder", required=True)
    dismiss_parser.add_argument("--reason", required=True, choices=sorted(FILTER_REASONS))
    dismiss_parser.add_argument("--note", default="")

    find_parser = commands.add_parser("find-url", help="find Applications by URL")
    find_parser.add_argument("url")
    return parser


def main(argv=None):
    args = _parser().parse_args(argv)
    workspace = workspace_from_cli(args.data)
    try:
        if args.command is None:
            json.dump(
                [app.to_dict() for app in scan(workspace)],
                sys.stdout,
                ensure_ascii=False,
                indent=2,
            )
            print()
        elif args.command == "patch":
            keys = {}
            for setting in args.set:
                name, separator, value = setting.partition("=")
                if not separator:
                    raise MetaError("--set needs KEY=VALUE")
                keys[name] = value
            patch(args.folder, keys)
        elif args.command == "append-status":
            append_status(
                args.folder,
                args.status,
                args.at or datetime.now(timezone.utc),
                only_if_current=args.only_if_current,
            )
        elif args.command == "create":
            fields = {
                "title": args.title,
                "company": args.company,
                "track": args.track,
                "url": args.url,
                "closes": args.closes,
                "fit": args.fit,
                "posted": args.posted,
                "posted_label": args.posted_label,
                "applicants": args.applicants,
                "source": args.source,
                **{key: getattr(args, "fact_" + key) for key in FACTS},
            }
            for key in FACTS:
                if fields[key]:
                    fact_value(key, fields[key])  # refuse a bad value before any write
            if fields["closes"] and fields["closes_unclear"] == "true":
                raise MetaError("give --closes or --closes-unclear true, not both")
            if not args.allow_duplicate:
                jd_text = Path(args.jd_file).read_text() if args.jd_file else ""
                dupes = duplicates_of(workspace, fields, jd_text)
                if dupes:
                    listed = "\n  ".join(f"{app.folder} ({why})" for app, why in dupes)
                    raise MetaError(
                        "not saved, it looks like a saved Application:\n  " + listed +
                        "\nUse that one, or pass --allow-duplicate if it is a different role"
                    )
            if args.posted and not valid_date(args.posted):
                raise MetaError(f"--posted wants YYYY-MM-DD, not {args.posted!r}")
            if args.applicants and not args.applicants.isdigit():
                raise MetaError(f"--applicants wants a whole number, not {args.applicants!r}")
            if not args.override_rules:
                findings = rule_findings(workspace, fields)
                certain = [f for f in findings if f.certain]
                if certain:
                    why = "; ".join(f"{f.reason}: {f.note}" for f in certain)
                    raise MetaError(
                        f"not saved, a Search rule rules it out ({why}). Leave it out, or pass "
                        "--override-rules if the Owner asked for it anyway"
                    )
                for f in findings:
                    print(f"rolecall: borderline, ask the Owner: {f.note}", file=sys.stderr)
            folder = create(
                workspace, fields, parse_moment(args.at) if args.at else None
            )
            print(folder)
        elif args.command == "show":
            app = rescan(resolve_folder(workspace, args.folder))
            if app is None:
                raise MetaError(f"no meta.yml in {args.folder}")
            json.dump(app.to_dict(), sys.stdout, ensure_ascii=False, indent=2)
            print()
        elif args.command == "follow-up":
            if not 1 <= args.days <= 90:
                raise MetaError("--days wants follow_up_days, a whole number from 1 to 90")
            app = rescan(resolve_folder(workspace, args.folder))
            if app is None:
                raise MetaError(f"no meta.yml in {args.folder}")
            if app.malformed:
                raise MetaError(app.malformed)
            json.dump(follow_up_status(app, days=args.days), sys.stdout, indent=2)
            print()
        elif args.command == "find-duplicates":
            jd_text = Path(args.jd_file).read_text() if args.jd_file else ""
            found = duplicates_of(
                workspace,
                {"company": args.company, "title": args.title, "recruiter": args.recruiter},
                jd_text,
            )
            json.dump([{"Folder": str(app.folder), "Why": why} for app, why in found],
                      sys.stdout, indent=2)
            print()
        elif args.command == "dismiss-by-rule":
            folder = resolve_folder(workspace, args.folder)
            if not dismiss_by_rule(folder, args.reason, args.note):
                print("rolecall: left as it is (already Dismissed, or the Owner overruled the rules)")
        elif args.command == "find-url":
            json.dump(
                [app.to_dict() for app in find_by_url(workspace, args.url)],
                sys.stdout,
                ensure_ascii=False,
                indent=2,
            )
            print()
    except (MetaError, FolderError, ValueError, OSError) as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
