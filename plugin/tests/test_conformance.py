"""meta.yml rules shared with the Go store. Both sides assert ``fixtures/conformance.json``
(``internal/store/tests/conformance_test.go``), so a rule changed on one side fails here
or there until the other agrees.
"""

import json
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "plugin" / "scripts"))

import adfacts  # noqa: E402
import meta  # noqa: E402

CONFORMANCE = json.loads((ROOT / "fixtures" / "conformance.json").read_text())


@pytest.mark.parametrize("case", CONFORMANCE["lines"], ids=lambda c: c["line"])
def test_meta_lines_match_the_shared_conformance_fixture(tmp_path, case):
    folder = tmp_path / "jobs" / "SWE" / "lead"
    folder.mkdir(parents=True)
    (folder / "meta.yml").write_text(CONFORMANCE["base"] + case["line"] + "\n")
    app = meta.rescan(folder)
    assert (app.malformed is None) == case["valid"], app.malformed


def test_ad_fact_tables_match_the_shared_conformance_fixture():
    for key, values in CONFORMANCE["enums"].items():
        assert meta.FACTS[key][2] == set(values), key
    enums = {k for k, (_, _, kind) in meta.FACTS.items() if isinstance(kind, set)}
    assert enums == set(CONFORMANCE["enums"])
    assert adfacts.ANNUAL == CONFORMANCE["payPerYear"]
    assert adfacts.FX_AUD == CONFORMANCE["payFX"]
