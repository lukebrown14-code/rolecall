#!/usr/bin/env python3
"""Guard against personal data in tracked files, by Workspace structure first.

1. Workspace-shaped paths (workspace.json, config.toml, jobs/, profile/, cv/,
   sent/, meta.yml, PDFs, .rolecall-* files) must not be tracked outside the
   fixtures/ and examples/ folders.
2. Tracked files are scanned for generic patterns (emails, phone numbers,
   LinkedIn URLs, street addresses). Inside fixtures/ and examples/ every
   email must also be at a reserved fictional domain. Files that are not valid
   UTF-8 are decoded with replacement characters and still scanned. In a PDF,
   phone numbers are looked for only in its tel: links, because its other
   numbers (offsets, font metrics) read as phone numbers.
3. Optional local extra: Owner-specific terms, one per line, in
   .rolecall-personal-data-terms (gitignored). ROLECALL_PERSONAL_DATA_TERMS_FILE
   selects another file, and ROLECALL_PERSONAL_DATA_TERMS supplies additional
   newline-separated terms. Nothing needs to be set for the checks above.

--history also checks every file version and commit message on any branch,
tag or remote ref (paths by rule 1, text by rules 2 and 3). --github OWNER/REPO also scans
that repository's issue and pull request titles, bodies, comments and reviews,
fetched with the gh CLI.

By default the working-tree copies of tracked files are read (CI, a clean
checkout). --staged reads the index instead, so the pre-commit hook checks
exactly what the commit will hold even when the working copy differs.
"""

from __future__ import annotations

import argparse
import itertools
import json
import os
import re
import subprocess
import sys
from collections.abc import Callable, Iterable, Iterator
from pathlib import Path

PATTERNS = (
    (
        "email",
        re.compile(
            r"(?<![\w.+-])[\w.+-]+@"
            r"[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?"
            r"(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)*"
            # A top-level domain is two or more letters, so stray bytes such as
            # "4@C.D" in image data are not addresses.
            r"\.[A-Z]{2,63}(?![\w.-]|:)",
            re.IGNORECASE,
        ),
    ),
    (
        "phone",
        re.compile(
            r"(?<!\w)(?:\+\d{1,3}[ .()-]?"
            r"(?:\(?\d{2,4}\)?[ .()-]?){2,4}\d{2,4}"
            r"|0\d{1,4}[ .()-](?:\d{3,4}[ .()-])\d{3,4}"
            r"|\(?\d{3}\)?[ .()-]?\d{3}[ .-]?\d{4})(?!\w)"
        ),
    ),
    (
        "LinkedIn profile URL",
        re.compile(r"(?i)(?:https?://)?(?:www\.)?linkedin\.com/in/[A-Z0-9_%.-]+"),
    ),
    (
        "street address",
        re.compile(
            r"(?i)\b\d{1,6}\s+[A-Z0-9.'-]+(?:\s+[A-Z0-9.'-]+){0,4}\s+"
            r"(?:street|st\.?|road|rd\.?|avenue|ave\.?|drive|dr\.?|lane|ln\.?|"
            r"boulevard|blvd\.?|court|ct\.?|crescent|cres\.?|parade|pde\.?|"
            r"place|pl\.?)\b(?=\s*(?:,|$))"
        ),
    ),
)
# Workspace data may only be tracked under these prefixes (fictional data).
FICTIONAL_PREFIXES = ("fixtures/", "examples/")
FICTIONAL_EMAIL_DOMAINS = ("example.test", "example.com", "example.org", "example.net")
EMAIL_PATTERN = PATTERNS[0][1]
# A built PDF carries the Contact's phone as a tel: link annotation, which is not
# compressed. Every other digit run in a PDF (cross-reference offsets, font metrics,
# coordinates) reads as a phone number, so in a PDF the phone pattern checks only these.
PDF_TEL_LINK = re.compile(r"tel:([^)>\s]+)")
WORKSPACE_FILE_NAMES = {"workspace.json": "Workspace file", "config.toml": "Workspace config", "meta.yml": "Application meta file"}
WORKSPACE_DIR_NAMES = {
    "jobs": "Workspace jobs folder",
    "profile": "Workspace profile folder",
    "cv": "Workspace cv folder",
    "sent": "Workspace sent folder",
}
DEFAULT_TERMS_FILE = ".rolecall-personal-data-terms"
TERMS_FILE_ENV = "ROLECALL_PERSONAL_DATA_TERMS_FILE"
TERMS_ENV = "ROLECALL_PERSONAL_DATA_TERMS"
ALLOWLIST_PATH = "scripts/personal-data-allowlist.json"
SUBMODULE_MODE = "160000"


