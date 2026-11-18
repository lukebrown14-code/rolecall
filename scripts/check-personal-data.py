#!/usr/bin/env python3
"""Guard against personal data in tracked files, by Workspace structure first.

1. Workspace-shaped paths (workspace.json, config.toml, jobs/, profile/, cv/,
   sent/, meta.yml, PDFs, .rolecall-* files) must not be tracked outside the
   fixtures/ and examples/ folders.
2. Tracked text files are scanned for generic patterns (emails, phone numbers,
   LinkedIn URLs, street addresses). Inside fixtures/ and examples/ every
   email must also be at a reserved fictional domain.
3. Optional local extra: Owner-specific terms, one per line, in
   .rolecall-personal-data-terms (gitignored). ROLECALL_PERSONAL_DATA_TERMS_FILE
   selects another file, and ROLECALL_PERSONAL_DATA_TERMS supplies additional
   newline-separated terms. Nothing needs to be set for the checks above.

--history also checks every file version and commit message on any branch,
tag or remote ref (paths by rule 1, text by rules 2 and 3). --github OWNER/REPO also scans
that repository's issue and pull request titles, bodies, comments and reviews,
fetched with the gh CLI.
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
            r"(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+(?![\w.-]|:)",
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


def tracked_files(root: Path) -> list[str]:
    result = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        check=True,
        capture_output=True,
    )
    return [name.decode("utf-8", errors="surrogateescape") for name in result.stdout.split(b"\0") if name]


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


def load_allowlist(root: Path) -> set[str]:
    path = root / "scripts" / "personal-data-allowlist.json"
    if not path.exists():
        return set()
    try:
        values = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise ValueError(f"cannot read {path.relative_to(root)}: {error}") from error
    if not isinstance(values, list) or any(not isinstance(value, str) for value in values):
        raise ValueError("scripts/personal-data-allowlist.json must be a JSON array of exact values")
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
            if any(match.group(0) not in allowlist for match in pattern.finditer(line)):
                if name == "email" and fictional_only:
                    continue  # fictional-domain rule above already covers emails here
                findings.append(f"{label}:{line_number}: {name}")
    return findings


# A source yields (label, repo path or None, text or None). Text None means the
# file is not readable text, so only its path is checked.
Source = Iterable[tuple[str, "str | None", "str | None"]]


def tracked_texts(root: Path) -> Iterator[tuple[str, str | None, str | None]]:
    for relative in tracked_files(root):
        try:
            text: str | None = (root / relative).read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            text = None
        yield relative, relative, text


# Refs that can be published; local refs such as refs/stash are left out.
REFS = ("--branches", "--tags", "--remotes")


def git(root: Path, *args: str, stdin: bytes | None = None) -> bytes:
    return subprocess.run(
        ["git", "-C", str(root), *args], check=True, capture_output=True, input=stdin
    ).stdout


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


def scan(root: Path, terms: set[str] | None = None, sources: Source | None = None) -> list[str]:
    allowlist = load_allowlist(root)
    configured_terms = terms if terms is not None else load_owner_terms(root)
    owner_terms = tuple(term.casefold() for term in configured_terms)
    findings: list[str] = []
    for label, relative, text in sources if sources is not None else tracked_texts(root):
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
    parser = argparse.ArgumentParser(description=__doc__)
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
    args = parser.parse_args()
    root = args.root.resolve()
    try:
        terms = load_owner_terms(root)
        sources: Source = tracked_texts(root)
        if args.history:
            sources = itertools.chain(sources, history_texts(root))
        if args.github:
            sources = itertools.chain(sources, github_texts(args.github))
        findings = scan(root, terms, sources)
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
