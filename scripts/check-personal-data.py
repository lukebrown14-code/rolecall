#!/usr/bin/env python3
"""Scan tracked repository files for common personal-data shapes."""

from __future__ import annotations

import argparse
import json
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
            r"(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+",
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


def scan(root: Path) -> list[str]:
    allowlist = load_allowlist(root)
    findings: list[str] = []
    for relative in tracked_files(root):
        path = root / relative
        try:
            contents = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue
        for line_number, line in enumerate(contents.splitlines(), start=1):
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
        findings = scan(root)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        return 2
    if findings:
        print("\n".join(findings))
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
