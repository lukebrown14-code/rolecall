package ui_test

import (
	"os"
	"testing"
)

// TestMain pins a UTF-8 locale so every rendered frame is the same on every machine. The
// TUI falls back to ASCII on a non-UTF-8 locale, so a developer running with LC_ALL=C
// would otherwise see goldens and glyph assertions disagree. Tests that exercise the
// fallback (TestAsciiFallsBackWhenTheLocaleIsNotUTF8) set LANG themselves.
func TestMain(m *testing.M) {
	for _, key := range []string{"LC_ALL", "LC_CTYPE"} {
		os.Unsetenv(key)
	}
	os.Setenv("LANG", "en_US.UTF-8")
	os.Exit(m.Run())
}