def git(root: Path, *args: str, stdin: bytes | None = None) -> bytes:
    return subprocess.run(
        ["git", "-C", str(root), *args],
        input=stdin,
        check=True,
        capture_output=True,
    ).stdout


def decode_path(name: bytes) -> str:
    return name.decode("utf-8", errors="surrogateescape")


def tracked_files(root: Path) -> list[str]:
    return [decode_path(name) for name in git(root, "ls-files", "-z").split(b"\0") if name]


def working_tree_contents(root: Path) -> Iterator[tuple[str, bytes | None]]:
    """Tracked paths with their working-tree bytes (None when unreadable)."""
    for relative in tracked_files(root):
        try:
            yield relative, (root / relative).read_bytes()
        except OSError:
            yield relative, None


def staged_contents(root: Path) -> Iterator[tuple[str, bytes | None]]:
    """Index paths with their staged blob bytes: what the next commit holds.

    A staged deletion is absent from the index and a staged rename appears
    under its new path. Submodule entries have no blob and yield None.
    """
    entries: list[tuple[str, str | None]] = []
    for record in git(root, "ls-files", "--stage", "-z").split(b"\0"):
        if not record:
            continue
        meta, name = record.split(b"\t", 1)
        mode, object_id, _stage = meta.decode("ascii").split()
        entries.append((decode_path(name), None if mode == SUBMODULE_MODE else object_id))
    object_ids = [object_id for _, object_id in entries if object_id]
    output = git(root, "cat-file", "--batch", stdin="".join(f"{oid}\n" for oid in object_ids).encode("ascii"))
    blobs: dict[str, bytes] = {}
    offset = 0
    for object_id in object_ids:
        header_end = output.index(b"\n", offset)
        header = output[offset:header_end].decode("ascii", errors="replace").split()
        if len(header) != 3 or header[1] != "blob":
            raise ValueError(f"cannot read staged object {object_id}: {' '.join(header)}")
        start = header_end + 1
        end = start + int(header[2])
        blobs[object_id] = output[start:end]
        offset = end + 1  # cat-file ends each object with a newline
    for relative, object_id in entries:
        yield relative, blobs[object_id] if object_id else None


def workspace_rule(relative: str) -> str | None:
    """Name the Workspace-shaped rule a tracked path breaks, or None."""
    if relative.startswith(FICTIONAL_PREFIXES):
        return None
    parts = relative.split("/")
    name = parts[-1]
    if name in WORKSPACE_FILE_NAMES:
        return WORKSPACE_FILE_NAMES[name]
    if name.startswith(".rolecall-"):
        return ".rolecall-* local file"
    if name.lower().endswith(".pdf"):
        return "built PDF"
    for directory in parts[:-1]:
        if directory in WORKSPACE_DIR_NAMES:
            return WORKSPACE_DIR_NAMES[directory]
    return None


def real_looking_email(match: str) -> bool:
    domain = match.rsplit("@", 1)[1].lower()
    return not any(domain == d or domain.endswith("." + d) for d in FICTIONAL_EMAIL_DOMAINS)


def load_allowlist(root: Path, staged: bool = False) -> set[str]:
    """Exact allowed values, read from the same place as the scanned files."""
    if staged:
        result = subprocess.run(
            ["git", "-C", str(root), "show", f":{ALLOWLIST_PATH}"],
            capture_output=True,
        )
        if result.returncode != 0:
            return set()  # not staged
        text = result.stdout.decode("utf-8", errors="replace")
    else:
        path = root / ALLOWLIST_PATH
        if not path.exists():
            return set()
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError) as error:
            raise ValueError(f"cannot read {ALLOWLIST_PATH}: {error}") from error
    try:
        values = json.loads(text)
    except json.JSONDecodeError as error:
        raise ValueError(f"cannot read {ALLOWLIST_PATH}: {error}") from error
    if not isinstance(values, list) or any(not isinstance(value, str) for value in values):
        raise ValueError(f"{ALLOWLIST_PATH} must be a JSON array of exact values")
    return set(values)


