package ui_test

import (
	"regexp"

	"github.com/charmbracelet/x/ansi"
)

// selChip matches the dark theme's selection chip, the active tab, strip cell or Track
// option: bold bright text on the selection fill, one blank cell each side.
var selChip = regexp.MustCompile("\x1b\\[1;38;2;230;237;243;48;2;47;85;96m ([^\x1b]*) \x1b\\[m")

// chipped is the view as plain text with each selection chip drawn as "[text]", the
// same cells, so a test can read which tab, cell or option is active.
func chipped(view string) string {
	return ansi.Strip(selChip.ReplaceAllString(view, "[$1]"))
}
