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
from workspace import add_data_argument, workspace_from_cli  # noqa: E402

FITS = tuple(sorted(fit for fit in meta_store.FITS if fit))


def main(argv=None):
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    add_data_argument(parser)
    parser.add_argument("--folder", required=True, help="Application folder")
    parser.add_argument("--fit", required=True, choices=FITS)
    args = parser.parse_args(argv)
    workspace = workspace_from_cli(args.data)

    try:
        folder = meta_store.resolve_folder(workspace, args.folder)
        if not (folder / "meta.yml").is_file():
            raise meta_store.FolderError(
                f"{folder} has no meta.yml, so it is not an Application"
            )
        meta_store.patch(folder, {"fit": args.fit})
    except (meta_store.FolderError, meta_store.MetaError, OSError) as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
