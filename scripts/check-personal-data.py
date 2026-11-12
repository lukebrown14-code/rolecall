#!/usr/bin/env python3
"""Scan tracked files for generic and configured personal-data terms.

Owner-specific terms are one per line in .rolecall-personal-data-terms by
default. ROLECALL_PERSONAL_DATA_TERMS_FILE selects another file, and
ROLECALL_PERSONAL_DATA_TERMS supplies additional newline-separated terms.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
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
            r"|0\d{1,4}[ .()-](?:\d{3,4}[ .()-])\d{3,4})(?!\w)"
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


def scan(root: Path, terms: set[str] | None = None) -> list[str]:
    allowlist = load_allowlist(root)
    configured_terms = terms if terms is not None else load_owner_terms(root)
    owner_terms = tuple(term.casefold() for term in configured_terms)
    findings: list[str] = []
    for relative in tracked_files(root):
        path = root / relative
        try:
            contents = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        for line_number, line in enumerate(contents.splitlines(), start=1):
            if any(term in line.casefold() for term in owner_terms):
                findings.append(f"{relative}:{line_number}: Owner-specific term")
            for label, pattern in PATTERNS:
                if any(match.group(0) not in allowlist for match in pattern.finditer(line)):
                    findings.append(f"{relative}:{line_number}: {label}")
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("root", type=Path, help="repository root to scan")
    args = parser.parse_args()
    root = args.root.resolve()
    try:
        terms = load_owner_terms(root)
        findings = scan(root, terms)
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
