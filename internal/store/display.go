package store

import "strings"

// DisplayText is s made safe to draw in a terminal: tab, newline, carriage return and the
// Unicode line and paragraph separators become a space, and every other control character
// (C0, DEL and C1, ESC among them) is dropped. meta.yml strings often come from scraped
// job ads, and a quoted scalar can decode "\e" or "\u009b", so nothing read from one
// reaches the screen raw.
func DisplayText(s string) string {
	if !strings.ContainsFunc(s, isControl) {
		return s
	}
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029:
			out.WriteByte(' ')
		case isControl(r):
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// isControl is a C0 or C1 control character, DEL, or a Unicode line break.
func isControl(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == 0x2028 || r == 0x2029
}