def load_owner_terms(root: Path) -> set[str]:
    configured_path = os.environ.get(TERMS_FILE_ENV, DEFAULT_TERMS_FILE)
    path = Path(configured_path)
    if not path.is_absolute():
        path = root / path
    terms: set[str] = set()
    if TERMS_FILE_ENV in os.environ and not path.exists():
        raise ValueError("configured Owner-specific terms file does not exist")
    if path.exists():
        try:
            terms.update(
                line.strip()
                for line in path.read_text(encoding="utf-8").splitlines()
                if line.strip() and not line.lstrip().startswith("#")
            )
        except (OSError, UnicodeDecodeError) as error:
            raise ValueError(f"cannot read Owner-specific terms file: {error}") from error
    terms.update(
        line.strip()
        for line in os.environ.get(TERMS_ENV, "").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    )
    return terms


def scan_text(
    label: str,
    relative: str | None,
    text: str,
    owner_terms: tuple[str, ...],
    allowlist: set[str],
) -> list[str]:
    """Check one text; relative is its repo path, or None for text with no path."""
    fictional_only = relative is not None and relative.startswith(FICTIONAL_PREFIXES)
    pdf = relative is not None and relative.lower().endswith(".pdf")
    findings: list[str] = []
    for line_number, line in enumerate(text.splitlines(), start=1):
        if any(term in line.casefold() for term in owner_terms):
            findings.append(f"{label}:{line_number}: Owner-specific term")
        if fictional_only and any(
            real_looking_email(m.group(0)) for m in EMAIL_PATTERN.finditer(line)
        ):
            findings.append(
                f"{label}:{line_number}: email outside a fictional domain "
                "(fixtures/ and examples/ allow only example.test, example.com, "
                "example.org, example.net)"
            )
        for name, pattern in PATTERNS:
            targets = PDF_TEL_LINK.findall(line) if pdf and name == "phone" else [line]
            matches = (m.group(0) for target in targets for m in pattern.finditer(target))
            if any(match not in allowlist for match in matches):
                if name == "email" and fictional_only:
                    continue  # fictional-domain rule above already covers emails here
                findings.append(f"{label}:{line_number}: {name}")
    return findings


# A source yields (label, repo path or None, text or None). Text None means the
# file is not readable text, so only its path is checked.
Source = Iterable[tuple[str, "str | None", "str | None"]]


def decoded(files: Iterator[tuple[str, bytes | None]]) -> Iterator[tuple[str, str | None, str | None]]:
    # Replacement decoding keeps ASCII personal data in latin-1 exports and
    # binaries visible to the patterns instead of skipping the file.
    for relative, data in files:
        text = data.decode("utf-8", errors="replace") if data is not None else None
        yield relative, relative, text


def tracked_texts(root: Path) -> Iterator[tuple[str, str | None, str | None]]:
    """Tracked files as they are in the working tree."""
    return decoded(working_tree_contents(root))


def staged_texts(root: Path) -> Iterator[tuple[str, str | None, str | None]]:
    """Index entries as the next commit will hold them."""
    return decoded(staged_contents(root))


# Refs that can be published; local refs such as refs/stash are left out.
REFS = ("--branches", "--tags", "--remotes")


def history_texts(root: Path) -> Iterator[tuple[str, str | None, str | None]]:
    """Yield every commit message and distinct file version on any branch, tag or remote ref."""
    log = git(root, "log", *REFS, "--format=%H%x00%B%x00")
    fields = log.split(b"\0")
    for sha, message in zip(fields[0::2], fields[1::2]):
        sha = sha.strip()
        if sha:
            label = f"commit {sha[:12].decode()} message"
            yield label, None, message.decode("utf-8", "replace")

    # rev-list names each blob once, at one of its paths, so every path ever
    # committed comes from the log instead and gets the path rules on its own.
    changed = git(root, "log", *REFS, "-m", "--no-renames", "--format=", "--name-only")
    for path in sorted(set(changed.decode("utf-8", "surrogateescape").splitlines())):
        if path:
            yield f"history:{path}", path, None

    blobs: dict[str, str] = {}
    objects = git(root, "rev-list", *REFS, "--objects").decode("utf-8", "surrogateescape")
    for entry in objects.splitlines():
        sha, _, path = entry.partition(" ")
        if path:
            blobs.setdefault(sha, path)
    if not blobs:
        return
    batch = git(
        root, "cat-file", "--batch", stdin="".join(f"{sha}\n" for sha in blobs).encode()
    )
    offset = 0
    while offset < len(batch):
        header_end = batch.index(b"\n", offset)
        sha, kind, size = batch[offset:header_end].decode().split()
        start = header_end + 1
        contents = batch[start : start + int(size)]
        offset = start + int(size) + 1
        if kind != "blob":
            continue
        try:
            text: str | None = contents.decode("utf-8")
        except UnicodeDecodeError:
            text = None
        yield f"{sha[:12]}:{blobs[sha]}", blobs[sha], text


