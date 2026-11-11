#!/usr/bin/env python3
"""Set only the ``fit`` key on one Application.

    python3 plugin/scripts/set-fit.py --data <workspace> \
        --folder <application-folder> --fit <Strong|Good|Marginal|Poor>

The write goes through ``meta.patch``, so re-reading, validation, atomic
replacement and the preservation of unknown keys and comments are the same as
every other writer. This script never appends a Status and never touches
``fit.md``, ``notes.md``, ``sent/`` or the Profile.

A relative ``--folder`` resolves against the Workspace, then ``jobs/``, and a
bare folder name resolves to the one track that holds it; an ambiguous name is
an error rather than a guess.
"""

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import meta as meta_store  # noqa: E402
from workspace import workspace_from_cli  # noqa: E402

FITS = ("Strong", "Good", "Marginal", "Poor")


class FolderError(Exception):
    """The folder argument does not name exactly one Application."""


def resolve_folder(workspace, value):
    """The Application folder named by ``value`` under ``workspace``."""
    raw = Path(value).expanduser()
    if raw.is_absolute():
        return raw
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
        raise FolderError(
            f"no Application folder {value!r} under {jobs}"
        )
    if len(unique) > 1:
        listed = "\n  ".join(str(path) for path in unique)
        raise FolderError(
            f"--folder {value!r} is ambiguous; name one of:\n  {listed}"
        )
    return unique[0]


def main(argv=None):
    workspace = workspace_from_cli()
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--data", help="Workspace path; or set ROLECALL_WORKSPACE")
    parser.add_argument("--folder", required=True, help="Application folder")
    parser.add_argument("--fit", required=True, choices=FITS)
    args = parser.parse_args(argv)

    try:
        folder = resolve_folder(workspace, args.folder)
        if not (folder / "meta.yml").is_file():
            raise FolderError(f"{folder} has no meta.yml, so it is not an Application")
        meta_store.patch(folder, {"fit": args.fit})
    except (FolderError, meta_store.MetaError, OSError) as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
