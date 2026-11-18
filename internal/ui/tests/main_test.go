package ui_test

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/testenv"
)

// TestMain pins the environment (internal/testenv) so every rendered frame is the same on
// every machine: a developer's NO_COLOR, TERM=linux or LC_ALL=C would otherwise make goldens
// and glyph assertions disagree, and config.toml resolves under each test's temp home.
// Tests that exercise one of those modes (TestAsciiFallsBackWhenTheLocaleIsNotUTF8) set
// the variable themselves.
func TestMain(m *testing.M) { testenv.Main(m) }