def gh_json_lines(path: str, jq: str) -> list[dict]:
    output = subprocess.run(
        ["gh", "api", "--paginate", path, "--jq", jq],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return [json.loads(line) for line in output.splitlines() if line.strip()]


def github_texts(
    repo: str, fetch: Callable[[str, str], list[dict]] = gh_json_lines
) -> Iterator[tuple[str, str | None, str | None]]:
    """Yield the text of every issue and pull request in repo, with its comments and reviews."""
    pulls: list[int] = []
    items = fetch(
        f"repos/{repo}/issues?state=all&per_page=100",
        ".[] | {number, title, body, pr: (.pull_request != null)}",
    )
    for item in items:
        kind = "pr" if item["pr"] else "issue"
        if item["pr"]:
            pulls.append(item["number"])
        yield f"{kind} #{item['number']} title", None, item["title"] or ""
        yield f"{kind} #{item['number']} body", None, item["body"] or ""
    for comment in fetch(
        f"repos/{repo}/issues/comments?per_page=100", ".[] | {id, issue_url, body}"
    ):
        number = comment["issue_url"].rsplit("/", 1)[-1]
        yield f"#{number} comment {comment['id']}", None, comment["body"] or ""
    for comment in fetch(
        f"repos/{repo}/pulls/comments?per_page=100", ".[] | {id, pull_request_url, body}"
    ):
        number = comment["pull_request_url"].rsplit("/", 1)[-1]
        yield f"pr #{number} review comment {comment['id']}", None, comment["body"] or ""
    for number in pulls:
        for review in fetch(
            f"repos/{repo}/pulls/{number}/reviews?per_page=100", ".[] | {id, body}"
        ):
            yield f"pr #{number} review {review['id']}", None, review["body"] or ""


def scan(
    root: Path,
    terms: set[str] | None = None,
    sources: Source | None = None,
    staged: bool = False,
) -> list[str]:
    """Report personal data in sources (default: tracked files, or the index if staged).

    staged also reads the allowlist from the index.
    """
    allowlist = load_allowlist(root, staged)
    if sources is None:
        sources = staged_texts(root) if staged else tracked_texts(root)
    configured_terms = terms if terms is not None else load_owner_terms(root)
    owner_terms = tuple(term.casefold() for term in configured_terms)
    findings: list[str] = []
    for label, relative, text in sources:
        rule = workspace_rule(relative) if relative is not None else None
        if rule:
            findings.append(
                f"{label}: {rule} is tracked outside fixtures/ and examples/ "
                "(Workspace data stays in the Workspace)"
            )
            continue
        if text is not None:
            findings.extend(scan_text(label, relative, text, owner_terms, allowlist))
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("root", type=Path, help="repository root to scan")
    parser.add_argument(
        "--history",
        action="store_true",
        help="also check every file version and commit message on any branch, tag or remote",
    )
    parser.add_argument(
        "--github",
        metavar="OWNER/REPO",
        help="also scan the repository's issues, pull requests, comments and reviews (needs gh)",
    )
    parser.add_argument(
        "--staged",
        action="store_true",
        help="scan the index (what the next commit holds) instead of the working tree",
    )
    args = parser.parse_args()
    root = args.root.resolve()
    try:
        terms = load_owner_terms(root)
        sources: Source = staged_texts(root) if args.staged else tracked_texts(root)
        if args.history:
            sources = itertools.chain(sources, history_texts(root))
        if args.github:
            sources = itertools.chain(sources, github_texts(args.github))
        findings = scan(root, terms, sources, staged=args.staged)
        if not terms:
            print(
                "No local Owner-specific terms; structural and generic checks only.",
                file=sys.stderr,
            )
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        return 2
    if findings:
        print("\n".join(findings))
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
