package testenv_test

import (
	"os"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/testenv"
	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// A developer shell that sets every display, locale and Editor variable still gets the
// default frame and the vi fallback once Hermetic has run.
func TestHermeticRestoresTheDefaults(t *testing.T) {
	for key, value := range map[string]string{
		"NO_COLOR": "1", "COLORFGBG": "15;0", "ROLECALL_THEME": "light", "ROLECALL_PLAIN": "1",
		"ROLECALL_ASCII": "1", "TERM": "linux", "LANG": "C", "LC_ALL": "C", "LC_CTYPE": "C",
		"CLICOLOR_FORCE": "1", "COLORTERM": "truecolor", "VISUAL": "code", "EDITOR": "nano",
		"ROLECALL_PLUGIN": "/nonexistent", "XDG_CONFIG_HOME": t.TempDir(),
	} {
		t.Setenv(key, value)
	}
	testenv.Hermetic()

	for _, key := range testenv.Unset {
		if value, ok := os.LookupEnv(key); ok {
			t.Errorf("%s = %q after Hermetic, want unset", key, value)
		}
	}
	if got := os.Getenv("LANG"); got != testenv.Locale {
		t.Errorf("LANG = %q, want %q", got, testenv.Locale)
	}
	if theme.NoColor() || theme.ASCIIMode() || theme.PlainMode() {
		t.Errorf("NoColor, ASCIIMode, PlainMode = %v, %v, %v; want all false",
			theme.NoColor(), theme.ASCIIMode(), theme.PlainMode())
	}
	if mode, auto := theme.FromEnv(); mode != theme.Dark || !auto {
		t.Errorf("FromEnv = %v, %v; want Dark, auto", mode, auto)
	}
	if editor := config.ResolveEditor(""); editor != "vi" {
		t.Errorf("ResolveEditor(\"\") = %q, want the vi fallback", editor)
	}
}
