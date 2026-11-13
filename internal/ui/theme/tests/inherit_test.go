package theme_test

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// A truecolor foreground becomes the nearest ANSI-16 foreground and any background becomes
// reverse video, so nothing is painted and bold survives.
func TestInheritANSIReducesToANSI16(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"\x1b[1;38;2;255;80;80m", "\x1b[1;91m"},
		{"\x1b[38;5;34m", "\x1b[32m"},
		{"\x1b[48;2;47;85;96m", "\x1b[7m"},
		{"\x1b[38;2;10;10;10;48;2;1;2;3m", "\x1b[30;7m"},
		{"\x1b[44m", "\x1b[7m"},
		{"\x1b[0m", "\x1b[0m"},
		{"\x1b[m", "\x1b[m"},
	} {
		if got := theme.InheritANSI(c.in); got != c.want {
			t.Errorf("InheritANSI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
