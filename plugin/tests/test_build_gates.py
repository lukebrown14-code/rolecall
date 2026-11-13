"""build.py uses the document gates the Settings screen writes to workspace.json."""

import importlib.util
import json
from pathlib import Path

import pytest

SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "build.py"


def load_build(data):
    spec = importlib.util.spec_from_file_location("build_gates", SCRIPT)
    build = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(build)
    build.DATA = str(data)
    build.load_config()
    return build


def test_changed_word_cap_is_used_by_the_letter_gate(tmp_path):
    letter = tmp_path / "cover-letter.md"
    letter.write_text("# Alex Quinn\n\n" + " ".join(["word"] * 200) + "\n")
    build = load_build(tmp_path)
    build.check_letter_length(letter.read_text())  # 200 words is within the default cap of 350

    (tmp_path / "workspace.json").write_text(json.dumps({"letterWordCap": 180}, indent=2))
    build = load_build(tmp_path)
    assert build.LETTER_MAX_WORDS == 180
    with pytest.raises(SystemExit):
        build.check_letter_length(letter.read_text())
