#!/usr/bin/env python3
"""Scan tracked files for generic and configured personal-data terms.

Owner-specific terms are one per line in .rolecall-personal-data-terms by
default. ROLECALL_PERSONAL_DATA_TERMS_FILE selects another file, and
ROLECALL_PERSONAL_DATA_TERMS supplies additional newline-separated terms.

--history also scans every file version and commit message reachable from any
ref. --github OWNER/REPO also scans that repository's issue and pull request
titles, bodies, comments and reviews, fetched with the gh CLI.
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
    label: str, text: str, owner_terms: tuple[str, ...], allowlist: set[str]
) -> list[str]:
    findings: list[str] = []
    for line_number, line in enumerate(text.splitlines(), start=1):
        if any(term in line.casefold() for term in owner_terms):
            findings.append(f"{label}:{line_number}: Owner-specific term")
        for name, pattern in PATTERNS:
            if any(match.group(0) not in allowlist for match in pattern.finditer(line)):
                findings.append(f"{label}:{line_number}: {name}")
    return findings


def tracked_texts(root: Path) -> Iterator[tuple[str, str]]:
    for relative in tracked_files(root):
        try:
            yield relative, (root / relative).read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue


def git(root: Path, *args: str, stdin: bytes | None = None) -> bytes:
    return subprocess.run(
        ["git", "-C", str(root), *args], check=True, capture_output=True, input=stdin
    ).stdout


def history_texts(root: Path) -> Iterator[tuple[str, str]]:
    """Yield every commit message and every distinct file version reachable from any ref."""
    log = git(root, "log", "--all", "--format=%H%x00%B%x00")
    fields = log.split(b"\0")
    for sha, message in zip(fields[0::2], fields[1::2]):
        sha = sha.strip()
        if sha:
            yield f"commit {sha[:12].decode()} message", message.decode("utf-8", "replace")

    blobs: dict[str, str] = {}
    objects = git(root, "rev-list", "--all", "--objects").decode("utf-8", "surrogateescape")
    for entry in objects.splitlines():
        sha, _, path = entry.partition(" ")
        if path and sha not in blobs:
            blobs[sha] = path
    if not blobs:
        return
    batch = git(
        root,
        "cat-file",
        "--batch",
        stdin="".join(f"{sha}\n" for sha in blobs).encode(),
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
            text = contents.decode("utf-8")
        except UnicodeDecodeError:
            continue
        yield f"{sha[:12]}:{blobs[sha]}", text


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
) -> Iterator[tuple[str, str]]:
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
        yield f"{kind} #{item['number']} title", item["title"] or ""
        yield f"{kind} #{item['number']} body", item["body"] or ""
    for comment in fetch(
        f"repos/{repo}/issues/comments?per_page=100", ".[] | {id, issue_url, body}"
    ):
        number = comment["issue_url"].rsplit("/", 1)[-1]
        yield f"#{number} comment {comment['id']}", comment["body"] or ""
    for comment in fetch(
        f"repos/{repo}/pulls/comments?per_page=100", ".[] | {id, pull_request_url, body}"
    ):
        number = comment["pull_request_url"].rsplit("/", 1)[-1]
        yield f"pr #{number} review comment {comment['id']}", comment["body"] or ""
    for number in pulls:
        for review in fetch(
            f"repos/{repo}/pulls/{number}/reviews?per_page=100", ".[] | {id, body}"
        ):
            yield f"pr #{number} review {review['id']}", review["body"] or ""


def scan(
    root: Path,
    terms: set[str] | None = None,
    sources: Iterable[tuple[str, str]] | None = None,
) -> list[str]:
    allowlist = load_allowlist(root)
    configured_terms = terms if terms is not None else load_owner_terms(root)
    owner_terms = tuple(term.casefold() for term in configured_terms)
    findings: list[str] = []
    for label, text in sources if sources is not None else tracked_texts(root):
        findings.extend(scan_text(label, text, owner_terms, allowlist))
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("root", type=Path, help="repository root to scan")
    parser.add_argument(
        "--history",
        action="store_true",
        help="also scan every file version and commit message reachable from any ref",
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
        sources: Iterable[tuple[str, str]] = tracked_texts(root)
        if args.history:
            sources = itertools.chain(sources, history_texts(root))
        if args.github:
            sources = itertools.chain(sources, github_texts(args.github))
        findings = scan(root, terms, sources)
        if not terms:
            print(
                "Owner-specific terms unavailable; scanning generic patterns only.",
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
