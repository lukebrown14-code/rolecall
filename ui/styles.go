package ui

import (
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
)

var headerStyle = lipgloss.NewStyle().Bold(true)
var selectedStyle = lipgloss.NewStyle().Reverse(true)
var plainStyle = lipgloss.NewStyle()

// markdownStyle renders Markdown with terminal default colours only: emphasis is
// bold, italic and strikethrough rather than colour.
func markdownStyle() ansi.StyleConfig {
	on := true
	c := styles.ASCIIStyleConfig
	c.Strong = ansi.StylePrimitive{Bold: &on}
	c.Emph = ansi.StylePrimitive{Italic: &on}
	c.Strikethrough = ansi.StylePrimitive{CrossedOut: &on}
	c.Heading.Bold = &on
	return c
}
