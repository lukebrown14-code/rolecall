#!/usr/bin/env python3
"""Check explicit Skill-tool dependencies in repository skills."""
from pathlib import Path
import re
import sys

root = Path(__file__).resolve().parents[1] / '.agents/skills'
errors = []
edges = set()
for source in sorted(root.glob('*/SKILL.md')):
    for line in source.read_text().splitlines():
        if 'Skill tool' not in line:
            continue
        for name in re.findall(r'[`\"]([a-z][a-z0-9-]*)[`\"]', line):
            edges.add((source.parent.name, name))
            if not (root / name / 'SKILL.md').is_file():
                errors.append(f'{source.parent.name}: missing skill {name}')
for error in errors:
    print(error, file=sys.stderr)
print(f'Checked {len(edges)} skill dependencies; {len(errors)} missing.')
sys.exit(bool(errors))
