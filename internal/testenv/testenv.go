// Package testenv makes a test binary's environment the same on every machine. It is for
// tests only: each test package that renders a frame, resolves an Editor or launches
// rolecall calls Main from its TestMain, so a developer's NO_COLOR, COLORFGBG, TERM,
// locale or $EDITOR never changes a test's outcome. A test that exercises one of these
// modes sets the variable itself with t.Setenv.
package testenv

import (
	"os"
	"testing"
)

// Unset lists the variables Hermetic clears. Each one changes what rolecall draws or which
// program and config it picks:
//   - NO_COLOR, COLORFGBG, ROLECALL_THEME, ROLECALL_PLAIN, ROLECALL_ASCII and TERM
//     choose colour, theme, glyphs and layout (internal/ui/theme);
//   - CLICOLOR, CLICOLOR_FORCE and COLORTERM change the colour profile lipgloss detects;
//   - LC_ALL and LC_CTYPE override LANG, which Hermetic pins to UTF-8 (a non-UTF-8 locale
//     turns ASCII on);
//   - VISUAL and EDITOR choose the Editor, ROLECALL_PLUGIN the Plugin, and
//     XDG_CONFIG_HOME where config.toml lives (it must resolve under each test's HOME).
var Unset = []string{
	"NO_COLOR", "COLORFGBG", "ROLECALL_THEME", "ROLECALL_PLAIN", "ROLECALL_ASCII", "TERM",
	"CLICOLOR", "CLICOLOR_FORCE", "COLORTERM",
	"LC_ALL", "LC_CTYPE",
	"VISUAL", "EDITOR", "ROLECALL_PLUGIN", "XDG_CONFIG_HOME",
}

// Locale is the LANG Hermetic pins: UTF-8, so every frame keeps its glyphs.
const Locale = "en_US.UTF-8"

// Hermetic clears Unset and pins LANG in this process, and so in every child started with
// os.Environ(). It panics if the environment cannot be changed, since every test after it
// would then run against the developer's shell.
func Hermetic() {
	for _, key := range Unset {
		if err := os.Unsetenv(key); err != nil {
			panic(err)
		}
	}
	if err := os.Setenv("LANG", Locale); err != nil {
		panic(err)
	}
}

// Main is a TestMain body: Hermetic, then the package's tests.
func Main(m *testing.M) {
	Hermetic()
	os.Exit(m.Run())
}
